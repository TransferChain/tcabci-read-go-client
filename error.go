package tcabcireadgoclient

import (
	"github.com/valyala/fasthttp"
)

type ErrCode int

const (
	SYSErr ErrCode = iota
	CLIENTErr
	PARAMETERErr
)

type Error struct {
	typ      ErrCode
	origin   error
	message  string
	code     int
	status   int
	response *fasthttp.Response
}

func (e *Error) Type() ErrCode {
	return e.typ
}

func (e *Error) Origin() error {
	return e.origin
}

func (e *Error) Error() string {
	return e.message
}

func (e *Error) Code() int {
	return e.code
}

func (e *Error) Status() int {
	return e.status
}

// Response returns an independent status-only response. Headers and body are
// omitted to avoid retaining credentials or pooled data.
func (e *Error) Response() *fasthttp.Response {
	return responseSnapshot(e.response)
}

func (e *Error) Unwrap() error {
	return e.origin
}

// responseSnapshot retains status only; pooled payloads and credentials are omitted.
func responseSnapshot(resp *fasthttp.Response) *fasthttp.Response {
	if resp == nil {
		return nil
	}
	snapshot := &fasthttp.Response{}
	snapshot.SetStatusCode(resp.StatusCode())
	return snapshot
}
