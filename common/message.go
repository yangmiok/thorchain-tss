package common

import btss "github.com/binance-chain/tss-lib/tss"

type BulkWireMsg struct {
	WiredMsg      []byte
	MsgIdentifier string
	Routing       *btss.MessageRouting
}

func NewBulkWireMsg(msg []byte, id string, r *btss.MessageRouting) BulkWireMsg {
	return BulkWireMsg{
		WiredMsg:      msg,
		MsgIdentifier: id,
		Routing:       r,
	}
}
