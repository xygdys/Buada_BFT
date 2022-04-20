package smvba

import (
	"Buada_BFT/internal/party"
	"Buada_BFT/pkg/core"
	"Buada_BFT/pkg/protobuf"
	"bytes"
	"encoding/binary"
)

func MainProcess(p *party.HonestParty, ID []byte, value []byte, validation []byte) {
	for r := uint32(0); ; r++ {

		//TODO: CheckValue

		//Initialize SPB instances
		var buf bytes.Buffer
		buf.Write(ID)
		buf.Write(Uint32ToBytes(r))
		for j := uint32(0); j < p.N; j++ {
			buf.Write(Uint32ToBytes(j))
			IDrj := buf.Bytes()
			go SPBReceiver(p, j, IDrj)
			buf.Truncate(len(IDrj) - 4)
		}
		IDr := buf.Bytes()
		buf.Write(Uint32ToBytes(p.PID))
		IDri := buf.Bytes()

		//run this party's SPB instance
		go func() {
			value, sig := SPBSender(p, IDri, value, validation)
			finishMessage := core.Encapsulation("Finish", IDr, p.PID, &protobuf.Finish{
				Value:     value,
				Signature: sig,
			})
			p.Broadcast(finishMessage)
		}()

		//run Message Handlers
		go MessageHandler(p, IDr)

	}
}

func MessageHandler(p *party.HonestParty, ID []byte) {
	go func() {
		for {
			for j := uint32(0); j < p.N; j++ {
				p.GetMessage(j, "Finish", ID)
			}
		}
	}()
}

func Uint32ToBytes(n uint32) []byte {
	bytebuf := bytes.NewBuffer([]byte{})
	binary.Write(bytebuf, binary.BigEndian, n)
	return bytebuf.Bytes()
}
