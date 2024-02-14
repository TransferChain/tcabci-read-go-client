package tcabcireadgoclient

import (
	"bytes"
	"encoding/json"
	"net/http"
)

type Summary struct {
	RecipientAddresses []string `json:"recipient_addrs,omitempty"`
	SenderAddresses    []string `json:"sender_addrs,omitempty"`
	Type               Type     `json:"typ,omitempty"`
}

func (s *Summary) URI() string {
	return "/v1/tx_summary"
}

func (s *Summary) IsValid() bool {
	if len(s.RecipientAddresses) > 251 {
		return false
	}

	if len(s.SenderAddresses) > 251 {
		return false
	}

	if s.Type != "" && !s.Type.IsValid() {
		return false
	}

	return true
}

func (s *Summary) ToJSON() ([]byte, error) {
	return json.Marshal(s)
}

func (s *Summary) ToRequest() (*http.Request, error) {
	b, err := s.ToJSON()
	if err != nil {
		return nil, err
	}

	return http.NewRequest(http.MethodPost, "", bytes.NewReader(b))
}

type SummaryResponse struct {
	Data struct {
		LastBlockHeight uint64       `json:"last_block_height"`
		LastTransaction *Transaction `json:"last_transaction"`
	} `json:"data"`
	TotalCount uint64 `json:"total_count"`
}
