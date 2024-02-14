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
	readNodeClient, _ := tcabcireadgoclient.NewClient("https://read-node-01.transferchain.io", "wss://read-node-01.transferchain.io/ws")

	if err := readNodeClient.Start(); err != nil {
		log.Fatal(err)
    }
	
	addresses := []string{
		"<your-public-address-one>",
		"<your-public-address-two>",
	}

	if err := readNodeClient.Subscribe(addresses); err != nil {
		log.Fatal(err)
	}

	done := make(chan struct{})
	// If a transaction has been sent to your addresses, the callback you set here will be called.
	readNodeClient.SetListenCallback(func(transaction *tcabcireadgoclient.Transaction) {
		// 
		done <- struct{}{}
	})
	
	<-done
	close(done)

	_ = readNodeClient.Unsubscribe()
	readNodeClient.Stop()
}
```

## Thanks

Websocket client code referenced here [https://github.com/webdeveloppro/golang-websocket-client](https://github.com/webdeveloppro/golang-websocket-client).  
  
## License

tcabci-read-go-client is licensed under the Apache License, Version 2.0. See [LICENSE](LICENSE) for the full license
text.