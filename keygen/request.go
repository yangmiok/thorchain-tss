package keygen

// Request request to do keygen
type Request struct {
	Keys   []string `json:"keys"`
	Protos []string `json:"protocols"`
}

// NewRequest creeate a new instance of keygen.Request
func NewRequest(keys, protos []string) Request {
	return Request{
		Keys:   keys,
		Protos: protos,
	}
}
