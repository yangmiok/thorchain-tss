package blame

import (
	"errors"
	"sync"

	"github.com/rs/zerolog"

	btss "github.com/binance-chain/tss-lib/tss"
	"github.com/libp2p/go-libp2p-core/peer"

	"gitlab.com/thorchain/tss/go-tss/messages"
)

const (
	HashCheckFail = "hash check failed"
	TssTimeout    = "Tss timeout"
	TssSyncFail   = "signers fail to sync before keygen/keysign"
	InternalError = "fail to start the join party "
)

var (
	ErrHashFromOwner     = errors.New(" hash sent from data owner")
	ErrNotEnoughPeer     = errors.New("not enough nodes to evaluate hash")
	ErrMsgHashCheck      = errors.New("message we received does not match the majority")
	ErrHashFromPeer      = errors.New("hashcheck error from peer")
	ErrTssTimeOut        = errors.New("error Tss Timeout")
	ErrHashCheck         = errors.New("error in processing hash check")
	ErrHashInconsistency = errors.New("fail to agree on the hash value")
)

type TssShareMgr struct {
	requested map[string]bool
	reqLocker *sync.Mutex
}

type TssRoundMgr struct {
	storedMsg   map[string]*messages.WireMessage
	storeLocker *sync.Mutex
}

// PartyInfo the information used by tss key gen and key sign
type PartyInfo struct {
	Party      btss.Party
	PartyIDMap map[string]*btss.PartyID
}

type Manager struct {
	logger          zerolog.Logger
	blame           *Blame
	lastUnicastPeer map[string][]peer.ID
	shareMgr        *TssShareMgr
	roundMgr        *TssRoundMgr
	partyInfo       *PartyInfo
	PartyIDtoP2PID  map[string]peer.ID
	lastMsgLocker   *sync.RWMutex
	lastMsg         btss.Message
}

type Node struct {
	Pubkey         string `json:"pubkey"`
	BlameData      []byte `json:"data"`
	BlameSignature []byte `json:"signature"`
}

// Blame is used to store the blame nodes and the fail reason
type Blame struct {
	FailReason string `json:"fail_reason"`
	BlameNodes []Node `json:"blame_peers,omitempty"`
}
