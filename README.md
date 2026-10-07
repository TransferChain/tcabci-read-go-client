# TCABCI Read Node Go WebSocket Client
[![Go Report Card](https://goreportcard.com/badge/github.com/TransferChain/tcabci-read-go-client)](https://goreportcard.com/report/github.com/TransferChain/tcabci-read-go-client)

TransferChain Fastest Read Network WebSocket Client  
Read Node Address: [https://read-node-01.transferchain.io](https://read-node-01.transferchain.io)  
Read Node WebSocket Address: [wss://read-node-01.transferchain.io/ws](wss://read-node-01.transferchain.io/ws)

## Installation

```shell
$ go get github.com/TransferChain/tcabci-read-go-client 
```

## Example

**Subscribe, Listen and Unsubscribe Example**

```go
package main

import (
	"log"
	tcabcireadgoclient "github.com/TransferChain/tcabci-read-go-client"
)

func main() {
	readNodeClient, _ := tcabcireadgoclient.NewClient("https://read-node-01.transferchain.io", "wss://read-node-01.transferchain.io/ws", "medusa", "v2", false, nil, nil)

	if err := readNodeClient.Start(); err != nil {
		log.Fatal(err)
    }
	
	addresses := []string{
		"<your-public-address-one>",
		"<your-public-address-two>",
	}
	signedData := map[string]string{
		"<your-public-address-one>": "<signature>",
		"<your-public-address-two>": "<signature>",
    }

	if err := readNodeClient.Subscribe(addresses, signedData); err != nil {
		log.Fatal(err)
	}

	done := make(chan struct{})
	// If a transaction has been sent to your addresses, the callback you set here will be called.
	readNodeClient.SetListenCallback(func(block *tcabcireadgoclient.Block, transaction *tcabcireadgoclient.Transaction) {
		// 
		done <- struct{}{}
	})
	
	<-done
	close(done)

	_ = readNodeClient.Unsubscribe()
	_ = readNodeClient.Stop()
}
```

## Security

### Optional TLS certificate pinning

Pinning is optional and independent of standard TLS certificate verification.
The `insecure` argument controls Go's `InsecureSkipVerify` setting for HTTPS
and WSS. With `insecure=false`, normal certificate-chain and hostname checks
remain enabled, including when pinning is disabled. TLS 1.2 is the minimum.

| Fingerprint / certificate input | Pinning behavior |
| --- | --- |
| `customFingerprint=nil`, `cert=nil` | Use the built-in default fingerprint. |
| Non-empty `customFingerprint` | Require that SHA-256 fingerprint of the server leaf certificate; it replaces the default pin. |
| `customFingerprint=nil`, `cert` supplied | Derive the leaf pin from the supplied PEM or DER server certificate. |
| Non-empty fingerprint and certificate | Both must describe the same leaf certificate; conflicting inputs are rejected. |
| Pointer to an empty fingerprint string | Disable pinning explicitly, including when a certificate is supplied. |

The `cert` reader contains a **public server certificate**, not an mTLS client
identity or a custom CA trust store. No private key is required. The reader is
consumed once during construction; `SetVerbose` does not read it again.

To disable pinning while keeping standard TLS verification:

```go
noPin := ""
readNodeClient, err := tcabcireadgoclient.NewClient(
    httpAddress, wsAddress, "medusa", "v2", false, &noPin, nil,
)
```

Setting `insecure=true` together with an empty fingerprint intentionally
accepts any server certificate, subject to TLS protocol compatibility. This
explicit opt-out is part of the client API contract, not a pinning failure.
With `insecure=true` and an enabled pin, the leaf must still match that pin;
normal CA, hostname and validity-period verification is skipped.

### Go security checks

Enabled pin checks run in `tls.Config.VerifyConnection`, which also runs for
resumed TLS connections. They bind to the server's leaf certificate; adding a
matching public certificate elsewhere in the presented chain does not satisfy
the pin.

`gosec` rule G402 flags the caller-controlled `InsecureSkipVerify` option. The
code documents this intentional option with a narrow G402 annotation. It does
not disable other security rules. The custom-verification/resumption warning
G123 is addressed by using `VerifyConnection` instead of
`VerifyPeerCertificate`.

HTTP responses and WebSocket messages are limited to 16 MiB. WebSocket reads
also enforce this limit after decompression. The send queue has a 16 MiB byte
budget and 64-message capacity; callback concurrency is bounded to 16. Slow
callbacks apply backpressure. Logs omit headers, payloads and server-provided
error text.  

## Thanks

Websocket client code referenced here [https://github.com/webdeveloppro/golang-websocket-client](https://github.com/webdeveloppro/golang-websocket-client).  
  
## License

tcabci-read-go-client is licensed under the Apache License, Version 2.0. See [LICENSE](LICENSE) for the full license
text.