package blame

import (
	"sync"

	btss "github.com/binance-chain/tss-lib/tss"
	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/rs/zerolog/log"
)

func NewBlameManager() *Manager {
	return &Manager{
		logger:          log.With().Str("module", "communication").Logger(),
		partyInfo:       nil,
		PartyIDtoP2PID:  make(map[string]peer.ID),
		lastUnicastPeer: make(map[string][]peer.ID),
		shareMgr:        NewTssShareMgr(),
		roundMgr:        NewTssRoundMgr(),
		blame:           &Blame{},
		lastMsgLocker:   &sync.RWMutex{},
	}
}

func (m *Manager) GetBlame() *Blame {
	return m.blame
}

func (m *Manager) GetShareMgr() *TssShareMgr {
	return m.shareMgr
}

func (m *Manager) GetRoundMgr() *TssRoundMgr {
	return m.roundMgr
}

func (m *Manager) SetLastMsg(lastMsg btss.Message) {
	m.lastMsgLocker.Lock()
	defer m.lastMsgLocker.Unlock()
	m.lastMsg = lastMsg
}

func (m *Manager) GetLastMsg() btss.Message {
	m.lastMsgLocker.RLock()
	defer m.lastMsgLocker.RUnlock()
	return m.lastMsg
}

func (m *Manager) SetPartyInfo(party btss.Party, partyIDMap map[string]*btss.PartyID) {
	partyInfo := &PartyInfo{
		Party:      party,
		PartyIDMap: partyIDMap,
	}
	m.partyInfo = partyInfo
}

func (m *Manager) SetLastUnicastPeer(peerID peer.ID, roundInfo string) {
	l, ok := m.lastUnicastPeer[roundInfo]
	if !ok {
		peerList := []peer.ID{peerID}
		m.lastUnicastPeer[roundInfo] = peerList
	} else {
		l = append(l, peerID)
		m.lastUnicastPeer[roundInfo] = l
	}
}
