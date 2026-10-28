package client

import (
	"bytes"
	"card_machine/internal/model"
	"encoding/json"
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

	err = json.NewDecoder(resp.Body).Decode(&authorizationResponse)
	if err != nil {
		return model.AuthorizationResponse{}, err
	}

	return authorizationResponse, nil
}
