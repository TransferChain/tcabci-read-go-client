package tcabcireadgoclient

import (
	"fmt"
	"github.com/stretchr/testify/assert"
	"testing"
	"time"
)

func TestNewClient(t *testing.T) {
	wsClient, err := NewClient("wss://read-node-01.transferchain.io/ws")
	assert.Nil(t, err)

	//done := make(chan struct{})
	fn := func(transaction Transaction) {
		fmt.Println(transaction)
		//done <- struct{}{}
	}
	wsClient.SetListenCallback(fn)

	addrs := []string{
		"43fvvc8wnMES4L1Qy17tfo5Fzvbd4CFqjFEHD3UvQjtFdDeQuShjkWbQdN4f1bqCG7qXUvri7E8ZXodpRkX4wPqh",
	}
	time.Sleep(time.Second * 1)
	err = wsClient.Subscribe(addrs)
	assert.Nil(t, err)

	//<-done
	_ = wsClient.Unsubscribe()
	wsClient.Stop()
}

func TestNewClientWithinvalidWSUrl(t *testing.T) {
	wsClient, err := NewClient("https://read-node-01.transferchain.io/ws")
	assert.NotNil(t, err)
	assert.Nil(t, wsClient)
}
