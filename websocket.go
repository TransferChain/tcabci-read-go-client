package tcabcireadgoclient

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"sort"
	"sync/atomic"
	"time"

	"github.com/fasthttp/websocket"
)

// A session owns its queue and all network goroutines until done closes.
// User callbacks are independent: Stop does not wait for caller-owned work.
type wsSession struct {
	ctx           context.Context
	cancel        context.CancelFunc
	done          chan struct{}
	send          chan []byte
	subscriptions chan struct{}
	queuedBytes   atomic.Int64
}

func (c *client) Start() error {
	c.lifecycle.Lock()
	defer c.lifecycle.Unlock()
	c.mut.Lock()
	defer c.mut.Unlock()
	if c.session != nil {
		select {
		case <-c.session.done:
		default:
			return ErrAlreadyStarted
		}
	}
	if err := c.ctx.Err(); err != nil {
		return err
	}
	ctx, cancel := context.WithCancel(c.ctx)
	s := &wsSession{ctx: ctx, cancel: cancel, done: make(chan struct{}), send: make(chan []byte, 64), subscriptions: make(chan struct{}, 1)}
	c.session = s
	go c.runSession(s)
	return nil
}

func (c *client) Stop() error {
	c.lifecycle.Lock()
	defer c.lifecycle.Unlock()
	c.mut.Lock()
	s := c.session
	if s == nil {
		c.mut.Unlock()
		return &Error{origin: ErrNotStarted, message: ErrNotStarted.Error(), typ: CLIENTErr, code: 3}
	}
	s.cancel()
	c.mut.Unlock()
	<-s.done
	c.mut.Lock()
	c.session = nil
	clear(c.subscribedAddresses)
	clear(c.subscribedSignedData)
	c.subscribedTypes = nil
	c.subscriptionRevision++
	c.mut.Unlock()
	c.httpMu.Lock()
	c.httpClient.CloseIdleConnections()
	c.httpMu.Unlock()
	return nil
}

// Write copies b and queues it for this session; success is not a server ACK.
func (c *client) Write(b []byte) error {
	if len(b) > maxMessageSize {
		return errors.New("websocket message too large")
	}
	c.mut.RLock()
	defer c.mut.RUnlock()
	s := c.session
	if s == nil {
		return ErrNotStarted
	}
	if err := s.ctx.Err(); err != nil {
		return err
	}
	size := int64(len(b))
	for {
		used := s.queuedBytes.Load()
		if used+size > maxMessageSize {
			return errors.New("websocket send byte budget exceeded")
		}
		if s.queuedBytes.CompareAndSwap(used, used+size) {
			break
		}
	}
	owned := bytes.Clone(b)
	timer := time.NewTimer(enqueueTimeout)
	defer timer.Stop()
	select {
	case <-s.ctx.Done():
		clear(owned)
		s.queuedBytes.Add(-size)
		return s.ctx.Err()
	case <-timer.C:
		clear(owned)
		s.queuedBytes.Add(-size)
		return errors.New("websocket send queue full")
	case s.send <- owned:
		return nil
	}
}

func (c *client) WSQuery(typ string, data []byte) error {
	b, err := json.Marshal(Message{IsWeb: false, Type: MessageType(typ), Data: data})
	if err != nil {
		return err
	}
	defer clear(b)
	return c.Write(b)
}

func (c *client) notifySubscriptionLocked() {
	if c.session != nil {
		select {
		case c.session.subscriptions <- struct{}{}:
		default:
		}
	}
}

// Subscribe merges desired addresses. A non-empty txTypes replaces the filter
// for the whole subscription. Changes are sent asynchronously as a snapshot.
func (c *client) Subscribe(addresses []string, signedDatas map[string]string, txTypes ...Type) error {
	if len(addresses) == 0 || len(addresses) > 251 {
		return errors.New("invalid addresses count")
	}
	for _, address := range addresses {
		if address == "" || len(address) > 2048 || signedDatas[address] == "" {
			return errors.New("missing address or signature")
		}
	}
	for _, typ := range txTypes {
		if !typ.IsValid() {
			return errors.New("invalid tx type")
		}
	}
	c.mut.Lock()
	defer c.mut.Unlock()
	if c.session == nil {
		return ErrNotStarted
	}
	if err := c.session.ctx.Err(); err != nil {
		return err
	}
	total := len(c.subscribedAddresses)
	seen := make(map[string]bool, len(addresses))
	for _, address := range addresses {
		if !c.subscribedAddresses[address] && !seen[address] {
			total++
			seen[address] = true
		}
	}
	if total > 251 {
		return errors.New("too many subscribed addresses")
	}
	for _, address := range addresses {
		c.subscribedAddresses[address] = true
		c.subscribedSignedData[address] = signedDatas[address]
	}
	if len(txTypes) > 0 {
		c.subscribedTypes = append([]Type(nil), txTypes...)
	}
	c.subscriptionRevision++
	c.notifySubscriptionLocked()
	return nil
}

func (c *client) Unsubscribe() error {
	c.mut.Lock()
	defer c.mut.Unlock()
	if c.session == nil {
		return ErrNotStarted
	}
	if err := c.session.ctx.Err(); err != nil {
		return err
	}
	if len(c.subscribedAddresses) == 0 {
		return errors.New("client has not yet subscribed")
	}
	clear(c.subscribedAddresses)
	clear(c.subscribedSignedData)
	c.subscribedTypes = nil
	c.subscriptionRevision++
	c.notifySubscriptionLocked()
	return nil
}

func (c *client) runSession(s *wsSession) {
	defer func() {
		s.cancel()
		// Wait for any concurrent enqueue before draining owned buffers.
		c.mut.Lock()
		defer c.mut.Unlock()
		defer close(s.done)
		for {
			select {
			case b := <-s.send:
				s.queuedBytes.Add(-int64(len(b)))
				clear(b)
			default:
				return
			}
		}
	}()
	delay := 100 * time.Millisecond
	for s.ctx.Err() == nil {
		c.callRetrievers()
		c.mut.RLock()
		dialer := *c.dialer
		headers := http.Header{}
		for k, v := range c.wsHeaders.All() {
			headers.Add(string(k), string(v))
		}
		endpoint := c.wsURL.String()
		c.mut.RUnlock()
		conn, resp, err := dialConnection(s.ctx, &dialer, endpoint, headers)
		if err != nil && resp != nil && resp.Body != nil {
			_ = resp.Body.Close()
		}
		if err == nil {
			delay = 100 * time.Millisecond
			err = c.serveConnection(s, conn)
		}
		if s.ctx.Err() != nil || websocket.IsCloseError(err, websocket.CloseNormalClosure) {
			return
		}
		if err != nil {
			c.callErrorCallbacks(err)
		}
		timer := time.NewTimer(delay)
		select {
		case <-s.ctx.Done():
			timer.Stop()
			return
		case <-timer.C:
		}
		if delay < 25*time.Second {
			delay *= 2
			if delay > 25*time.Second {
				delay = 25 * time.Second
			}
		}
	}
}

// The websocket dialer's handshake I/O uses a deadline but does not close
// the socket on context cancellation. Keep cancellation active until it returns.
func dialConnection(ctx context.Context, dialer *websocket.Dialer, endpoint string, headers http.Header) (*websocket.Conn, *http.Response, error) {
	var stopCancel func() bool
	originalDial := dialer.NetDialContext
	dialer.NetDialContext = func(dialCtx context.Context, network, address string) (net.Conn, error) {
		var conn net.Conn
		var err error
		if originalDial != nil {
			conn, err = originalDial(dialCtx, network, address)
		} else {
			conn, err = (&net.Dialer{}).DialContext(dialCtx, network, address)
		}
		if err == nil {
			stopCancel = context.AfterFunc(ctx, func() { _ = conn.Close() })
		}
		return conn, err
	}
	defer func() {
		if stopCancel != nil {
			stopCancel()
		}
	}()
	return dialer.DialContext(ctx, endpoint, headers)
}

func (c *client) serveConnection(s *wsSession, conn *websocket.Conn) error {
	// Cancellation interrupts writes and the reader as well as dialing.
	stopClose := context.AfterFunc(s.ctx, func() { _ = conn.Close() })
	defer stopClose()
	defer conn.Close()
	conn.SetReadLimit(maxMessageSize)
	if err := conn.SetReadDeadline(time.Now().Add(pongWait)); err != nil {
		return err
	}
	conn.SetPongHandler(func(string) error { return conn.SetReadDeadline(time.Now().Add(pongWait)) })
	readDone := make(chan error, 1)
	go func() {
		for {
			typ, reader, err := conn.NextReader()
			var msg []byte
			if err == nil {
				msg, err = io.ReadAll(io.LimitReader(reader, maxMessageSize+1))
				if err == nil && len(msg) > maxMessageSize {
					err = errors.New("websocket message too large")
				}
			}
			if err != nil {
				readDone <- err
				return
			}
			if typ == websocket.TextMessage {
				c.parseIncoming(msg)
			}
		}
	}()
	// Join the reader before reconnecting or completing Stop.
	defer func() { _ = conn.Close(); <-readDone }()
	ticker := time.NewTicker(pingPeriod)
	defer ticker.Stop()
	var previous []string
	var revision uint64
	first := true
	for {
		c.mut.RLock()
		current := c.subscriptionRevision
		c.mut.RUnlock()
		if first || revision != current {
			var err error
			previous, revision, err = c.syncSubscription(conn, previous)
			if err != nil {
				return err
			}
			first = false
		}
		select {
		case <-s.ctx.Done():
			return s.ctx.Err()
		case err := <-readDone:
			// The deferred join consumes the same completion notification.
			readDone <- err
			return err
		case <-s.subscriptions:
		case b := <-s.send:
			err := writeFrame(conn, websocket.TextMessage, b)
			s.queuedBytes.Add(-int64(len(b)))
			clear(b)
			if err != nil {
				return err
			}
		case <-ticker.C:
			if err := conn.WriteControl(websocket.PingMessage, nil, time.Now().Add(writeTimeout)); err != nil {
				return err
			}
		}
	}
}

func writeFrame(conn *websocket.Conn, typ int, b []byte) error {
	if err := conn.SetWriteDeadline(time.Now().Add(writeTimeout)); err != nil {
		return err
	}
	return conn.WriteMessage(typ, b)
}

func (c *client) syncSubscription(conn *websocket.Conn, previous []string) ([]string, uint64, error) {
	c.mut.RLock()
	revision := c.subscriptionRevision
	msg := Message{Type: Subscribe, SignedAddrs: make(map[string]string), TXTypes: append([]Type(nil), c.subscribedTypes...)}
	if c.mode == Subscription {
		for address := range c.subscribedAddresses {
			msg.Addrs = append(msg.Addrs, address)
			msg.SignedAddrs[address] = c.subscribedSignedData[address]
		}
	}
	c.mut.RUnlock()
	sort.Strings(msg.Addrs)
	var removed []string
	for _, address := range previous {
		if _, ok := msg.SignedAddrs[address]; !ok {
			removed = append(removed, address)
		}
	}
	if len(removed) > 0 {
		b, err := json.Marshal(Message{Type: Unsubscribe, Addrs: removed})
		if err != nil {
			return nil, revision, err
		}
		err = writeFrame(conn, websocket.TextMessage, b)
		clear(b)
		if err != nil {
			return nil, revision, err
		}
	}
	if len(msg.Addrs) > 0 {
		b, err := json.Marshal(msg)
		if err != nil {
			return nil, revision, err
		}
		err = writeFrame(conn, websocket.TextMessage, b)
		clear(b)
		if err != nil {
			return nil, revision, err
		}
	}
	return msg.Addrs, revision, nil
}

func decodeTransaction(raw []byte) (*Transaction, error) {
	var tx Transaction
	if err := json.Unmarshal(raw, &tx); err != nil {
		return nil, errors.New("invalid transaction JSON")
	}
	id, ok := tx.ID.(string)
	if !ok || id == "" {
		return nil, errors.New("invalid transaction id")
	}
	return &tx, nil
}

func (c *client) parseIncoming(raw []byte) {
	var envelope struct {
		ID    json.RawMessage `json:"id"`
		Type  json.RawMessage `json:"type"`
		Data  json.RawMessage `json:"data"`
		State *State          `json:"state"`
	}
	if err := json.Unmarshal(raw, &envelope); err != nil || bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
		c.callErrorCallbacks(errors.New("invalid websocket JSON"))
		return
	}
	if len(envelope.ID) > 0 && !bytes.Equal(envelope.ID, []byte("null")) {
		tx, err := decodeTransaction(raw)
		if err != nil {
			c.callErrorCallbacks(err)
			return
		}
		c.callListenCallbacks(nil, tx)
		return
	}
	var typ int
	if len(envelope.Type) == 0 || bytes.Equal(bytes.TrimSpace(envelope.Type), []byte("null")) || json.Unmarshal(envelope.Type, &typ) != nil {
		c.callErrorCallbacks(errors.New("invalid websocket message type"))
		return
	}
	switch typ {
	case 0:
		var block Block
		if len(envelope.Data) == 0 || bytes.Equal(bytes.TrimSpace(envelope.Data), []byte("null")) || json.Unmarshal(envelope.Data, &block) != nil {
			c.callErrorCallbacks(errors.New("invalid block message"))
			return
		}
		c.callListenCallbacks(&block, nil)
	case 1:
		tx, err := decodeTransaction(envelope.Data)
		if err != nil {
			c.callErrorCallbacks(err)
			return
		}
		c.callListenCallbacks(nil, tx)
	case 2:
		if envelope.State != nil && *envelope.State == Fail {
			c.callErrorCallbacks(errors.New("subscription rejected"))
		}
	default:
		c.callErrorCallbacks(errors.New("unknown websocket message type"))
	}
}

func (c *client) callListenCallbacks(block *Block, tx *Transaction) {
	c.mut.RLock()
	callbacks := append([]func(*Block, *Transaction){}, c.listenCallbacks...)
	c.mut.RUnlock()
	for _, fn := range callbacks {
		if !c.dispatch(func() { fn(block, tx) }) {
			return
		}
	}
}

func (c *client) callErrorCallbacks(err error) {
	c.mut.RLock()
	callbacks := append([]func(error){}, c.errorCallbacks...)
	c.mut.RUnlock()
	for _, fn := range callbacks {
		if !c.dispatch(func() { fn(err) }) {
			return
		}
	}
}

func (c *client) callRetrievers() {
	c.mut.RLock()
	callbacks := append([]func(){}, c.retrieveCallbacks...)
	c.mut.RUnlock()
	for _, fn := range callbacks {
		if !c.dispatch(fn) {
			return
		}
	}
}

// Slow user callbacks apply backpressure instead of creating unlimited goroutines.
func (c *client) dispatch(fn func()) bool {
	c.mut.RLock()
	ctx := c.ctx
	if c.session != nil {
		ctx = c.session.ctx
	}
	c.mut.RUnlock()
	select {
	case <-ctx.Done():
		return false
	case c.callbackSlots <- struct{}{}:
	}
	go func() { defer func() { <-c.callbackSlots }(); fn() }()
	return true
}
