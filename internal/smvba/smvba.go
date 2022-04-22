package smvba

import (
	"Buada_BFT/internal/party"
	"Buada_BFT/pkg/core"
	"Buada_BFT/pkg/protobuf"
	"Buada_BFT/pkg/utils"
	"bytes"
	"sync"

	"go.dedis.ch/kyber/v3/pairing"
	"go.dedis.ch/kyber/v3/sign/bls"
	"golang.org/x/crypto/sha3"
)

//MainProcess is the main process of smvba instances
func MainProcess(p *party.HonestParty, ID []byte, value []byte, validation []byte) ([]byte, []byte) {
	for r := uint32(0); ; r++ {

		//TODO: CheckValue

		//Initialize SPB instances
		var idBuf bytes.Buffer
		idBuf.Write(ID)
		idBuf.Write(utils.Uint32ToBytes(r))
		IDrj := [][]byte{}
		for j := uint32(0); j < p.N; j++ {
			idBuf.Write(utils.Uint32ToBytes(j))
			IDrj = append(IDrj, idBuf.Bytes())
			go SPBReceiver(p, j, IDrj[len(IDrj)-1])
			idBuf.Truncate(len(IDrj[len(IDrj)-1]) - 4)
		}
		IDr := idBuf.Bytes()

		//Run this party's SPB instance
		go func() {
			value, sig := SPBSender(p, IDrj[p.PID], value, validation)
			finishMessage := core.Encapsulation("Finish", IDr, p.PID, &protobuf.Finish{
				Value:     value,
				Signature: sig,
			})
			p.Broadcast(finishMessage)
		}()

		//Run Message Handlers
		Fr := sync.Map{}
		doneFlagChannel := make(chan bool, 1)
		go messageHandler(p, IDr, IDrj, &Fr, doneFlagChannel)

		//doneFlag -> common coin
		for doneFlag := <-doneFlagChannel; doneFlag; {
			l := election(p, IDr)

			//TODO:shut down all SPB instances

			value, ok := Fr.Load(l)
			if ok {
				finish := value.(*protobuf.Finish)
				haltMessage := core.Encapsulation("Halt", IDr, p.PID, &protobuf.Halt{
					Value:     finish.Value,
					Signature: finish.Signature,
				})
				p.Broadcast(haltMessage)
				return finish.Value, finish.Signature //short-cut

			}
			break //run once per round
		}

	}
}

func messageHandler(p *party.HonestParty, IDr []byte, IDrj [][]byte, Fr *sync.Map, doneFlagChannel chan bool) {
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
}
