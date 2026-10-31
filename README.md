# Card Machine · H-BANK

**PT:** Simulador de maquininha de pagamentos em Go, com vendas por **Pix, débito e crédito**, persistência local, processamento concorrente, eventos Kafka e comprovantes. O projeto faz parte da construção do ecossistema **H-BANK** e trabalha em conjunto com a [PSP Go](https://github.com/hugoferreiraTI/psp_go).

**EN:** A payment-terminal simulator written in Go. It supports **Pix, debit and credit** workflows, local persistence, concurrent payment handling, Kafka events and receipts. It is being developed as part of the **H-BANK** ecosystem and integrates with [PSP Go](https://github.com/hugoferreiraTI/psp_go).

**Em desenvolvimento / Work in progress.** O Pix é uma simulação local. **Os pagamentos com cartão só funcionam em conjunto com a PSP:** a Card Machine prepara a requisição; a PSP valida o cartão e decide a autorização. / Pix is simulated locally. **Card payments require the PSP:** the terminal prepares the request, while the PSP validates the card and makes the authorization decision.

[Português](#português) · [English](#english) · [Fontes e referências / Sources and references](#sources-and-references)

## Português

### O que o projeto faz

A Card Machine representa o lado do estabelecimento em um fluxo de pagamento. Ela recebe os dados do lojista, cria uma venda, aguarda uma tentativa de pagamento e registra o resultado no SQLite.

| Forma de pagamento | Fluxo atual | Dependência da PSP |
|---|---|---|
| Pix | Gera BR Code e QR Code, recebe uma tentativa e compara o valor com a venda ativa. | Não; aprovação simulada localmente. |
| Débito | Monta o payload EMV/TLV, protege o PIN e solicita autorização HTTP. | Sim; a PSP consulta o cartão, autentica o PIN e verifica/debita o saldo. |
| Crédito | Usa a mesma preparação do débito, enviando `PaymentType: "CREDIT"`. | Sim; a PSP autentica e soma a compra ao valor utilizado, respeitando o limite. |

O projeto não captura dados de um chip ou leitor NFC: os dados entram por JSON. Ele estuda o processamento do backend, sem integração com infraestrutura Pix real, adquirentes, bandeiras ou bancos reais. Os identificadores EMV e os algoritmos criptográficos são usados em um protocolo simulado entre estes dois projetos; isso não representa uma implementação EMV completa ou certificada.

### Arquitetura e responsabilidades

```text
Postman / operador
       │
       ├── POST /card_machine → cadastro do estabelecimento
       │
       └── POST /Sale → venda PENDING no SQLite
                       │
                       ├── PIX
                       │    → BR Code → qr.png
                       │    → aguarda POST /Pay/pix
                       │    → compara valor e finaliza localmente
                       │
                       └── CARD: DEBIT ou CREDIT
                            → aguarda POST /Pay/card
                            → número imprevisível + CMAC + TLV + AES-GCM
                            → POST PSP /authorize/card
                            ← AuthorizationResponse na mesma conexão HTTP
                            → atualiza SQLite e retorna status/log

Estados e eventos → Kafka
Aprovação → comprovante local
```

A PSP responde à própria requisição HTTP feita pela maquininha. Não é necessário outro client da PSP para devolver o resultado, nem um callback Kafka para concluir a autorização.

| Camada | Responsabilidade |
|---|---|
| `cmd/main.go` | Conexões, dependências, client da PSP, Kafka e rotas. |
| `internal/handler` | JSON de entrada, validação de binding e respostas HTTP. |
| `internal/useCases` | Cadastro, criação de vendas, espera por pagamento, Pix, TLV, criptografia e comprovantes. |
| `internal/client` | Envia `CardPayment` à PSP e lê `AuthorizationResponse`. |
| `internal/repository` | Acesso ao SQLite e atualização do estado local. |
| `internal/model` | Contratos de cadastro, venda, pagamento, autorização e eventos. |
| `internal/brcode_simplify` | Construção, leitura e validação do BR Code/TLV textual e CRC. |
| `internal/kafka_config` | Produção e consumo de eventos da maquininha. |
| `documents` | Swagger UI e especificação embarcados. |
| `tests` | Testes HTTP, SQL e de usecases. |

O SQLite da maquininha armazena dados do estabelecimento e das vendas. O cadastro bancário dos cartões, hashes de PIN, saldos e limites pertence ao PostgreSQL da PSP.

### Tecnologias usadas

| Tecnologia | Uso no código |
|---|---|
| Go | API, regras, goroutines, channels, `select` e timers. |
| Gin | Rotas HTTP e binding dos JSONs. |
| SQLite + `database/sql` | Persistência local em `card_machine.db`; a conexão usa o driver `mattn/go-sqlite3`. |
| Apache Kafka + Confluent Go Client | Eventos de estado e consumidor local. |
| `go-qrcode` | Geração de `qr.png` a partir do BR Code. |
| BR Code / TLV textual | Payload do QR Code Pix simulado. |
| Tags EMV / BCD | Organização e codificação dos dados de cartão enviados à PSP. |
| `AES_CMAC_Go/cmac` | Criptograma AES-CMAC com chave de 128 bits. |
| `crypto/aes`, `crypto/cipher`, `crypto/rand` | AES-GCM, nonce aleatório e número imprevisível. |
| `encoding/hex`, `encoding/json` | Hexadecimal, JSON e Base64 automático de `[]byte`. |
| `godotenv` | Leitura das chaves compartilhadas no `.env`. |
| UUID | Identificador e número de série da maquininha. |
| Swagger UI | Consulta da documentação HTTP embarcada. |
| Docker | Execução local de dependências, como Kafka e PostgreSQL da PSP. |

Bcrypt é utilizado **na PSP**, não pela Card Machine. Veja a distinção entre codificação, criptografia, MAC e hash abaixo.

### Preparar e iniciar

1. Use uma versão de Go compatível com `go.mod` (atualmente `1.26.8`) e um compilador C/CGO para os drivers SQLite/Kafka usados pelo projeto.
2. Inicie Kafka em `localhost:9092`. O tópico da maquininha é `sale`.
3. Para testar cartões, prepare e inicie a [PSP](https://github.com/hugoferreiraTI/psp_go), incluindo PostgreSQL, migrações e chaves. **Cadastre o cartão na PSP antes da compra** e disponibilize saldo de débito ou limite de crédito.
4. Crie um arquivo `.env` na raiz da Card Machine com as mesmas chaves configuradas para esse usuário na PSP:

```dotenv
AES_CMAC_KEY=<chave hexadecimal de 128 bits compartilhada com a PSP>
AES_GCM=<chave AES hexadecimal compartilhada com a PSP>
```

As rotinas de cartão carregam `.env` durante a operação; mantenha o arquivo presente. As chaves não são fornecidas pelo JSON da compra e não devem ser versionadas. O uso de chaves compartilhadas aqui representa o contexto configurado do usuário deste MVP, não uma chave universal para todos os usuários.

```bash
git clone https://github.com/hugoferreiraTI/go_card_machine
cd go_card_machine
go mod download
go run ./cmd
```

Execute a aplicação na raiz do projeto: o caminho do SQLite, o `.env`, o QR Code e os comprovantes são relativos ao diretório de execução. As tabelas `bank_machine` e `sales` são criadas por `CREATE TABLE IF NOT EXISTS` na inicialização.

| Serviço | Endereço configurado no código |
|---|---|
| Card Machine | `http://localhost:8081` |
| PSP | `http://localhost:8080` |
| Kafka | `localhost:9092` |
| Swagger UI da maquininha | `http://localhost:8081/swagger/index.html` |

A porta HTTP, o endereço do broker e a URL da PSP estão definidos no código atual, não em variáveis de ambiente da Card Machine. O timeout do client HTTP da PSP é de 10 segundos. A API da PSP possui seu próprio README, configuração e ciclo de persistência.

Para um container Kafka local chamado `kafka`, o tópico pode ser preparado assim:

```bash
docker exec kafka /opt/kafka/bin/kafka-topics.sh \
  --bootstrap-server localhost:9092 --create --if-not-exists \
  --topic sale --partitions 1 --replication-factor 1
```

### Ordem dos requests e exemplos

Use `Content-Type: application/json`. As rotas são sensíveis ao caminho: `/Sale`, `/Pay/pix` e `/Pay/card` são os nomes atuais; a antiga rota `/Pay` não é usada.

#### 1. Inicializar a maquininha

`POST http://localhost:8081/card_machine`

```json
{
  "bank": "ITAU",
  "person_name_storage": "Hugo Teste",
  "storage_name": "Loja de Testes",
  "person_cpf": "12345678901",
  "city": "Sao Paulo",
  "state": "SP"
}
```

`bank` também é obrigatório. UUID e número de série são gerados pelo código. Um CPF já cadastrado resulta em HTTP 409. O contrato usa CPF com 11 caracteres; não existe um campo CNPJ dedicado neste cadastro.

#### 2. Criar a venda

`POST http://localhost:8081/Sale`

| Pagamento | `Type` | `payment_type` |
|---|---|---|
| Pix | `PIX` | Omitir; o campo é específico de cartão. |
| Débito | `CARD` | `DEBIT` |
| Crédito | `CARD` | `CREDIT` |

Pix:

```json
{"Amount":"1.33","Type":"PIX"}
```

Débito:

```json
{"Amount":"1.33","Type":"CARD","payment_type":"DEBIT"}
```

Crédito:

```json
{"Amount":"1.33","Type":"CARD","payment_type":"CREDIT"}
```

A venda é persistida como `PENDING`. Para Pix, o resultado da criação indica a geração do QR Code; para cartão, indica a espera pelo pagamento. Isso **não significa aprovação**. O JSON atual de criação usa a chave `sucess`, conforme o handler.

Existe uma venda ativa por vez neste MVP. Se a última venda estiver `PENDING`, a próxima criação é rejeitada. Depois de criar cada venda, envie o pagamento correspondente dentro de **30 segundos**. Os canais relacionam a tentativa à venda em espera; não há `sale_id` no request manual de `/Pay/pix` ou `/Pay/card`.

#### 3A. Pagar com Pix

`POST http://localhost:8081/Pay/pix`

```json
{
  "amount": "1.33",
  "cpf": "12345678901",
  "name": "Cliente Teste",
  "bank": "ITAU"
}
```

A maquininha reconstrói o valor da venda como texto com duas casas e o compara com `amount`. Envie `"1.33"`, não um número JSON ou uma representação decimal diferente.

- Valor correspondente: grava `APPROVED`, registra o horário de aprovação, publica evento e gera `comprovante.txt`.
- Valor divergente: retorna `REPROVED`, publica o evento da tentativa e mantém a venda `PENDING` para outra tentativa dentro da janela original.
- Sem pagamento aprovado até o timeout: grava `EXPIRED` e publica evento de expiração.

O QR Code é gerado em `qr.png`, a partir de um payload BR Code que inclui uma URL simulada baseada no ID da venda. A URL usada pelo código, `https://pix.com.br/caminho/{id}`, é ilustrativa: não representa uma cobrança Pix real. A aprovação é disparada pelo endpoint de simulação, sem liquidação no SPI ou confirmação de uma instituição financeira.

#### 3B. Pagar com débito ou crédito

`POST http://localhost:8081/Pay/card`

Exemplo de **débito**, desde que cadastrado na PSP:

```json
{
  "PAN": "5201000000000006",
  "dateExpirity": "31/12/31",
  "Country": "076",
  "Pin": {
    "PIN": "2468",
    "CVM": "ONLINE_PIN"
  }
}
```

Exemplo de **crédito**, desde que cadastrado na PSP:

```json
{
  "PAN": "5201500000000005",
  "dateExpirity": "31/12/31",
  "Country": "076",
  "Pin": {
    "PIN": "2468",
    "CVM": "ONLINE_PIN"
  }
}
```

Os dados são sintéticos; os exemplos não criam cartões automaticamente. O tipo vem da venda anterior, não de uma tentativa de deduzir débito/crédito pelo PAN. Um cartão de débito enviado em uma venda `CREDIT` será consultado e validado pela PSP como crédito e pode ser recusado por não existir naquele cadastro.

A entrada da maquininha usa `dateExpirity` em `DD/MM/YY`. A PSP recebe a validade codificada em `YYMMDD`; `31/12/31` vira `311231`. Atualmente o builder da Card Machine limita o ano de dois dígitos ao intervalo `21–31`; a PSP valida a data existente, o cadastro e a expiração.

A reprovação de cartão **encerra a espera dessa venda** no fluxo atual, ao contrário da divergência de valor no Pix. Crie uma nova venda antes de tentar pagar novamente. Uma aprovação gera `comprovante_card.txt` e atualiza status/horário locais.

### O que é enviado à PSP

`PaymentSaleCard` combina os dados da venda e do cartão em um `CardPayment`:

| Campo | Origem |
|---|---|
| `SaleId` | ID gerado no SQLite. |
| `TimeNow` | Horário RFC3339 preparado para a venda de cartão. |
| `Amount` | Valor decimal reconstruído, como `"1.33"`. |
| `Status` | `PENDING` para solicitar a autorização. |
| `PaymentType` | `DEBIT` ou `CREDIT`, vindo da venda. |
| `Emv` | Tags e valores TLV, criptograma e PIN protegido. |

O client faz `json.Marshal`, envia `POST /authorize/card` à PSP e desserializa o corpo da resposta. A rede/bandeira, o emissor, o PIN armazenado, o saldo e o limite são validados **pela PSP**. A Card Machine não substitui essa decisão por uma aprovação local de cartão.

### EMV/TLV e codificações

TLV significa **Tag, Length, Value**: identificador do dado, comprimento e conteúdo. O projeto usa tags associadas ao EMV em uma struct Go. O transporte entre os serviços é JSON com campos TLV; ele não é um fluxo binário BER-TLV completo de um leitor de chip. As especificações EMV descrevem um conjunto mais amplo de requisitos do que esta simulação. [Referência EMVCo](https://www.emvco.com/emv-technologies/emv-contact-chip/).

```go
type TLV struct {
    Tag    []byte
    Length int
    Value  []byte
}
```

| Dado | Tag | Representação usada |
|---|---|---|
| PAN | `5A` | BCD compactado: dois dígitos por byte; PAN ímpar recebe um nibble `F` de preenchimento no fim. |
| Validade | `5F24` | Três bytes BCD em `YYMMDD`. |
| País da transação | `9F1A` | Dois bytes BCD; `076` vira `0076` para o Brasil. |
| Número imprevisível | `9F37` | Quatro bytes gerados por `crypto/rand`, preservados como bytes. |
| Criptograma | `9F26` | Dezesseis bytes de AES-CMAC completo neste protocolo de MVP. |
| PIN | `Emv.Pin` | Payload AES-GCM e informação de CVM; não é empacotado como as demais tags TLV. |

Hexadecimal é usado para construir/interpretar bytes. Quando `json.Marshal` encontra `[]byte`, ele os serializa como **Base64**, conforme a [documentação do Go](https://pkg.go.dev/encoding/json#Marshal). `Length` continua contando bytes do valor, não o tamanho da string Base64.

Exemplo apenas da tag PAN:

```text
PAN:          5201000000000006
Bytes (hex):  52 01 00 00 00 00 00 06
Length:       8
```

```json
{"Tag":"Wg==","Length":8,"Value":"UgEAAAAAAAY="}
```

Esse fragmento não é um pagamento completo. O PAN continua recuperável: BCD, hexadecimal e Base64 são **codificações, não criptografia**.

Há dois usos diferentes de TLV no repositório. O módulo `brcode_simplify` monta TLV **textual** para QR Code, com tag e comprimento de dois caracteres; o fluxo de cartão organiza tags e valores **binários** nas structs acima. O CRC-16 do BR Code detecta alterações no payload, mas não é uma autenticação criptográfica com chave.

### Criptograma, criptografia e hash

| Mecanismo | Papel no fluxo |
|---|---|
| AES-CMAC | Código de autenticação da mensagem com chave compartilhada; verifica integridade/autenticidade dos componentes definidos pelo projeto. |
| AES-GCM | Criptografia autenticada para transportar o PIN da Card Machine à PSP. |
| Bcrypt | Hash com salt para armazenar e comparar o PIN na PSP. A maquininha não recebe esse hash. |
| BCD, hexadecimal e Base64 | Codificam os dados; não os mantêm secretos. |
| CRC-16 | Checksum do BR Code; não substitui MAC, criptografia ou hash de senha. |

`Crypto` usa a biblioteca do autor [AES_CMAC_Go](https://github.com/hugoferreiraTI/AES_CMAC_GO), com chave AES de 128 bits. A mensagem é montada exatamente como na PSP:

```go
fmt.Sprintf("%s|%s|%s|%s",
    cryptogram.StatusCard,
    cryptogram.AmountCrypt,
    cryptogram.TimeNowCrypt,
    cryptogram.NumberUnpredictableCrypt,
)
```

O último componente é o `[4]byte` original. Não o substitua por uma string hexadecimal ou Base64. O valor é a string decimal original, não os centavos do banco. Alterar valor, horário, status ou número exige gerar outro CMAC. Um MAC com chave não é um hash simples nem uma criptografia reversível do payload. [NIST SP 800-38B](https://csrc.nist.gov/pubs/sp/800/38/b/upd1/final).

`aesGcm` gera um nonce aleatório e usa `gcm.Seal` com AAD `nil`. O payload é `nonce || ciphertext || tag`, serializado em hexadecimal. A PSP autentica e descriptografa esse conteúdo com a chave correspondente. [NIST SP 800-38D](https://csrc.nist.gov/pubs/sp/800/38/d/final), [API Go](https://pkg.go.dev/crypto/cipher#NewGCM).

Na PSP, o PIN recuperado em memória é comparado ao `pin_hash` usando `bcrypt.CompareHashAndPassword`. Como bcrypt usa salt, não se gera um novo hash para comparar as duas strings. Isso pertence ao cadastro/autenticação da PSP; não é uma etapa de persistência de PIN no SQLite da maquininha. [Documentação bcrypt](https://pkg.go.dev/golang.org/x/crypto/bcrypt#CompareHashAndPassword).

O CMAC atual cobre os quatro componentes indicados acima, não todos os campos da compra. O PIN entra em texto no endpoint manual da maquininha e é protegido antes do envio à PSP; isso não equivale a uma captura por hardware seguro. São os limites do protocolo de estudo implementado, sem afirmar cobertura completa de EMV ou proteção de transporte HTTP.

### Respostas, estados e erros

A PSP retorna internamente os sete campos de `AuthorizationResponse`: `status`, `payment_type`, `card_network`, `issuer_bank`, `amount`, `time_now` e `log`. A Card Machine usa esses dados para o resultado e comprovante. **O endpoint público `/Pay/card` expõe atualmente apenas `status` e `log`**, não a struct inteira.

Aprovação, HTTP 200:

```json
{"status":"APPROVED","log":""}
```

Reprovação de cartão, HTTP 400, com motivo seguro devolvido pela PSP:

```json
{"status":"REPROVED","log":"Saldo insuficiente."}
```

Falhas locais ou de comunicação podem não ter um `log` da PSP disponível. Não se deve interpretar um motivo vazio como aprovação. Na PSP, uma decisão de negócio reprovada pode usar HTTP 200; a Card Machine lê o status do JSON e responde conforme seu próprio contrato.

| Situação | Comportamento atual |
|---|---|
| JSON obrigatório ausente/inválido | HTTP 400. |
| CPF já cadastrado na maquininha | HTTP 409. |
| Venda anterior ainda `PENDING` | Nova criação retorna HTTP 400. |
| Sem listener de pagamento ativo | O envio ao canal aguarda até um segundo e retorna HTTP 409. |
| Pix com valor diferente | HTTP 400 com `REPROVED`; a espera continua dentro do prazo. |
| Cartão recusado pela PSP | HTTP 400 com `REPROVED` e `log` disponível; a espera termina. |
| Saldo/limite insuficiente, PIN incorreto ou cartão inexistente | Decisão da PSP, propagada como reprovação. |
| Nenhum pagamento concluído no prazo | Estado local `EXPIRED`; é preciso criar outra venda. |

O timer é usado no `select` de espera; ele não cancela automaticamente um processamento que já entrou na função de pagamento. O client da PSP possui seu próprio timeout HTTP de 10 segundos. Essas durações têm papéis distintos.

### Concorrência, persistência e dinheiro

Os usecases iniciam goroutines que aguardam canais de pagamento ou timers. Cada tentativa contém um canal de resposta, permitindo ao handler receber o resultado da operação. Pix usa `PaymentAttempt`; cartão usa `PaymentCard`, cujo canal transporta `AuthorizationResponse` para preservar o motivo da PSP.

O banco local contém:

| Tabela | Dados principais |
|---|---|
| `bank_machine` | UUID, instituição, responsável, estabelecimento, CPF, cidade, estado e número de série. |
| `sales` | ID, horários `time_sale`/`time_aprove`, partes inteiras `real`/`cents`, status e categoria `type_pay`. |

O schema local guarda a categoria `PIX` ou `CARD`; ele não possui colunas próprias para limite/saldo ou para cada metadado da autorização. O horário `time_aprove` é preenchido no caminho de aprovação. O `time_now` recebido da PSP é o horário enviado pela Card Machine na autorização; não representa, por si só, o instante da aprovação bancária.

Valores chegam como strings decimais. `breakTheAmount` separa reais e centavos e usa inteiros; o valor é reconstruído com `fmt.Sprintf("%d.%02d", real, cents)`. A representação local ainda usa duas colunas, enquanto a PSP usa um único inteiro em centavos. Não há cálculo financeiro em `float` nesses fluxos.

Após aprovação são gerados `comprovante.txt` (Pix) ou `comprovante_card.txt` (cartão); `qr.png` contém o último QR Code gerado. Os arquivos são locais e sobrescritos, não um serviço de armazenamento de comprovantes. Os comprovantes exibem dados do lojista/pagador; não representam anonimização automática desses dados.

### Kafka

A Card Machine produz e consome no tópico **`sale`**, em `localhost:9092`. O consumidor usa o grupo **`card-machine`**, com `auto.offset.reset=earliest`. Um grupo novo pode exibir eventos históricos já existentes no tópico.

O evento atual da maquininha contém:

```json
{"SaleId":42,"value":"1.33","status":"APPROVED"}
```

A chave Kafka é o ID da venda convertido para texto. São publicados eventos nos caminhos de criação (`PENDING`), aprovação, reprovação e expiração. Alguns caminhos de falha/expiração podem não ter valor disponível.

A PSP possui seu próprio tópico `psp_authorizations` e contrato de eventos, documentados no repositório dela. **O evento da Card Machine não inclui atualmente `payment_type`**; não o confunda com o evento da PSP. A autorização e a resposta continuam sendo HTTP.

O consumidor da maquininha observa e imprime os eventos; ele não recebe o resultado financeiro da PSP para alterar a venda. O código atual não implementa outbox, deduplicação de eventos nem garantia de entrega exatamente uma vez. Os eventos são uma camada de observação do ciclo de pagamento.

### Testes e próximos passos

A suíte cobre contratos HTTP, SQLite em memória, criação de venda, criptograma, TLV, BR Code, eventos e propagação do `log`. Os testes de fluxo integram os componentes da Card Machine em um ambiente isolado; não enviam compras à PSP real.

```bash
# Testes do codec BR Code, sem executar compras.
go test ./internal/brcode_simplify

# Regressão do client/handler para manter log em reprovações.
go test ./tests -run '^TestPaymentCardReprovedLog$' -v

# Suíte completa isolada; requer Go/CGO, sem serviços externos.
go test -v ./...
```

`tests/http_test.go` verifica binding, códigos HTTP, contratos da PSP e os fluxos completos de Pix, débito e crédito. `tests/sql_test.go` usa SQLite em memória para conferir persistência e erros SQL. `tests/usecase_test.go` cobre valores monetários, CMAC com vetor conhecido, TLV/BCD/Base64, GCM, eventos e estados. Kafka usa um broker de teste em memória; o transporte da PSP é simulado dentro do processo. `.env`, QR Codes e comprovantes são criados em diretórios temporários, e o timeout de 30 segundos é testado com relógio virtual. Não são modificados o banco local, as chaves do projeto ou os dados da PSP. Esses testes não substituem a integração com a PSP/PostgreSQL reais.

O estado atual já inclui cartões e comunicação HTTP com a PSP. Próximas evoluções podem incluir idempotência, correlação mais forte de tentativas, reconciliação entre o estado local e a PSP, melhoria do tratamento de falhas e persistência monetária em uma única coluna de centavos. Esses itens não são apresentados como garantias implementadas.

## English

### Overview and supported payments

Card Machine simulates the merchant-facing side of a payment terminal. It registers merchant details, creates a sale, waits for a payment attempt and stores the local result in SQLite. The project is part of the developing **H-BANK** ecosystem.

| Payment method | Current behavior | PSP required? |
|---|---|---|
| Pix | Builds a BR Code/QR image and approves a simulated payment when its amount matches the active sale. | No. |
| Debit | Prepares EMV-style data and sends an authorization request. The PSP authenticates the card and checks/debits its balance. | **Yes.** |
| Credit | Uses the same terminal-side preparation, with `PaymentType: "CREDIT"`. The PSP authenticates the card and increases used credit within its limit. | **Yes.** |

**Card payments do not work without the PSP.** The terminal cannot replace the PSP's decision with a local card approval. A valid PAN alone is not enough: the card must be registered in the PSP, with a matching PIN and expiry, and sufficient funds or available credit.

The project accepts JSON rather than reading a physical chip or NFC device. Pix approval is simulated locally, without settlement through the actual Pix infrastructure. EMV tag identifiers and cryptographic algorithms are used in a custom protocol between these projects; this is not a complete or certified EMV implementation.

### Architecture and technology

```text
POST /card_machine → register the merchant
POST /Sale         → persist a PENDING sale
  ├─ PIX  → BR Code → qr.png → POST /Pay/pix → local decision
  └─ CARD → POST /Pay/card → CMAC + TLV + AES-GCM
                            → POST PSP /authorize/card
                            ← JSON decision on the original HTTP connection
                            → update local status and return status/log

Lifecycle events → Kafka
Approved payment → local receipt
```

There is no reverse callback from the PSP to the terminal. Kafka events describe the lifecycle; they do not carry the authorization request or replace the HTTP response.

The code uses Go and Gin for HTTP, goroutines/channels/timers for payment waiting, `database/sql` with `mattn/go-sqlite3` for local storage, Kafka with the Confluent Go client for events, `go-qrcode` for PNG generation, UUIDs for terminal identifiers, and `godotenv` for shared-key configuration. The `AES_CMAC_Go/cmac` library generates CMAC. Standard Go packages provide AES-GCM, secure randomness, hex encoding and JSON. Docker runs local dependencies. Swagger UI and its specification are embedded under `documents`.

Handlers adapt JSON to usecases; usecases coordinate sales and payments; repositories access SQLite; the PSP client handles HTTP authorization. `brcode_simplify` contains the BR Code codec and CRC logic. Local SQLite holds merchants and sales. Card registration, PIN hashes, balances and credit limits belong to the PSP's PostgreSQL database. Bcrypt is used by the PSP, not by the terminal.

### Setup and request order

Use the Go version required by `go.mod` (currently `1.26.8`), with CGO and a C compiler for the SQLite/Kafka dependencies. Start Kafka on `localhost:9092` and create the terminal's `sale` topic if needed. The `kafka-topics.sh` command in the Portuguese setup section shows how to create it in a local container.

For card payments, set up and start [PSP Go](https://github.com/hugoferreiraTI/psp_go) first, including PostgreSQL and migrations. **Register the card before attempting a purchase.** Fund debit cards and provision credit cards with the required limit. The terminal does not create those bank-side records.

Create a `.env` file in the Card Machine repository root with `AES_CMAC_KEY` and `AES_GCM`, matching the keys configured for this terminal/user in the PSP. The CMAC key is a 128-bit AES key expressed as hexadecimal; the GCM value is a hex-encoded AES key. The card routines load this file when processing a payment, so it must be present. Keys are not sent in payment JSON and must not be committed. This MVP uses one configured shared-key context, not a universal key for all users.

```bash
git clone https://github.com/hugoferreiraTI/go_card_machine
cd go_card_machine
go mod download
go run ./cmd
```

Run from the repository root. SQLite, `.env`, images and receipts use paths relative to the working directory. Startup creates the `bank_machine` and `sales` tables if they do not already exist.

Current addresses are set in code: **Card Machine on port 8081**, **PSP on port 8080**, Kafka on port 9092. The PSP HTTP client has a ten-second timeout. The terminal's Swagger UI is available at `http://localhost:8081/swagger/index.html`. These terminal addresses are not controlled by the PSP's environment variables.

The request sequence is:

1. `POST /card_machine` to register the merchant.
2. `POST /Sale` to create one pending sale.
3. Within the current 30-second payment window, call `/Pay/pix` or `/Pay/card` for that sale.

All requests use `Content-Type: application/json`. Paths are case-sensitive; `/Pay` is no longer the payment endpoint.

### API examples

**Initialize the terminal — `POST http://localhost:8081/card_machine`:**

```json
{
  "bank":"ITAU",
  "person_name_storage":"Hugo Teste",
  "storage_name":"Test Store",
  "person_cpf":"12345678901",
  "city":"Sao Paulo",
  "state":"SP"
}
```

All shown fields are required. UUID and serial number are generated automatically. A previously registered CPF returns HTTP 409. The current registration field is an eleven-character CPF, not a separate CNPJ field.

**Create a sale — `POST http://localhost:8081/Sale`:**

```json
{"Amount":"1.33","Type":"PIX"}
```

```json
{"Amount":"1.33","Type":"CARD","payment_type":"DEBIT"}
```

```json
{"Amount":"1.33","Type":"CARD","payment_type":"CREDIT"}
```

Omit `payment_type` for Pix. Debit and credit both use `Type: "CARD"`; the separate payment-type field determines which authorization path the PSP must use. Creation persists a pending sale, not an approved payment. The current creation response uses the key `sucess`, matching the existing handler.

Only one active sale is supported at a time. A pending sale blocks creation of the next one. Payment requests are delivered to the waiting sale through channels, so the manual payment bodies do not include a sale ID.

**Pay with Pix — `POST http://localhost:8081/Pay/pix`:**

```json
{"amount":"1.33","cpf":"12345678901","name":"Test Customer","bank":"ITAU"}
```

Use a string with exactly the reconstructed decimal amount. The code compares that string with the pending sale amount. A match records approval and its timestamp, emits an event and writes `comprovante.txt`. A mismatch returns `REPROVED` but leaves the sale pending for another attempt within the original window. If no payment succeeds, the waiting flow records `EXPIRED`.

`qr.png` contains the generated BR Code. Its URL, `https://pix.com.br/caminho/{id}`, is illustrative and includes the sale ID; it is not a live Pix charge. Calling the simulation endpoint produces the local decision. No real bank confirms or settles the payment.

**Pay with a card — `POST http://localhost:8081/Pay/card`:** debit example, assuming prior registration in the PSP:

```json
{
  "PAN":"5201000000000006",
  "dateExpirity":"31/12/31",
  "Country":"076",
  "Pin":{"PIN":"2468","CVM":"ONLINE_PIN"}
}
```

For a credit sale, use a card registered for credit, for example PAN `5201500000000005` with the same test expiry/PIN **if you provisioned it that way**. The examples do not seed the PSP. The payment type comes from the preceding sale, not from guessing whether a PAN is debit or credit.

The terminal accepts `dateExpirity` as `DD/MM/YY` and converts it to EMV `YYMMDD`; `31/12/31` becomes `311231`. The current builder restricts two-digit years to `21–31`; the PSP checks the actual date, registered expiry and expiration.

A rejected card attempt ends the waiting flow for that sale. Create a new sale before retrying; this differs from a mismatched Pix amount, which leaves the listener active. An approved card payment updates local status/time and generates `comprovante_card.txt`.

### EMV data, TLV and JSON encoding

The terminal constructs `CardPayment` from the local sale ID, an RFC3339 timestamp, the decimal amount string, initial status `PENDING`, `DEBIT`/`CREDIT`, and `Emv`. It marshals that struct as JSON, posts it to the PSP and decodes `AuthorizationResponse`.

TLV stands for **Tag, Length, Value**. The project uses Go structs with binary tag and value fields and an integer byte length. This JSON representation is not a complete BER-TLV stream exchanged with a chip reader. EMV specifications cover substantially more than this terminal simulation. [EMVCo reference](https://www.emvco.com/emv-technologies/emv-contact-chip/).

| Field | Tag | Representation in this project |
|---|---|---|
| PAN | `5A` | Packed BCD, two digits per byte; a trailing `F` nibble pads odd-length PANs. |
| Expiry | `5F24` | Three BCD bytes containing `YYMMDD`. |
| Transaction country | `9F1A` | Two BCD bytes; Brazil's input `076` becomes `0076`. |
| Unpredictable number | `9F37` | Four bytes generated with `crypto/rand`. |
| Cryptogram | `9F26` | The full sixteen-byte AES-CMAC used by this MVP. |
| PIN | `Emv.Pin` | GCM payload and CVM information; stored separately from the TLV fields. |

Hexadecimal notation is used to build and inspect bytes. Go's JSON marshaler encodes `[]byte` as **Base64 strings**; the `Length` field still counts decoded value bytes. [Go JSON documentation](https://pkg.go.dev/encoding/json#Marshal).

For PAN `5201000000000006`, the value bytes are `52 01 00 00 00 00 00 06`, producing:

```json
{"Tag":"Wg==","Length":8,"Value":"UgEAAAAAAAY="}
```

This is only a PAN TLV fragment, not an authorization request. BCD, hex and Base64 are encodings, so the PAN remains recoverable. They do not encrypt it.

The Pix BR Code module uses a different, textual TLV representation with two-character tags and lengths. Its CRC-16 checks payload consistency; it is not keyed cryptographic authentication. Do not confuse that QR representation with the binary fields used for card authorization.

### Authentication, encryption and hashing

The implementation uses three distinct mechanisms:

- **AES-CMAC** authenticates the components defined by the shared protocol, using a secret key.
- **AES-GCM** encrypts and authenticates the PIN payload sent to the PSP.
- **Bcrypt**, on the PSP side, stores a salted PIN hash and verifies the decrypted PIN against it.

`Crypto` uses the author's [AES_CMAC_Go](https://github.com/hugoferreiraTI/AES_CMAC_GO) library and a 128-bit AES key. Both services build the same message:

```go
fmt.Sprintf("%s|%s|%s|%s", status, amount, timeNow, unpredictableNumber)
```

The amount is the original decimal string. The last argument is the original `[4]byte`, not its hex or Base64 representation. Changing a signed component requires a new CMAC. A keyed MAC is neither an unkeyed hash nor reversible payload encryption. [NIST SP 800-38B](https://csrc.nist.gov/pubs/sp/800/38/b/upd1/final).

For GCM, the terminal generates a random nonce, calls `Seal` with nil additional authenticated data, and hex-encodes `nonce || ciphertext || tag`. The PSP must authenticate this payload before using its plaintext PIN. [NIST SP 800-38D](https://csrc.nist.gov/pubs/sp/800/38/d/final), [Go API](https://pkg.go.dev/crypto/cipher#NewGCM).

The PSP compares the decrypted PIN with its stored hash through `bcrypt.CompareHashAndPassword`. Generating a new bcrypt hash and comparing the resulting strings would not work because salts vary. The terminal does not receive or store this bank-side hash. [Bcrypt documentation](https://pkg.go.dev/golang.org/x/crypto/bcrypt#CompareHashAndPassword).

The current CMAC message covers only its four specified components, not every payment field. The manual terminal endpoint receives a plaintext PIN before protecting it for the PSP; this is not secure hardware capture or a claim that local HTTP transport is encrypted. These are boundaries of the implemented educational protocol.

### Responses, states and persistence

The PSP's internal response contains `status`, `payment_type`, `card_network`, `issuer_bank`, `amount`, `time_now` and `log`. The terminal uses this information internally. **Its public `/Pay/card` endpoint currently returns only `status` and `log`.**

Approved card payment, HTTP 200:

```json
{"status":"APPROVED","log":""}
```

Rejected card payment, HTTP 400:

```json
{"status":"REPROVED","log":"Saldo insuficiente."}
```

PSP rejection messages currently remain in Portuguese. Local processing or communication failures may have no PSP reason available, so an empty `log` does not imply approval. The PSP can return HTTP 200 for a declined business decision; the terminal reads the JSON status and applies its own HTTP contract.

Malformed input returns 400. A duplicate merchant CPF returns 409. Creating a sale while the previous one is pending returns 400. If no payment listener accepts a request within one second, the handler returns 409. A waiting sale expires after the thirty-second timer if no payment completes. That timer does not automatically cancel work already executing inside a payment function; the PSP client has a separate ten-second network timeout.

SQLite stores merchant details in `bank_machine` and sale ID, timestamps, status, payment category and integer `real`/`cents` components in `sales`. The local schema stores `PIX` or `CARD`, not a full copy of bank-side card metadata. Approved paths set `time_aprove`. The PSP response's `time_now` is the terminal timestamp sent for authorization, not necessarily the approval timestamp.

Amounts enter as decimal strings and are split into integer reais/cents before persistence. They are reconstructed with two fractional digits; the terminal has not yet switched to the PSP's single-integer-cent representation. These flows do not use floating-point financial arithmetic.

The generated files are `qr.png`, `comprovante.txt` and `comprovante_card.txt`. They are local files overwritten by later output, not an archival receipt service. Receipts can contain merchant/payer details; masking is not implemented merely because the README examples use synthetic data.

### Events, tests and development scope

The terminal publishes and consumes topic **`sale`** on `localhost:9092`. Its consumer group is **`card-machine`**, with `earliest` for a group without offsets. The Kafka key is the string form of the sale ID. A terminal event has this shape:

```json
{"SaleId":42,"value":"1.33","status":"APPROVED"}
```

Creation, approval, rejection and expiration paths publish lifecycle events. Some failure/expiry paths have no amount available. This terminal event does **not** currently include `payment_type`. The PSP has a separate `psp_authorizations` topic and event contract; the two should not be conflated. The terminal consumer logs events rather than processing financial authorizations. There is no outbox, event deduplication or exactly-once delivery guarantee in the current terminal implementation.

Go goroutines, channels, `select` and timers coordinate waiting without blocking the whole HTTP server. `PaymentAttempt` carries a Pix response channel; `PaymentCard` carries an `AuthorizationResponse` channel so PSP rejection reasons reach the handler.

The suite covers HTTP contracts, in-memory SQLite, usecases, BR Code, CMAC/TLV, GCM, events and rejected-response propagation. Component-level flows run in an isolated environment and never send purchases to the real PSP:

```bash
go test ./internal/brcode_simplify
go test ./tests -run '^TestPaymentCardReprovedLog$' -v

# Isolated full suite: Go/CGO required, no external services.
go test -v ./...
```

`http_test.go` checks binding, HTTP status codes, PSP contracts and complete Pix/debit/credit flows. `sql_test.go` verifies persistence and SQL errors with in-memory SQLite. `usecase_test.go` checks monetary conversion, a known CMAC vector, TLV/BCD/Base64, GCM, events and payment states. Kafka uses an in-memory test broker; the PSP transport is simulated in-process. `.env`, QR codes and receipts live in temporary directories, and a virtual clock tests the thirty-second timeout. The suite does not change local data, project keys or PSP records. It does not replace integration testing against the actual PSP and PostgreSQL.

Card support and HTTP PSP integration are already implemented. Future work can address idempotency, stronger attempt correlation, reconciliation between terminal and PSP state, failure recovery and consolidation of monetary storage into integer cents. These remain development goals rather than existing guarantees.

<a id="sources-and-references"></a>

## Fontes e referências / Sources and references

As URLs abaixo registram as fontes e materiais de estudo do projeto. Os links da versão anterior foram preservados e organizados por finalidade; materiais introdutórios e ferramentas não substituem especificações normativas. Os detalhes de comportamento descritos neste README foram conferidos no código atual.

The links below document the project's study material and technical references. The original links are retained and grouped by purpose. Introductory articles and research tools are not substitutes for normative specifications. Implementation-specific behavior in this README was checked against the current source code.

### Especificações e documentação técnica / Specifications and technical documentation

- [Banco Central — Manual de Padrões para Iniciação do Pix](https://www.bcb.gov.br/content/estabilidadefinanceira/pix/Regulamento_Pix/II_ManualdePadroesparaIniciacaodoPix.pdf).
- [EMVCo — EMV Contact Chip](https://www.emvco.com/emv-technologies/emv-contact-chip/).
- [NIST SP 800-38B — CMAC, versão atualizada / updated publication](https://csrc.nist.gov/pubs/sp/800/38/b/upd1/final).
- [NIST SP 800-38B — página original preservada / original reference, superseded](https://csrc.nist.gov/pubs/sp/800/38/b/final).
- [NIST SP 800-38D — GCM e GMAC / GCM and GMAC](https://csrc.nist.gov/pubs/sp/800/38/d/final).
- [Go — JSON e serialização de bytes / JSON and byte serialization](https://pkg.go.dev/encoding/json#Marshal).
- [Go — AES-GCM](https://pkg.go.dev/crypto/cipher#NewGCM).
- [Go — comparação bcrypt / bcrypt comparison](https://pkg.go.dev/golang.org/x/crypto/bcrypt#CompareHashAndPassword).
- [Confluent — Kafka Go Client](https://github.com/confluentinc/confluent-kafka-go).
- [Docker — Kafka guide](https://docs.docker.com/guides/kafka/).
- [go-qrcode — biblioteca utilizada / library used](https://github.com/skip2/go-qrcode?utm_source=chatgpt.com).
- [Pagar.me — cartão de crédito / credit-card documentation](https://docs.pagar.me/docs/cart%C3%A3o-de-cr%C3%A9dito?utm_source=chatgpt.com).
- [Visa — Visa Accept API reference](https://developer.visa.com/api-reference/visa-accept?utm_source=chatgpt.com).

### Materiais de estudo / Background reading

- [Money as integer cents rather than float](https://www.misbah.co/blog/store-money-as-integer-cents-never-a-float?utm_source=chatgpt.com).
- [Designing a payment-processing system in Go](https://blog.stackademic.com/designing-a-payment-processing-system-in-golang-using-microservices-cce7a8213bba).
- [Santander — bandeiras de cartão / card networks](https://www.santander.com.br/blog/para-voce/bandeira-do-cartao/).
- [Santander — CVV](https://www.santander.com.br/blog/para-voce/o-que-e-codigo-cvv/).
- [CryptoID — introdução ao AES / introduction to AES](https://cryptoid.com.br/criptografia/aes-padrao-de-criptografia-avancado-o-que-e-e-como-funciona/).
- [Zencopa — introduction to AES-GCM](https://zencopa.com/learn/introduction-to-aes-gcm-mode-and-its-advantages).

### Registro de pesquisa / Research log

- [Pesquisa original sobre tags EMV / Original EMV-tag search](https://www.google.com/search?q=EMV+global+tag&sca_esv=995ca91a96abc5c1&rlz=1C1GCEU_enBR1213BR1213&sxsrf=APpeQntB_F-km0Cqg-pw1bqOfwMXbygh7Q%3A1792426012471&ei=HEDWaq-iHKmI5OUP1IrRmAI&biw=1920&bih=919&uact=5&oq=EMV+global+tag&gs_lp=Egxnd3Mtd2l6LXNlcnAiDkVNViBnbG9iYWwgdGFnMgUQIRigATIFECEYoAFI2wNQAFi3AnAAeAGQAQCYAfIBoAHBB6oBBTAuMy4yuAEDyAEA-AEBmAIFoAK6CMICCBAAGIAEGMsBwgIGEAAYFhgemAMAkgcFMC4xLjSgB7kTsgcFMC4xLjS4B7oIwgcFMi0yLjPIByiACAE&sclient=gws-wiz-serp).
- [Gemini — ferramenta registrada entre as fontes originais / tool listed in the original sources](https://gemini.google.com/app?hl=pt-BR).

### Projetos relacionados / Related projects

- [Card Machine — código deste projeto / this project](https://github.com/hugoferreiraTI/go_card_machine).
- [PSP Go — autorização e persistência bancária / authorization and bank-side persistence](https://github.com/hugoferreiraTI/psp_go).
- [AES_CMAC_Go — biblioteca de CMAC do autor / author’s CMAC library](https://github.com/hugoferreiraTI/AES_CMAC_GO).
