package model

// TLV default
type TLV struct {
	Tag    []byte
	Length int
	Value  []byte
}

// tags EMV in TLV default.
type EmvTlv struct {
	PAN                 TLV //Tag EMV: 5A
	DateExpiry          TLV //Tag EMV: 5F24
	TransactionCountry  TLV //Tag EMV: 9F1A
	Pin                 PIN //Tag EMV: 9F37
	NumberUnpredictable TLV //Tag EMV: 9F37
	Cryptogram          TLV //Tag EMV: 9f26
}

// tags EMV minimal IN json
type EmvTags struct {
	PAN                string `json:"PAN" binding:"required, max=19"`          //Tag EMV: 5A
	DateExpiry         string `json:"dateExpirity" binding:"required, max=6"`  //Tag EMV: 5F24
	TransactionCountry string `json:"Country"  binding:"required,min=3,max=3"` //Tag EMV: 9F1A
	Pin                PIN    `json:"Pin" binding:"required,min=4,max=6"`      //Tratamento proprio
}

type PIN struct {
	PIN string
	CVM CVM
}

type CVM string

const (
	CVMOnlinePIN  CVM = "ONLINE_PIN"
)

type CardPaymentType string

const (
	Debit  CardPaymentType = "DEBIT"
	Credit CardPaymentType = "CREDIT"
)

// the cryptograma use all data of cardpay, for crypotografe and in bank compare the values.
type Cryptogram struct {
	StatusCard               SaleStatus //Status the sale
	AmountCrypt              string     //Amount value
	TimeNowCrypt             string     //Time of sale
	NumberUnpredictableCrypt string     //hash generated in terminal
}

type CardPayment struct {
	SaleId      int
	TimeNow     string
	Amount      string
	Status      SaleStatus
	PaymentType CardPaymentType `json:"TyPay" binding:"required"`
	Emv         EmvTlv          `json:"Emv" binding:"required"`
}

// initialize the sale
type CardSale struct {
	PaymentType CardPaymentType
	Amount      string
}
