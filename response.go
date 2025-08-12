package tcabcireadgoclient

type Response struct {
	Message    *string           `json:"message,omitempty"`
	Data       interface{}       `json:"data,omitempty"`
	TotalCount uint64            `json:"total_count,omitempty"`
	Error      bool              `json:"error"`
	Errors     map[string]string `json:"errors,omitempty"`
	Detail     *string           `json:"detail,omitempty"`
}

func (r *Response) GetMessage() string {
	if r.Message != nil {
		return *r.Message
	}

	return ""
}

func (r *Response) GetData() interface{} {
	if r.Data != nil {
		return r.Data
	}

	return nil
}

func (r *Response) GetDetail() string {
	if r.Detail != nil {
		return *r.Detail
	}

	return ""
}
