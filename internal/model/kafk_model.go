package model

type PaymentKafka struct {
	SaleId int        `json:"SaleId"`
	Value  string     `json:"value"`
	Status SaleStatus `json:"status"`
}
