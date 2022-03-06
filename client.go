// Package tcabci_read_go_client
//
// Copyright 2013-2018 TransferChain A.G
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

package tcabci_read_go_client

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/fasthttp/websocket"
	"log"
	"net/http"
	"net/url"
	"time"
)

const (
	retryingSecond   = 25
	retryingInterval = time.Second * retryingSecond
	writeWait        = 10 * time.Second
	// Time allowed to read the next pong message from the peer.
	pongWait = 10 * time.Second
	// Send pings to peer with this period. Must be less than pongWait.
	pingPeriod = (pongWait * 9) / 10
	// Maximum message size allowed from peer.
	maxMessageSize = 5 * 1024 * 1024
	closedMessage  = "CLOSE"
)

// Client TCABCI Read Node Websocket Client
type Client interface {
	Start() error
	Stop()
	SetListenCallback(func(transaction Transaction))
	Subscribe(addresses []string) error
	Unsubscribe() error
	Listen() error
}

type client struct {
	conn                *websocket.Conn
	address             string
	ctx                 context.Context
	mainCtx             context.Context
	mainCtxCancel       context.CancelFunc
	version             string
	started             bool
	connected           bool
	subscribed          bool
	retrieverTicker     *time.Ticker
	listenCallback      func(transaction Transaction)
	subscribedAddresses map[string]bool
	listenCtx           context.Context
	listenCtxCancel     context.CancelFunc
}

// NewClient make ws client
func NewClient(address string) Client {
	c := new(client)
	c.version = "v0.1.0"
	c.address = address
	c.ctx = context.Background()
	c.retrieverTicker = time.NewTicker(retryingInterval)
	c.subscribedAddresses = make(map[string]bool)
	c.connected = false
	c.subscribed = false

	return c
}

func (c *client) retriever() {
	if c.started {
		return
	}
	for {
		select {
		case <-c.retrieverTicker.C:
			_ = c.makeConn()
		}
	}
}

func (c *client) makeConn() error {
	if c.started {
		return errors.New("client already started")
	}
	var err error
	headers := http.Header{}
	headers.Set("X-Client", fmt.Sprintf("tcabaci-read-go-client%s", c.version))

	wsURL, err := url.Parse(c.address)
	if err != nil {
		return err
	}

	c.conn, _, err = websocket.DefaultDialer.DialContext(c.ctx,
		wsURL.String(),
		headers)
	if err != nil {
		c.connected = false
		log.Println("dial: ", err)
		return err
	}

	if !c.connected && c.subscribed {
		_ = c.unsubscribe(false)
		if err := c.subscribe(true, nil); err != nil {
			log.Println(fmt.Sprintf("client not subscribed. It will try again in %d seconds", retryingSecond))
			return err
		}
	}

	c.conn.SetReadLimit(maxMessageSize)
	_ = c.conn.SetReadDeadline(time.Now().Add(pongWait))
	c.conn.SetPongHandler(func(string) error { _ = c.conn.SetReadDeadline(time.Now().Add(pongWait)); return nil })
	c.connected = true

	return nil
}

// SetListenCallback Callback that will be called when the WS client captures a
// transaction event
func (c *client) SetListenCallback(fn func(transaction Transaction)) {
	c.listenCallback = fn
}

// Start contexts and ws client and client retriever
func (c *client) Start() error {
	if c.started {
		return errors.New("client has ben already started")
	}
	c.mainCtx, c.mainCtxCancel = context.WithCancel(c.ctx)
	c.listenCtx, c.listenCtxCancel = context.WithCancel(c.mainCtx)

	if err := c.makeConn(); err != nil {
		return err
	}
	go c.retriever()

	c.started = true

	return nil
}

// Stop ws client and ws contexts
func (c *client) Stop() {
	c.retrieverTicker.Stop()

	if c.connected {
		_ = c.conn.Close()
	}
	//
	c.listenCtxCancel()
	c.mainCtxCancel()
	c.subscribed = false
	c.started = false
}

// Subscribe to given addresses
func (c *client) Subscribe(addresses []string) error {
	return c.subscribe(false, addresses)
}

func (c *client) subscribe(already bool, addresses []string) error {
	if err := c.check(); err != nil {
		return err
	}

	if c.connected && c.subscribed {
		return errors.New("client has been already subscribed")
	}

	tAddresses := make([]string, 0)
	if already {
		for address, _ := range c.subscribedAddresses {
			tAddresses = append(tAddresses, address)
		}
	} else {
		if len(addresses) == 0 {
			return errors.New("addresses count is zero")
		}

		tAddresses = addresses

		if len(c.subscribedAddresses) > 0 {
			newAddress := make([]string, 0)
			for i := 0; i < len(addresses); i++ {
				if _, ok := c.subscribedAddresses[addresses[i]]; !ok {
					newAddress = append(newAddress, addresses[i])
				}
			}

			tAddresses = newAddress
		}
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

	if err := c.conn.WriteMessage(websocket.TextMessage, b); err != nil {
		return err
	}

	for i := 0; i < len(tAddresses); i++ {
		c.subscribedAddresses[tAddresses[i]] = true
	}

	c.subscribed = true

	return nil
}

// Unsubscribe to given addresses
func (c *client) Unsubscribe() error {
	return c.unsubscribe(false)
}

func (c *client) unsubscribe(_ bool) error {
	if !c.subscribed {
		return errors.New("client has not yet subscribed")
	}

	if err := c.check(); err != nil {
		return err
	}

	addresses := make([]string, 0)

	for address, _ := range c.subscribedAddresses {
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

	if err := c.conn.WriteMessage(websocket.TextMessage, b); err != nil {
		return err
	}

	c.subscribed = false

	return nil
}

func (c client) check() error {
	if !c.connected {
		return errors.New("client not connected")
	}

	if c.conn == nil {
		return errors.New("client is not initialized")
	}

	return nil
}

// Listen to given addresses
func (c client) Listen() error {
	if err := c.check(); err != nil {
		return err
	}

	if !c.started {
		return errors.New("client has not been started yet")
	}

	retrying := false
	closed := false
	listening := true
	for listening {
		messageType, readingMessage, err := c.conn.ReadMessage()
		if err != nil {
			return err
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
			}

			if string(readingMessage) == closedMessage {
				listening = false
				closed = true
				continue
			}
		case websocket.PingMessage,
			websocket.PongMessage,
			websocket.BinaryMessage:
			break
		case websocket.CloseMessage:
			listening = false
			closed = true
			continue
		case websocket.CloseTryAgainLater:
			listening = false
			closed = true
			retrying = true
			continue
		default:
			break
		}
	}

	if closed {
		c.Stop()

		if retrying {
			c.started = false
			if err := c.Start(); err != nil {
				return err
			}
			if err := c.Listen(); err != nil {
				return err
			}
		}
	}

	return nil
}
