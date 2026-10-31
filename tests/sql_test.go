package test_sql

import (
	"card_machine/internal/model"
	"card_machine/internal/repository"
	"database/sql"
	"reflect"
	"testing"
)

// Cada teste usa uma conexão SQLite própria. Nenhum ID depende de outro teste.
func newTestRepository(t *testing.T) (*sql.DB, *repository.Repository) {
	t.Helper()
	connection, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	connection.SetMaxOpenConns(1)
	t.Cleanup(func() { connection.Close() })
	repo := repository.NewRepository(connection)
	if _, err = repo.CreateTableOfCardMachine(); err != nil {
		t.Fatal(err)
	}
	if _, err = repo.CreateTableSale(); err != nil {
		t.Fatal(err)
	}
	return connection, repo
}

func testMerchant() model.CardMachine {
	return model.CardMachine{UUID: "merchant-test", SerialNumber: "serial-test", BankInstituition: "ITAU", PersonNameStorage: "Lojista Teste", StorageName: "Loja Teste", PersonCpf: "12345678901", City: "Sao Paulo", State: "SP"}
}

func insertTestMerchant(t *testing.T, repo *repository.Repository) {
	t.Helper()
	merchant := testMerchant()
	if _, err := repo.InsertValueInCardMachine(&merchant); err != nil {
		t.Fatal(err)
	}
}

func insertTestSale(t *testing.T, repo *repository.Repository, kind model.PaymentType, real, cents int64) int {
	t.Helper()
	_, id, err := repo.InsertValueInTableSale(testTime, nil, model.StatusPending, cents, real, kind)
	if err != nil || id <= 0 {
		t.Fatalf("falha ao preparar venda: id=%d, erro=%v", id, err)
	}
	return id
}

func assertSaleState(t *testing.T, connection *sql.DB, id int, want model.SaleStatus, approvedTime bool) {
	t.Helper()
	var status model.SaleStatus
	var timestamp sql.NullString
	if err := connection.QueryRow("SELECT status,time_aprove FROM sales WHERE id=?", id).Scan(&status, &timestamp); err != nil {
		t.Fatal(err)
	}
	if status != want || timestamp.Valid != approvedTime || approvedTime && timestamp.String == "" {
		t.Fatalf("estado=%s, horário preenchido=%t; esperado=%s, horário=%t", status, timestamp.Valid, want, approvedTime)
	}
}

func TestConnection(t *testing.T) {
	t.Chdir(t.TempDir())
	connection, err, version := repository.DbConnection()
	if err != nil {
		t.Fatal(err)
	}
	defer connection.Close()
	if version == "" || connection.Ping() != nil {
		t.Fatal("conexão SQLite sem versão ou indisponível")
	}
	t.Logf("Teste aprovado, valor: SQLite %s", version)
}

func TestCreateTables(t *testing.T) {
	connection, repo := newTestRepository(t)
	// Repetir a inicialização não deve apagar o cadastro nem a venda existente.
	insertTestMerchant(t, repo)
	id := insertTestSale(t, repo, model.PaymentPix, 25, 33)
	if _, err := repo.CreateTableOfCardMachine(); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.CreateTableSale(); err != nil {
		t.Fatal(err)
	}
	var count int
	if err := connection.QueryRow("SELECT count(*) FROM bank_machine").Scan(&count); err != nil || count != 1 {
		t.Fatal("inicialização repetida alterou o cadastro")
	}
	assertSaleState(t, connection, id, model.StatusPending, false)
	t.Log("Teste aprovado, valor: tabelas criadas sem apagar dados")
}

func TestMerchantRepository(t *testing.T) {
	_, repo := newTestRepository(t)
	exists, err := repo.CpfExists(testMerchant().PersonCpf)
	if err != nil || exists {
		t.Fatal("CPF novo não deve constar como cadastrado")
	}
	insertTestMerchant(t, repo)
	found, err := repo.GetAllValuesInCardMachine()
	if err != nil || found == nil || !reflect.DeepEqual(*found, testMerchant()) {
		t.Fatal("cadastro recuperado diverge dos dados inseridos")
	}
	exists, err = repo.CpfExists(testMerchant().PersonCpf)
	if err != nil || !exists {
		t.Fatal("CPF inserido deveria existir")
	}
	t.Log("Teste aprovado, valor: cadastro recuperado e CPF identificado")
}

func TestSaleRepository(t *testing.T) {
	connection, repo := newTestRepository(t)
	status, id, err := repo.VerifyTheStatus()
	if err != nil || id != 0 || status != "" {
		t.Fatal("banco vazio deve retornar ausência de venda")
	}
	for _, kind := range []model.PaymentType{model.PaymentPix, model.PaymentCard} {
		t.Run(string(kind), func(t *testing.T) {
			id := insertTestSale(t, repo, kind, 25, 33)
			real, cents, err := repo.RecoverValues(id)
			if err != nil || real != 25 || cents != 33 {
				t.Fatal("valor recuperado deve preservar reais e centavos")
			}
			var storedKind model.PaymentType
			if err := connection.QueryRow("SELECT type_pay FROM sales WHERE id=?", id).Scan(&storedKind); err != nil || storedKind != kind {
				t.Fatal("tipo de pagamento não persistido")
			}
			valid, status, err := repo.ValidatePendingSale(id)
			if err != nil || !valid || status != model.StatusPending {
				t.Fatal("venda recém-criada deve estar pendente")
			}
			assertSaleState(t, connection, id, model.StatusPending, false)
			if _, err := repo.PutValueStatus(id, model.StatusApproved); err != nil {
				t.Fatal(err)
			}
			if _, err := repo.InserTimeAprove(testTime, id); err != nil {
				t.Fatal(err)
			}
			timestamp, err := repo.GetTimeArove(id)
			if err != nil || timestamp != testTime {
				t.Fatal("horário de aprovação não corresponde ao valor salvo")
			}
			valid, status, err = repo.ValidatePendingSale(id)
			if err != nil || valid || status != model.StatusApproved {
				t.Fatal("venda aprovada não pode continuar pendente")
			}
			latest, latestID, err := repo.VerifyTheStatus()
			if err != nil || latest != model.StatusApproved || latestID != id {
				t.Fatal("a consulta deve recuperar a venda mais recente")
			}
			assertSaleState(t, connection, id, model.StatusApproved, true)
			t.Logf("Teste aprovado, valor: id=%d, valor=25.33, estado=%s", id, status)
		})
	}
	_, _, err = repo.ValidatePendingSale(99999)
	if err == nil {
		t.Fatal("ID inexistente deve retornar erro")
	}
	if _, _, err := repo.RecoverValues(99999); err == nil {
		t.Fatal("valor de venda inexistente deve retornar erro")
	}
}

// Estes cenários impedem que falhas do SQL sejam confundidas com sucesso.
func TestRepositoryErrors(t *testing.T) {
	connection, repo := newTestRepository(t)
	if _, err := repo.GetTimeArove(99999); err == nil {
		t.Error("leitura de horário de venda inexistente deve retornar erro")
	}
	if err := connection.Close(); err != nil {
		t.Fatal(err)
	}
	checks := []struct {
		name string
		run  func() error
	}{
		{"gravar horário", func() error { _, err := repo.InserTimeAprove(testTime, 1); return err }},
		{"ler horário", func() error { _, err := repo.GetTimeArove(1); return err }},
		{"consultar CPF", func() error { _, err := repo.CpfExists(testMerchant().PersonCpf); return err }},
		{"consultar cadastro", func() error { _, err := repo.GetAllValuesInCardMachine(); return err }},
		{"consultar última venda", func() error { _, _, err := repo.VerifyTheStatus(); return err }},
		{"mudar status", func() error { _, err := repo.PutValueStatus(1, model.StatusApproved); return err }},
	}
	for _, check := range checks {
		t.Run(check.name, func(t *testing.T) {
			if err := check.run(); err == nil {
				t.Fatal("SQL indisponível deve retornar erro")
			}
			t.Log("Teste aprovado, valor: erro SQL propagado")
		})
	}
}
