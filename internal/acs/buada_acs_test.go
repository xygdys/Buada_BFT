package acs

import (
	"Buada_BFT/internal/party"
	"bytes"
	"crypto/rand"
	"sync"
	"testing"
)

func TestBuadaACS(t *testing.T) {
	ipList := []string{"127.0.0.1", "127.0.0.1", "127.0.0.1", "127.0.0.1", "127.0.0.1", "127.0.0.1", "127.0.0.1", "127.0.0.1", "127.0.0.1", "127.0.0.1",
		"127.0.0.1", "127.0.0.1", "127.0.0.1", "127.0.0.1", "127.0.0.1", "127.0.0.1", "127.0.0.1", "127.0.0.1", "127.0.0.1", "127.0.0.1",
		"127.0.0.1", "127.0.0.1", "127.0.0.1", "127.0.0.1", "127.0.0.1", "127.0.0.1", "127.0.0.1", "127.0.0.1", "127.0.0.1", "127.0.0.1",
		"127.0.0.1"}
	portList := []string{"8880", "8881", "8882", "8883", "8884", "8885", "8886", "8887", "8888", "8889",
		"8870", "8871", "8872", "8873", "8874", "8875", "8876", "8877", "8878", "8879",
		"8860", "8861", "8862", "8863", "8864", "8865", "8866", "8867", "8868", "8869", "8859"}

	N := uint32(31)
	F := uint32(10)
	sk, pk := party.SigKeyGen(N, 2*F+1)
	tpke := party.EncKeyGen(N, F+1)

	var p []*party.HonestParty = make([]*party.HonestParty, N)
	for i := uint32(0); i < N; i++ {
		p[i] = party.NewHonestParty(N, F, i, ipList, portList, pk, sk[i], tpke[i])
	}

	for i := uint32(0); i < N; i++ {
		p[i].InitReceiveChannel()
	}

	for i := uint32(0); i < N; i++ {
		p[i].InitSendChannel()
	}

	result := make([][]byte, N)
	var wg sync.WaitGroup
	var mu sync.Mutex
	wg.Add(int(N))

	txSize := 250

	batchSize := 100000

	for i := uint32(0); i < N; i++ {
		go func(i uint32) {
			proposal := make([]byte, batchSize/int(N)*txSize)
			rand.Read(proposal)
			Mr := BuadaACS(p[i], 0, proposal)
			var buf bytes.Buffer
			for j := uint32(0); j < N; j++ {
				value, ok := Mr.Load(j)
				if ok {
					buf.Write(value.([]byte))
				}
			}
			mu.Lock()
			result = append(result, buf.Bytes())
			mu.Unlock()
			//fmt.Println("party", i, "'s output:", buf.Bytes())
			wg.Done()
		}(i)
	}
	wg.Wait()
	for i := uint32(1); i < N; i++ {
		if !bytes.Equal(result[i], result[i-1]) {
			t.Error()
		}
	}

}
