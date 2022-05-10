package aab

import (
	"Buada_BFT/internal/acs"
	"Buada_BFT/internal/party"
	"Buada_BFT/pkg/core"
	"Buada_BFT/pkg/protobuf"
	"Buada_BFT/pkg/tpke"
	"Buada_BFT/pkg/utils"
)

//Buada is an asynchronous atomic broadcast
func Buada(p *party.HonestParty, r uint32, value []byte) (map[uint32][]byte, int) {
	//st := time.Now()

	ID := utils.Uint32ToBytes(r)
	cValue, _ := p.TPKE.Encrypt(value)
	//fmt.Println("encrypt success", len(cValue))

	// t1 := time.Since(st)
	// if p.PID == 0 {
	// 	fmt.Println("encrypt time:", t1)
	// }

	cResult := acs.BuadaACS(p, r, cValue)

	// t2 := time.Since(st)
	// if p.PID == 0 {
	// 	fmt.Println("acs time:", t2-t1)
	// }

	//fmt.Println("acs success")

	ids := []uint32{}
	ciphers := [][]byte{}
	shares := [][]byte{}

	for i := uint32(0); i < p.N; i++ {
		value, ok := cResult.Load(i)
		if ok {
			ids = append(ids, i)
			ciphers = append(ciphers, value.([]byte))
		}
	}

	for i := 0; i < len(ids); i++ {
		shares = append(shares, tpke.DecShare(ciphers[i], p.TPKE))
	}

	// t3 := time.Since(st)
	// if p.PID == 0 {
	// 	fmt.Println("share time:", t3-t2)
	// }

	//fmt.Println("calulate share success")

	decMessage := core.Encapsulation("Dec", ID, p.PID, &protobuf.Dec{
		DecShares: shares,
	})
	p.Broadcast(decMessage)

	decShareMaps := make([](map[int][]byte), len(ids))
	for i := 0; i < len(ids); i++ {
		decShareMaps[i] = map[int][]byte{}
	}

	for {
		m := <-p.GetMessage("Dec", ID)
		payload := core.Decapsulation("Dec", m).(*protobuf.Dec)
		for i, share := range payload.DecShares {
			decShareMaps[i][int(m.Sender)] = share
		}
		if len(decShareMaps[0]) > int(p.F+1) {
			break
		}
	}

	// t4 := time.Since(st)
	// if p.PID == 0 {
	// 	fmt.Println("share exchange time:", t4-t3)
	// }

	pResult := map[uint32][]byte{}
	resultLen := 0
	for i := 0; i < len(ids); i++ {
		pValue := tpke.Dec(decShareMaps[i], ciphers[i], p.TPKE)
		pResult[ids[i]] = pValue
		resultLen += len(pValue)
	}

	// t5 := time.Since(st)
	// if p.PID == 0 {
	// 	fmt.Println("decrypt time:", t5-t4)

	// 	fmt.Println("additional compute time:", t1+t3-t2+t5-t4)

	// 	fmt.Println("pResult time:", time.Since(st))
	// }

	return pResult, resultLen
}
