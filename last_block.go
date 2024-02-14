package tcabcireadgoclient

type LastBlock struct {
	Blocks     []*Transaction `json:"data"`
	TotalCount uint64         `json:"total_count"`
}
