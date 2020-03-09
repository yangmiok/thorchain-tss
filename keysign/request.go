package keysign

// Request request to sign a message
type Request struct {
	PoolPubKey    string   `json:"pool_pub_key"` // pub key of the pool that we would like to send this message from
	Messages      []string `json:"messages"`     // base64 encoded messages to be signed
	SignerPubKeys []string `json:"signer_pub_keys"`
}

func NewRequest(pk string, msgs, signers []string) Request {
	return Request{
		PoolPubKey:    pk,
		Messages:      msgs,
		SignerPubKeys: signers,
	}
}
