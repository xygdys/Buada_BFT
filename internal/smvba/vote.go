package smvba

import (
	"Buada_BFT/internal/party"
	"Buada_BFT/pkg/core"
	"Buada_BFT/pkg/protobuf"
	"context"
)

func vote(ctx context.Context, p *party.HonestParty, IDr []byte, l uint32, preVoteFlagChannel chan bool, preVoteYesChannel chan []byte, preVoteNoChannel chan []byte) {
	select {
	case <-ctx.Done():
		return
	case VoteFlag := <-preVoteFlagChannel:
		if VoteFlag { //have received a valid Yes PreVote
			value := <-preVoteYesChannel
			sig := <-preVoteYesChannel
			sigShare := <-preVoteYesChannel
			voteMessage := core.Encapsulation("Vote", IDr, p.PID, &protobuf.Vote{
				Vote:     true,
				Value:    value,
				Sig:      sig,
				Sigshare: sigShare,
			})
			p.Broadcast(voteMessage)
		} else { //have received 2f+1 valid No PreVote
			sig := <-preVoteNoChannel
			sigShare := <-preVoteNoChannel
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
