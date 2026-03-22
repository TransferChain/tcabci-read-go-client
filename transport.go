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
	verbose           bool
	insecure          bool
	TLSClientConfig   *tls.Config
	customFingerprint *string
	certs             []tls.Certificate
}

func newTransport(pool *x509.CertPool, verbose bool, insecure bool, customFingerprint *string, certs []tls.Certificate) (*transport, error) {
	return &transport{
		verbose:           verbose,
		insecure:          insecure,
		customFingerprint: customFingerprint,
		certs:             certs,
		TLSClientConfig: &tls.Config{
			ClientCAs:          pool,
			Certificates:       certs,
			InsecureSkipVerify: insecure,
			VerifyPeerCertificate: func(rawCerts [][]byte, verifiedChains [][]*x509.Certificate) error {
				return verifyPeer(rawCerts, verifiedChains, customFingerprint)
			},
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
