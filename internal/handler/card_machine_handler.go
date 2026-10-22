package handler

import (
	"card_machine/internal/model"
	usecases "card_machine/internal/useCases"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

type HandlerCase struct {
	NewCardMachineCase *usecases.NewCardMachineCase
	NewPaymentPixCase  *usecases.NewPaymenteCase
	NewPaymentCardCase *usecases.NewPaymentCardCase
}

func NewHandlerCase(
	cardMachineCase *usecases.NewCardMachineCase,
	paymentPixCase *usecases.NewPaymenteCase,
) *HandlerCase {
	return &HandlerCase{
		NewCardMachineCase: cardMachineCase,
		NewPaymentPixCase:  paymentPixCase,
	}
}

func (h *HandlerCase) InsertValueInCardMachine(c *gin.Context) {
	var value model.CardMachine

	if err := c.BindJSON(&value); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}

	_, err := h.NewCardMachineCase.InsertValueInCardMachine(&value)

	if err != nil {
		if err.Error() == "Cpf exists" { //verifi the error, if cpf existis return a conflict status code else return a internal server error
			c.JSON(http.StatusConflict, gin.H{"error": "Cpf already exists"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to insert value"})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"message": "Card machine initialized successfully",
	})

}

func (h *HandlerCase) PostSalePix(c *gin.Context) {
	var sale model.Sale

	if err := c.BindJSON(&sale); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "json inválido"})
		return
	}

	result, err := h.NewCardMachineCase.PostSale(sale)

	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error(), "result": result})
		return
	}

	c.JSON(http.StatusCreated, gin.H{"sucess": result})
}

func (h *HandlerCase) PaymentPix(c *gin.Context) {
	var pix model.Pix                           //json recive
	responseChan := make(chan model.SaleStatus) //create a channel

	if err := c.BindJSON(&pix); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "json inválido"})
		return
	}

	attempt := usecases.PaymentAttempt{
		Pix: model.Pix{
			AmountPix: pix.AmountPix,
			Name:      pix.Name,
			Cpf:       pix.Cpf,
			Bank:      pix.Bank,
		},
		Response: responseChan,
	}

	//Prevent the request from blocking if no payment listener is active.
	select {
	case h.NewPaymentPixCase.PaymentChan <- attempt:
		//value envied for PaymentChan
	case <-time.After(1 * time.Second):
		c.JSON(http.StatusConflict, gin.H{
			"error": "The time for payment expired or sale dont existed",
		})
		return
	}

	status := <-attempt.Response

	if status == model.StatusApproved {
		c.JSON(http.StatusOK, gin.H{
			"status": status,
		})
		return
	}
	c.JSON(http.StatusBadRequest, gin.H{
		"status": status,
	})
}
