package smvba

import (
	"Buada_BFT/internal/party"
	"Buada_BFT/pkg/core"
	"Buada_BFT/pkg/protobuf"
	"Buada_BFT/pkg/utils"
	"bytes"
	"context"
	"sync"

	"go.dedis.ch/kyber/v3/pairing"
	"go.dedis.ch/kyber/v3/sign/bls"
	"go.dedis.ch/kyber/v3/sign/tbls"
	"golang.org/x/crypto/sha3"
)

//MainProcess is the main process of smvba instances
func MainProcess(p *party.HonestParty, ID []byte, value []byte, validation []byte) []byte {
	haltChannel := make(chan []byte, 1)

	for r := uint32(0); ; r++ {
		var buf bytes.Buffer
		ctx, cancel := context.WithCancel(context.Background())
		wg := sync.WaitGroup{}
		wg.Add(int(p.N + 1)) //n SPBReceiver and 1 SPBSender instances

		Lr := sync.Map{} //Lock Set
		Fr := sync.Map{} //Finish Set
		doneFlagChannel := make(chan bool, 1)
		leaderChannel := make(chan uint32, 1)

		//TODO: CheckValue

		//Initialize SPB instances
		buf.Write(ID)
		buf.Write(utils.Uint32ToBytes(r))
		IDrj := [][]byte{}
		for j := uint32(0); j < p.N; j++ {
			buf.Write(utils.Uint32ToBytes(j))
			IDrj = append(IDrj, buf.Bytes())
			go func() {
				value, sig, ok := SPBReceiver(ctx, p, j, IDrj[len(IDrj)-1])
				if ok { //save Lock
					Lr.Store(j, &protobuf.Lock{
						Value:     value,
						Signature: sig,
					})
				}
				wg.Done()
			}()
			buf.Truncate(len(IDrj[len(IDrj)-1]) - 4)
		}
		IDr := buf.Bytes()

		//Run this party's SPB instance
		go func() {
			value, sig, ok := SPBSender(ctx, p, IDrj[p.PID], value, validation)
			if ok {
				finishMessage := core.Encapsulation("Finish", IDr, p.PID, &protobuf.Finish{
					Value:     value,
					Signature: sig,
				})
				p.Broadcast(finishMessage)
			}
			wg.Done()
		}()

		//Run Message Handlers
		go messageHandler(p, IDr, IDrj, &Fr, doneFlagChannel, leaderChannel, haltChannel)

		//doneFlag -> common coin
		<-doneFlagChannel
		l := election(p, IDr)

		cancel()  //shut down all SPB instances
		wg.Wait() //wait for all SPB instances to shut donw

		//Short-cut
		value1, ok1 := Fr.Load(l)
		if ok1 {
			finish := value1.(*protobuf.Finish)
			haltMessage := core.Encapsulation("Halt", IDr, p.PID, &protobuf.Halt{
				Value:     finish.Value,
				Signature: finish.Signature,
			})
			p.Broadcast(haltMessage)
			return finish.Value
		}

		//PreVote
		value2, ok2 := Lr.Load(l)
		if ok2 {
			lock := value2.(*protobuf.Lock)
			preVoteMessage := core.Encapsulation("PreVote", IDr, p.PID, &protobuf.PreVote{
				Vote:     true,
				Value:    lock.Value,
				Sigshare: lock.Signature,
			})
			p.Broadcast(preVoteMessage)
		} else {
			var buf bytes.Buffer
			buf.WriteByte(byte(0)) //false
			buf.Write(IDr)
			sm := buf.Bytes()
			sigShare, _ := tbls.Sign(pairing.NewSuiteBn256(), p.SigSK, sm) //sign(false||ID||r)
			preVoteMessage := core.Encapsulation("PreVote", IDr, p.PID, &protobuf.PreVote{
				Vote:     false,
				Value:    nil,
				Sigshare: sigShare,
			})
			p.Broadcast(preVoteMessage)
		}

	}
}

func messageHandler(p *party.HonestParty, IDr []byte, IDrj [][]byte, Fr *sync.Map, doneFlagChannel chan bool, leaderChannel chan uint32, haltChannel chan []byte) {
	//FinishMessage Handler
	go func() {
		FrLength := 0
		for {
			for j := uint32(0); j < p.N; j++ {
				m, ok := p.GetMessage(j, "Finish", IDr)
				if ok {
					payload := core.Decapsulation("Finish", m).(*protobuf.Finish)

					h := sha3.Sum512(payload.Value)
					var buf bytes.Buffer
					buf.Write([]byte("Echo"))
					buf.Write(IDrj[m.Sender])
					buf.WriteByte(2)
					buf.Write(h[:])
					sm := buf.Bytes()
					err := bls.Verify(pairing.NewSuiteBn256(), p.SigPK.Commit(), sm, payload.Signature) //verify("Echo"||ID||r||j||2||h)
					if err == nil {
						Fr.Store(m.Sender, payload)
						FrLength++
						if FrLength > int(2*p.F) {
							doneFlagChannel <- true
						}
					}

				}
			}
		}
	}()

	l := <-leaderChannel

	//HaltMessage Handler
	go func() {
		for {
			for j := uint32(0); j < p.N; j++ {
				m, ok := p.GetMessage(j, "Halt", IDr)
				if ok {
					payload := core.Decapsulation("Halt", m).(*protobuf.Halt)

					h := sha3.Sum512(payload.Value)
					var buf bytes.Buffer
					buf.Write([]byte("Echo"))
					buf.Write(IDrj[l])
					buf.WriteByte(2)
					buf.Write(h[:])
					sm := buf.Bytes()
					err := bls.Verify(pairing.NewSuiteBn256(), p.SigPK.Commit(), sm, payload.Signature) //verify("Echo"||ID||r||l||2||h)
					if err == nil {
						haltChannel <- payload.Value //TODO:how to halt?
					}

				}
			}
		}
	}()

	//PreVoteMessage Handler
	go func() {
		PNr := [][]byte{}
		for {
			for j := uint32(0); j < p.N; j++ {
				m, ok := p.GetMessage(j, "PreVote", IDr)
				if ok {
					payload := core.Decapsulation("PreVote", m).(*protobuf.PreVote)
					if payload.Vote {
						h := sha3.Sum512(payload.Value)
						var buf bytes.Buffer
						buf.Write([]byte("Echo"))
						buf.Write(IDrj[l])
						buf.WriteByte(1)
						buf.Write(h[:])
						sm := buf.Bytes()
						err := bls.Verify(pairing.NewSuiteBn256(), p.SigPK.Commit(), sm, payload.Signature) //verify("Echo"||ID||r||l||1||h)
						if err == nil {

						}
					} else {
						var buf bytes.Buffer
						buf.WriteByte(byte(0)) //false
						buf.Write(IDr)
						sm := buf.Bytes()
						err := tbls.Verify(pairing.NewSuiteBn256(), p.SigPK, sm, payload.Signature) //verifyShare(false||ID||r)
						if err == nil {
							PNr = append(PNr, payload.Signature)
							if len(PNr) > int(2*p.F) {
								tbls.Recover(pairing.NewSuiteBn256(), p.SigPK, sm, PNr, int(2*p.F+1), int(p.N))
							}
						}
					}

				}
			}
		}
	}()
}
