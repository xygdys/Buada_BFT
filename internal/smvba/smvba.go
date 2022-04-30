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
	haltChannel := make(chan []byte, 1) //control all round
	ctx, cancel := context.WithCancel(context.Background())

	for r := uint32(0); ; r++ {
		var buf bytes.Buffer
		spbCtx, spbCancel := context.WithCancel(ctx) //son of ctx
		wg := sync.WaitGroup{}
		wg.Add(int(p.N + 1)) //n SPBReceiver and 1 SPBSender instances

		Lr := sync.Map{} //Lock Set
		Fr := sync.Map{} //Finish Set
		doneFlagChannel := make(chan bool, 1)
		leaderChannel := make(chan uint32, 1)    //for main progress
		preVoteFlagChannel := make(chan bool, 1) //Yes or No
		preVoteYesChannel := make(chan []byte, 3)
		preVoteNoChannel := make(chan []byte, 2)
		voteFlagChannel := make(chan byte, 1)
		voteYesChannel := make(chan []byte, 2)
		voteNoChannel := make(chan []byte, 1)
		voteOtherChannel := make(chan []byte, 1)

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
		go messageHandler(ctx, p, IDr, IDrj, &Fr,
			doneFlagChannel,
			preVoteFlagChannel, preVoteYesChannel, preVoteNoChannel,
			voteFlagChannel, voteYesChannel, voteNoChannel, voteOtherChannel,
			leaderChannel, haltChannel)

		//doneFlag -> common coin
		go election(ctx, p, IDr, doneFlagChannel)

		//leaderChannel -> shortcut -> prevote||vote||viewchange
		select {
		case result := <-haltChannel:
			spbCancel()
			cancel()
			return result
		case l := <-leaderChannel:
			spbCancel()
			wg.Wait()

			//short-cut
			value1, ok1 := Fr.Load(l)
			if ok1 {
				finish := value1.(*protobuf.Finish)
				haltMessage := core.Encapsulation("Halt", IDr, p.PID, &protobuf.Halt{
					Value: finish.Value,
					Sig:   finish.Sig,
				})
				p.Broadcast(haltMessage)
				cancel()
				return finish.Value
			}

			//preVote
			go preVote(ctx, p, IDr, l, &Lr)

			//vote
			go vote(ctx, p, IDr, l, preVoteFlagChannel, preVoteYesChannel, preVoteNoChannel)

			//result
			select {
			case result := <-haltChannel:
				cancel()
				return result
			case flag := <-voteFlagChannel:
				if flag == 0 { //Yes
					value := <-voteYesChannel
					sig := <-voteYesChannel
					haltMessage := core.Encapsulation("Halt", IDr, p.PID, &protobuf.Halt{
						Value: value,
						Sig:   sig,
					})
					p.Broadcast(haltMessage)
					cancel()
					return value
				} else if flag == 1 { //No
					sig := <-voteNoChannel
					validation = append(validation, sig...)
				} else {
					//overwrite
					value = <-voteOtherChannel
					validation = <-voteOtherChannel
				}
			}
		}
	}
}

func messageHandler(ctx context.Context, p *party.HonestParty, IDr []byte, IDrj [][]byte, Fr *sync.Map,
	doneFlagChannel chan bool,
	preVoteFlagChannel chan bool, preVoteYesChannel chan []byte, preVoteNoChannel chan []byte,
	voteFlagChannel chan byte, voteYesChannel chan []byte, voteNoChannel chan []byte, voteOtherChannel chan []byte,
	leaderChannel chan uint32, haltChannel chan []byte) {

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

	thisRoundLeader := make(chan uint32, 1)

	go func() {
		var buf bytes.Buffer
		buf.Write([]byte("Done"))
		buf.Write(IDr)
		coinName := buf.Bytes()
		coins := [][]byte{}
		for j := uint32(0); j < p.N; j = (j + 1) % p.N {
			select {
			case <-ctx.Done():
				return
			default:
				m, ok := p.GetMessage(j, "Done", IDr)
				if !ok {
					continue
				}
				payload := core.Decapsulation("Done", m).(*protobuf.Done)
				err := tbls.Verify(pairing.NewSuiteBn256(), p.SigPK, coinName, payload.CoinShare) //verifyshare("Done"||ID||r)

				if err == nil {
					coins = append(coins, payload.CoinShare)
					if len(coins) > int(p.F) {
						doneFlagChannel <- true
					}
					if len(coins) > int(2*p.F) {
						coin, _ := tbls.Recover(pairing.NewSuiteBn256(), p.SigPK, coinName, coins, int(2*p.F+1), int(p.N))
						l := utils.BytesToUint32(coin) % p.N //leader of round r
						thisRoundLeader <- l                 //for message handler
						leaderChannel <- l                   //for main process
						return
					}
				}
			}

		}
	}()

	l := <-thisRoundLeader

	//HaltMessage Handler
	go func() {
		for j := uint32(0); j < p.N; j = (j + 1) % p.N {
			select {
			case <-ctx.Done():
				return
			default:
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
						haltChannel <- payload.Value
					}

				}
			}
		}

	}()

	//PreVoteMessage Handler
	go func() {
		PNr := [][]byte{}
		for j := uint32(0); j < p.N; j = (j + 1) % p.N {
			select {
			case <-ctx.Done():
				return
			default:
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
							preVoteFlagChannel <- true
							preVoteYesChannel <- payload.Value
							preVoteYesChannel <- payload.Sig
							preVoteYesChannel <- sigShare
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
								preVoteFlagChannel <- false
								preVoteNoChannel <- noSignature
								preVoteNoChannel <- sigShare
							}
						}
					}
				}
			}
		}

	}()

	//VoteMessage Handler
	go func() {
		VYr := [][]byte{}
		VNr := [][]byte{}
		for j := uint32(0); j < p.N; j = (j + 1) % p.N {
			select {
			case <-ctx.Done():
				return
			default:
				m, ok := p.GetMessage(j, "Vote", IDr)
				if ok {
					payload := core.Decapsulation("Vote", m).(*protobuf.Vote)
					if payload.Vote {
						h := sha3.Sum512(payload.Value)
						var buf bytes.Buffer
						buf.Write([]byte("Echo"))
						buf.Write(IDrj[l])
						buf.WriteByte(1)
						buf.Write(h[:])
						sm := buf.Bytes()
						err1 := bls.Verify(pairing.NewSuiteBn256(), p.SigPK.Commit(), sm, payload.Sig) //verify("Echo"||ID||r||l||1||h)
						sm[len([]byte("Echo"))+len(IDrj[l])] = 2
						err2 := tbls.Verify(pairing.NewSuiteBn256(), p.SigPK, sm, payload.Sigshare) //verifyShare("Echo"||ID||r||l||2||h)
						if err1 == nil && err2 == nil {
							VYr = append(VYr, payload.Sigshare)
							if len(VYr) > int(2*p.F) {
								sig, _ := tbls.Recover(pairing.NewSuiteBn256(), p.SigPK, sm, VYr, int(2*p.F+1), int(p.N))
								voteFlagChannel <- 0
								voteYesChannel <- payload.Value
								voteYesChannel <- sig
							} else if len(VYr)+len(VNr) > int(2*p.F) {
								voteFlagChannel <- 2
								voteOtherChannel <- payload.Value
								voteOtherChannel <- payload.Sig
							}
						}
					} else {
						var buf1 bytes.Buffer
						buf1.WriteByte(byte(0)) //false
						buf1.Write(IDr)
						sm1 := buf1.Bytes()
						err1 := bls.Verify(pairing.NewSuiteBn256(), p.SigPK.Commit(), sm1, payload.Sig) //verify(false||ID||r)

						var buf2 bytes.Buffer
						buf2.Reset()
						buf2.Write([]byte("Unlock"))
						buf2.Write(IDr)
						sm2 := buf2.Bytes()
						err2 := tbls.Verify(pairing.NewSuiteBn256(), p.SigPK, sm2, payload.Sigshare) //verifyShare("Unlock"||ID||r)
						if err1 == nil && err2 == nil {
							VNr = append(VNr, payload.Sigshare)
							if len(VNr) > int(2*p.F) {
								sig, _ := tbls.Recover(pairing.NewSuiteBn256(), p.SigPK, sm2, VYr, int(2*p.F+1), int(p.N))
								voteFlagChannel <- 1
								voteNoChannel <- sig
							}
						}
					}
				}
			}
		}

	}()
}
