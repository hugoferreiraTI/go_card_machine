package client

import (
	"bytes"
	"card_machine/internal/model"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
)

type PSPClient struct {
	BaseURL    string
	HTTPClient *http.Client
}

func (p *PSPClient) PSP(cardPayment model.CardPayment) (model.AuthorizationResponse, error) {
	var authorizationResponse model.AuthorizationResponse

	bodyJson, err := json.Marshal(cardPayment)
	if err != nil {
		return model.AuthorizationResponse{}, err
	}

	req, err := http.NewRequest(
		http.MethodPost,
		p.BaseURL+"/authorize/card",
		bytes.NewBuffer(bodyJson),
	)
	if err != nil {
		return model.AuthorizationResponse{}, err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := p.HTTPClient.Do(req)
	if err != nil {
		return model.AuthorizationResponse{}, err
	}
	defer resp.Body.Close()

	// Uma falha HTTP não pode ser convertida em aprovação pelo conteúdo do corpo.
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return model.AuthorizationResponse{}, fmt.Errorf("PSP retornou HTTP %d", resp.StatusCode)
	}

	err = json.NewDecoder(resp.Body).Decode(&authorizationResponse)
	if err != nil {
		return model.AuthorizationResponse{}, err
	}

	if authorizationResponse.Status != model.StatusApproved {
		return authorizationResponse, errors.New("Pagamento negado - consulte seu banco")
	}
	return authorizationResponse, nil
}
