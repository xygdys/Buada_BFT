package mvba

import (
	"Buada_BFT/internal/mvba/smvba"
	"Buada_BFT/internal/party"
	"Buada_BFT/pkg/protobuf"
	"Buada_BFT/pkg/utils"
	"bytes"
	"sync"
)

//MainProcess is the main process of mvba instances
func MainProcess(p *party.HonestParty, ID []byte, value []byte, validation []byte) []byte {

	Sr := sync.Map{} //Lock Set
	pdResult := make(chan []byte, 2)

	//Initialize PD instances

	IDj := make([][]byte, 0, p.N)
	for j := uint32(0); j < p.N; j++ {
		var buf bytes.Buffer
		buf.Write(ID)
		buf.Write(utils.Uint32ToBytes(j))
		IDj = append(IDj, buf.Bytes())
	}

	for i := uint32(0); i < p.N; i++ {
		go func(j uint32) {
			vc, shard, proof1, proof2, ok := PDReceiver(p, j, IDj[j])
			if ok { //save Store
				Sr.Store(j, &protobuf.Store{
					Vc:     vc,
					Shard:  shard,
					Proof1: proof1,
					Proof2: proof2,
				})
			}
		}(i)
	}

	//Run this party's PD instance
	go func() {
		var buf bytes.Buffer
		buf.Write(value)
		buf.Write(validation)
		buf.Write(utils.IntToBytes(len(validation))) //last 4 bytes is the length of validation
		valueAndValidation := buf.Bytes()

		vc, sig := PDSender(p, IDj[p.PID], valueAndValidation)
		pdResult <- vc
		pdResult <- sig

	}()

	//waiting until pd
	vc := <-pdResult
	sig := <-pdResult

	//vc -> pid||vc
	var buf bytes.Buffer
	buf.Write(utils.Uint32ToBytes(p.PID))
	buf.Write(vc)
	idAndVC := buf.Bytes()

	for r := uint32(0); ; r++ {
		var buf bytes.Buffer
		buf.Write(ID)
		buf.Write(utils.Uint32ToBytes(r))
		IDr := buf.Bytes()

		//run underlying smvba
		leaderAndVC := smvba.MainProcess(p, IDr, idAndVC, sig)
		leader := utils.BytesToUint32(leaderAndVC[:4])
		leaderVC := leaderAndVC[4:]

		//recast
		tmp, ok1 := Sr.Load(leader)
		var valueAndValidation []byte
		var ok2 bool
		if ok1 {
			//have leader's Store
			valueAndValidation, ok2 = Recast(p, IDr, leader, leaderVC, tmp.(*protobuf.Store).Shard, tmp.(*protobuf.Store).Proof1, tmp.(*protobuf.Store).Proof2)
		} else {
			//don't have leader's Store
			valueAndValidation, ok2 = Recast(p, IDr, leader, leaderVC, nil, nil, nil)
		}
		if ok2 {
			validationLen := utils.BytesToUint32(valueAndValidation[len(valueAndValidation)-4:])
			resultValue := valueAndValidation[:len(valueAndValidation)-int(validationLen)-4]
			//resultValidation := valueAndValidation[len(valueAndValidation)-int(validationLen):]
			//TODO:Check whether resultValue is valid
			return resultValue
		}
		//otherwise: recast failed, goto next round
	}

}
