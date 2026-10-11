package tcabcireadgoclient

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"strconv"
	"strings"
	"time"
)

// WSAuthoritySigner signs the supplied v3 envelope. The envelope is valid only
// for the call; honor ctx cancellation and do not retain it. Returned signature
// bytes remain caller-owned. No private key is stored by this library.
type WSAuthoritySigner func(context.Context, []byte) ([]byte, error)

// CreateWSAuthorityV3 creates a fresh, domain-separated WS authority header.
// Never log or cache the result. It is valid for ten seconds and one handshake.
func CreateWSAuthorityV3(ctx context.Context, address string, sign WSAuthoritySigner) (string, error) {
	if ctx == nil || sign == nil || address == "" || len(address) > 2048 || strings.ContainsAny(address, ";\r\n") {
		return "", errors.New("invalid websocket authority configuration")
	}
	if err := ctx.Err(); err != nil {
		return "", errors.New("websocket authority canceled")
	}
	nonce := make([]byte, 32)
	defer clear(nonce)
	if _, err := rand.Read(nonce); err != nil {
		return "", errors.New("websocket authority randomness failed")
	}
	stamp := strconv.FormatInt(time.Now().UnixNano(), 10)
	nonceHex := hex.EncodeToString(nonce)
	payload := []byte("read-node/ws-authority/v3\n" + address + "\n" + stamp + "\n" + nonceHex)
	defer clear(payload)
	result, err := sign(ctx, payload)
	if err != nil {
		return "", errors.New("websocket authority signing failed")
	}
	if ctx.Err() != nil {
		return "", errors.New("websocket authority canceled")
	}
	if len(result) == 0 || len(result) > 8192 {
		return "", errors.New("invalid websocket authority signature")
	}
	signature := append([]byte(nil), result...)
	defer clear(signature)
	return address + ";v3;" + stamp + ";" + nonceHex + ";" + hex.EncodeToString(signature), nil
}

// WithWSAuthority generates a fresh v3 header for every dial/reconnect. Passing
// nil disables automatic authority and releases the signer reference. Configure
// only trusted endpoints; the signer must honor cancellation promptly.
func (c *client) WithWSAuthority(address string, sign WSAuthoritySigner) (Client, error) {
	if sign != nil && (address == "" || len(address) > 2048 || strings.ContainsAny(address, ";\r\n")) {
		return c, errors.New("invalid websocket authority configuration")
	}
	c.mut.Lock()
	defer c.mut.Unlock()
	c.authorityAddress = address
	c.authoritySigner = sign
	if sign == nil {
		c.authorityAddress = ""
	}
	return c, nil
}
