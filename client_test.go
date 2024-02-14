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
	readNodeClient, err := NewClient("https://read-node-01.transferchain.io", "wss://read-node-01.transferchain.io/ws")
	assert.Nil(b, err)

	//done := make(chan struct{})
	fn := func(transaction *Transaction) {
		fmt.Println(transaction)
		//done <- struct{}{}
	}
	readNodeClient.SetListenCallback(fn)

	addrs := make([]string, n)
	for i := 0; i < n; i++ {
		addrs = append(addrs, randomString(88))
	}
	time.Sleep(time.Second * 1)
	err = readNodeClient.Subscribe(addrs)
	assert.Nil(b, err)

	//<-done
	_ = readNodeClient.Unsubscribe()
	err = readNodeClient.Stop()
	assert.Nil(b, err)
}

func newClientWithoutStop(t *testing.T) Client {
	readNodeClient, err := NewClient("https://read-node-01.transferchain.io", "wss://read-node-01.transferchain.io/ws")
	assert.Nil(t, err)

	//done := make(chan struct{})
	fn := func(transaction *Transaction) {
		fmt.Println(transaction)
		//done <- struct{}{}
	}
	readNodeClient.SetListenCallback(fn)

	addrs := make([]string, 251)
	for i := 0; i < 251; i++ {
		addrs = append(addrs, randomString(88))
	}
	err = readNodeClient.Subscribe(addrs)
	assert.Nil(t, err)

	return readNodeClient
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
	readNodeClient, err := NewClient("https://read-node-01.transferchain.io", "wss://read-node-01.transferchain.io/ws")
	assert.Nil(t, err)

	//done := make(chan struct{})
	fn := func(transaction *Transaction) {
		fmt.Println(transaction)
		//done <- struct{}{}
	}
	readNodeClient.SetListenCallback(fn)

	addrs := []string{
		randomString(88),
	}
	time.Sleep(time.Second * 1)
	err = readNodeClient.Subscribe(addrs)
	assert.Nil(t, err)

	//<-done
	_ = readNodeClient.Unsubscribe()
	err = readNodeClient.Stop()
	assert.Nil(t, err)
}

func TestWriteParallel(t *testing.T) {
	readNodeClient, err := NewClient("https://read-node-01.transferchain.io", "wss://read-node-01.transferchain.io/ws")
	assert.Nil(t, err)

	//done := make(chan struct{})
	fn := func(transaction *Transaction) {
		fmt.Println(transaction)
		//done <- struct{}{}
	}
	readNodeClient.SetListenCallback(fn)

	adr1 := randomString(88)
	adr2 := randomString(88)
	addrs := []string{
		adr1,
		adr2,
	}
	time.Sleep(time.Second * 1)
	err = readNodeClient.Subscribe(addrs)
	assert.Nil(t, err)

	lim := 100000
	ch := make(chan bool)
	for i := 0; i < lim; i++ {
		go func(i int, ch chan bool) {
			err := readNodeClient.Write([]byte(fmt.Sprintf("%d", i)))
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
	_ = readNodeClient.Unsubscribe()
	err = readNodeClient.Stop()
	assert.Nil(t, err)
}

func TestNewClientWithinvalidWSUrl(t *testing.T) {
	readNodeClient, err := NewClient("htt://read-node-01.transferchain.io", "wss://read-node-01.transferchain.io/ws")
	assert.NotNil(t, err)
	assert.Nil(t, readNodeClient)
}

func TestLastBlock(t *testing.T) {
	readNodeClient, err := NewClient("https://read-node-01.transferchain.io", "wss://read-node-01.transferchain.io/ws")
	assert.Nil(t, err)

	lastBlock, err := readNodeClient.LastBlock()
	assert.Nil(t, err)

	assert.Less(t, uint64(1), lastBlock.TotalCount)
	assert.Len(t, lastBlock.Blocks, 1)
}

func TestTxSummary(t *testing.T) {
	readNodeClient, err := NewClient("https://read-node-01.transferchain.io", "wss://read-node-01.transferchain.io/ws")
	assert.Nil(t, err)

	lastBlochHeight, lastTransaction, totalCount, err := readNodeClient.TxSummary(&Summary{
		RecipientAddresses: []string{"2mSCzresfg8Gwu7LZ9k9BTWkQAcQEkvYHFUSCZE2ubM4QV89PTeSYwQDqBas3ykq2emHEK6VRvxdgoe1vrhBbQGN"},
	})
	assert.Nil(t, err)

	assert.Less(t, uint64(1), lastBlochHeight)
	assert.NotNil(t, lastTransaction)
	assert.Less(t, uint64(1), totalCount)
}

func TestShouldErrorTxSummaryWithInvalidType(t *testing.T) {
	readNodeClient, err := NewClient("https://read-node-01.transferchain.io", "wss://read-node-01.transferchain.io/ws")
	assert.Nil(t, err)

	lastBlochHeight, lastTransaction, totalCount, err := readNodeClient.TxSummary(&Summary{
		RecipientAddresses: []string{"2mSCzresfg8Gwu7LZ9k9BTWkQAcQEkvYHFUSCZE2ubM4QV89PTeSYwQDqBas3ykq2emHEK6VRvxdgoe1vrhBbQGN"},
		Type:               "yp",
	})
	assert.NotNil(t, err)

	assert.Equal(t, uint64(0), lastBlochHeight)
	assert.Nil(t, lastTransaction)
	assert.Equal(t, uint64(0), totalCount)
}

func TestTxSearch(t *testing.T) {
	readNodeClient, err := NewClient("https://read-node-01.transferchain.io", "wss://read-node-01.transferchain.io/ws")
	assert.Nil(t, err)

	txs, totalCount, err := readNodeClient.TxSearch(&Search{
		HeightOperator:     ">=",
		Height:             0,
		RecipientAddresses: []string{"2mSCzresfg8Gwu7LZ9k9BTWkQAcQEkvYHFUSCZE2ubM4QV89PTeSYwQDqBas3ykq2emHEK6VRvxdgoe1vrhBbQGN"},
		Limit:              1,
		Offset:             0,
		OrderBy:            "ASC",
	})
	assert.Nil(t, err)

	assert.Less(t, 0, len(txs))
	assert.Less(t, uint64(1), totalCount)
}

func TestShouldErrorTxSearchWithInvalidHeightOperator(t *testing.T) {
	readNodeClient, err := NewClient("https://read-node-01.transferchain.io", "wss://read-node-01.transferchain.io/ws")
	assert.Nil(t, err)

	txs, totalCount, err := readNodeClient.TxSearch(&Search{
		HeightOperator:     "!=",
		Height:             0,
		RecipientAddresses: []string{"2mSCzresfg8Gwu7LZ9k9BTWkQAcQEkvYHFUSCZE2ubM4QV89PTeSYwQDqBas3ykq2emHEK6VRvxdgoe1vrhBbQGN"},
		Limit:              1,
		Offset:             0,
		OrderBy:            "ASC",
	})
	assert.NotNil(t, err)

	assert.Equal(t, 0, len(txs))
	assert.Equal(t, uint64(0), totalCount)
}

func TestShouldErrorTxBroadcast(t *testing.T) {
	readNodeClient, err := NewClient("https://read-node-01.transferchain.io", "wss://read-node-01.transferchain.io/ws")
	assert.Nil(t, err)

	broadcast, err := readNodeClient.Broadcast("id",
		0,
		TypeTransfer,
		[]byte("1"),
		"address",
		"address",
		[]byte("1"),
		0)
	assert.NotNil(t, err)
	assert.Nil(t, broadcast)
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
