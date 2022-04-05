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
	var wsClient = tcabcireadgoclient.NewClient("wss://read-node-01.transferchain.io/ws")
	
	addresses := []string{
		"<your-public-address-one>",
		"<your-public-address-two>",
	}

	if err := wsClient.Subscribe(addresses); err != nil {
		log.Fatal(err)
	}

	done := make(chan struct{})
	// If a transaction has been sent to your addresses, the callback you set here will be called.
	wsClient.SetListenCallback(func(transaction tcabcireadgoclient.Transaction) {
		// 
		done <- struct{}{}
	})
	
	<-done
	close(done)

	_ = wsClient.Unsubscribe()
	wsClient.Stop()
}
```

## Thanks

Websocket client code referenced here [https://github.com/webdeveloppro/golang-websocket-client](https://github.com/webdeveloppro/golang-websocket-client).  
  
## License

tcabci-read-go-client is licensed under the Apache License, Version 2.0. See [LICENSE](LICENSE) for the full license
text.