package keygen

import "gitlab.com/thorchain/tss/go-tss/common"

// Request request to do keygen
type Request struct {
	Keys []string `json:"keys"`
}

// Response keygen response
type Response struct {
	PubKey      string        `json:"pub_key"`
	PoolAddress string        `json:"pool_address"`
	Status      common.Status `json:"status"`
	Blame       common.Blame  `json:"blame"`
}

// NewRequest create a new Request
func NewRequest(keys []string) Request {
	return Request{
		Keys: keys,
	}
}

// NewResponse create a new response
func NewResponse(pk, addr string, status common.Status, blame common.Blame) Response {
	return Response{
		PubKey:      pk,
		PoolAddress: addr,
		Status:      status,
		Blame:       blame,
	}
}
