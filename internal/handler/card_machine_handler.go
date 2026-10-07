package handler

import (
	"card_machine/internal/model"
	usecases "card_machine/internal/useCases"
	"net/http"

	"github.com/gin-gonic/gin"
)

type HandlerCase struct {
	NewCardMachineCase *usecases.NewCardMachineCase
}

func NewHandlerCase(repo *usecases.NewCardMachineCase) *HandlerCase {
	return &HandlerCase{
		NewCardMachineCase: repo,
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
