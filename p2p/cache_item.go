package p2p

import (
	"sync"

	"github.com/libp2p/go-libp2p-core/peer"
)

// StandbyMessage is a structure to represent a message we receive , but the other parties didn't confirm yet.
// it lives in memory before it get confirmed and passed along to local keygen/keysign party
type StandbyMessage struct {
	Msg           *WireMessage
	Hash          string
	lock          *sync.Mutex
	ConfirmedList map[peer.ID]string
}

func NewStandbyMessage(msg *WireMessage, hash string) *StandbyMessage {
	return &StandbyMessage{
		Msg:           msg,
		Hash:          hash,
		lock:          &sync.Mutex{},
		ConfirmedList: make(map[peer.ID]string),
	}
}

// UpdateConfirmList add the given party's hash into the confirm list
func (l *StandbyMessage) UpdateConfirmList(id peer.ID, hash string) {
	l.lock.Lock()
	defer l.lock.Unlock()
	l.ConfirmedList[id] = hash
}

// TotalConfirm the number of parties that already confirmed their hash
func (l *StandbyMessage) TotalConfirmed() int {
	l.lock.Lock()
	defer l.lock.Unlock()
	return len(l.ConfirmedList)
}

func (l *StandbyMessage) GetPeers() []peer.ID {
	peers := make([]peer.ID, 0, len(l.ConfirmedList))
	l.lock.Lock()
	defer l.lock.Unlock()
	for p := range l.ConfirmedList {
		peers = append(peers, p)
	}
	return peers
}
