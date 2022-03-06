package tcabci_read_go_client

import (
	"fmt"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestNewClient(t *testing.T) {
	wsClient := NewClient("read-node-01.transferchain.io")

	err := wsClient.Start()
	assert.Nil(t, err)

	done := make(chan struct{})
	fn := func(transaction Transaction) {
		fmt.Println(transaction)
		done <- struct{}{}
	}
	wsClient.SetListenCallback(fn)

	addrs := []string{
		"adr1",
		"adr2",
	}
	err = wsClient.Subscribe(addrs)
	assert.Nil(t, err)

	go func() {
		if err := wsClient.Listen(); err != nil {
			panic(err)
		}
	}()

	<-done
	_ = wsClient.Unsubscribe()
	wsClient.Stop()
}
