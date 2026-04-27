package tcabcireadgoclient

type Block struct {
	Height       uint64         `json:"height"`
	TXS          int64          `json:"txs"`
	Hash         *string        `json:"hash,omitempty"`
	Transactions []*Transaction `json:"transactions"`
	ChainName    *string        `json:"chain_name,omitempty"`
	ChainVersion *string        `json:"chain_version,omitempty"`
}
