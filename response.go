package tcabcireadgoclient

type Response struct {
	Data       interface{}       `json:"data"`
	TotalCount uint64            `json:"total_count"`
	Error      bool              `json:"error"`
	Errors     map[string]string `json:"errors"`
	Detail     string            `json:"detail"`
}
