package p2p

import (
	"sort"
	"time"
)

type CeremonyStatus int

const (
	NA CeremonyStatus = iota
	GatheringParties
	Failed
	Finished
)

// Ceremony can be a keygen ceremony or key sign ceremony
type Ceremony struct {
	ID                string         // ID , it should be the hash of keygen / keysign payload
	Threshold         uint32         // how many peers are required to finish the ceremony
	JoinPartyRequests []*JoinParty   // all the join party requests
	Status            CeremonyStatus // ceremony status
	Started           time.Time      // when the ceremony created
}

// IsReady do we have enough players to start the game?
func (c *Ceremony) IsReady() bool {
	// we got enough nodes
	if len(c.JoinPartyRequests) >= int(c.Threshold) {
		return true
	}
	return false
}

// GetParties return a list of peer id that will be doing the upcoming ceremony
func (c *Ceremony) GetParties() []string {
	if !c.IsReady() {
		return nil
	}
	var parties []string
	for _, item := range c.JoinPartyRequests {
		parties = append(parties, item.Peer.String())
	}
	sort.SliceStable(parties, func(i, j int) bool {
		return parties[i] < parties[j]
	})
	return parties
}
