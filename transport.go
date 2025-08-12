package tcabcireadgoclient

import (
	"bytes"
	"crypto/tls"
	"crypto/x509"
	"io"
	"os"

	"github.com/valyala/fasthttp"
)

type transport struct {
	verbose         bool
	TLSClientConfig *tls.Config
}

func newTransport(pool *x509.CertPool, verbose bool) (*transport, error) {
	return &transport{
		verbose: verbose,
		TLSClientConfig: &tls.Config{
			RootCAs:               pool,
			InsecureSkipVerify:    false,
			VerifyPeerCertificate: verifyPeer,
		},
	}, nil
}

func (r *transport) RoundTrip(hc *fasthttp.HostClient, req *fasthttp.Request, resp *fasthttp.Response) (retry bool, err error) {
	hc.TLSConfig = r.TLSClientConfig

	if !r.verbose {
		retry, err = fasthttp.DefaultTransport.RoundTrip(hc, req, resp)
		if err != nil {
			return false, err
		}

		return retry, nil
	}

	buf := bytes.NewBuffer([]byte{})
	defer buf.Reset()
	_, _ = req.WriteTo(buf)

	retry, err = fasthttp.DefaultTransport.RoundTrip(hc, req, resp)
	if err != nil {
		return false, err
	}
	_, _ = buf.Write([]byte("\n"))
	_, _ = buf.Write(resp.Body())
	_, _ = buf.Write([]byte("\n"))

	_, _ = io.Copy(os.Stderr, buf)

	return retry, err
}
