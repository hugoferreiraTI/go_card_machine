package model

type SaleStatus string

const (
	StatusPending  SaleStatus = "PENDING"
	StatusApproved SaleStatus = "APPROVED"
)

type Pix struct {
	AmountPix string `json:"amount"`
}
