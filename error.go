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

func (e *Error) Response() *fasthttp.Response {
	return e.response
}
