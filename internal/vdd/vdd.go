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

type shareSet struct {
	recoverShare    []infectious.Share //to recover original message
	starShareCommit []infectious.Share //to commit the (p.PID) th shard
}

//CallHelp call for help to other parties
func CallHelp(p *party.HonestParty, ID []byte, vectorHash [][]byte, requiredPID []uint32, msgStarShardChannels []chan []byte, Mr *sync.Map) {
	callMessage := core.Encapsulation("Call", ID, p.PID, &protobuf.Call{
		RequiredPID: requiredPID,
	})
	p.Broadcast(callMessage)

	coder := reedsolomon.NewRScoder(int(p.F+1), int(p.N))

	shares := make([]shareSet, p.N)

	for i := uint32(0); i < p.N; i++ {
		shares[i] = shareSet{
			recoverShare:    []infectious.Share{},
			starShareCommit: []infectious.Share{},
		}
	}
	count := 0
	for {
		select {
		case m := <-p.GetMessage("Strong", ID):
			payload := (core.Decapsulation("Strong", m)).(*protobuf.Strong)
			for i := 0; i < len(payload.ShardID); i++ {
				//already get the originalMessage of payload.ShardID[i]
				_, ok := Mr.Load(payload.ShardID[i])
				if ok {
					continue
				}

				tmpShare1 := infectious.Share{
					Number: int(m.Sender),
					Data:   payload.SenderShard[i],
				}
				shares[payload.ShardID[i]].recoverShare = append(shares[payload.ShardID[i]].recoverShare, tmpShare1)

				tmpShare2 := infectious.Share{
					Number: int(p.PID),
					Data:   payload.ReceiverShard[i],
				}
				shares[payload.ShardID[i]].starShareCommit = append(shares[payload.ShardID[i]].starShareCommit, tmpShare2)

				if len(shares[payload.ShardID[i]].starShareCommit) == int(p.F+1) { //TODO: check equals
					shares[payload.ShardID[i]].recoverShare = append(shares[payload.ShardID[i]].recoverShare, tmpShare2)
					msgStarShardChannels[payload.ShardID[i]] <- payload.ReceiverShard[i]
				}
				if len(shares[payload.ShardID[i]].recoverShare) > int(2*p.F) { //start OEC
					//OEC
					resultMesssage := coder.Decode(shares[payload.ShardID[i]].recoverShare)
					h := sha3.Sum512(resultMesssage)
					if bytes.Equal(h[:], vectorHash[payload.ShardID[i]]) {
						_, ok := Mr.LoadOrStore(payload.ShardID[i], resultMesssage)
						if !ok {
							count++
							if count >= len(requiredPID) {
								return
							}
						}
					}
				}
			}
		case m := <-p.GetMessage("Weak", ID):
			payload := (core.Decapsulation("Weak", m)).(*protobuf.Weak)
			for i := 0; i < len(payload.ShardID); i++ {
				//already get the originalMessage of payload.ShardID[i]
				_, ok := Mr.Load(payload.ShardID[i])
				if ok {
					continue
				}
				tmpShare1 := infectious.Share{
					Number: int(m.Sender),
					Data:   payload.SenderShard[i],
				}
				shares[payload.ShardID[i]].recoverShare = append(shares[payload.ShardID[i]].recoverShare, tmpShare1)
				if len(shares[payload.ShardID[i]].recoverShare) > int(2*p.F) {
					//OEC
					resultMesssage := coder.Decode(shares[payload.ShardID[i]].recoverShare)
					h := sha3.Sum512(resultMesssage)
					if bytes.Equal(h[:], vectorHash[payload.ShardID[i]]) {
						_, ok := Mr.LoadOrStore(payload.ShardID[i], resultMesssage)
						if !ok {
							count++
							if count == len(requiredPID) {
								return
							}
						}
					}
				}
			}
		}
	}

}

//HelpListener listen to callMessage form other parties
func HelpListener(p *party.HonestParty, ID []byte, Mr *sync.Map, msgStarShardChannels []chan []byte) {
	for {
		m := <-p.GetMessage("Call", ID)
		payload := (core.Decapsulation("Call", m)).(*protobuf.Call)

		strongHelpPID := []uint32{}
		strongHelpMsg := [][]byte{}

		weakHelpPID := []uint32{}

		for _, requiredPID := range payload.RequiredPID {
			value, ok := Mr.Load(requiredPID)
			if ok {
				strongHelpPID = append(strongHelpPID, requiredPID)
				strongHelpMsg = append(strongHelpMsg, value.([]byte))
			} else {
				weakHelpPID = append(weakHelpPID, requiredPID)
			}
		}
		go StrongHelp(p, ID, m.Sender, strongHelpPID, strongHelpMsg)
		go WeakHelp(p, ID, m.Sender, weakHelpPID, msgStarShardChannels)
	}
}

//StrongHelp reply strong help message
func StrongHelp(p *party.HonestParty, ID []byte, caller uint32, strongHelpPID []uint32, strongHelpMsg [][]byte) {
	coder := reedsolomon.NewRScoder(int(p.F+1), int(p.N))
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

//WeakHelp reply weak help message
func WeakHelp(p *party.HonestParty, ID []byte, caller uint32, weakHelpPID []uint32, msgStarShardChannels []chan []byte) {
	senderShard := make([][]byte, len(weakHelpPID))
	for i, pid := range weakHelpPID {
		senderShard[i] = <-msgStarShardChannels[pid]
		msgStarShardChannels[pid] <- senderShard[i]
	}

	weakMessage := core.Encapsulation("Weak", ID, p.PID, &protobuf.Weak{
		ShardID:     weakHelpPID,
		SenderShard: senderShard,
	})

	p.Send(weakMessage, caller)

}
