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
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"runtime"
	"strconv"
	"sync"
	"time"

	"github.com/fasthttp/websocket"
	"github.com/valyala/fasthttp"
)

// ErrNoConnected ...
// Reference: https://github.com/recws-org/recws/blob/master/recws.go
var ErrNoConnected = errors.New("websocket: not connected")
var ErrAlreadyStarted = errors.New("already started")
var ErrNotStarted = errors.New("not started yet")

const (
	retryingSecond   = 25
	retryingInterval = time.Second * retryingSecond
	// Time allowed to read the next pong message from the peer.
	pongWait = 10 * time.Second
	// Send pings to peer with this period. Must be less than pongWait.
	pingPeriod       = (pongWait * 9) / 10
	HandshakeTimeout = 7 * time.Second
	writeTimeout     = 15 * time.Millisecond
)

type typ int

const (
	mclose  typ = 0
	ping    typ = 1
	message typ = 2
)

// Client TCABCI Read Node Websocket Client
type Client interface {
	SetLogger(l Logger) Client
	SetVerbose(verbose bool) (Client, error)
	Start() error
	Stop() error
	SetListenCallback(func(transaction *Transaction))
	Subscribe(addresses []string, signedDatas map[string]string, txTypes ...Type) error
	Unsubscribe() error
	Write(b []byte) error
	LastBlock(chainName, chainVersion *string) (*LastBlock, error)
	Tx(id string, signature string, chainName, chainVersion *string) (*Transaction, error)
	TxSummary(summary *Summary) (lastBlockHeight uint64, lastTransaction *Transaction, totalCount uint64, err error)
	TxSearch(search *Search) (txs []*Transaction, totalCount uint64, err error)
	Broadcast(id string, version uint32, typ Type, data []byte, senderAddress, recipientAddress string, sign []byte, fee uint64) (*BroadcastResponse, error)
	BroadcastSync(id string, version uint32, typ Type, data []byte, senderAddress, recipientAddress string, sign []byte, fee uint64) (*BroadcastResponse, error)
	BroadcastCommit(id string, version uint32, typ Type, data []byte, senderAddress, recipientAddress string, sign []byte, fee uint64) (*BroadcastResponse, error)
}

type client struct {
	ctx                   context.Context
	mainCtx               context.Context
	mainCtxCancel         context.CancelFunc
	conn                  *websocket.Conn
	lgr                   Logger
	verbose               bool
	address               string
	wsAddress             string
	chainName             string
	chainVersion          string
	url                   *url.URL
	wsURL                 *url.URL
	headers               fasthttp.RequestHeader
	wsHeaders             fasthttp.RequestHeader
	version               string
	subscribed            bool
	connected             bool
	started               bool
	handshakeTimeout      time.Duration
	listenCallback        func(transaction *Transaction)
	subscribedAddresses   map[string]bool
	subscribedSignedDatas map[string]string
	listenCtx             context.Context
	listenCtxCancel       context.CancelFunc
	mut                   sync.RWMutex
	sendBuf               chan sendMsg
	pingTicker            *time.Ticker
	dialer                *websocket.Dialer
	receivedCh            chan Received
	httpClient            *fasthttp.Client
}

type sendMsg struct {
	typ        typ
	messageTyp int
	msg        []byte
}

// NewClient make ws client
func NewClient(address string, wsAddress string, chainName, chainVersion string) (Client, error) {
	return newClient(context.Background(), address, wsAddress, chainName, chainVersion)
}

// NewClientContext make ws client with context
func NewClientContext(ctx context.Context, address string, wsAddress string, chainName, chainVersion string) (Client, error) {
	return newClient(ctx, address, wsAddress, chainName, chainVersion)
}

func newClient(ctx context.Context, address string, wsAddress string, chainName, chainVersion string) (Client, error) {
	aURL, err := url.Parse(address)
	if err != nil {
		return nil, err
	}

	if aURL.Scheme != "https" && aURL.Scheme != "http" {
		return nil, errors.New("invalid address")
	}

	wsURL, err := url.Parse(wsAddress)
	if err != nil {
		return nil, err
	}

	if wsURL.Scheme != "wss" && wsURL.Scheme != "ws" {
		return nil, errors.New("invalid websocket address")
	}

	pool, err := x509.SystemCertPool()
	if err != nil {
		return nil, err
	}

	maxIdleConnDuration, _ := time.ParseDuration("1h")

	c := &client{
		ctx:                 ctx,
		version:             "v1.6.0",
		lgr:                 NewLogger(ctx),
		address:             address,
		wsAddress:           wsAddress,
		chainName:           chainName,
		chainVersion:        chainVersion,
		url:                 aURL,
		wsURL:               wsURL,
		subscribedAddresses: make(map[string]bool),
		handshakeTimeout:    HandshakeTimeout,
		dialer: &websocket.Dialer{
			TLSClientConfig: &tls.Config{
				RootCAs:               pool,
				InsecureSkipVerify:    false,
				VerifyPeerCertificate: verifyPeer,
			},
			HandshakeTimeout:  HandshakeTimeout,
			ReadBufferSize:    5 * 1024 * 1024,
			WriteBufferSize:   5 * 1024 * 1024,
			EnableCompression: true,
		},
		sendBuf:    make(chan sendMsg, runtime.NumCPU()),
		receivedCh: make(chan Received, runtime.NumCPU()),
		httpClient: &fasthttp.Client{
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
		},
	}

	trn, err := newTransport(pool, false)
	if err != nil {
		return nil, err
	}

	c.httpClient.Transport = trn

	c.headers.Set("Client", fmt.Sprintf("tcabaci-read-go-client-%s", c.version))
	c.headers.Set("User-Agent", fmt.Sprintf("tcabaci-read-go-client-%s", c.version))
	c.wsHeaders.Set("Client", fmt.Sprintf("tcabaci-read-go-client-%s", c.version))
	c.wsHeaders.Set("User-Agent", fmt.Sprintf("tcabaci-read-go-client-%s", c.version))

	return c, nil
}

func (c *client) parsedAddr() []string {
	uri, _ := url.Parse(c.address)
	return []string{uri.Scheme, uri.Host}
}

func (c *client) setStarted(b bool) {
	c.mut.Lock()
	c.started = b
	c.mut.Unlock()
}

func (c *client) getStarted() bool {
	return c.started
}

func (c *client) setConnected(b bool) {
	c.mut.Lock()
	c.connected = b
	c.mut.Unlock()
}

func (c *client) isConnected() bool {
	return c.connected
}

func (c *client) setSubscribed(b bool) {
	c.mut.Lock()
	c.subscribed = b
	c.mut.Unlock()
}

func (c *client) getSubscribed() bool {
	return c.subscribed
}

func (c *client) getConn() *websocket.Conn {
	return c.conn
}

func (c *client) getSubscribedAddress() map[string]bool {
	return c.subscribedAddresses
}

func (c *client) setSubscribedAddress(v map[string]bool) {
	c.mut.Lock()
	c.subscribedAddresses = v
	c.mut.Unlock()
}

func (c *client) getSubscribedSignedDatas() map[string]string {
	return c.subscribedSignedDatas
}

func (c *client) setSubscribedSignedDatas(v map[string]string) {
	c.mut.Lock()
	c.subscribedSignedDatas = v
	c.mut.Unlock()
}

func (c *client) SetLogger(l Logger) Client {
	c.lgr = l

	return c
}

func (c *client) SetVerbose(v bool) (Client, error) {
	c.verbose = v

	pool, err := x509.SystemCertPool()
	if err != nil {
		return nil, err
	}

	trn, err := newTransport(pool, v)
	if err != nil {
		return nil, err
	}
	c.httpClient.Transport = trn

	return c, nil
}

// Start contexts and ws client and client retriever
func (c *client) Start() error {
	if c.getStarted() {
		return ErrAlreadyStarted
	}
	c.mut.Lock()
	c.mainCtx, c.mainCtxCancel = context.WithCancel(c.ctx)
	c.listenCtx, c.listenCtxCancel = context.WithCancel(c.mainCtx)
	c.sendBuf = make(chan sendMsg, runtime.NumCPU())
	c.receivedCh = make(chan Received, runtime.NumCPU())
	c.pingTicker = time.NewTicker(pingPeriod)
	c.mut.Unlock()
	c.setStarted(true)

	//
	go func() {
		_, _ = c.connect(false)
	}()
	go c.listen()
	go c.listenWrite()
	go c.ping()

	return nil
}

// Stop ws client and ws contexts
func (c *client) Stop() error {
	if !c.getStarted() {
		return ErrNotStarted
	}
	c.listenCtxCancel()
	c.mainCtxCancel()
	if c.pingTicker != nil {
		c.pingTicker.Stop()
	}
	c.closeWS()
	//
	c.setSubscribed(false)
	c.setStarted(false)
	c.setConnected(false)

	return nil
}

func (c *client) connect(reconnect bool) (*websocket.Conn, error) {
	if c.getConn() != nil {
		return c.getConn(), nil
	}

	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

	for ; ; <-ticker.C {
		select {
		case <-c.mainCtx.Done():
			return nil, errors.New("main context canceled")
		default:
			currentState := c.isConnected()
			headers := http.Header{}
			for kk, vv := range c.wsHeaders.All() {
				for _, v := range vv {
					headers.Add(string(kk), strconv.Itoa(int(v)))
				}
			}
			conn, response, err := c.dialer.DialContext(c.ctx, c.wsURL.String(), headers)
			c.mut.Lock()
			c.conn = conn
			c.connected = err == nil
			if response.StatusCode >= 400 {
				c.connected = false
				err = errors.New(response.Status)
			}
			c.mut.Unlock()

			if err == nil {
				if reconnect || (!currentState && c.getSubscribed()) {
					_ = c.unsubscribe(false)
					_ = c.subscribe(true, nil, nil)
				}
			} else {
				c.lgr.Error(err)
				continue
			}

			return c.getConn(), nil
		}
	}
}

// readMessage ...
// Reference: https://github.com/recws-org/recws/blob/master/recws.go
func (c *client) readMessage() <-chan Received {
	go func() {
		mt, rm, err := c.getConn().ReadMessage()
		if websocket.IsCloseError(err, websocket.CloseNormalClosure) {
			_ = c.Stop()
			c.receivedCh <- Received{
				MessageType:    mt,
				ReadingMessage: rm,
				Err:            err,
			}
			return
		}
		if err != nil {
			c.lgr.Error(err)
			c.closeWS()
		}
		c.receivedCh <- Received{
			MessageType:    mt,
			ReadingMessage: rm,
			Err:            err,
		}
	}()
	return c.receivedCh
}

func (c *client) Write(b []byte) error {
	return c.write(sendMsg{
		typ:        message,
		messageTyp: websocket.TextMessage,
		msg:        b,
	})
}

func (c *client) write(sm sendMsg) (err error) {
	_, ok := c.mainCtx.Deadline()
	if ok {
		return errors.New("deadline exceeded")
	}

	ctx, cancel := context.WithTimeout(c.mainCtx, time.Millisecond*15)
	writing := false
	for writing {
		select {
		case c.sendBuf <- sm:
			err = nil
			writing = false
		case <-ctx.Done():
			err = errors.New("context deadline exceeded")
			c.lgr.Error(err)
			writing = false
		}
	}
	cancel()
	return err
}

func (c *client) listenWrite() {
	for buf := range c.sendBuf {
		if !c.isConnected() {
			continue
		}
		conn := c.getConn()
		if conn == nil {
			continue
		}
		var err error

		switch buf.typ {
		case ping:
			c.mut.Lock()
			err = conn.WriteControl(buf.messageTyp, buf.msg, time.Now().Add(pingPeriod/2))
			c.mut.Unlock()
			if err != nil {
				c.lgr.Error(err)
			}
			break
		case message, mclose:
			c.mut.Lock()
			err = conn.WriteMessage(buf.messageTyp, buf.msg)
			c.mut.Unlock()
			if err != nil {
				c.lgr.Error(err)
			}
			break
		default:
			err = nil
		}
		if websocket.IsCloseError(err, websocket.CloseNormalClosure) {
			_ = c.Stop()
			return
		}
		if err != nil {
			log.Println(fmt.Errorf("write message error %v", err))
			c.closeWS()
		}
	}
}

func (c *client) listen() {
	listening := true
	for listening {
		if !c.isConnected() {
			time.Sleep(time.Second * 1)
			continue
		}
		select {
		case <-c.listenCtx.Done():
			listening = false
			break
		case received := <-c.readMessage():
			if received.Err != nil {
				c.lgr.Error(received.Err)
			}

			if websocket.IsCloseError(received.Err, websocket.CloseNormalClosure) {
				listening = false
				return
			}
			if received.Err != nil {
				c.closeWS()
				continue
			}

			if c.listenCallback != nil {
				switch received.MessageType {
				case websocket.TextMessage:
					if !json.Valid(received.ReadingMessage) {
						continue
					}

					var transaction Transaction
					if err := json.Unmarshal(received.ReadingMessage, &transaction); err == nil {
						c.lgr.Error(err)
						c.listenCallback(&transaction)
					}
				}
			}
		}
	}
}

func (c *client) ping() {
	writing := true
	for writing {
		select {
		case <-c.pingTicker.C:
			if !c.isConnected() {
				_, err := c.connect(false)
				if err != nil {
					c.lgr.Error(err)
					continue
				}
			} else {
				if err := c.write(sendMsg{
					typ:        ping,
					messageTyp: websocket.PingMessage,
					msg:        []byte{},
				}); err != nil {
					c.lgr.Error(err)

					if websocket.IsCloseError(err, websocket.CloseNormalClosure) {
						_ = c.Stop()
						writing = false
						break
					}
					if err != nil {
						c.closeWS()
						continue
					}
				}

			}
		case <-c.mainCtx.Done():
			c.pingTicker.Stop()
			return
		}
	}
	c.pingTicker.Stop()
}

func (c *client) closeWS() {
	if conn := c.getConn(); conn != nil {
		if err := c.write(sendMsg{
			typ:        mclose,
			messageTyp: websocket.CloseMessage,
			msg:        websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""),
		}); err != nil {
			c.lgr.Error(err)
		}
		if err := conn.Close(); err != nil {
			c.lgr.Error(err)
		}
	}
	c.connected = false
	c.conn = nil
}

// SetListenCallback Callback that will be called when the WS client captures a
// transaction event
func (c *client) SetListenCallback(fn func(transaction *Transaction)) {
	c.listenCallback = fn
}

// Subscribe to given addresses
func (c *client) Subscribe(addresses []string, signedDatas map[string]string, txTypes ...Type) error {
	return c.subscribe(false, addresses, signedDatas, txTypes...)
}

func (c *client) subscribe(already bool, addresses []string, signedDatas map[string]string, txTypes ...Type) error {
	if len(txTypes) > len(TypesSlice) {
		return errors.New("invalid tx types")
	}

	tAddresses := make([]string, 0)
	tSignedDatas := make(map[string]string)
	subscribedAddress := c.getSubscribedAddress()
	subscribedSignedDatas := c.getSubscribedSignedDatas()
	if already {
		if len(subscribedAddress) <= 0 {
			return nil
		}
		for address := range subscribedAddress {
			tAddresses = append(tAddresses, address)
		}

		if len(subscribedSignedDatas) == 0 {
			return nil
		}

		for kk, vv := range subscribedSignedDatas {
			tSignedDatas[kk] = vv
		}
	} else {
		if len(addresses) == 0 {
			return errors.New("addresses count is zero")
		}

		tAddresses = addresses

		if len(subscribedAddress) > 0 {
			newAddress := make([]string, 0)
			for i := 0; i < len(addresses); i++ {
				if _, ok := subscribedAddress[addresses[i]]; !ok {
					newAddress = append(newAddress, addresses[i])
				}
			}

			tAddresses = newAddress
		}

		tSignedDatas = signedDatas
	}

	if len(tAddresses) == 0 || len(tSignedDatas) == 0 {
		return nil
	}

	subscribeMessage := Message{
		IsWeb:       false,
		Type:        Subscribe,
		Addrs:       tAddresses,
		SignedAddrs: tSignedDatas,
		TXTypes:     txTypes,
	}

	b, err := json.Marshal(subscribeMessage)
	if err != nil {
		c.lgr.Error(err)
		return err
	}

	go func() {
		c.sendBuf <- sendMsg{
			typ:        message,
			messageTyp: websocket.TextMessage,
			msg:        b,
		}
	}()

	tmp := make(map[string]bool)
	for i := 0; i < len(tAddresses); i++ {
		tmp[tAddresses[i]] = true
	}
	c.setSubscribedAddress(tmp)
	c.setSubscribedSignedDatas(tSignedDatas)
	c.setSubscribed(true)

	return nil
}

// Unsubscribe to given addresses
func (c *client) Unsubscribe() error {
	return c.unsubscribe(false)
}

func (c *client) unsubscribe(_ bool) error {
	if !c.getSubscribed() {
		return errors.New("client has not yet subscribed")
	}

	subscribedAddress := c.getSubscribedAddress()
	if len(subscribedAddress) <= 0 {
		return nil
	}

	addresses := make([]string, 0)

	for address := range subscribedAddress {
		addresses = append(addresses, address)
	}

	unsubscribeMessage := Message{
		Type:  Unsubscribe,
		Addrs: addresses,
	}

	b, err := json.Marshal(unsubscribeMessage)
	if err != nil {
		c.lgr.Error(err)
		return err
	}

	c.sendBuf <- sendMsg{
		typ:        message,
		messageTyp: websocket.TextMessage,
		msg:        b,
	}

	c.setSubscribed(false)

	return nil
}

// LastBlock fetch last block in blockchain network
func (c *client) LastBlock(chainName, chainVersion *string) (*LastBlock, error) {
	var lastBlock LastBlock

	u := "limit=1&offset=0"
	if chainName != nil && chainVersion != nil {
		u += "&chain_name=" + *chainName + "&chain_version=" + *chainVersion
	} else {
		u += "&chain_name=" + c.chainName + "&chain_version=" + c.chainVersion
	}

	req := fasthttp.AcquireRequest()
	defer fasthttp.ReleaseRequest(req)
	c.headers.CopyTo(&req.Header)
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
	if err := c.httpClient.Do(req, resp); err != nil {
		c.lgr.Error(err)
		return nil, err
	}

	if resp.StatusCode() != 200 {
		if resp.StatusCode() < 500 {
			var errorResponse Response
			if err := json.Unmarshal(resp.Body(), &errorResponse); err != nil {
				c.lgr.Error(err)
				return nil, err
			}

			c.lgr.Error(errors.New(StringOR(errorResponse.GetMessage(), errorResponse.GetMessage())))
			return nil, errors.New(StringOR(errorResponse.GetMessage(), errorResponse.GetMessage()))
		}

		return nil, errors.New(fasthttp.StatusMessage(resp.StatusCode()))
	}

	if err := json.Unmarshal(resp.Body(), &lastBlock); err != nil {
		c.lgr.Error(err)
		return nil, err
	}

	return &lastBlock, nil
}

func (c *client) Tx(id string, signature string, chainName, chainVersion *string) (*Transaction, error) {
	if id == "" {
		return nil, errors.New("invalid tx id")
	}

	var txResponse Response
	txResponse.Data = &Transaction{}

	u := ""
	if chainName != nil && chainVersion != nil {
		u += "chain_name=" + *chainName + "&chain_version=" + *chainVersion
	} else {
		u += "chain_name=" + c.chainName + "&chain_version=" + c.chainVersion
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

	c.headers.CopyTo(&req.Header)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Signature", signature)

	resp := fasthttp.AcquireResponse()
	defer fasthttp.ReleaseResponse(resp)
	if err := c.httpClient.Do(req, resp); err != nil {
		c.lgr.Error(err)
		return nil, err
	}

	if resp.StatusCode() != 200 {
		if resp.StatusCode() < 500 {
			var errorResponse Response
			if err := json.Unmarshal(resp.Body(), &errorResponse); err != nil {
				c.lgr.Error(err)
				return nil, err
			}

			c.lgr.Error(errors.New(StringOR(errorResponse.GetMessage(), errorResponse.GetMessage())))
			return nil, errors.New(StringOR(errorResponse.GetMessage(), errorResponse.GetMessage()))
		}

		return nil, errors.New(fasthttp.StatusMessage(resp.StatusCode()))
	}

	if err := json.Unmarshal(resp.Body(), &txResponse); err != nil {
		c.lgr.Error(err)
		return nil, err
	}

	return txResponse.GetData().(*Transaction), nil
}

// TxSummary fetch summary with given parameters
func (c *client) TxSummary(summary *Summary) (lastBlockHeight uint64, lastTransaction *Transaction, totalCount uint64, err error) {
	if !summary.IsValid() {
		err = errors.New("invalid parameters")
		return 0, nil, 0, err
	}

	if summary.ChainName == nil {
		summary.ChainName = &c.chainName
	}

	if summary.ChainVersion == nil {
		summary.ChainVersion = &c.chainVersion
	}

	req, err := summary.ToRequest()
	if err != nil {
		c.lgr.Error(err)
		return 0, nil, 0, err
	}
	defer fasthttp.ReleaseRequest(req)

	uri := fasthttp.AcquireURI()
	defer fasthttp.ReleaseURI(uri)
	uri.SetScheme(c.parsedAddr()[0])
	uri.SetHost(c.parsedAddr()[1])
	uri.SetPath(summary.URI())

	req.SetURI(uri)

	c.headers.CopyTo(&req.Header)
	req.Header.Set("Content-Type", "application/json")

	var summaryResponse SummaryResponse

	resp := fasthttp.AcquireResponse()
	defer fasthttp.ReleaseResponse(resp)

	req.Header.SetMethod(fasthttp.MethodPost)
	if err = c.httpClient.Do(req, resp); err != nil {
		c.lgr.Error(err)
		return 0, nil, 0, err
	}

	if resp.StatusCode() != 200 {
		if resp.StatusCode() < 500 {
			var errorResponse Response
			if err = json.Unmarshal(resp.Body(), &errorResponse); err != nil {
				c.lgr.Error(err)
				return 0, nil, 0, err
			}

			c.lgr.Error(errors.New(StringOR(errorResponse.GetMessage(), errorResponse.GetMessage())))
			return 0, nil, 0, errors.New(StringOR(errorResponse.GetMessage(), errorResponse.GetMessage()))
		}

		return 0, nil, 0, errors.New(fasthttp.StatusMessage(resp.StatusCode()))
	}

	if err = json.Unmarshal(resp.Body(), &summaryResponse); err != nil {
		c.lgr.Error(err)
		return 0, nil, 0, err
	}

	return summaryResponse.Data.LastBlockHeight, summaryResponse.Data.LastTransaction, summaryResponse.TotalCount, nil
}

// TxSearch search with given parameters
func (c *client) TxSearch(search *Search) (txs []*Transaction, totalCount uint64, err error) {
	if !search.IsValid() {
		err = errors.New("invalid parameters")
		c.lgr.Error(err)
		return nil, 0, err
	}

	if search.ChainName == nil {
		search.ChainName = &c.chainName
	}

	if search.ChainVersion == nil {
		search.ChainVersion = &c.chainVersion
	}

	req, err := search.ToRequest()
	if err != nil {
		c.lgr.Error(err)
		return nil, 0, err
	}
	defer fasthttp.ReleaseRequest(req)

	c.headers.CopyTo(&req.Header)
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
	if err = c.httpClient.Do(req, resp); err != nil {
		c.lgr.Error(err)
		return nil, 0, err
	}

	if resp.StatusCode() != 200 {
		if resp.StatusCode() < 500 {
			var errorResponse Response
			if err = json.Unmarshal(resp.Body(), &errorResponse); err != nil {
				c.lgr.Error(err)
				return nil, 0, err
			}

			c.lgr.Error(errors.New(StringOR(errorResponse.GetMessage(), errorResponse.GetMessage())))
			return nil, 0, errors.New(StringOR(errorResponse.GetMessage(), errorResponse.GetMessage()))
		}

		return nil, 0, errors.New(fasthttp.StatusMessage(resp.StatusCode()))
	}

	if err = json.Unmarshal(resp.Body(), &searchResponse); err != nil {
		c.lgr.Error(err)
		return nil, 0, err
	}

	return searchResponse.TXS, searchResponse.TotalCount, nil
}

func (c *client) Broadcast(id string, version uint32, typ Type, data []byte, senderAddress, recipientAddress string, sign []byte, fee uint64) (*BroadcastResponse, error) {
	resp, err := c.broadcast(id, version, typ, data, senderAddress, recipientAddress, sign, fee, false, false)
	if err != nil {
		c.lgr.Error(err)
		return nil, err
	}

	return resp, nil
}

func (c *client) BroadcastSync(id string, version uint32, typ Type, data []byte, senderAddress, recipientAddress string, sign []byte, fee uint64) (*BroadcastResponse, error) {
	resp, err := c.broadcast(id, version, typ, data, senderAddress, recipientAddress, sign, fee, false, true)
	if err != nil {
		c.lgr.Error(err)
		return nil, err
	}

	return resp, nil
}

func (c *client) BroadcastCommit(id string, version uint32, typ Type, data []byte, senderAddress, recipientAddress string, sign []byte, fee uint64) (*BroadcastResponse, error) {
	resp, err := c.broadcast(id, version, typ, data, senderAddress, recipientAddress, sign, fee, true, false)
	if err != nil {
		c.lgr.Error(err)
		return nil, err
	}

	return resp, nil
}

// Broadcast ...
func (c *client) broadcast(id string, version uint32, typ Type, data []byte, senderAddress, recipientAddress string, sign []byte, fee uint64, commit, sync bool) (*BroadcastResponse, error) {
	if !typ.IsValid() {
		c.lgr.Error("invalid type")
		return nil, errors.New("invalid type")
	}

	broadcast := &Broadcast{
		ID:            id,
		Version:       version,
		Type:          typ,
		SenderAddr:    senderAddress,
		RecipientAddr: recipientAddress,
		Data:          data,
		Sign:          sign,
		Fee:           fee,
	}

	req, err := broadcast.ToRequest()
	if err != nil {
		c.lgr.Error(err)
		return nil, err
	}
	defer fasthttp.ReleaseRequest(req)

	c.headers.CopyTo(&req.Header)
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
	if err = c.httpClient.Do(req, resp); err != nil {
		c.lgr.Error(err)
		return nil, err
	}

	if resp.StatusCode() != 201 {
		if resp.StatusCode() < 500 {
			var errorResponse Response
			if err = json.Unmarshal(resp.Body(), &errorResponse); err != nil {
				c.lgr.Error(err)
				return nil, err
			}

			c.lgr.Error(errors.New(StringOR(errorResponse.GetMessage(), errorResponse.GetMessage())))
			return nil, errors.New(StringOR(errorResponse.GetMessage(), errorResponse.GetMessage()))
		}

		return nil, errors.New(fasthttp.StatusMessage(resp.StatusCode()))
	}

	var broadcastResponse BroadcastResponse

	if err = json.Unmarshal(resp.Body(), &broadcastResponse); err != nil {
		return nil, err
	}

	return &broadcastResponse, nil
}
