package bank

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
)

type MockProvider struct {
	apiURL string
}

func NewMockProvider(apiURL string) *MockProvider {
	return &MockProvider{apiURL: apiURL}
}

func (m *MockProvider) Register3DS(req *Register3DSRequest) (*Register3DSResponse, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return nil, fmt.Errorf("generate trans id: %w", err)
	}
	transID := "3ds_" + hex.EncodeToString(b)

	auth := struct {
		ThreeDSServerTransID string `json:"threeDSServerTransID"`
		MessageVersion       string `json:"messageVersion"`
	}{
		ThreeDSServerTransID: transID,
		MessageVersion:       "2.2.0",
	}
	raw, _ := json.Marshal(auth)
	creq := base64.RawURLEncoding.EncodeToString(raw)

	acsURL := fmt.Sprintf("%s/auth/%s", m.apiURL, transID)

	return &Register3DSResponse{
		ThreeDSServerTransID: transID,
		AcsURL:              acsURL,
		CReq:                creq,
	}, nil
}

func (m *MockProvider) Submit3DS(req *Submit3DSRequest) (*Submit3DSResponse, error) {
	cres := req.CRes

	if cres == "simulated_cres" {
		return &Submit3DSResponse{Success: true}, nil
	}

	parts := strings.SplitN(cres, ".", 3)
	if len(parts) < 2 {
		return &Submit3DSResponse{Success: false}, nil
	}

	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		payload, err = base64.StdEncoding.DecodeString(parts[1])
		if err != nil {
			return &Submit3DSResponse{Success: false}, nil
		}
	}

	var claims struct {
		ThreeDSServerTransID string `json:"threeDSServerTransID"`
		TransStatus         string `json:"transStatus"`
	}
	if err := json.Unmarshal(payload, &claims); err != nil {
		return &Submit3DSResponse{Success: false}, nil
	}

	if claims.ThreeDSServerTransID != req.ThreeDSServerTransID {
		return &Submit3DSResponse{Success: false}, nil
	}

	return &Submit3DSResponse{Success: claims.TransStatus == "Y"}, nil
}

