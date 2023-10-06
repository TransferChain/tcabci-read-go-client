package tcabcireadgoclient

import (
	"crypto/rand"
	"fmt"
	"github.com/stretchr/testify/assert"
	"math/big"
	"runtime"
	"sync"
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

func newClientWithoutStop(t *testing.T) Client {
	wsClient, err := NewClient("wss://read-node-01.transferchain.io/ws")
	assert.Nil(t, err)

	//done := make(chan struct{})
	fn := func(transaction Transaction) {
		fmt.Println(transaction)
		//done <- struct{}{}
	}
	wsClient.SetListenCallback(fn)

	addrs := make([]string, 251)
	for i := 0; i < 251; i++ {
		addrs = append(addrs, randomString(88))
	}
	err = wsClient.Subscribe(addrs)
	assert.Nil(t, err)

	return wsClient
}

func TestNewClientWW(t *testing.T) {
	size := 100
	clients := make([]Client, 0)
	runtime.GOMAXPROCS(runtime.NumCPU())

	for i := 0; i < 10; i++ {
		fmt.Println("Came here 0")
		var wg sync.WaitGroup
		wg.Add(size)
		for j := 0; j < size; j++ {
			go func(i, j int, wg *sync.WaitGroup) {
				cc := newClientWithoutStop(t)
				fmt.Println("Client -> ", i+j)
				clients = append(clients, cc)
				wg.Done()
			}(i, j, &wg)
		}
		wg.Wait()
	}

	fmt.Println("Came here 1")
	time.Sleep(time.Second * 5)
	fmt.Println("Came here 2")
	for i := 0; i < len(clients); i++ {
		cc := clients[i]
		err := cc.Unsubscribe()
		assert.Nil(t, err)
		err = cc.Stop()
		assert.Nil(t, err)
	}
	fmt.Println("Came here 3")
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

func TestWriteParallel(t *testing.T) {
	wsClient, err := NewClient("wss://read-node-01.transferchain.io/ws")
	assert.Nil(t, err)

	//done := make(chan struct{})
	fn := func(transaction Transaction) {
		fmt.Println(transaction)
		//done <- struct{}{}
	}
	wsClient.SetListenCallback(fn)

	adr1 := randomString(88)
	adr2 := randomString(88)
	addrs := []string{
		adr1,
		adr2,
	}
	time.Sleep(time.Second * 1)
	err = wsClient.Subscribe(addrs)
	assert.Nil(t, err)

	lim := 100000
	ch := make(chan bool)
	for i := 0; i < lim; i++ {
		go func(i int, ch chan bool) {
			err := wsClient.Write([]byte(fmt.Sprintf("%d", i)))
			if err != nil {
				fmt.Println(err)
			}
			time.Sleep(time.Second)
			if i == lim-1 {
				ch <- true
			}
		}(i, ch)
	}
	<-ch
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
