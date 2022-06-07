package tcabcireadgoclient

import (
	"crypto/rand"
	"fmt"
	"github.com/stretchr/testify/assert"
	"math/big"
	"testing"
	"time"
)

func randomString(n int) string {
	const letters = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz"
	ret := make([]byte, n)
	for i := 0; i < n; i++ {
		num, _ := rand.Int(rand.Reader, big.NewInt(int64(len(letters))))
		ret[i] = letters[num.Int64()]
	}

	return string(ret)
}

func newClient(b *testing.B, n int) {
	wsClient, err := NewClient("wss://read-node-01.transferchain.io/ws")
	assert.Nil(b, err)

	//done := make(chan struct{})
	fn := func(transaction Transaction) {
		fmt.Println(transaction)
		//done <- struct{}{}
	}
	wsClient.SetListenCallback(fn)

	addrs := make([]string, n)
	for i := 0; i < n; i++ {
		addrs = append(addrs, randomString(88))
	}
	time.Sleep(time.Second * 1)
	err = wsClient.Subscribe(addrs)
	assert.Nil(b, err)

	//<-done
	_ = wsClient.Unsubscribe()
	err = wsClient.Stop()
	assert.Nil(b, err)
}

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
		randomString(88),
	}
	time.Sleep(time.Second * 1)
	err = wsClient.Subscribe(addrs)
	assert.Nil(t, err)

	//<-done
	_ = wsClient.Unsubscribe()
	err = wsClient.Stop()
	assert.Nil(t, err)
}

func TestNewClientWithinvalidWSUrl(t *testing.T) {
	wsClient, err := NewClient("https://read-node-01.transferchain.io/ws")
	assert.NotNil(t, err)
	assert.Nil(t, wsClient)
}

func BenchmarkNewClient(b *testing.B) {
	for n := 0; n < b.N; n++ {
		newClient(b, 10)
	}
}

func BenchmarkNewClient20(b *testing.B) {
	for n := 0; n < b.N; n++ {
		newClient(b, 20)
	}
}

func BenchmarkNewClient40(b *testing.B) {
	for n := 0; n < b.N; n++ {
		newClient(b, 40)
	}
}

func BenchmarkNewClient100(b *testing.B) {
	for n := 0; n < b.N; n++ {
		newClient(b, 100)
	}
}

func BenchmarkNewClient250(b *testing.B) {
	for n := 0; n < b.N; n++ {
		newClient(b, 250)
	}
}
