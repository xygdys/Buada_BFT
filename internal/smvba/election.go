package smvba

import (
	"Buada_BFT/internal/party"
	"Buada_BFT/pkg/core"
	"Buada_BFT/pkg/protobuf"
	"bytes"
	"context"

	"go.dedis.ch/kyber/v3/pairing"
	"go.dedis.ch/kyber/v3/sign/tbls"
)

func election(ctx context.Context, p *party.HonestParty, IDr []byte, doneFlageChannel chan bool) {
	select {
	case <-ctx.Done():
		return
	case <-doneFlageChannel:
		var buf bytes.Buffer
		buf.Write([]byte("Done"))
		buf.Write(IDr)
		coinName := buf.Bytes()

		coinShare, _ := tbls.Sign(pairing.NewSuiteBn256(), p.SigSK, coinName) //sign("Done"||ID||r||coin share)
		doneMessage := core.Encapsulation("Done", IDr, p.PID, &protobuf.Done{
			CoinShare: coinShare,
		})
		p.Broadcast(doneMessage)

	}
}
