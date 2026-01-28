package tcabcireadgoclient

import (
	"encoding/json"

	"github.com/valyala/fasthttp"
)

type Broadcast struct {
	ID             string  `json:"id"` //Hash of the transaction
	Version        uint32  `json:"version"`
	Type           Type    `json:"type"`
	SenderAddr     string  `json:"sender_addr"`
	RecipientAddr  string  `json:"recipient_addr"`
	Data           []byte  `json:"data"`
	AdditionalData *[]byte `json:"additional_data,omitempty"`
	CipherData     *[]byte `json:"cipher_data,omitempty"`
	Sign           []byte  `json:"sign"`
	Fee            uint64  `json:"fee"`
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

func (b *Broadcast) ToRequest() (*fasthttp.Request, error) {
	body, err := b.ToJSON()
	if err != nil {
		return nil, err
	}

	req := fasthttp.AcquireRequest()
	req.SetBody(body)
	return req, nil
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
