package model

type PaymentType string

const (
	PaymentPix  PaymentType = "PIX"
	PaymentCard PaymentType = "CARD"
)

type Sale struct {
	Amount  string      `json:"Amount" binding:"required"`
	PayType PaymentType `json:"Type" binding:"required"`
}
