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
	"sync"
	"time"
)

const (
	retryingSecond   = 25
	retryingInterval = time.Second * retryingSecond
	// Time allowed to read the next pong message from the peer.
	pongWait = 10 * time.Second
	// Send pings to peer with this period. Must be less than pongWait.
	pingPeriod = (pongWait * 9) / 10
)

// Client TCABCI Read Node Websocket Client
type Client interface {
	Stop()
	SetListenCallback(func(transaction Transaction))
	Subscribe(addresses []string) error
	Unsubscribe() error
}

type client struct {
	conn                *websocket.Conn
	address             string
	ctx                 context.Context
	mainCtx             context.Context
	mainCtxCancel       context.CancelFunc
	version             string
	subscribed          bool
	check               bool
	retrieverTicker     *time.Ticker
	listenCallback      func(transaction Transaction)
	subscribedAddresses map[string]bool
	listenCtx           context.Context
	listenCtxCancel     context.CancelFunc
	mut                 sync.RWMutex
	sendBuf             chan []byte
}

// NewClient make ws client
func NewClient(address string) Client {
	c := new(client)
	c.version = "v0.1.0"
	c.address = address
	c.ctx = context.Background()
	c.retrieverTicker = time.NewTicker(retryingInterval)
	c.subscribedAddresses = make(map[string]bool)
	c.subscribed = false
	c.check = true
	c.sendBuf = make(chan []byte)

	c.start()

	go c.listen()
	go c.listenWrite()
	go c.ping()

	return c
}

// Start contexts and ws client and client retriever
func (c *client) start() {
	c.mainCtx, c.mainCtxCancel = context.WithCancel(c.ctx)
	c.listenCtx, c.listenCtxCancel = context.WithCancel(c.mainCtx)
}

func (c *client) connect() (*websocket.Conn, error) {
	c.mut.Lock()
	defer c.mut.Unlock()
	if c.conn != nil {
		return c.conn, nil
	}

	var err error
	headers := http.Header{}
	headers.Set("Client", fmt.Sprintf("tcabaci-read-go-client%s", c.version))

	wsURL, err := url.Parse(c.address)
	if err != nil {
		return nil, err
	}

	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

	for ; ; <-ticker.C {
		select {
		case <-c.mainCtx.Done():
			return nil, errors.New("main context canceled")
		default:
			c.conn, _, err = websocket.DefaultDialer.DialContext(c.mainCtx,
				wsURL.String(),
				headers)
			if err != nil {
				c.check = false
				continue
			}

			if !c.check {
				time.AfterFunc(time.Second*1, func() {
					_ = c.unsubscribe(false)
					_ = c.subscribe(true, nil)
				})
			}

			c.check = true

			return c.conn, nil
		}
	}
}

func (c *client) listen() {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-c.listenCtx.Done():
			return
		case <-ticker.C:
			for {
				conn, err := c.connect()
				if err != nil {
					return
				}
				messageType, readingMessage, err := conn.ReadMessage()
				if err != nil {
					c.closeWS()
					break
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
						//fmt.Println("MSG: ", string(readingMessage))
					}
				}
			}
		}
	}
}

func (c *client) ping() {
	ticker := time.NewTicker(pingPeriod)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			_, err := c.connect()
			if err != nil {
				continue
			}
			if err := c.conn.WriteControl(websocket.PingMessage, []byte{}, time.Now().Add(pingPeriod/2)); err != nil {
				c.closeWS()
			}
		case <-c.mainCtx.Done():
			return
		}
	}
}

func (c *client) closeWS() {
	c.mut.Lock()
	if c.conn != nil {
		_ = c.conn.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
		_ = c.conn.Close()
		c.conn = nil
	}
	c.mut.Unlock()
}

// SetListenCallback Callback that will be called when the WS client captures a
// transaction event
func (c *client) SetListenCallback(fn func(transaction Transaction)) {
	c.listenCallback = fn
}

// Stop ws client and ws contexts
func (c *client) Stop() {
	c.retrieverTicker.Stop()

	c.closeWS()

	if c.conn != nil {
		_ = c.conn.Close()
	}
	//
	c.listenCtxCancel()
	c.mainCtxCancel()
	c.subscribed = false
}

func (c *client) write(body []byte) error {
	ctx, cancel := context.WithTimeout(c.mainCtx, time.Millisecond*15)
	defer cancel()

	for {
		select {
		case c.sendBuf <- body:
			return nil
		case <-ctx.Done():
			return errors.New("context canceled")
		}
	}
}

func (c *client) listenWrite() {
	for data := range c.sendBuf {
		conn, err := c.connect()
		if err != nil {
			log.Println(fmt.Errorf("write conn is nil %v", err))
			continue
		}
		if err := conn.WriteMessage(websocket.TextMessage,
			data); err != nil {
			log.Println(fmt.Errorf("write message error %v", err))
		} else {
			//
		}
	}
}

// Subscribe to given addresses
func (c *client) Subscribe(addresses []string) error {
	return c.subscribe(false, addresses)
}

func (c *client) subscribe(already bool, addresses []string) error {
	tAddresses := make([]string, 0)
	if already {
		if len(c.subscribedAddresses) <= 0 {
			return nil
		}
		for address := range c.subscribedAddresses {
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

	c.sendBuf <- b

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

	if len(c.subscribedAddresses) <= 0 {
		return nil
	}

	addresses := make([]string, 0)

	for address := range c.subscribedAddresses {
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

	c.sendBuf <- b

	c.subscribed = false

	return nil
}
