package common

import (
	"sync"

	"gitlab.com/thorchain/tss/go-tss/messages"
)

type TssShareMgr struct {
	storedMsg   map[string]*messages.WireMessage
	requested   map[string]bool
	storeLocker *sync.Mutex
	reqLocker   *sync.Mutex
}

func NewTssShareMgr() *TssShareMgr {
	return &TssShareMgr{
		reqLocker:   &sync.Mutex{},
		storeLocker: &sync.Mutex{},
		storedMsg:   make(map[string]*messages.WireMessage),
		requested:   make(map[string]bool),
	}
}

func (sm *TssShareMgr) getTssMsgStored(key string) *messages.WireMessage {
	sm.storeLocker.Lock()
	defer sm.storeLocker.Unlock()
	return sm.storedMsg[key]
}

func (sm *TssShareMgr) storeTssMsg(key string, msg *messages.WireMessage) {
	sm.storeLocker.Lock()
	defer sm.storeLocker.Unlock()
	sm.storedMsg[key] = msg
}

func (sm *TssShareMgr) setRequest(key string) {
	sm.reqLocker.Lock()
	defer sm.reqLocker.Unlock()
	sm.requested[key] = true
}

func (sm *TssShareMgr) queryAndDelete(key string) bool {
	sm.reqLocker.Lock()
	defer sm.reqLocker.Unlock()
	_, ok := sm.requested[key]
	if ok {
		delete(sm.requested, key)
		return true
	}
	return false
}
