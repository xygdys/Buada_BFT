package vdd

import (
	"Buada_BFT/internal/party"
	"Buada_BFT/pkg/protobuf"
	"bytes"
	"crypto/rand"
	"fmt"
	"sync"
	"testing"

	"golang.org/x/crypto/sha3"
)

func TestCallHelp(t *testing.T) {
	ipList := []string{"127.0.0.1", "127.0.0.1", "127.0.0.1", "127.0.0.1"}
	portList := []string{"8880", "8881", "8882", "8883"}

	N := uint32(4)
	F := uint32(1)
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

	value := [][]byte{}
	for i := uint32(0); i < N; i++ {
		data := make([]byte, 10)
		rand.Read(data)
		value = append(value, data)
	}
	h := [][]byte{}
	for i := uint32(0); i < N; i++ {
		tmp := sha3.Sum512(value[i])
		h = append(h, tmp[:])
	}

	finalSet := &protobuf.FinalSetValue{
		Pid: []uint32{3, 2, 1, 0},
		Hash: [][]byte{
			h[3], h[2], h[1], h[0],
		},
	}

	result := [][]byte{}
	var wg sync.WaitGroup
	var mu sync.Mutex
	wg.Add(int(N))

	for i := uint32(0); i < N; i++ {
		go func(i uint32) {
			Pr := sync.Map{}
			Pr.Store(i, value[i])
			Pr.Store((i+1)%N, value[(i+1)%N])
			output := CallHelp(p[i], []byte{0}, finalSet, &Pr)
			var buf bytes.Buffer
			for j := uint32(0); j < N; j++ {
				value, ok := output.Load(j)
				if ok {
					buf.Write(value.([]byte))
				}

			}
			mu.Lock()
			result = append(result, buf.Bytes())
			mu.Unlock()

			wg.Done()
		}(i)
	}

	wg.Wait()
	fmt.Println("party", 0, "'s output:", result[0])
	for i := uint32(1); i < N; i++ {
		if !bytes.Equal(result[i], result[i-1]) {
			t.Error()
		}
		fmt.Println("party", i, "'s output:", result[i])
	}

}
