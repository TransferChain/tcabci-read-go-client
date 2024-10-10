package tcabcireadgoclient

import (
	"context"
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

func newTestClient(b *testing.B, n int) {
	readNodeClient, err := NewClientContext(context.Background(), "https://read-node-01.transferchain.io", "wss://read-node-01.transferchain.io/ws")
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
	err = readNodeClient.Start()
	assert.Nil(b, err)
	time.Sleep(time.Second * 1)
	err = readNodeClient.Subscribe(addrs)
	assert.Nil(b, err)

	//<-done
	_ = readNodeClient.Unsubscribe()
	err = readNodeClient.Stop()
	assert.Nil(b, err)
}

func newTestClientWithoutStop(t *testing.T) Client {
	readNodeClient, err := NewClientContext(context.Background(), "https://read-node-01.transferchain.io", "wss://read-node-01.transferchain.io/ws")
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
	err = readNodeClient.Start()
	assert.Nil(t, err)
	err = readNodeClient.Subscribe(addrs)
	assert.Nil(t, err)

	return readNodeClient
}

//func TestNewClientWW(t *testing.T) {
//	size := 100
//	clients := make([]Client, 0)
//	runtime.GOMAXPROCS(runtime.NumCPU())
//
//	for i := 0; i < 5; i++ {
//		var wg sync.WaitGroup
//		for j := 0; j < size; j++ {
//			wg.Add(1)
//			go func(i, j int, wg *sync.WaitGroup) {
//				defer wg.Done()
//				cc := newTestClientWithoutStop(t)
//				clients = append(clients, cc)
//			}(i, j, &wg)
//		}
//		wg.Wait()
//	}
//
//	time.Sleep(time.Second * 5)
//	for i := 0; i < len(clients); i++ {
//		cc := clients[i]
//		err := cc.Unsubscribe()
//		assert.Nil(t, err)
//		err = cc.Stop()
//		assert.Nil(t, err)
//	}
//}

func TestNewClientContext(t *testing.T) {
	readNodeClient, err := NewClientContext(context.Background(), "https://read-node-01.transferchain.io", "wss://read-node-01.transferchain.io/ws")
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
	err = readNodeClient.Start()
	assert.Nil(t, err)
	time.Sleep(time.Second * 1)
	err = readNodeClient.Subscribe(addrs)
	assert.Nil(t, err)

	//<-done
	_ = readNodeClient.Unsubscribe()
	err = readNodeClient.Stop()
	assert.Nil(t, err)
}

//func TestWriteParallel(t *testing.T) {
//	readNodeClient, err := NewClientContext(context.Background(), "https://read-node-01.transferchain.io", "wss://read-node-01.transferchain.io/ws")
//	assert.Nil(t, err)
//
//	//done := make(chan struct{})
//	fn := func(transaction *Transaction) {
//		fmt.Println(transaction)
//		//done <- struct{}{}
//	}
//	readNodeClient.SetListenCallback(fn)
//
//	adr1 := randomString(88)
//	adr2 := randomString(88)
//	addrs := []string{
//		adr1,
//		adr2,
//	}
//	err = readNodeClient.Start()
//	assert.Nil(t, err)
//	time.Sleep(time.Second * 1)
//	err = readNodeClient.Subscribe(addrs)
//	assert.Nil(t, err)
//
//	lim := 100000
//	ch := make(chan bool)
//	for i := 0; i < lim; i++ {
//		go func(i int, ch chan bool) {
//			err := readNodeClient.Write([]byte(fmt.Sprintf("%d", i)))
//			assert.Nil(t, err)
//			time.Sleep(time.Second)
//			if i == lim-1 {
//				ch <- true
//			}
//		}(i, ch)
//	}
//	<-ch
//	_ = readNodeClient.Unsubscribe()
//	err = readNodeClient.Stop()
//	assert.Nil(t, err)
//}

func TestNewClientWithinvalidWSUrl(t *testing.T) {
	readNodeClient, err := NewClientContext(context.Background(), "htt://read-node-01.transferchain.io", "wss://read-node-01.transferchain.io/ws")
	assert.NotNil(t, err)
	assert.Nil(t, readNodeClient)
}

func TestLastBlock(t *testing.T) {
	readNodeClient, err := NewClientContext(context.Background(), "https://read-node-01.transferchain.io", "wss://read-node-01.transferchain.io/ws")
	assert.Nil(t, err)
	err = readNodeClient.Start()
	assert.Nil(t, err)
	lastBlock, err := readNodeClient.LastBlock()
	assert.Nil(t, err)

	assert.Less(t, uint64(1), lastBlock.TotalCount)
	assert.Len(t, lastBlock.Blocks, 1)

	err = readNodeClient.Stop()
	assert.Nil(t, err)
}

func TestTxSummary(t *testing.T) {
	readNodeClient, err := NewClientContext(context.Background(), "https://read-node-01.transferchain.io", "wss://read-node-01.transferchain.io/ws")
	assert.Nil(t, err)
	err = readNodeClient.Start()
	assert.Nil(t, err)
	lastBlockHeight, lastTransaction, totalCount, err := readNodeClient.TxSummary(&Summary{
		RecipientAddresses: []string{"2csPt2z76d397MVEinhRqUR35QT1Kk2BguKK1cUBAqnM3HCpuTZet8Avc7LQS7RWQfFgHbeQYQNnMjsWrbdx3rcc"},
	})
	assert.Nil(t, err)

	assert.Greater(t, lastBlockHeight, uint64(0))
	assert.NotNil(t, lastTransaction)
	assert.Greater(t, totalCount, uint64(0))

	err = readNodeClient.Stop()
	assert.Nil(t, err)
}

func TestShouldErrorTxSummaryWithInvalidType(t *testing.T) {
	readNodeClient, err := NewClientContext(context.Background(), "https://read-node-01.transferchain.io", "wss://read-node-01.transferchain.io/ws")
	assert.Nil(t, err)
	err = readNodeClient.Start()
	assert.Nil(t, err)
	lastBlockHeight, lastTransaction, totalCount, err := readNodeClient.TxSummary(&Summary{
		RecipientAddresses: []string{"2csPt2z76d397MVEinhRqUR35QT1Kk2BguKK1cUBAqnM3HCpuTZet8Avc7LQS7RWQfFgHbeQYQNnMjsWrbdx3rcc"},
		Type:               "yp",
	})
	assert.NotNil(t, err)

	assert.Equal(t, uint64(0), lastBlockHeight)
	assert.Nil(t, lastTransaction)
	assert.Equal(t, uint64(0), totalCount)

	err = readNodeClient.Stop()
	assert.Nil(t, err)
}

func TestTxSearch(t *testing.T) {
	readNodeClient, err := NewClientContext(context.Background(), "https://read-node-01.transferchain.io", "wss://read-node-01.transferchain.io/ws")
	assert.Nil(t, err)
	err = readNodeClient.Start()
	assert.Nil(t, err)
	txs, totalCount, err := readNodeClient.TxSearch(&Search{
		HeightOperator:     ">=",
		Height:             0,
		RecipientAddresses: []string{"2csPt2z76d397MVEinhRqUR35QT1Kk2BguKK1cUBAqnM3HCpuTZet8Avc7LQS7RWQfFgHbeQYQNnMjsWrbdx3rcc"},
		Limit:              1,
		Offset:             0,
		OrderBy:            "ASC",
	})
	assert.Nil(t, err)

	assert.Greater(t, len(txs), 0)
	assert.Greater(t, totalCount, uint64(0))

	err = readNodeClient.Stop()
	assert.Nil(t, err)
}

func TestShouldErrorTxSearchWithInvalidHeightOperator(t *testing.T) {
	readNodeClient, err := NewClientContext(context.Background(), "https://read-node-01.transferchain.io", "wss://read-node-01.transferchain.io/ws")
	assert.Nil(t, err)
	err = readNodeClient.Start()
	assert.Nil(t, err)
	txs, totalCount, err := readNodeClient.TxSearch(&Search{
		HeightOperator:     "!=",
		Height:             0,
		RecipientAddresses: []string{"2csPt2z76d397MVEinhRqUR35QT1Kk2BguKK1cUBAqnM3HCpuTZet8Avc7LQS7RWQfFgHbeQYQNnMjsWrbdx3rcc"},
		Limit:              1,
		Offset:             0,
		OrderBy:            "ASC",
	})
	assert.NotNil(t, err)

	assert.Equal(t, 0, len(txs))
	assert.Equal(t, uint64(0), totalCount)

	err = readNodeClient.Stop()
	assert.Nil(t, err)
}

func TestShouldErrorTxBroadcast(t *testing.T) {
	readNodeClient, err := NewClientContext(context.Background(), "https://read-node-01.transferchain.io", "wss://read-node-01.transferchain.io/ws")
	assert.Nil(t, err)
	err = readNodeClient.Start()
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
	err = readNodeClient.Stop()
	assert.Nil(t, err)
}

func BenchmarkNewClientContext(b *testing.B) {
	for n := 0; n < b.N; n++ {
		newTestClient(b, 3)
	}
}

func BenchmarkNewClient20(b *testing.B) {
	for n := 0; n < b.N; n++ {
		newTestClient(b, 10)
	}
}

func BenchmarkNewClient40(b *testing.B) {
	for n := 0; n < b.N; n++ {
		newTestClient(b, 20)
	}
}

func BenchmarkNewClient100(b *testing.B) {
	for n := 0; n < b.N; n++ {
		newTestClient(b, 40)
	}
}

func BenchmarkNewClient250(b *testing.B) {
	for n := 0; n < b.N; n++ {
		newTestClient(b, 60)
	}
}
