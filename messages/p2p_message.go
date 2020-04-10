package messages

import (
	"fmt"

	btss "github.com/binance-chain/tss-lib/tss"
	"github.com/libp2p/go-libp2p-core/peer"
)

// THORChainTSSMessageType  represent the messgae type used in THORChain TSS
type THORChainTSSMessageType uint8

const (
	// TSSKeyGenMsg is the message directly generated by tss-lib package
	TSSKeyGenMsg THORChainTSSMessageType = iota
	// TSSKeySignMsg is the message directly generated by tss lib for sign
	TSSKeySignMsg
	// TSSKeyGenVerMsg is the message we create on top to make sure everyone received the same message
	TSSKeyGenVerMsg
	// TSSKeySignVerMsg is the message we create to make sure every party receive the same broadcast message
	TSSKeySignVerMsg
	// Unknown is the message indicates the undefined message type
	Unknown
)

// String implement fmt.Stringer
func (msgType THORChainTSSMessageType) String() string {
	switch msgType {
	case TSSKeyGenMsg:
		return "TSSKeyGenMsg"
	case TSSKeySignMsg:
		return "TSSKeySignMsg"
	case TSSKeyGenVerMsg:
		return "TSSKeyGenVerMsg"
	case TSSKeySignVerMsg:
		return "TSSKeySignVerMsg"
	default:
		return "Unknown"
	}
}

// WrappedMessage is a message with type in it
type WrappedMessage struct {
	MessageType THORChainTSSMessageType `json:"message_type"`
	MsgID       string                  `json:"message_id"`
	Payload     []byte                  `json:"payload"`
}

// BroadcastMsgChan is the channel structure for keygen/keysign submit message to p2p network
type BroadcastMsgChan struct {
	WrappedMessage WrappedMessage
	PeersID        []peer.ID
}

// BroadcastConfirmMessage is used to broadcast to all parties what message they receive
type BroadcastConfirmMessage struct {
	P2PID string `json:"P2PID"`
	Key   string `json:"key"`
	Hash  string `json:"hash"`
}

// WireMessage the message that produced by tss-lib package
type WireMessage struct {
	Routing   *btss.MessageRouting `json:"routing"`
	RoundInfo string               `json:"round_info"`
	Message   []byte               `json:"message"`
	Sig       []byte               `json:"signature"`
	Key       []byte               `json:"key"`
}

// GetCacheKey return the key we used to cache it locally
func (m *WireMessage) GetCacheKey() string {
	return fmt.Sprintf("%s-%s", m.Routing.From.Id, m.RoundInfo)
}
