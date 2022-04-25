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
	ctx, cancel := context.WithCancel(context.Background())

	for r := uint32(0); ; r++ {
		var buf bytes.Buffer
		spbCtx, spbCancel := context.WithCancel(ctx) //son of ctx
		wg := sync.WaitGroup{}
		wg.Add(int(p.N + 1)) //n SPBReceiver and 1 SPBSender instances

		Lr := sync.Map{} //Lock Set
		Fr := sync.Map{} //Finish Set
		doneFlagChannel := make(chan bool, 1)
		leaderChannel1 := make(chan uint32, 1) //for message handler
		leaderChannel2 := make(chan uint32, 1) //for main process
		voteFlagChannel := make(chan bool, 1)  //Yes or No
		voteYesChannel := make(chan []byte, 3)
		voteNoChannel := make(chan []byte, 2)

		//TODO: CheckValue

		//Initialize SPB instances
		buf.Write(ID)
		buf.Write(utils.Uint32ToBytes(r))
		IDrj := [][]byte{}
		for j := uint32(0); j < p.N; j++ {
			buf.Write(utils.Uint32ToBytes(j))
			IDrj = append(IDrj, buf.Bytes())
			go func() {
				value, sig, ok := SPBReceiver(spbCtx, p, j, IDrj[len(IDrj)-1])
				if ok { //save Lock
					Lr.Store(j, &protobuf.Lock{
						Value: value,
						Sig:   sig,
					})
				}
				wg.Done()
			}()
			buf.Truncate(len(IDrj[len(IDrj)-1]) - 4)
		}
		IDr := buf.Bytes()

		//Run this party's SPB instance
		go func() {
			value, sig, ok := SPBSender(spbCtx, p, IDrj[p.PID], value, validation)
			if ok {
				finishMessage := core.Encapsulation("Finish", IDr, p.PID, &protobuf.Finish{
					Value: value,
					Sig:   sig,
				})
				p.Broadcast(finishMessage)
			}
			wg.Done()
		}()

		//Run Message Handlers
		go messageHandler(ctx, p, IDr, IDrj, &Fr, doneFlagChannel, voteFlagChannel, voteYesChannel, voteNoChannel, leaderChannel1, haltChannel)

		//doneFlag -> common coin
		go election(ctx, p, IDr, doneFlagChannel, leaderChannel1, leaderChannel2)

		//Short-cut or Prevote
		select {
		case result := <-haltChannel:
			spbCancel() //???
			cancel()
			return result
		case l := <-leaderChannel2:
			spbCancel() //shut down all SPB instances
			wg.Wait()   //wait for all SPB instances to shut donw
			value1, ok1 := Fr.Load(l)
			if ok1 {
				finish := value1.(*protobuf.Finish)
				haltMessage := core.Encapsulation("Halt", IDr, p.PID, &protobuf.Halt{
					Value: finish.Value,
					Sig:   finish.Sig,
				})
				p.Broadcast(haltMessage)
				cancel()
				return finish.Value //short-cut
			}
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
			//Vote
			select {
			case result := <-haltChannel:
				cancel()
				return result
			case VoteFlag := <-voteFlagChannel:
				if VoteFlag { //have received a valid Yes PreVote
					value := <-voteYesChannel
					sig := <-voteYesChannel
					sigShare := <-voteYesChannel
					voteMessage := core.Encapsulation("Vote", IDr, p.PID, &protobuf.Vote{
						Vote:     true,
						Value:    value,
						Sig:      sig,
						Sigshare: sigShare,
					})
					p.Broadcast(voteMessage)
				} else { //have received 2f+1 valid No PreVote
					sig := <-voteNoChannel
					sigShare := <-voteNoChannel
					voteMessage := core.Encapsulation("Vote", IDr, p.PID, &protobuf.Vote{
						Vote:     false,
						Value:    nil,
						Sig:      sig,
						Sigshare: sigShare,
					})
					p.Broadcast(voteMessage)
				}
			}

		}

	}
}

func messageHandler(ctx context.Context, p *party.HonestParty, IDr []byte, IDrj [][]byte, Fr *sync.Map, doneFlagChannel chan bool, voteFlagChannel chan bool, voteYesChannel chan []byte, voteNoChannel chan []byte, leaderChannel chan uint32, haltChannel chan []byte) {
	//FinishMessage Handler
	go func() {
		FrLength := 0
		for j := uint32(0); j < p.N; j = (j + 1) % p.N {
			select {
			case <-ctx.Done():
				return
			default:
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
					err := bls.Verify(pairing.NewSuiteBn256(), p.SigPK.Commit(), sm, payload.Sig) //verify("Echo"||ID||r||j||2||h)
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

		for j := uint32(0); j < p.N; j = (j + 1) % p.N {
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
				err := bls.Verify(pairing.NewSuiteBn256(), p.SigPK.Commit(), sm, payload.Sig) //verify("Echo"||ID||r||l||2||h)
				if err == nil {
					haltChannel <- payload.Value //TODO:how to halt?
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
						err := bls.Verify(pairing.NewSuiteBn256(), p.SigPK.Commit(), sm, payload.Sig) //verify("Echo"||ID||r||l||1||h)
						if err == nil {
							sm[len([]byte("Echo"))+len(IDrj[l])] = 2
							sigShare, _ := tbls.Sign(pairing.NewSuiteBn256(), p.SigSK, sm) //sign("Echo"||ID||r||l||2||h)
							voteFlagChannel <- true
							voteYesChannel <- payload.Value
							voteYesChannel <- payload.Sig
							voteYesChannel <- sigShare
						}
					} else {
						var buf bytes.Buffer
						buf.WriteByte(byte(0)) //false
						buf.Write(IDr)
						sm := buf.Bytes()
						err := tbls.Verify(pairing.NewSuiteBn256(), p.SigPK, sm, payload.Sig) //verifyShare(false||ID||r)
						if err == nil {
							PNr = append(PNr, payload.Sig)
							if len(PNr) > int(2*p.F) {
								noSignature, _ := tbls.Recover(pairing.NewSuiteBn256(), p.SigPK, sm, PNr, int(2*p.F+1), int(p.N))
								var buf bytes.Buffer
								buf.Write([]byte("Unlock"))
								buf.Write(IDr)
								sm := buf.Bytes()
								sigShare, _ := tbls.Sign(pairing.NewSuiteBn256(), p.SigSK, sm) //sign("Unlock"||ID||r)
								voteFlagChannel <- false
								voteNoChannel <- noSignature
								voteNoChannel <- sigShare
							}
						}
					}

				}
			}
		}
	}()
}
