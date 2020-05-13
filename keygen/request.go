package keygen

// Request request to do keygen
type Request struct {
	Keys         []string `json:"keys"`
	StopPhase    string   `json:"stop_phase"`
	ChangedPeers []string `json:"changed_peers"`
}

// NewRequest creeate a new instance of keygen.Request
func NewRequest(keys []string, stopPhase string, changedPeers []string) Request {
	return Request{
		Keys:         keys,
		StopPhase:    stopPhase,
		ChangedPeers: changedPeers,
	}
}
