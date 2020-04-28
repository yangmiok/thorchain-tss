package keygen

// Request request to do keygen
type Request struct {
	Keys            []string `json:"keys"`
	StopPhase       string   `json:"stop_phase"`
	ChangedPeers    []string `json:"changed_peers"`
	WrongSharePeers []string `json:"wrong_share_peers"`
	WrongShare      []byte   `json:"share"`
}

// NewRequest creeate a new instance of keygen.Request
func NewRequest(keys []string, stopPhase string, changedPeers []string, wrongSharePeers []string, wrongShare []byte) Request {
	return Request{
		Keys:            keys,
		StopPhase:       stopPhase,
		ChangedPeers:    changedPeers,
		WrongSharePeers: wrongSharePeers,
		WrongShare:      wrongShare,
	}
}
