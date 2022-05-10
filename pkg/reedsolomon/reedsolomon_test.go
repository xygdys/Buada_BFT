package reedsolomon

import (
	"Buada_BFT/internal/party"
	"bytes"
	"crypto/rand"
	"sync"
	"testing"
	"time"
)

func TestEncode(t *testing.T) {
	ipList := []string{"127.0.0.1", "127.0.0.1", "127.0.0.1", "127.0.0.1"}
	portList := []string{"8880", "8881", "8882", "8883"}

	N := uint32(40)
	F := uint32(10)
	sk, pk := party.SigKeyGen(N, 2*F+1)
	tpke := party.EncKeyGen(N, F+1)

	p := party.NewHonestParty(N, F, 0, ipList, portList, pk, sk[0], tpke[0])

	coder := NewRScoder(int(p.F+1), int(p.N))

	start := time.Now()

	var wg sync.WaitGroup
	wg.Add(4)

	go func() {

		batchsize := 1000000
		msg := make([]byte, 250*batchsize)
		rand.Read(msg)
		shares := coder.Encode(msg)

		if !bytes.Equal(msg, coder.Decode(shares)) {
			t.Error()
		}
		wg.Done()
	}()
	go func() {

		batchsize := 1000000
		msg := make([]byte, 250*batchsize)
		rand.Read(msg)
		shares := coder.Encode(msg)

		if !bytes.Equal(msg, coder.Decode(shares)) {
			t.Error()
		}
		wg.Done()
	}()
	go func() {

		batchsize := 1000000
		msg := make([]byte, 250*batchsize)
		rand.Read(msg)
		shares := coder.Encode(msg)

		if !bytes.Equal(msg, coder.Decode(shares)) {
			t.Error()
		}
		wg.Done()
	}()
	go func() {

		batchsize := 1000000
		msg := make([]byte, 250*batchsize)
		rand.Read(msg)
		shares := coder.Encode(msg)

		if !bytes.Equal(msg, coder.Decode(shares)) {
			t.Error()
		}
		wg.Done()
	}()

	// for i := 0; i < len(shares); i++ {
	// 	fmt.Println(shares[i])
	// }
	wg.Wait()

	// batchsize := 100
	// msg := make([]byte, 250*batchsize)
	// rand.Read(msg)
	// shares := coder.Encode(msg)

	// if !bytes.Equal(msg, coder.Decode(shares)) {
	// 	t.Error()
	// }
	cost := time.Since(start)
	t.Log(cost)

}
