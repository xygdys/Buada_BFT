package acs

import (
	"Buada_BFT/internal/party"
	"bytes"
	"crypto/rand"
	"fmt"
	"sync"
	"testing"
)

func TestBuadaACS(t *testing.T) {
	ipList := []string{"127.0.0.1", "127.0.0.1", "127.0.0.1", "127.0.0.1"}
	portList := []string{"8880", "8881", "8882", "8883"}

	N := uint32(4)
	F := uint32(1)
	sk, pk := party.SigKeyGen(N, 2*F+1)

	var p []*party.HonestParty = make([]*party.HonestParty, N)
	for i := uint32(0); i < N; i++ {
		p[i] = party.NewHonestParty(N, F, i, ipList, portList, pk, sk[i])
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

	for i := uint32(0); i < N; i++ {
		go func(i uint32) {
			proposal := make([]byte, 10)
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
			fmt.Println("party", i, "'s output:", buf.Bytes())
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
