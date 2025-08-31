package tcabcireadgoclient

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/sha512"
	"encoding/hex"
	"fmt"
	"math/big"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

var (
	ctx               = context.Background()
	readNodeAddress   = "https://test-read-node-01.transferchain.io"
	readNodeWSAddress = "wss://test-read-node-01.transferchain.io/ws"
	chainName         = "medusa"
	chainVersion      = "v2"
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
	readNodeClient, err := NewClientContext(ctx, readNodeAddress, readNodeWSAddress, chainName, chainVersion)
	assert.Nil(b, err)
	_, _ = readNodeClient.SetVerbose(true)

	//done := make(chan struct{})
	fn := func(transaction *Transaction) {
		fmt.Println(transaction)
		//done <- struct{}{}
	}
	readNodeClient.SetListenCallback(fn)

	signedDatas := make(map[string]string)
	addrs := make([]string, 251)
	for i := 0; i < 251; i++ {
		_, signKey, adr, err := KeyFillFromSeed(sha256.Sum256([]byte(randomString(88))))
		assert.Nil(b, err)
		if err != nil {
			break
		}
		data := sha512.Sum512([]byte(randomString(32)))
		signed := Sign(signKey, data[:])
		signedDatas[adr] = hex.EncodeToString(data[:]) + ";" + hex.EncodeToString(signed[:])

		addrs = append(addrs, adr)

	}
	err = readNodeClient.Start()
	assert.Nil(b, err)
	time.Sleep(time.Second * 1)

	err = readNodeClient.Subscribe(addrs, signedDatas)
	assert.Nil(b, err)

	//<-done
	_ = readNodeClient.Unsubscribe()
	err = readNodeClient.Stop()
	assert.Nil(b, err)
}

func newTestClientWithoutStop(t *testing.T) Client {
	readNodeClient, err := NewClientContext(ctx, readNodeAddress, readNodeWSAddress, chainName, chainVersion)
	assert.Nil(t, err)
	_, _ = readNodeClient.SetVerbose(true)

	//done := make(chan struct{})
	fn := func(transaction *Transaction) {
		fmt.Println(transaction)
		//done <- struct{}{}
	}
	readNodeClient.SetListenCallback(fn)

	signedDatas := make(map[string]string)
	addrs := make([]string, 251)
	for i := 0; i < 251; i++ {
		_, signKey, adr, err := KeyFillFromSeed(sha256.Sum256([]byte(randomString(88))))
		assert.Nil(t, err)
		if err != nil {
			break
		}
		data := sha512.Sum512([]byte(randomString(32)))
		signed := Sign(signKey, data[:])
		signedDatas[adr] = hex.EncodeToString(data[:]) + ";" + hex.EncodeToString(signed[:])

		addrs = append(addrs, adr)

	}
	err = readNodeClient.Start()
	assert.Nil(t, err)
	err = readNodeClient.Subscribe(addrs, signedDatas)
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
	readNodeClient, err := NewClientContext(ctx, readNodeAddress, readNodeWSAddress, chainName, chainVersion)
	assert.Nil(t, err)
	_, _ = readNodeClient.SetVerbose(true)

	//done := make(chan struct{})
	fn := func(transaction *Transaction) {
		fmt.Println(transaction)
		//done <- struct{}{}
	}
	readNodeClient.SetListenCallback(fn)

	signedDatas := make(map[string]string)
	addrs := make([]string, 251)
	for i := 0; i < 251; i++ {
		_, signKey, adr, err := KeyFillFromSeed(sha256.Sum256([]byte(randomString(88))))
		assert.Nil(t, err)
		if err != nil {
			break
		}
		data := sha512.Sum512([]byte(randomString(32)))
		signed := Sign(signKey, data[:])
		signedDatas[adr] = hex.EncodeToString(data[:]) + ";" + hex.EncodeToString(signed[:])

		addrs = append(addrs, adr)

	}
	err = readNodeClient.Start()
	assert.Nil(t, err)
	time.Sleep(time.Second * 1)
	err = readNodeClient.Subscribe(addrs, signedDatas)
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

func TestNewClientWithInvalidWSUrl(t *testing.T) {
	readNodeClient, err := NewClientContext(ctx, readNodeAddress, "invalid", chainName, chainVersion)
	assert.NotNil(t, err)
	assert.Nil(t, readNodeClient)
}

func TestLastBlock(t *testing.T) {
	readNodeClient, err := NewClientContext(ctx, readNodeAddress, readNodeWSAddress, chainName, chainVersion)
	assert.Nil(t, err)
	_, _ = readNodeClient.SetVerbose(true)
	err = readNodeClient.Start()
	assert.Nil(t, err)
	lastBlock, err := readNodeClient.LastBlock(nil, nil)
	assert.Nil(t, err)
	assert.NotNil(t, lastBlock)

	assert.Less(t, uint64(1), lastBlock.TotalCount)
	assert.Len(t, lastBlock.Blocks, 1)

	err = readNodeClient.Stop()
	assert.Nil(t, err)
}

func TestLastBlockWithChainInfo(t *testing.T) {
	readNodeClient, err := NewClientContext(ctx, readNodeAddress, readNodeWSAddress, chainName, chainVersion)
	assert.Nil(t, err)
	_, _ = readNodeClient.SetVerbose(true)
	err = readNodeClient.Start()
	assert.Nil(t, err)
	lastBlock, err := readNodeClient.LastBlock(&chainName, &chainVersion)
	assert.Nil(t, err)

	assert.Less(t, uint64(1), lastBlock.TotalCount)
	assert.Len(t, lastBlock.Blocks, 1)

	err = readNodeClient.Stop()
	assert.Nil(t, err)
}

func TestShowTx(t *testing.T) {
	readNodeClient, err := NewClientContext(ctx, readNodeAddress, readNodeWSAddress, chainName, chainVersion)
	assert.Nil(t, err)
	_, _ = readNodeClient.SetVerbose(true)
	err = readNodeClient.Start()
	assert.Nil(t, err)
	ttx, err := readNodeClient.Tx("7a465504e84b3169ac4770c1daf6bbdd326a253e514d8c521ae0dc0db26c12eb71b869ac191e87f89b3e00ec2b97f740fb6b08f3c28736ed11ba4c4d93d520a5", "609964525909181fc39c1a81c22d69d850f7ce51faebe0435241de9f5be29440af8f97d4e966757b1b1aae711a;5a8b22ac5317b4003adc078f4247bd5f22b132963e921f89a60c3067d69fe9a281bf75c3d9d7d341e69661468baf41972ba96f9b129aa5791ab319ec2cee2005", &chainName, &chainVersion)
	assert.Nil(t, err)
	assert.NotNil(t, ttx)

	err = readNodeClient.Stop()
	assert.Nil(t, err)
}

func TestTxSummary(t *testing.T) {
	readNodeClient, err := NewClientContext(ctx, readNodeAddress, readNodeWSAddress, chainName, chainVersion)
	assert.Nil(t, err)
	_, _ = readNodeClient.SetVerbose(true)
	err = readNodeClient.Start()
	assert.Nil(t, err)
	lastBlockHeight, lastTransaction, totalCount, err := readNodeClient.TxSummary(&Summary{
		RecipientAddresses: []string{"i7jaE2DCoUxq1uhaYu6Qre1Z8yhhaas3pgd4FvXbNwin8pSQJmBxmPepxTU7P5xR1YnmiqdYangHdXSCWCR7FnV"},
		SignedAddresses: map[string]string{
			"i7jaE2DCoUxq1uhaYu6Qre1Z8yhhaas3pgd4FvXbNwin8pSQJmBxmPepxTU7P5xR1YnmiqdYangHdXSCWCR7FnV": "609964525909181fc39c1a81c22d69d850f7ce51faebe0435241de9f5be29440af8f97d4e966757b1b1aae711a;5a8b22ac5317b4003adc078f4247bd5f22b132963e921f89a60c3067d69fe9a281bf75c3d9d7d341e69661468baf41972ba96f9b129aa5791ab319ec2cee2005",
		},
		ChainName:    &chainName,
		ChainVersion: &chainVersion,
	})
	assert.Nil(t, err)

	assert.Greater(t, lastBlockHeight, uint64(0))
	assert.NotNil(t, lastTransaction)
	assert.Greater(t, totalCount, uint64(0))

	err = readNodeClient.Stop()
	assert.Nil(t, err)
}

func TestShouldErrorTxSummaryWithInvalidType(t *testing.T) {
	readNodeClient, err := NewClientContext(ctx, readNodeAddress, readNodeWSAddress, chainName, chainVersion)
	assert.Nil(t, err)
	_, _ = readNodeClient.SetVerbose(true)
	err = readNodeClient.Start()
	assert.Nil(t, err)
	lastBlockHeight, lastTransaction, totalCount, err := readNodeClient.TxSummary(&Summary{
		RecipientAddresses: []string{"i7jaE2DCoUxq1uhaYu6Qre1Z8yhhaas3pgd4FvXbNwin8pSQJmBxmPepxTU7P5xR1YnmiqdYangHdXSCWCR7FnV"},
		SignedAddresses: map[string]string{
			"i7jaE2DCoUxq1uhaYu6Qre1Z8yhhaas3pgd4FvXbNwin8pSQJmBxmPepxTU7P5xR1YnmiqdYangHdXSCWCR7FnV": "609964525909181fc39c1a81c22d69d850f7ce51faebe0435241de9f5be29440af8f97d4e966757b1b1aae711a;5a8b22ac5317b4003adc078f4247bd5f22b132963e921f89a60c3067d69fe9a281bf75c3d9d7d341e69661468baf41972ba96f9b129aa5791ab319ec2cee2005",
		},
		Type:         "yp",
		ChainName:    &chainName,
		ChainVersion: &chainVersion,
	})
	assert.NotNil(t, err)

	assert.Equal(t, uint64(0), lastBlockHeight)
	assert.Nil(t, lastTransaction)
	assert.Equal(t, uint64(0), totalCount)

	err = readNodeClient.Stop()
	assert.Nil(t, err)
}

func TestTxSearch(t *testing.T) {
	readNodeClient, err := NewClientContext(ctx, readNodeAddress, readNodeWSAddress, chainName, chainVersion)
	assert.Nil(t, err)
	_, _ = readNodeClient.SetVerbose(true)
	err = readNodeClient.Start()
	assert.Nil(t, err)
	txs, totalCount, err := readNodeClient.TxSearch(&Search{
		HeightOperator:     ">=",
		Height:             0,
		RecipientAddresses: []string{"i7jaE2DCoUxq1uhaYu6Qre1Z8yhhaas3pgd4FvXbNwin8pSQJmBxmPepxTU7P5xR1YnmiqdYangHdXSCWCR7FnV"},
		SignedAddresses: map[string]string{
			"i7jaE2DCoUxq1uhaYu6Qre1Z8yhhaas3pgd4FvXbNwin8pSQJmBxmPepxTU7P5xR1YnmiqdYangHdXSCWCR7FnV": "609964525909181fc39c1a81c22d69d850f7ce51faebe0435241de9f5be29440af8f97d4e966757b1b1aae711a;5a8b22ac5317b4003adc078f4247bd5f22b132963e921f89a60c3067d69fe9a281bf75c3d9d7d341e69661468baf41972ba96f9b129aa5791ab319ec2cee2005",
		},
		Limit:   1,
		Offset:  0,
		OrderBy: "ASC",
	})
	assert.Nil(t, err)

	assert.Greater(t, len(txs), 0)
	assert.Greater(t, totalCount, uint64(0))

	err = readNodeClient.Stop()
	assert.Nil(t, err)
}

func TestShouldErrorTxSearchWithInvalidHeightOperator(t *testing.T) {
	readNodeClient, err := NewClientContext(ctx, readNodeAddress, readNodeWSAddress, chainName, chainVersion)
	assert.Nil(t, err)
	_, _ = readNodeClient.SetVerbose(true)
	err = readNodeClient.Start()
	assert.Nil(t, err)
	txs, totalCount, err := readNodeClient.TxSearch(&Search{
		HeightOperator:     "!=",
		Height:             0,
		RecipientAddresses: []string{"i7jaE2DCoUxq1uhaYu6Qre1Z8yhhaas3pgd4FvXbNwin8pSQJmBxmPepxTU7P5xR1YnmiqdYangHdXSCWCR7FnV"},
		SignedAddresses: map[string]string{
			"i7jaE2DCoUxq1uhaYu6Qre1Z8yhhaas3pgd4FvXbNwin8pSQJmBxmPepxTU7P5xR1YnmiqdYangHdXSCWCR7FnV": "609964525909181fc39c1a81c22d69d850f7ce51faebe0435241de9f5be29440af8f97d4e966757b1b1aae711a;5a8b22ac5317b4003adc078f4247bd5f22b132963e921f89a60c3067d69fe9a281bf75c3d9d7d341e69661468baf41972ba96f9b129aa5791ab319ec2cee2005",
		},
		Limit:        1,
		Offset:       0,
		OrderBy:      "ASC",
		ChainName:    &chainName,
		ChainVersion: &chainVersion,
	})
	assert.NotNil(t, err)

	assert.Equal(t, 0, len(txs))
	assert.Equal(t, uint64(0), totalCount)

	err = readNodeClient.Stop()
	assert.Nil(t, err)
}

func TestShouldErrorTxBroadcast(t *testing.T) {
	readNodeClient, err := NewClientContext(ctx, readNodeAddress, readNodeWSAddress, chainName, chainVersion)
	assert.Nil(t, err)
	_, _ = readNodeClient.SetVerbose(true)
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
