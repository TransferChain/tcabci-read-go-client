package tcabci_read_go_client

import (
	"fmt"
	"github.com/stretchr/testify/assert"
	"testing"
	"time"
)

func TestNewClient(t *testing.T) {
	wsClient := NewClient("wss://read-node-01.transferchain.io/ws")

	done := make(chan struct{})
	fn := func(transaction Transaction) {
		fmt.Println(transaction)
		done <- struct{}{}
	}
	wsClient.SetListenCallback(fn)

	addrs := []string{
		"43fvvc8wnMES4L1Qy17tfo5Fzvbd4CFqjFEHD3UvQjtFdDeQuShjkWbQdN4f1bqCG7qXUvri7E8ZXodpRkX4wPqh",
	}
	time.Sleep(time.Second * 1)
	err := wsClient.Subscribe(addrs)
	assert.Nil(t, err)

	<-done
	_ = wsClient.Unsubscribe()
	wsClient.Stop()
}
