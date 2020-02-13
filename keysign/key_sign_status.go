package keysign

import (
	"gitlab.com/thorchain/tss/go-tss/common"
)

// KeySignReq request to sign a message
type KeySignReq struct {
	PoolPubKey string   `json:"pool_pub_key"` // pub key of the pool that we would like to send this message from
	Messages   []string `json:"messages"`     // base64 encoded message to be signed
}

// KeySignResp key sign response with batch signatures
type KeySignRespBatch struct {
	KeySignResp []KeySignResp
}

// Signature for each keysign request message
type Signature struct {
	R      string        `json:"r"`
	S      string        `json:"s"`
	Status common.Status `json:"status"`
	Blame  common.Blame  `json:"Blame"`
}

type KeySignResp struct {
	Msg string `json:"request_msg"`
	Sig Signature
}
