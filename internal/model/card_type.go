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
	Pin                 PIN
	NumberUnpredictable TLV //Tag EMV: 9F37
	Cryptogram          TLV //Tag EMV: 9f26
}

// tags EMV minimal IN json
type EmvTags struct {
	PAN                string `json:"PAN" binding:"required,min=12,max=19"`
	DateExpiry         string `json:"dateExpirity" binding:"required,len=8"`
	TransactionCountry string `json:"Country" binding:"required,len=3"`
	Pin                PIN    `json:"Pin" binding:"required"`
}
type PIN struct {
	PIN string
	CVM CVM
}

type CVM string

const (
	CVMOnlinePIN CVM = "ONLINE_PIN"
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
	NumberUnpredictableCrypt [4]byte    //hash generated in terminal
}

type CardPayment struct {
	SaleId      int             `json:"SaleId"`
	TimeNow     string          `json:"TimeNow"`
	Amount      string          `json:"Amount"`
	Status      SaleStatus      `json:"Status"`
	PaymentType CardPaymentType `json:"PaymentType" binding:"required"`
	Emv         EmvTlv          `json:"Emv" binding:"required"`
}

// initialize the sale
type CardSale struct {
	PaymentType CardPaymentType
	Amount      string
	SaleId      int
	TimeNow     string
	Status      SaleStatus
}
