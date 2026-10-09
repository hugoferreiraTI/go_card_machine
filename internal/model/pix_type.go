package model

type SaleStatus string

const (
	StatusPending  SaleStatus = "PENDING"
	StatusApproved SaleStatus = "APPROVED"
	StatusReproved SaleStatus = "REPROVED"
)

type Pix struct {
	AmountPix string `json:"amount"`
}
