package common

import (
	"gitlab.com/thorchain/tss/go-tss/messages"
	"sync"
)

func NewTssMsgStored() TssMsgStored {
	return TssMsgStored{reqLocker: &sync.Mutex{},
		storeLocker: &sync.Mutex{},
		storedMsg:   make(map[string]*messages.WireMessage),
		requested:   make(map[string]bool)}
}

func (sm *TssMsgStored) getTssMsgStored(key string) *messages.WireMessage {
	sm.storeLocker.Lock()
	defer sm.storeLocker.Unlock()
	ret, ok := sm.storedMsg[key]
	if !ok {
		return nil
	}
	return ret
}

func (sm *TssMsgStored) storeTssMsg(key string, msg *messages.WireMessage) {
	sm.storeLocker.Lock()
	defer sm.storeLocker.Unlock()
	sm.storedMsg[key] = msg
}

func (sm *TssMsgStored) setRequest(key string) {
	sm.reqLocker.Lock()
	defer sm.reqLocker.Unlock()
	sm.requested[key] = true
}

func (sm *TssMsgStored) queryAndDelete(key string) bool {
	sm.reqLocker.Lock()
	defer sm.reqLocker.Unlock()
	_, ok := sm.requested[key]
	if ok {
		delete(sm.requested, key)
		return true
	}
	return false
}
