// package tcabcireadgoclient
//
// Copyright 2019 TransferChain A.G
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
//You may obtain a copy of the License at
//
// https://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package tcabcireadgoclient

import (
	"context"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/fasthttp/websocket"
	"github.com/valyala/fasthttp"
	"github.com/valyala/fasthttp/fasthttpproxy"
)

// ErrNoConnected ...
// Reference: https://github.com/recws-org/recws/blob/master/recws.go
var ErrNoConnected = errors.New("websocket: not connected")
var ErrAlreadyStarted = errors.New("already started")
var ErrNotStarted = errors.New("not started yet")

const (
	HandshakeTimeout = 7 * time.Second
	writeTimeout     = 7 * time.Second
	enqueueTimeout   = 100 * time.Millisecond
	pongWait         = 30 * time.Second
	pingPeriod       = 10 * time.Second
	maxMessageSize   = 16 * 1024 * 1024
)

// Client TCABCI Read Node Websocket Client
type Client interface {
	WithMode(mode Mode) Client
	WithProxy(proxyURL *url.URL) Client
	WithLogger(l Logger) Client
	SetVerbose(verbose bool) (Client, error)
	// SetListenCallback ...
	// Deprecated
	SetListenCallback(func(block *Block, transaction *Transaction)) Client
	AddListenCallback(func(block *Block, transaction *Transaction)) Client
	AddErrorCallback(func(err error)) Client
	AddRetrieveCallback(fn func()) Client
	AddHeader(key, value string) Client
	RemoveHeader(key string) Client
	AddWSHeader(key, value string) Client
	RemoveWSHeader(key string) Client
	Start() error
	Stop() error
	WSQuery(typ string, data []byte) error
	Subscribe(addresses []string, signedDatas map[string]string, txTypes ...Type) error
	Unsubscribe() error
	Write(b []byte) error
	LastBlock(chainName, chainVersion *string) (*LastBlock, error)
	Tx(id string, signature string, chainName, chainVersion *string) (*Transaction, error)
	TxSummary(summary *Summary) (lastBlockHeight uint64, lastTransaction *Transaction, totalCount uint64, err error)
	TxSearch(search *Search) (txs []*Transaction, totalCount uint64, err error)
	Broadcast(id string, version uint32, typ Type, data []byte, additionalData, cipherData *[]byte, senderAddress, recipientAddress string, sign []byte, fee uint64) (*BroadcastResponse, error)
	BroadcastSync(id string, version uint32, typ Type, data []byte, additionalData, cipherData *[]byte, senderAddress, recipientAddress string, sign []byte, fee uint64) (*BroadcastResponse, error)
	BroadcastCommit(id string, version uint32, typ Type, data []byte, additionalData, cipherData *[]byte, senderAddress, recipientAddress string, sign []byte, fee uint64) (*BroadcastResponse, error)
	FetchNS(identifier string) (*NS, error)
	Query(method string, path string, data []byte, headers map[string][]string) (*Response, error)
}

type client struct {
	ctx                  context.Context
	lgr                  Logger
	mode                 Mode
	address              string
	wsURL                *url.URL
	chainName            string
	chainVersion         string
	version              string
	retrieveCallbacks    []func()
	listenCallbacks      []func(*Block, *Transaction)
	errorCallbacks       []func(error)
	headers              fasthttp.RequestHeader
	wsHeaders            fasthttp.RequestHeader
	mut                  sync.RWMutex
	lifecycle            sync.Mutex
	httpMu               sync.RWMutex
	session              *wsSession
	subscribedAddresses  map[string]bool
	subscribedSignedData map[string]string
	subscribedTypes      []Type
	subscriptionRevision uint64
	dialer               *websocket.Dialer
	httpClient           *fasthttp.Client
	transport            *transport
	callbackSlots        chan struct{}
}

// NewClient make ws client
func NewClient(address string, wsAddress string, chainName, chainVersion string, insecure bool, customFingerprint *string, cert io.Reader) (Client, error) {
	return newClient(context.Background(), address, wsAddress, chainName, chainVersion, insecure, customFingerprint, cert)
}

// NewClientContext make ws client with context
func NewClientContext(ctx context.Context, address string, wsAddress string, chainName, chainVersion string, insecure bool, customFingerprint *string, cert io.Reader) (Client, error) {
	return newClient(ctx, address, wsAddress, chainName, chainVersion, insecure, customFingerprint, cert)
}

func newClient(ctx context.Context, address string, wsAddress string, chainName, chainVersion string, insecure bool, customFingerprint *string, cert io.Reader) (Client, error) {
	if ctx == nil {
		return nil, errors.New("nil context")
	}
	if customFingerprint != nil {
		value := *customFingerprint
		customFingerprint = &value
		if value != "" {
			pin, err := hex.DecodeString(value)
			if err != nil || len(pin) != 32 {
				return nil, errors.New("invalid certificate pin")
			}
		}
	}
	aURL, err := url.Parse(address)
	if err != nil {
		return nil, err
	}

	if (aURL.Scheme != "https" && aURL.Scheme != "http") || aURL.Hostname() == "" {
		return nil, errors.New("invalid address")
	}

	wsURL, err := url.Parse(wsAddress)
	if err != nil {
		return nil, err
	}

	if (wsURL.Scheme != "wss" && wsURL.Scheme != "ws") || wsURL.Hostname() == "" {
		return nil, errors.New("invalid websocket address")
	}

	maxIdleConnDuration, _ := time.ParseDuration("3s")

	// cert is a public server certificate pin, not an mTLS client identity.
	if cert != nil {
		raw, err := io.ReadAll(io.LimitReader(cert, 1024*1024+1))
		defer clear(raw)
		if err != nil {
			return nil, errors.New("cannot read server certificate")
		}
		if len(raw) > 1024*1024 {
			return nil, errors.New("server certificate too large")
		}
		der := raw
		if block, _ := pem.Decode(raw); block != nil {
			if block.Type != "CERTIFICATE" {
				return nil, errors.New("expected server certificate")
			}
			der = block.Bytes
		}
		parsed, err := x509.ParseCertificate(der)
		if err != nil {
			return nil, errors.New("invalid server certificate")
		}
		sum := sha256.Sum256(parsed.Raw)
		certPin := hex.EncodeToString(sum[:])
		// Explicit empty fingerprint always keeps pinning disabled.
		if customFingerprint == nil {
			customFingerprint = &certPin
		} else if *customFingerprint != "" && !strings.EqualFold(*customFingerprint, certPin) {
			return nil, errors.New("certificate and fingerprint disagree")
		}
	}

	var tlsConfig *tls.Config
	if aURL.Scheme == "https" || wsURL.Scheme == "wss" || cert != nil {
		pool, err := x509.SystemCertPool()
		if err != nil {
			return nil, err
		}

		tlsConfig = &tls.Config{
			RootCAs:            pool,
			MinVersion:         tls.VersionTLS12,
			InsecureSkipVerify: insecure, // #nosec G402 -- Explicit caller opt-out; optional pinning remains independent.
			VerifyConnection: func(state tls.ConnectionState) error {
				var raw [][]byte
				if len(state.PeerCertificates) > 0 {
					raw = [][]byte{state.PeerCertificates[0].Raw}
				}
				return verifyPeer(raw, state.VerifiedChains, customFingerprint)
			},
		}
	}

	c := &client{
		ctx:                  ctx,
		callbackSlots:        make(chan struct{}, 16),
		version:              "1.6.37",
		lgr:                  NewLogger(ctx),
		mode:                 Subscription,
		address:              address,
		chainName:            chainName,
		chainVersion:         chainVersion,
		wsURL:                wsURL,
		retrieveCallbacks:    make([]func(), 0),
		subscribedAddresses:  make(map[string]bool),
		subscribedSignedData: make(map[string]string),
		subscribedTypes:      make([]Type, 0),
		listenCallbacks:      make([]func(block *Block, transaction *Transaction), 0),
		errorCallbacks:       make([]func(err error), 0),
		dialer: &websocket.Dialer{
			TLSClientConfig:   tlsConfig,
			HandshakeTimeout:  HandshakeTimeout,
			ReadBufferSize:    5 * 1024 * 1024,
			WriteBufferSize:   5 * 1024 * 1024,
			EnableCompression: false,
		},
		httpClient: &fasthttp.Client{
			MaxResponseBodySize:           16 * 1024 * 1024,
			WriteTimeout:                  7 * time.Second,
			ReadTimeout:                   7 * time.Second,
			NoDefaultUserAgentHeader:      true,
			DisableHeaderNamesNormalizing: true,
			DisablePathNormalizing:        true,
			MaxIdleConnDuration:           maxIdleConnDuration,
			Dial: (&fasthttp.TCPDialer{
				Concurrency:      4096,
				DNSCacheDuration: time.Hour,
			}).Dial,
			TLSConfig: tlsConfig,
		},
	}

	c.transport = &transport{}
	c.httpClient.Transport = c.transport

	c.headers.Set("Client", fmt.Sprintf("tcabaci-read-go-client/%s (%s;%s)", c.version, runtime.GOOS, runtime.GOARCH))
	c.headers.Set("User-Agent", fmt.Sprintf("tcabaci-read-go-client/%s (%s;%s)", c.version, runtime.GOOS, runtime.GOARCH))
	c.wsHeaders.Set("Client", fmt.Sprintf("tcabaci-read-go-client/%s (%s;%s)", c.version, runtime.GOOS, runtime.GOARCH))
	c.wsHeaders.Set("User-Agent", fmt.Sprintf("tcabaci-read-go-client/%s (%s;%s)", c.version, runtime.GOOS, runtime.GOARCH))

	return c, nil
}

// WithMode applies to the next connection and subscription update.
func (c *client) WithMode(mode Mode) Client {
	c.mut.Lock()
	c.mode = mode
	c.subscriptionRevision++
	c.notifySubscriptionLocked()
	c.mut.Unlock()
	return c
}

func (c *client) WithProxy(proxyURL *url.URL) Client {
	c.httpMu.Lock()
	defer c.httpMu.Unlock()
	c.mut.Lock()
	defer c.mut.Unlock()
	if proxyURL == nil {
		c.httpClient.Dial = (&fasthttp.TCPDialer{Concurrency: 4096, DNSCacheDuration: time.Hour}).Dial
		c.dialer.Proxy = nil
	} else {
		proxy := *proxyURL
		c.httpClient.Dial = fasthttpproxy.FasthttpHTTPDialerTimeout(proxy.String(), HandshakeTimeout)
		c.dialer.Proxy = http.ProxyURL(&proxy)
	}
	c.httpClient.CloseIdleConnections()
	return c
}

func (c *client) WithLogger(l Logger) Client {
	if l != nil {
		c.mut.Lock()
		c.lgr = l
		c.mut.Unlock()
	}
	return c
}

// SetVerbose logs status and response size only.
func (c *client) SetVerbose(v bool) (Client, error) {
	c.transport.verbose.Store(v)
	return c, nil
}

// Deprecated: use AddListenCallback.
func (c *client) SetListenCallback(fn func(*Block, *Transaction)) Client {
	c.mut.Lock()
	defer c.mut.Unlock()
	c.listenCallbacks = nil
	if fn != nil {
		c.listenCallbacks = append(c.listenCallbacks, fn)
	}
	return c
}

func (c *client) AddListenCallback(fn func(*Block, *Transaction)) Client {
	if fn != nil {
		c.mut.Lock()
		c.listenCallbacks = append(c.listenCallbacks, fn)
		c.mut.Unlock()
	}
	return c
}

func (c *client) AddRetrieveCallback(fn func()) Client {
	if fn != nil {
		c.mut.Lock()
		c.retrieveCallbacks = append(c.retrieveCallbacks, fn)
		c.mut.Unlock()
	}
	return c
}

func (c *client) AddErrorCallback(fn func(error)) Client {
	if fn != nil {
		c.mut.Lock()
		c.errorCallbacks = append(c.errorCallbacks, fn)
		c.mut.Unlock()
	}
	return c
}

func (c *client) AddHeader(key, value string) Client {
	c.mut.Lock()
	defer c.mut.Unlock()
	c.headers.Add(key, value)
	return c
}

func (c *client) RemoveHeader(key string) Client {
	c.mut.Lock()
	defer c.mut.Unlock()
	c.headers.Del(key)
	return c
}

func (c *client) AddWSHeader(key, value string) Client {
	c.mut.Lock()
	defer c.mut.Unlock()
	c.wsHeaders.Add(key, value)
	return c
}

func (c *client) RemoveWSHeader(key string) Client {
	c.mut.Lock()
	defer c.mut.Unlock()
	c.wsHeaders.Del(key)
	return c
}

func (c *client) copyHeaders(req *fasthttp.Request) {
	c.mut.RLock()
	defer c.mut.RUnlock()
	c.headers.CopyTo(&req.Header)
}

func (c *client) do(req *fasthttp.Request, resp *fasthttp.Response) error {
	c.httpMu.RLock()
	defer c.httpMu.RUnlock()
	return c.httpClient.Do(req, resp)
}

func (c *client) logError(err error) {
	c.mut.RLock()
	lgr := c.lgr
	c.mut.RUnlock()
	if lgr != nil {
		lgr.Error(err)
	}
}

// LastBlock fetch last block in blockchain network
func (c *client) LastBlock(chainName, chainVersion *string) (*LastBlock, error) {
	var lastBlock LastBlock

	u := "limit=1&offset=0"
	if chainName != nil && chainVersion != nil {
		u += "&chain_name=" + url.QueryEscape(*chainName) + "&chain_version=" + url.QueryEscape(*chainVersion)
	} else {
		u += "&chain_name=" + url.QueryEscape(c.chainName) + "&chain_version=" + url.QueryEscape(c.chainVersion)
	}

	req := fasthttp.AcquireRequest()
	defer fasthttp.ReleaseRequest(req)
	c.copyHeaders(req)
	req.Header.Set("Content-Type", "application/json")

	uri := fasthttp.AcquireURI()
	defer fasthttp.ReleaseURI(uri)
	uri.SetScheme(c.parsedAddr()[0])
	uri.SetHost(c.parsedAddr()[1])
	uri.SetPath("/v1/blocks")
	uri.SetQueryString(u)

	req.SetURI(uri)

	resp := fasthttp.AcquireResponse()
	defer fasthttp.ReleaseResponse(resp)
	if err := c.do(req, resp); err != nil {
		c.logError(err)
		return nil, &Error{origin: err, message: err.Error(), typ: CLIENTErr, code: 5}
	}

	if resp.StatusCode() >= 400 && resp.StatusCode() < 500 {
		var errorResponse Response
		if err := json.Unmarshal(resp.Body(), &errorResponse); err != nil {
			c.logError(err)
			return nil, &Error{origin: err, message: err.Error(), typ: CLIENTErr, code: 6, status: resp.StatusCode(), response: responseSnapshot(resp)}
		}

		c.logError(errors.New("read node request rejected"))
		return nil, errors.New("read node request rejected")
	}

	if resp.StatusCode() >= 500 {
		return nil, &Error{origin: errors.New(fasthttp.StatusMessage(resp.StatusCode())), message: errors.New(fasthttp.StatusMessage(resp.StatusCode())).Error(), typ: CLIENTErr, code: 7, status: resp.StatusCode(), response: responseSnapshot(resp)}
	}

	if resp.StatusCode() != 200 {
		return nil, errors.New("unexpected status code: " + strconv.Itoa(resp.StatusCode()))
	}

	if err := json.Unmarshal(resp.Body(), &lastBlock); err != nil {
		c.logError(err)
		return nil, &Error{origin: err, message: err.Error(), typ: CLIENTErr, code: 8, status: resp.StatusCode(), response: responseSnapshot(resp)}
	}

	return &lastBlock, nil
}

func (c *client) Tx(id string, signature string, chainName, chainVersion *string) (*Transaction, error) {
	if id == "" {
		return nil, &Error{origin: errors.New("invalid tx id"), message: "invalid tx id", typ: PARAMETERErr, code: 9}
	}

	var txResponse Response
	txResponse.Data = &Transaction{}

	u := ""
	if chainName != nil && chainVersion != nil {
		u += "chain_name=" + url.QueryEscape(*chainName) + "&chain_version=" + url.QueryEscape(*chainVersion)
	} else {
		u += "chain_name=" + url.QueryEscape(c.chainName) + "&chain_version=" + url.QueryEscape(c.chainVersion)
	}

	req := fasthttp.AcquireRequest()
	defer fasthttp.ReleaseRequest(req)

	uri := fasthttp.AcquireURI()
	defer fasthttp.ReleaseURI(uri)
	uri.SetScheme(c.parsedAddr()[0])
	uri.SetHost(c.parsedAddr()[1])
	uri.SetPath("/v1/tx/" + id)
	uri.SetQueryString(u)

	req.SetURI(uri)

	c.copyHeaders(req)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Signature", signature)

	resp := fasthttp.AcquireResponse()
	defer fasthttp.ReleaseResponse(resp)
	if err := c.do(req, resp); err != nil {
		c.logError(err)
		return nil, err
	}

	if resp.StatusCode() >= 400 && resp.StatusCode() < 500 {
		var errorResponse Response
		if err := json.Unmarshal(resp.Body(), &errorResponse); err != nil {
			c.logError(err)
			return nil, err
		}

		c.logError(errors.New("read node request rejected"))
		return nil, errors.New("read node request rejected")
	}

	if resp.StatusCode() >= 500 {
		return nil, errors.New(fasthttp.StatusMessage(resp.StatusCode()))
	}

	if resp.StatusCode() != 200 {
		return nil, errors.New("unexpected status code: " + strconv.Itoa(resp.StatusCode()))
	}

	if err := json.Unmarshal(resp.Body(), &txResponse); err != nil {
		c.logError(err)
		return nil, err
	}

	tx, ok := txResponse.GetData().(*Transaction)
	if !ok || tx == nil {
		return nil, errors.New("invalid transaction response")
	}
	return tx, nil
}

// TxSummary fetch summary with given parameters
func (c *client) TxSummary(summary *Summary) (lastBlockHeight uint64, lastTransaction *Transaction, totalCount uint64, err error) {
	if summary == nil || !summary.IsValid() {
		err = errors.New("invalid parameters")
		return 0, nil, 0, err
	}

	copy := *summary
	summary = &copy
	if summary.ChainName == nil {
		summary.ChainName = &c.chainName
	}

	if summary.ChainVersion == nil {
		summary.ChainVersion = &c.chainVersion
	}

	req, err := summary.ToRequest()
	if err != nil {
		c.logError(err)
		return 0, nil, 0, err
	}
	defer fasthttp.ReleaseRequest(req)

	uri := fasthttp.AcquireURI()
	defer fasthttp.ReleaseURI(uri)
	uri.SetScheme(c.parsedAddr()[0])
	uri.SetHost(c.parsedAddr()[1])
	uri.SetPath(summary.URI())

	req.SetURI(uri)

	c.copyHeaders(req)
	req.Header.Set("Content-Type", "application/json")

	var summaryResponse SummaryResponse

	resp := fasthttp.AcquireResponse()
	defer fasthttp.ReleaseResponse(resp)

	req.Header.SetMethod(fasthttp.MethodPost)
	if err = c.do(req, resp); err != nil {
		c.logError(err)
		return 0, nil, 0, err
	}

	if resp.StatusCode() >= 400 && resp.StatusCode() < 500 {
		var errorResponse Response
		if err := json.Unmarshal(resp.Body(), &errorResponse); err != nil {
			c.logError(err)
			return 0, nil, 0, err
		}

		c.logError(errors.New("read node request rejected"))
		return 0, nil, 0, errors.New("read node request rejected")
	}

	if resp.StatusCode() >= 500 {
		return 0, nil, 0, errors.New(fasthttp.StatusMessage(resp.StatusCode()))
	}

	if resp.StatusCode() != 200 {
		return 0, nil, 0, errors.New("unexpected status code: " + strconv.Itoa(resp.StatusCode()))
	}

	if err = json.Unmarshal(resp.Body(), &summaryResponse); err != nil {
		c.logError(err)
		return 0, nil, 0, err
	}

	return summaryResponse.Data.LastBlockHeight, summaryResponse.Data.LastTransaction, summaryResponse.TotalCount, nil
}

// TxSearch search with given parameters
func (c *client) TxSearch(search *Search) (txs []*Transaction, totalCount uint64, err error) {
	if search == nil || !search.IsValid() {
		err = errors.New("invalid parameters")
		c.logError(err)
		return nil, 0, err
	}

	copy := *search
	search = &copy
	if search.ChainName == nil {
		search.ChainName = &c.chainName
	}

	if search.ChainVersion == nil {
		search.ChainVersion = &c.chainVersion
	}

	req, err := search.ToRequest()
	if err != nil {
		c.logError(err)
		return nil, 0, err
	}
	defer fasthttp.ReleaseRequest(req)

	c.copyHeaders(req)
	req.Header.Set("Content-Type", "application/json")

	uri := fasthttp.AcquireURI()
	defer fasthttp.ReleaseURI(uri)
	uri.SetScheme(c.parsedAddr()[0])
	uri.SetHost(c.parsedAddr()[1])
	uri.SetPath(search.URI())

	req.SetURI(uri)

	var searchResponse SearchResponse

	resp := fasthttp.AcquireResponse()
	defer fasthttp.ReleaseResponse(resp)

	req.Header.SetMethod(fasthttp.MethodPost)
	if err = c.do(req, resp); err != nil {
		c.logError(err)
		return nil, 0, err
	}

	if resp.StatusCode() >= 400 && resp.StatusCode() < 500 {
		var errorResponse Response
		if err := json.Unmarshal(resp.Body(), &errorResponse); err != nil {
			c.logError(err)
			return nil, 0, err
		}

		c.logError(errors.New("read node request rejected"))
		return nil, 0, errors.New("read node request rejected")
	}

	if resp.StatusCode() >= 500 {
		return nil, 0, errors.New(fasthttp.StatusMessage(resp.StatusCode()))
	}

	if resp.StatusCode() != 200 {
		return nil, 0, errors.New("unexpected status code: " + strconv.Itoa(resp.StatusCode()))
	}

	if err = json.Unmarshal(resp.Body(), &searchResponse); err != nil {
		c.logError(err)
		return nil, 0, err
	}

	return searchResponse.TXS, searchResponse.TotalCount, nil
}

func (c *client) Broadcast(id string, version uint32, typ Type, data []byte, additionalData, cipherData *[]byte, senderAddress, recipientAddress string, sign []byte, fee uint64) (*BroadcastResponse, error) {
	resp, err := c.broadcast(id, version, typ, data, additionalData, cipherData, senderAddress, recipientAddress, sign, fee, false, false)
	if err != nil {
		c.logError(err)
		return nil, err
	}

	return resp, nil
}

func (c *client) BroadcastSync(id string, version uint32, typ Type, data []byte, additionalData, cipherData *[]byte, senderAddress, recipientAddress string, sign []byte, fee uint64) (*BroadcastResponse, error) {
	resp, err := c.broadcast(id, version, typ, data, additionalData, cipherData, senderAddress, recipientAddress, sign, fee, false, true)
	if err != nil {
		c.logError(err)
		return nil, err
	}

	return resp, nil
}

func (c *client) BroadcastCommit(id string, version uint32, typ Type, data []byte, additionalData, cipherData *[]byte, senderAddress, recipientAddress string, sign []byte, fee uint64) (*BroadcastResponse, error) {
	resp, err := c.broadcast(id, version, typ, data, additionalData, cipherData, senderAddress, recipientAddress, sign, fee, true, false)
	if err != nil {
		c.logError(err)
		return nil, err
	}

	return resp, nil
}

func (c *client) Query(method string, path string, data []byte, headers map[string][]string) (*Response, error) {
	if e, _, _ := InArray(method, []string{fasthttp.MethodGet, fasthttp.MethodPost}); !e {
		return nil, errors.New("invalid method")
	}

	var err error
	req := fasthttp.AcquireRequest()
	defer fasthttp.ReleaseRequest(req)

	c.copyHeaders(req)
	req.Header.Set("Content-Type", "application/json")
	for k, v := range headers {
		for _, vv := range v {
			req.Header.Add(k, vv)
		}
	}

	uri := fasthttp.AcquireURI()
	defer fasthttp.ReleaseURI(uri)

	uri.SetScheme(c.parsedAddr()[0])
	uri.SetHost(c.parsedAddr()[1])
	uri.SetPath(path)
	req.SetURI(uri)

	if data != nil {
		req.SetBody(data)
	}

	resp := fasthttp.AcquireResponse()
	defer fasthttp.ReleaseResponse(resp)

	req.Header.SetMethod(method)
	if err = c.do(req, resp); err != nil {
		c.logError(err)
		return nil, err
	}

	if resp.StatusCode() >= 400 && resp.StatusCode() < 500 {
		var errorResponse Response
		if err := json.Unmarshal(resp.Body(), &errorResponse); err != nil {
			c.logError(err)
			return nil, err
		}

		c.logError(errors.New("read node request rejected"))
		return nil, errors.New("read node request rejected")
	}

	if resp.StatusCode() >= 500 {
		return nil, errors.New(fasthttp.StatusMessage(resp.StatusCode()))
	}

	if resp.StatusCode() < 200 || resp.StatusCode() >= 300 {
		return nil, errors.New("unexpected status code: " + strconv.Itoa(resp.StatusCode()))
	}

	var response Response
	if err = json.Unmarshal(resp.Body(), &response); err != nil {
		c.logError(err)
		return nil, err
	}

	return &response, nil
}

func (c *client) FetchNS(identifier string) (*NS, error) {
	var err error
	req := fasthttp.AcquireRequest()
	defer fasthttp.ReleaseRequest(req)

	c.copyHeaders(req)
	req.Header.Set("Content-Type", "application/json")

	uri := fasthttp.AcquireURI()
	defer fasthttp.ReleaseURI(uri)

	uri.SetScheme(c.parsedAddr()[0])
	uri.SetHost(c.parsedAddr()[1])
	uri.SetPath("/v1/ns/" + identifier)
	req.SetURI(uri)

	resp := fasthttp.AcquireResponse()
	defer fasthttp.ReleaseResponse(resp)

	req.Header.SetMethod(fasthttp.MethodGet)
	if err = c.do(req, resp); err != nil {
		c.logError(err)
		return nil, err
	}

	if resp.StatusCode() >= 400 && resp.StatusCode() < 500 {
		var errorResponse Response
		if err := json.Unmarshal(resp.Body(), &errorResponse); err != nil {
			c.logError(err)
			return nil, err
		}

		c.logError(errors.New("read node request rejected"))
		return nil, errors.New("read node request rejected")
	}

	if resp.StatusCode() >= 500 {
		return nil, errors.New(fasthttp.StatusMessage(resp.StatusCode()))
	}

	if resp.StatusCode() != 200 {
		return nil, errors.New("unexpected status code: " + strconv.Itoa(resp.StatusCode()))
	}

	var response Response
	response.Data = &NS{}
	if err = json.Unmarshal(resp.Body(), &response); err != nil {
		c.logError(err)
		return nil, err
	}

	ns, ok := response.Data.(*NS)
	if !ok || ns == nil {
		return nil, errors.New("invalid namespace response")
	}
	return ns, nil
}

func (c *client) broadcast(id string, version uint32, typ Type, data []byte, additionalData, cipherData *[]byte, senderAddress, recipientAddress string, sign []byte, fee uint64, commit, sync bool) (*BroadcastResponse, error) {
	if !typ.IsValid() {
		c.logError(errors.New("invalid type"))
		return nil, errors.New("invalid type")
	}

	broadcast := &Broadcast{
		ID:             id,
		Version:        version,
		Type:           typ,
		SenderAddr:     senderAddress,
		RecipientAddr:  recipientAddress,
		Data:           data,
		AdditionalData: additionalData,
		CipherData:     cipherData,
		Sign:           sign,
		Fee:            fee,
	}

	req, err := broadcast.ToRequest()
	if err != nil {
		c.logError(err)
		return nil, err
	}
	defer fasthttp.ReleaseRequest(req)

	c.copyHeaders(req)
	req.Header.Set("Content-Type", "application/json")

	uri := fasthttp.AcquireURI()
	defer fasthttp.ReleaseURI(uri)
	uri.SetScheme(c.parsedAddr()[0])
	uri.SetHost(c.parsedAddr()[1])
	uri.SetPath(broadcast.URI(commit, sync))

	req.SetURI(uri)

	resp := fasthttp.AcquireResponse()
	defer fasthttp.ReleaseResponse(resp)

	req.Header.SetMethod(fasthttp.MethodPost)
	if err = c.do(req, resp); err != nil {
		c.logError(err)
		return nil, err
	}

	if resp.StatusCode() >= 400 && resp.StatusCode() < 500 {
		var errorResponse Response
		if err := json.Unmarshal(resp.Body(), &errorResponse); err != nil {
			c.logError(err)
			return nil, err
		}

		c.logError(errors.New("read node request rejected"))
		return nil, errors.New("read node request rejected")
	}

	if resp.StatusCode() >= 500 {
		return nil, errors.New(fasthttp.StatusMessage(resp.StatusCode()))
	}

	if resp.StatusCode() != 201 {
		return nil, errors.New("unexpected status code: " + strconv.Itoa(resp.StatusCode()))
	}

	var broadcastResponse BroadcastResponse

	if err = json.Unmarshal(resp.Body(), &broadcastResponse); err != nil {
		return nil, err
	}

	return &broadcastResponse, nil
}

func (c *client) parsedAddr() []string {
	uri, _ := url.Parse(c.address)
	return []string{uri.Scheme, uri.Host}
}
