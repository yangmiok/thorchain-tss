package p2p

import (
	btss "github.com/binance-chain/tss-lib/tss"

	"gitlab.com/thorchain/tss/go-tss/common"
)

// GetRanddomWireMessage
func GetRandomWireMessage(isBroadcast bool) *WireMessage {
	return &WireMessage{
		Routing: &btss.MessageRouting{
			From: GetRandomPartyID(),
			To: []*btss.PartyID{
				GetRandomPartyID(),
			},
			IsBroadcast:             isBroadcast,
			IsToOldCommittee:        false,
			IsToOldAndNewCommittees: false,
		},
		RoundInfo: RandStringBytesMask(64),
		Message:   []byte(RandStringBytesMask(64)),
		MessageID: RandStringBytesMask(64),
	}
}

func GetRandomPartyID() *btss.PartyID {
	k, err := common.MsgToHashInt([]byte(RandStringBytesMask(64)))
	if err != nil {
		panic(err)
	}
	return btss.NewPartyID(RandStringBytesMask(10), RandStringBytesMask(10), k)
}
