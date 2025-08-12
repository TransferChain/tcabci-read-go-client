package tcabcireadgoclient

import (
	"encoding/json"

	"github.com/valyala/fasthttp"
)

type Summary struct {
	RecipientAddresses []string          `json:"recipient_addrs,omitempty"`
	SenderAddresses    []string          `json:"sender_addrs,omitempty"`
	Type               Type              `json:"typ,omitempty"`
	ChainName          *string           `json:"chain_name,omitempty"`
	ChainVersion       *string           `json:"chain_version,omitempty"`
	SignedAddresses    map[string]string `json:"signed_addrs,omitempty"`
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

	if len(s.RecipientAddresses) > 0 && len(s.RecipientAddresses) != len(s.SignedAddresses) {
		return false
	}

	if len(s.SenderAddresses) > 0 && len(s.SenderAddresses) != len(s.SignedAddresses) {
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

func (s *Summary) ToRequest() (*fasthttp.Request, error) {
	b, err := s.ToJSON()
	if err != nil {
		return nil, err
	}

	req := fasthttp.AcquireRequest()
	req.SetBody(b)
	return req, nil
}

type SummaryResponse struct {
	Data struct {
		LastBlockHeight uint64       `json:"last_block_height"`
		LastTransaction *Transaction `json:"last_transaction"`
	} `json:"data"`
	TotalCount uint64 `json:"total_count"`
}
