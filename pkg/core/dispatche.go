package core

import (
	"Buada_BFT/pkg/protobuf"
	"sync"
)

// MakeDispatcheChannels dispatche messages from receiveChannel
// and make a double layer Map : (messageType) --> (id) --> (channel)
func MakeDispatcheChannels(receiveChannel chan *protobuf.Message, N uint32) *sync.Map {
	dispatcheChannels := new(sync.Map)

	go func() { //dispatcher
		for {
			m := <-(receiveChannel)
			value1, _ := dispatcheChannels.LoadOrStore(m.Type, new(sync.Map))

			value2, _ := value1.(*sync.Map).LoadOrStore(string(m.Id), make(chan *protobuf.Message, N))

			value2.(chan *protobuf.Message) <- m
			//TODO: check replay attack?
			//TODO: check m.Sender with ip||port

		}
	}()
	return dispatcheChannels
}
