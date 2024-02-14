package tcabcireadgoclient

import (
	"bytes"
	"encoding/json"
	"net/http"
)

type Broadcast struct {
	ID      string `json:"id"`
	Version int    `json:"version"`
	Type    Type   `json:"type"`
	Data    []byte `json:"data"`
	Sign    []byte `json:"sign"`
	Fee     int    `json:"fee"`
}

func (b *Broadcast) URI() string {
	return "/v1/tx"
}

func (b *Broadcast) ToJSON() ([]byte, error) {
	return json.Marshal(b)
}

func (b *Broadcast) ToRequest() (*http.Request, error) {
	body, err := b.ToJSON()
	if err != nil {
		return nil, err
	}

	return http.NewRequest(http.MethodPost, "", bytes.NewReader(body))
}

type BroadcastResponse struct {
	Data struct {
		Hash      string `json:"hash"`
		Code      uint32 `json:"code"`
		Data      []byte `json:"data"`
		Log       string `json:"log"`
		Codespace string `json:"codespace"`
	} `json:"data"`
}
