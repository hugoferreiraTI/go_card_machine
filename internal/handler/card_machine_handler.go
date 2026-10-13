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
	NewPaymentCase     *usecases.NewPaymenteCase
}

func NewHandlerCase(
	cardMachineCase *usecases.NewCardMachineCase,
	paymentCase *usecases.NewPaymenteCase,
) *HandlerCase {
	return &HandlerCase{
		NewCardMachineCase: cardMachineCase,
		NewPaymentCase:     paymentCase,
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

func (h *HandlerCase) PostSales(c *gin.Context) {
	var pixAmount model.Pix

	if err := c.BindJSON(&pixAmount); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "json inválido"})
		return
	}

	result, err := h.NewCardMachineCase.PostSale(pixAmount.AmountPix)

	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error(), "result": result})
		return
	}

	c.JSON(http.StatusCreated, gin.H{"sucess": result})
}

func (h *HandlerCase) Payment(c *gin.Context) {
	var pixAmount model.Pix                     //json recive
	responseChan := make(chan model.SaleStatus) //create a channel

	if err := c.BindJSON(&pixAmount); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "json inválido"})
		return
	}

	attempt := usecases.PaymentAttempt{
		Pix:      pixAmount,
		Response: responseChan,
	} //created the struc

	//use a case for "case" a buffer are void, evited loops and loadings infinite
	select {
	case h.NewCardMachineCase.PaymentChan <- attempt:
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
