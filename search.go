package tcabcireadgoclient

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
)

type OrderBy string

func (t OrderBy) IsValid() bool {
	switch t {
	case ASC:
		return true
	case DESC:
		return true
	default:
		return false
	}
}

const (
	ASC  OrderBy = "ASC"
	DESC OrderBy = "DESC"
)

type HeightOperator string

func (ho HeightOperator) IsValid() bool {
	switch ho {
	case Equal:
		return true
	case Less:
		return true
	case Greater:
		return true
	case EqualOrLess:
		return true
	case EqualOrGreater:
		return true
	default:
		return false
	}
}

const (
	Equal          HeightOperator = "="
	Less           HeightOperator = "<"
	Greater        HeightOperator = ">"
	EqualOrLess    HeightOperator = "<="
	EqualOrGreater HeightOperator = ">="
)

type Search struct {
	Limit              uint              `json:"limit"`
	Height             uint64            `json:"-"`
	Offset             uint64            `json:"offset"`
	MaxHeight          uint64            `json:"max_height"`
	LastOrder          uint64            `json:"last_order"`
	Type               Type              `json:"typ,omitempty"`
	Types              []Type            `json:"types,omitempty"`
	PHeight            string            `json:"height,omitempty"`
	OrderBy            OrderBy           `json:"order_by,omitempty"`
	OrderField         string            `json:"order_field,omitempty"`
	HeightOperator     HeightOperator    `json:"-"`
	RecipientAddresses []string          `json:"recipient_addrs,omitempty"`
	SenderAddresses    []string          `json:"sender_addrs,omitempty"`
	SignedAddresses    map[string]string `json:"signed_addrs,omitempty"`
	Hashes             []string          `json:"hashes,omitempty"`
	ChainName          *string           `json:"chain_name,omitempty"`
	ChainVersion       *string           `json:"chain_version,omitempty"`
}

func (s *Search) URI() string {
	return "/v1/tx_search/p"
}

func (s *Search) IsValid() bool {
	if s.Height < 0 {
		return false
	}

	if !s.HeightOperator.IsValid() {
		return false
	}

	if len(s.SignedAddresses) == 0 {
		return false
	}

	if len(s.RecipientAddresses) > 251 {
		return false
	}

	if len(s.SenderAddresses) > 251 {
		return false
	}

	if len(s.RecipientAddresses) > 0 && len(s.SignedAddresses) != len(s.RecipientAddresses) {
		return false
	}

	if len(s.SenderAddresses) > 0 && len(s.SignedAddresses) != len(s.SenderAddresses) {
		return false
	}

	if len(s.Hashes) > 100 {
		return false
	}

	if s.Type != "" && !s.Type.IsValid() {
		return false
	}

	if len(s.Types) > 0 {
		for i := 0; i < len(s.Types); i++ {
			if !s.Types[i].IsValid() {
				return false
			}
		}
	}

	if s.Limit > 100 {
		return false
	}

	if s.OrderBy != "" && !s.OrderBy.IsValid() {
		return false
	}

	return true
}

func (s *Search) ToJSON() ([]byte, error) {
	s.PHeight = fmt.Sprintf("%s %d", s.HeightOperator, s.Height)

	buf := bytes.NewBuffer([]byte{})
	enc := json.NewEncoder(buf)

	enc.SetEscapeHTML(false)

	if err := enc.Encode(s); err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}

func (s *Search) ToRequest() (*http.Request, error) {
	b, err := s.ToJSON()
	if err != nil {
		return nil, err
	}

	return http.NewRequest(http.MethodPost, "", bytes.NewReader(b))
}

type SearchResponse struct {
	TXS        []*Transaction `json:"data"`
	TotalCount uint64         `json:"total_count"`
}
