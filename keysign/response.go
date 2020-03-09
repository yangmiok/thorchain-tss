package keysign

import "gitlab.com/thorchain/tss/go-tss/common"

// signature
type Signature struct {
	Msg string `json:"signed_msg"`
	R   string `json:"r"`
	S   string `json:"s"`
}

// Response key sign response
type Response struct {
	MsgID      string        `json:"msg_id"`
	Signatures []Signature   `json:"signatures"`
	Status     common.Status `json:"status"`
	Blame      common.Blame  `json:"blame"`
}

func NewResponse(msgID string, signatures []Signature, status common.Status, blame common.Blame) Response {
	return Response{
		MsgID:      msgID,
		Signatures: signatures,
		Status:     status,
		Blame:      blame,
	}
}

func NewSignature(msg, r, s string) Signature {
	return Signature{Msg: msg,
		R: r,
		S: s}
}
