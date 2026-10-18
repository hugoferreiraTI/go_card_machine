package model

type SaleStatus string

const (
	StatusPending  SaleStatus = "PENDING"
	StatusApproved SaleStatus = "APPROVED"
	StatusExpired  SaleStatus = "EXPIRED"
	StatusReproved SaleStatus = "REPROVED"
)

type Pix struct { //pix for create a Sale
	AmountPix string `json:"amount"`
	Cpf       string `json:"cpf" binding:"required,min=11,max=11"`
	Name      string `json:"name" binding:"required"`
	Bank      string `json:"bank" binding:"required"`
}

type PixSale struct {
	PixSale string `json:"PixSale"`
}

//value string, time string, status model.SaleStatus, name string, cpf string, bank string
