package p2p

import (
	btss "github.com/binance-chain/tss-lib/tss"
	"github.com/libp2p/go-libp2p-core/peer"
)

// WireMessage the message that produced by tss-lib package
type WireMessage struct {
	RemotePeer peer.ID              `json:"remote_peer"`
	Routing    *btss.MessageRouting `json:"routing"`
	RoundInfo  string               `json:"round_info"`
	Message    []byte               `json:"message"`
	MessageID  string               `json:"message_id"`
}
