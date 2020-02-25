package keysign

import (
	"gitlab.com/thorchain/tss/go-tss/common"
)

// KeySignReq request to sign a message
type KeySignReq struct {
	PoolPubKey    string   `json:"pool_pub_key"`    // pub key of the pool that we would like to send this message from
	Message       string   `json:"message"`         // base64 encoded message to be signed
	SignersPubKey []string `json:"signers_pub_key"` // all the signers that involved in this round's signing
}

// KeySignResp key sign response
type KeySignResp struct {
	Msg    string        `json:"message"`   // base64 encoded message that have been signed
	R      string        `json:"r"`
	S      string        `json:"s"`
	Status common.Status `json:"status"`
	Blame  common.Blame  `json:"blame"`
}

func NewKeySignResp(r, s, msg string, status common.Status, blame common.Blame) KeySignResp {
	return KeySignResp{
		Msg:   msg,
		R:      r,
		S:      s,
		Status: status,
		Blame:  blame,
	}
}
