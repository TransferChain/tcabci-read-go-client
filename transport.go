package tcabcireadgoclient

import (
	"fmt"
	"os"
	"sync/atomic"

	"github.com/valyala/fasthttp"
)

type transport struct {
	verbose atomic.Bool
}

func (r *transport) RoundTrip(hc *fasthttp.HostClient, req *fasthttp.Request, resp *fasthttp.Response) (bool, error) {
	retry, err := fasthttp.DefaultTransport.RoundTrip(hc, req, resp)
	if r.verbose.Load() {
		// Deliberately omit URL, headers, bodies and raw error text.
		fmt.Fprintf(os.Stderr, "tcabci HTTP status=%d bytes=%d failed=%t\n", resp.StatusCode(), len(resp.Body()), err != nil)
	}
	return retry, err
}
