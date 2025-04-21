package tcabcireadgoclient

import (
	"bytes"
	"encoding/json"
	"net/http"
)

type Broadcast struct {
	ID            string `json:"id"` //Hash of the transaction
	Version       uint32 `json:"version"`
	Type          Type   `json:"type"`
	SenderAddr    string `json:"sender_addr"`
	RecipientAddr string `json:"recipient_addr"`
	Data          []byte `json:"data"`
	Sign          []byte `json:"sign"`
	Fee           uint64 `json:"fee"`
}

func (b *Broadcast) URI(commit, sync bool) string {
	if commit {
		return "/v1/tx/commit"
	}

	if sync {
		return "/v1/tx/sync"
	}
	
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
