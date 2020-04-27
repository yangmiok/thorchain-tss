package blame

import (
	"sync"

	"gitlab.com/thorchain/tss/go-tss/messages"
)

func NewTssRoundMgr() *TssRoundMgr {
	return &TssRoundMgr{
		storeLocker: &sync.Mutex{},
		storedMsg:   make(map[string]*messages.WireMessage),
	}
}

func (tr *TssRoundMgr) GetTssRoundStored(key string) *messages.WireMessage {
	tr.storeLocker.Lock()
	defer tr.storeLocker.Unlock()
	ret, ok := tr.storedMsg[key]
	if !ok {
		return nil
	}
	return ret
}

func (tr *TssRoundMgr) StoreTssRound(key string, msg *messages.WireMessage) {
	tr.storeLocker.Lock()
	defer tr.storeLocker.Unlock()
	tr.storedMsg[key] = msg
}

func (tr *TssRoundMgr) GetNodesForGivenRound(msgType string) []string {
	var standbyNodes []string
	for _, el := range tr.storedMsg {
		if el.RoundInfo == msgType {
			standbyNodes = append(standbyNodes, el.Routing.From.Id)
		}
	}
	return standbyNodes
}
