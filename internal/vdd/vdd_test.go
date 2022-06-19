package vdd

import (
	"Buada_BFT/internal/party"
	"Buada_BFT/pkg/protobuf"
	"bytes"
	"fmt"
	"sync"
	"testing"

	"golang.org/x/crypto/sha3"
)

func TestCallHelp(t *testing.T) {
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

	value := [][]byte{}
	for i := uint32(0); i < N; i++ {
		data := make([]byte, 1)
		data[0] = byte(i)
		//rand.Read(data)
		value = append(value, data)
	}
	h := [][]byte{}
	for i := uint32(0); i < N; i++ {
		tmp := sha3.Sum512(value[i])
		h = append(h, tmp[:])
	}

	finalSet := &protobuf.FinalSetValue{
		Pid: []uint32{30, 29, 28, 27, 26, 25, 24, 23, 22, 21, 20, 19, 18, 17, 16, 15, 14, 13, 12, 11, 10, 9, 8, 7, 6, 5, 4, 3, 2, 1, 0},
		Hash: [][]byte{
			h[30], h[29], h[28], h[27], h[26], h[25], h[24], h[23], h[22], h[21],
			h[20], h[19], h[18], h[17], h[16], h[15], h[14], h[13], h[12], h[11],
			h[10], h[9], h[8], h[7], h[6], h[5], h[4], h[3], h[2], h[1], h[0],
		},
	}

	result := [][]byte{}
	var wg sync.WaitGroup
	var mu sync.Mutex
	wg.Add(int(N))

	for i := uint32(0); i < N; i++ {
		go func(i uint32) {
			Pr := sync.Map{}

			for l := uint32(0); l < F+1; l++ {
				Pr.Store((i+l)%N, value[(i+l)%N])
			}

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
