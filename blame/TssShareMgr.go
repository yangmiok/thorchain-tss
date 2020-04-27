package blame

import (
	"sync"
)

func NewTssShareMgr() *TssShareMgr {
	return &TssShareMgr{
		reqLocker: &sync.Mutex{},
		requested: make(map[string]bool),
	}
}

func (sm *TssShareMgr) SetRequest(key string) {
	sm.reqLocker.Lock()
	defer sm.reqLocker.Unlock()
	sm.requested[key] = true
}

func (sm *TssShareMgr) QueryAndDelete(key string) bool {
	sm.reqLocker.Lock()
	defer sm.reqLocker.Unlock()
	_, ok := sm.requested[key]
	if ok {
		delete(sm.requested, key)
		return true
	}
	return false
}
