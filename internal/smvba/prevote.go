package smvba

import (
	"Buada_BFT/internal/party"
	"Buada_BFT/pkg/core"
	"Buada_BFT/pkg/protobuf"
	"bytes"
	"context"
	"sync"

	"go.dedis.ch/kyber/v3/pairing"
	"go.dedis.ch/kyber/v3/sign/tbls"
)

func preVote(ctx context.Context, p *party.HonestParty, IDr []byte, l uint32, Lr *sync.Map) {
	value2, ok2 := Lr.Load(l)
	if ok2 {
		lock := value2.(*protobuf.Lock)
		preVoteMessage := core.Encapsulation("PreVote", IDr, p.PID, &protobuf.PreVote{
			Vote:  true,
			Value: lock.Value,
			Sig:   lock.Sig,
		})
		p.Broadcast(preVoteMessage)
	} else {
		var buf bytes.Buffer
		buf.WriteByte(byte(0)) //false
		buf.Write(IDr)
		sm := buf.Bytes()
		sigShare, _ := tbls.Sign(pairing.NewSuiteBn256(), p.SigSK, sm) //sign(false||ID||r)
		preVoteMessage := core.Encapsulation("PreVote", IDr, p.PID, &protobuf.PreVote{
			Vote:  false,
			Value: nil,
			Sig:   sigShare,
		})
		p.Broadcast(preVoteMessage)
	}
}
