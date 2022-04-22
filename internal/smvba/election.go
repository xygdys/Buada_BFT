package smvba

import (
	"Buada_BFT/internal/party"
	"Buada_BFT/pkg/core"
	"Buada_BFT/pkg/protobuf"
	"Buada_BFT/pkg/utils"
	"bytes"

	"go.dedis.ch/kyber/v3/pairing"
	"go.dedis.ch/kyber/v3/sign/tbls"
)

func election(p *party.HonestParty, IDr []byte) uint32 {
	var buf bytes.Buffer
	buf.Write([]byte("Done"))
	buf.Write(IDr)
	coinName := buf.Bytes()
	coinShare, _ := tbls.Sign(pairing.NewSuiteBn256(), p.SigSK, coinName) //sign("Done"||ID||r||coin share)
	doneMessage := core.Encapsulation("Done", IDr, p.PID, &protobuf.Done{
		CoinShare: coinShare,
	})
	p.Broadcast(doneMessage)

	coins := [][]byte{}

wating:
	for {
		for i := uint32(0); i < p.N; i++ {
			m, ok := p.GetMessage(i, "Done", IDr)
			if !ok {
				continue
			}
			payload := core.Decapsulation("Done", m).(*protobuf.Done)
			err := tbls.Verify(pairing.NewSuiteBn256(), p.SigPK, coinName, payload.CoinShare) //verifyshare("Done"||ID||r)

			if err == nil {
				coins = append(coins, payload.CoinShare)
				if len(coins) > int(2*p.F) {
					break wating
				}
			}
		}
	}

	coin, _ := tbls.Recover(pairing.NewSuiteBn256(), p.SigPK, coinName, coins, int(2*p.F+1), int(p.N))
	l := utils.BytesToUint32(coin) % p.N
	return l //leader of round r
}
