package vdd //vector data dissemination

import (
	"Buada_BFT/internal/party"
	"Buada_BFT/pkg/core"
	"Buada_BFT/pkg/protobuf"
	"Buada_BFT/pkg/reedsolomon"
	"bytes"
	"sync"

	"github.com/vivint/infectious"
	"golang.org/x/crypto/sha3"
)

type requiredMsg struct {
	pid  uint32
	hash []byte

	//for weakHelp message
	star      []byte
	starMutex *sync.Mutex

	starWait   *sync.WaitGroup
	starShares []infectious.Share

	finishFlag      bool
	finishFlagMutex *sync.Mutex

	decodeShareChannel chan infectious.Share
}

func (rm *requiredMsg) isFinish() bool {
	rm.finishFlagMutex.Lock()
	result := rm.finishFlag
	rm.finishFlagMutex.Unlock()
	return result
}
func (rm *requiredMsg) Finish() {
	rm.finishFlagMutex.Lock()
	rm.finishFlag = true
	rm.finishFlagMutex.Unlock()
}

func onlineErrorCorrection(p *party.HonestParty, ID []byte, coder *reedsolomon.RScoder, rMs []*requiredMsg, rMsMap map[uint32]int, resultSet *sync.Map, finishWait *sync.WaitGroup) {
	for _, rM := range rMs {
		go func(rM *requiredMsg) {
			decodeShare := []infectious.Share{}
			for {
				decodeShare = append(decodeShare, <-rM.decodeShareChannel)
				if len(decodeShare) > int(2*p.F) {
					resultMesssage, err := coder.Decode(decodeShare)
					if err != nil {
						panic(err)
					}
					h := sha3.Sum512(resultMesssage)
					if bytes.Equal(h[:], rM.hash) {
						resultSet.LoadOrStore(rM.pid, resultMesssage)
						rM.Finish()
						break
					}
				}
			}
			finishWait.Done()
		}(rM)
	}
}

//CallHelp call for help to other parties
func CallHelp(p *party.HonestParty, ID []byte, finalSet *protobuf.FinalSetValue, Pr *sync.Map) *sync.Map {

	finishWait := sync.WaitGroup{}
	haltChannel := make(chan bool, 1)

	resultSet := sync.Map{}
	isRequired := make([]bool, p.N)
	requiredPIDs := []uint32{}

	rMs := []*requiredMsg{}
	rMsMap := map[uint32]int{}

	coder := reedsolomon.NewRScoder(int(p.F+1), int(p.N))

	for i := 0; i < len(finalSet.Pid); i++ {

		value, ok := Pr.Load(finalSet.Pid[i])
		if ok {
			resultSet.Store(finalSet.Pid[i], value)
			//fmt.Println("party", p.PID, "store", finalSet.Pid[i], value)
			isRequired[finalSet.Pid[i]] = false
		} else {
			//new required message
			rMsg := requiredMsg{
				pid:                finalSet.Pid[i],
				hash:               finalSet.Hash[i],
				star:               []byte{},
				starMutex:          new(sync.Mutex),
				starWait:           new(sync.WaitGroup),
				starShares:         []infectious.Share{},
				finishFlag:         false,
				finishFlagMutex:    new(sync.Mutex),
				decodeShareChannel: make(chan infectious.Share, p.N),
			}
			isRequired[finalSet.Pid[i]] = true
			rMsg.starWait.Add(1)
			rMs = append(rMs, &rMsg)
			rMsMap[finalSet.Pid[i]] = len(rMs) - 1

			requiredPIDs = append(requiredPIDs, finalSet.Pid[i])
		}
	}

	go helpListener(p, ID, coder, rMs, rMsMap, &resultSet)

	if len(requiredPIDs) == 0 {
		return &resultSet
	}

	finishWait.Add(len(requiredPIDs))
	go onlineErrorCorrection(p, ID, coder, rMs, rMsMap, &resultSet, &finishWait)
	go func() {
		finishWait.Wait()
		haltChannel <- true
	}()

	callMessage := core.Encapsulation("Call", ID, p.PID, &protobuf.Call{
		RequiredPID: requiredPIDs,
	})
	for i := uint32(0); i < p.N; i++ {
		if i != p.PID {
			p.Send(callMessage, i) //Important:Don't send to itself
		}
	}

	for {
		select {
		case <-haltChannel:
			// x, _ := resultSet.Load(uint32(0))
			// fmt.Println("party", p.PID, x)
			return &resultSet
		case m := <-p.GetMessage("Strong", ID):
			payload := (core.Decapsulation("Strong", m)).(*protobuf.Strong)
			for i := 0; i < len(payload.ShardID); i++ {

				if !isRequired[payload.ShardID[i]] { //not required
					continue
				}
				if rMs[rMsMap[payload.ShardID[i]]].isFinish() {
					continue //already finish the reconstruction of this message
				}

				tmpShare1 := infectious.Share{
					Number: int(m.Sender),
					Data:   payload.SenderShard[i],
				}
				rMs[rMsMap[payload.ShardID[i]]].decodeShareChannel <- tmpShare1

				tmpShare2 := infectious.Share{
					Number: int(p.PID),
					Data:   payload.ReceiverShard[i],
				}
				rMs[rMsMap[payload.ShardID[i]]].starShares = append(rMs[rMsMap[payload.ShardID[i]]].starShares, tmpShare2)

				if len(rMs[rMsMap[payload.ShardID[i]]].starShares) == int(p.F+1) { //TODO: check equals
					rMs[rMsMap[payload.ShardID[i]]].decodeShareChannel <- tmpShare2

					rMs[rMsMap[payload.ShardID[i]]].starMutex.Lock()
					rMs[rMsMap[payload.ShardID[i]]].star = tmpShare2.Data
					rMs[rMsMap[payload.ShardID[i]]].starMutex.Unlock()

					rMs[rMsMap[payload.ShardID[i]]].starWait.Done()
				}
			}
		case m := <-p.GetMessage("Weak", ID):
			payload := (core.Decapsulation("Weak", m)).(*protobuf.Weak)
			for i := 0; i < len(payload.ShardID); i++ {
				if !isRequired[payload.ShardID[i]] { //not required
					continue
				}
				if rMs[rMsMap[payload.ShardID[i]]].isFinish() {
					continue //already finish the reconstruction of this message
				}
				tmpShare1 := infectious.Share{
					Number: int(m.Sender),
					Data:   payload.SenderShard[i],
				}
				rMs[rMsMap[payload.ShardID[i]]].decodeShareChannel <- tmpShare1
			}
		}
	}

}

func helpListener(p *party.HonestParty, ID []byte, coder *reedsolomon.RScoder, rMs []*requiredMsg, rMsMap map[uint32]int, resultSet *sync.Map) {
	for {
		m := <-p.GetMessage("Call", ID)
		payload := (core.Decapsulation("Call", m)).(*protobuf.Call)

		strongHelpPID := []uint32{}
		strongHelpMsg := [][]byte{}

		weakHelpPID := []uint32{}

		for _, requiredPID := range payload.RequiredPID {
			value, ok := resultSet.Load(requiredPID)
			if ok {
				strongHelpPID = append(strongHelpPID, requiredPID)
				strongHelpMsg = append(strongHelpMsg, value.([]byte))
			} else {
				weakHelpPID = append(weakHelpPID, requiredPID)
			}
		}
		go strongHelp(p, ID, coder, m.Sender, strongHelpPID, strongHelpMsg)
		go weakHelp(p, ID, m.Sender, weakHelpPID, rMs, rMsMap)
	}
}

func strongHelp(p *party.HonestParty, ID []byte, coder *reedsolomon.RScoder, caller uint32, strongHelpPID []uint32, strongHelpMsg [][]byte) {
	senderShard := make([][]byte, len(strongHelpMsg))
	receiverShard := make([][]byte, len(strongHelpMsg))
	for i, msg := range strongHelpMsg {
		shares := coder.Encode(msg)
		senderShard[i] = shares[p.PID].Data
		receiverShard[i] = shares[caller].Data
	}

	strongMessage := core.Encapsulation("Strong", ID, p.PID, &protobuf.Strong{
		ShardID:       strongHelpPID,
		SenderShard:   senderShard,
		ReceiverShard: receiverShard,
	})

	p.Send(strongMessage, caller)
}

func weakHelp(p *party.HonestParty, ID []byte, caller uint32, weakHelpPID []uint32, rMs []*requiredMsg, rMsMap map[uint32]int) {
	senderShard := make([][]byte, len(weakHelpPID))
	for i, pid := range weakHelpPID {
		rMs[rMsMap[pid]].starWait.Wait()

		rMs[rMsMap[pid]].starMutex.Lock()
		senderShard[i] = make([]byte, len(rMs[rMsMap[pid]].star))
		copy(senderShard[i], rMs[rMsMap[pid]].star)
		rMs[rMsMap[pid]].starMutex.Unlock()
	}

	weakMessage := core.Encapsulation("Weak", ID, p.PID, &protobuf.Weak{
		ShardID:     weakHelpPID,
		SenderShard: senderShard,
	})

	p.Send(weakMessage, caller)
}
