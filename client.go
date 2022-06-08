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
	"encoding/json"
	"errors"
	"fmt"
	"github.com/fasthttp/websocket"
	"log"
	"net/http"
	"net/url"
	"sync"
	"time"
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
	pingPeriod = (pongWait * 9) / 10
)

type typ int

const (
	mclose  typ = 0
	ping    typ = 1
	message typ = 2
)

// Client TCABCI Read Node Websocket Client
type Client interface {
	Start() error
	Stop() error
	SetListenCallback(func(transaction Transaction))
	Subscribe(addresses []string) error
	Unsubscribe() error
}

type client struct {
	conn                *websocket.Conn
	address             string
	wsURL               *url.URL
	wsHeaders           http.Header
	ctx                 context.Context
	mainCtx             context.Context
	mainCtxCancel       context.CancelFunc
	version             string
	subscribed          bool
	connected           bool
	started             bool
	handshakeTimeout    time.Duration
	listenCallback      func(transaction Transaction)
	subscribedAddresses map[string]bool
	listenCtx           context.Context
	listenCtxCancel     context.CancelFunc
	mut                 sync.RWMutex
	sendBuf             chan sendMsg
	pingTicker          *time.Ticker
	dialer              *websocket.Dialer
}

type sendMsg struct {
	typ        typ
	messageTyp int
	msg        []byte
}

// NewClient make ws client
func NewClient(address string) (Client, error) {
	c := new(client)
	c.version = "v0.1.0"
	c.address = address

	wsURL, err := url.Parse(c.address)
	if err != nil {
		return nil, err
	}

	if wsURL.Scheme != "wss" && wsURL.Scheme != "ws" {
		return nil, errors.New("invalid websocket url")
	}
	c.wsURL = wsURL

	c.wsHeaders = http.Header{}
	c.wsHeaders.Set("Client", fmt.Sprintf("tcabaci-read-go-client%s", c.version))

	c.ctx = context.Background()
	c.subscribedAddresses = make(map[string]bool)
	c.subscribed = false
	c.connected = false
	c.handshakeTimeout = 3 * time.Second
	c.dialer = &websocket.Dialer{
		Proxy:            http.ProxyFromEnvironment,
		HandshakeTimeout: c.handshakeTimeout,
		ReadBufferSize:   512 * 1024,
		WriteBufferSize:  512 * 1024,
		Jar:              nil,
	}

	_ = c.Start()
	//
	go func() {
		_, _ = c.connect(false)
	}()
	time.AfterFunc(c.handshakeTimeout, func() {
		go c.listen()
		go c.listenWrite()
		go c.ping()
	})

	return c, nil
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

// Start contexts and ws client and client retriever
func (c *client) Start() error {
	if c.getStarted() {
		return ErrAlreadyStarted
	}
	c.mut.Lock()
	c.mainCtx, c.mainCtxCancel = context.WithCancel(c.ctx)
	c.listenCtx, c.listenCtxCancel = context.WithCancel(c.mainCtx)
	c.sendBuf = make(chan sendMsg)
	c.pingTicker = time.NewTicker(pingPeriod)
	c.mut.Unlock()
	c.setStarted(true)

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
			conn, _, err := c.dialer.Dial(c.wsURL.String(),
				c.wsHeaders)

			c.mut.Lock()
			c.conn = conn
			c.connected = err == nil
			c.mut.Unlock()

			if err == nil {
				if reconnect || (!currentState && c.getSubscribed()) {
					_ = c.unsubscribe(false)
					_ = c.subscribe(true, nil)
				}
			} else {
				continue
			}

			return c.getConn(), nil
		}
	}
}

// readMessage ...
// Reference: https://github.com/recws-org/recws/blob/master/recws.go
func (c *client) readMessage() (messageType int, readingMessage []byte, err error) {
	err = ErrNoConnected
	if c.isConnected() {
		messageType, readingMessage, err = c.conn.ReadMessage()
		if websocket.IsCloseError(err, websocket.CloseNormalClosure) {
			c.closeWS()
			return messageType, readingMessage, err
		}
		if err != nil {
			c.closeWS()
		} else {
			//
		}
	}
	return
}

func (c *client) write(sm sendMsg) error {
	ctx, cancel := context.WithTimeout(c.mainCtx, time.Millisecond*15)
	defer cancel()

	for {
		select {
		case c.sendBuf <- sm:
			return nil
		case <-ctx.Done():
			return errors.New("context canceled")
		}
	}
}

func (c *client) listenWrite() {
	for buf := range c.sendBuf {
		if !c.isConnected() {
			continue
		}
		c.mut.Lock()
		var err error
		switch buf.typ {
		case ping:
			err = c.conn.WriteControl(buf.messageTyp, buf.msg, time.Now().Add(pingPeriod/2))
			break
		case message, mclose:
			err = c.conn.WriteMessage(buf.messageTyp, buf.msg)
			break
		default:
			err = nil
		}
		c.mut.Unlock()
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
		go func() {
			select {
			case <-c.listenCtx.Done():
				listening = false
				break
			}
		}()
		if !c.isConnected() {
			time.Sleep(time.Second * 1)
			continue
		}

		messageType, readingMessage, err := c.readMessage()
		if websocket.IsCloseError(err, websocket.CloseNormalClosure) {
			_ = c.Stop()
			return
		}
		if err != nil {
			c.closeWS()
			continue
		}

		switch messageType {
		case websocket.TextMessage:
			if json.Valid(readingMessage) {
				var transaction Transaction
				if err := json.Unmarshal(readingMessage, &transaction); err == nil {
					c.listenCallback(transaction)
				}
			} else {
				//
				//
			}
		}
	}
}

func (c *client) ping() {
	defer c.pingTicker.Stop()
	for {
		select {
		case <-c.pingTicker.C:
			if !c.isConnected() {
				_, err := c.connect(false)
				if err != nil {
					continue
				}
			} else {

				if err := c.write(sendMsg{
					typ:        ping,
					messageTyp: websocket.PingMessage,
					msg:        []byte{},
				}); err != nil {
					if websocket.IsCloseError(err, websocket.CloseNormalClosure) {
						_ = c.Stop()
						return
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
}

func (c *client) closeWS() {
	if conn := c.getConn(); conn != nil {
		_ = c.write(sendMsg{
			typ:        mclose,
			messageTyp: websocket.CloseMessage,
			msg:        websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""),
		})
		_ = conn.Close()
	}
	c.connected = false
	c.conn = nil
}

// SetListenCallback Callback that will be called when the WS client captures a
// transaction event
func (c *client) SetListenCallback(fn func(transaction Transaction)) {
	c.listenCallback = fn
}

// Subscribe to given addresses
func (c *client) Subscribe(addresses []string) error {
	return c.subscribe(false, addresses)
}

func (c *client) subscribe(already bool, addresses []string) error {
	tAddresses := make([]string, 0)
	subscribedAddress := c.getSubscribedAddress()
	if already {
		if len(subscribedAddress) <= 0 {
			return nil
		}
		for address := range subscribedAddress {
			tAddresses = append(tAddresses, address)
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
	}

	if len(tAddresses) == 0 {
		return nil
	}

	subscribeMessage := Message{
		IsWeb: false,
		Type:  Subscribe,
		Addrs: tAddresses,
	}

	b, err := json.Marshal(subscribeMessage)
	if err != nil {
		return err
	}

	c.sendBuf <- sendMsg{
		typ:        message,
		messageTyp: websocket.TextMessage,
		msg:        b,
	}

	tmp := make(map[string]bool)
	for i := 0; i < len(tAddresses); i++ {
		tmp[tAddresses[i]] = true
	}
	c.setSubscribedAddress(tmp)
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
