package acs

import (
	"Buada_BFT/internal/mvba"
	"Buada_BFT/internal/party"
	"Buada_BFT/internal/pb"
	"Buada_BFT/internal/vdd"
	"Buada_BFT/pkg/core"
	"Buada_BFT/pkg/protobuf"
	"Buada_BFT/pkg/utils"
	"bytes"
	"context"
	"log"
	"sync"

	"go.dedis.ch/kyber/v3/pairing"
	"go.dedis.ch/kyber/v3/sign/bls"
	"google.golang.org/protobuf/proto"
)

//BuadaACS is the main process of Buada-ACS
func BuadaACS(p *party.HonestParty, r uint32, proposal []byte) *sync.Map {
	ctx, cancel := context.WithCancel(context.Background())

	Pr := sync.Map{} //Proposal Set
	mvbaValueChannel := make(chan []byte, 1)
	mvbaValidationChannel := make(chan []byte, 1)

	//Initialize PB instances
	ID := utils.Uint32ToBytes(r)
	IDj := make([][]byte, 0, p.N)
	for j := uint32(0); j < p.N; j++ {
		var buf bytes.Buffer
		buf.Write(ID)
		buf.Write(utils.Uint32ToBytes(j))
		IDj = append(IDj, buf.Bytes())
	}
	for i := uint32(0); i < p.N; i++ {
		go func(j uint32) {
			value, _, ok := pb.Receiver(ctx, p, j, IDj[j], nil)
			if ok {
				Pr.Store(j, value)
			}
		}(i)
	}

	//Run this party's PB instance
	go func() {
		h, sig, ok := pb.Sender(ctx, p, IDj[p.PID], proposal, nil)
		if ok {
			finalMessage := core.Encapsulation("Final", ID, p.PID, &protobuf.Final{
				Hash: h,
				Sig:  sig,
			})
			p.Broadcast(finalMessage)
		}
	}()

	//Run Final Message Handler
	go func() {
		pids := []uint32{}
		hashes := [][]byte{}
		sigs := [][]byte{}
		for {
			m := <-p.GetMessage("Final", ID)
			payload := core.Decapsulation("Final", m).(*protobuf.Final)
			var buf bytes.Buffer
			buf.Write([]byte("Echo"))
			buf.Write(IDj[m.Sender])
			buf.Write(payload.Hash)
			sm := buf.Bytes()
			err := bls.Verify(pairing.NewSuiteBn256(), p.SigPK.Commit(), sm, payload.Sig) //verify("Echo"||ID||j||h)
			if err == nil {
				pids = append(pids, m.Sender)
				hashes = append(hashes, payload.Hash)
				sigs = append(sigs, payload.Sig)
			}

			if len(pids) == int(2*p.F+1) {
				value, err1 := proto.Marshal(&protobuf.FinalSetValue{
					Pid:  pids,
					Hash: hashes,
				})
				validation, err2 := proto.Marshal(&protobuf.FinalSetValidation{
					Sig: sigs,
				})
				if err1 != nil || err2 != nil {
					log.Fatalln(err1, err2)
				}
				mvbaValueChannel <- value
				mvbaValidationChannel <- validation
			}

		}
	}()

	//wating until MVBA output
	value := <-mvbaValueChannel
	validation := <-mvbaValidationChannel
	resultValue := mvba.MainProcess(p, ID, value, validation)

	//fmt.Println(resultValue)

	//call help
	var finalSet protobuf.FinalSetValue
	proto.Unmarshal(resultValue, &finalSet)
	resulutSet := vdd.CallHelp(p, ID, &finalSet, &Pr)

	cancel()
	return resulutSet
}
