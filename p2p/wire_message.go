package p2p

import (
	btss "github.com/binance-chain/tss-lib/tss"
)

// WireMessage the message that produced by tss-lib package
type WireMessage struct {
	Routing   *btss.MessageRouting `json:"routing"`
	RoundInfo string               `json:"round_info"`
	Message   []byte               `json:"message"`
	MessageID string               `json:"message_id"`
}
