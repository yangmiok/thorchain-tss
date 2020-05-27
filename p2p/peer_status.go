package p2p

import (
	"errors"
	"sync"

	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/libp2p/go-libp2p-core/protocol"

	"gitlab.com/thorchain/tss/go-tss/conversion"
)

type PeerStatus struct {
	peersResponse  map[peer.ID]bool
	peerProtocol   map[peer.ID][]string
	peerStatusLock *sync.RWMutex
	newFound       chan bool
}

func NewPeerStatus(peerNodes []peer.ID, myPeerID peer.ID) *PeerStatus {
	dat := make(map[peer.ID]bool)
	for _, el := range peerNodes {
		if el == myPeerID {
			continue
		}
		dat[el] = false
	}
	peerStatus := &PeerStatus{
		peersResponse:  dat,
		peerProtocol:   make(map[peer.ID][]string),
		peerStatusLock: &sync.RWMutex{},
		newFound:       make(chan bool, len(peerNodes)),
	}
	return peerStatus
}

func (ps *PeerStatus) getCoordinationStatus() bool {
	_, offline := ps.getPeersStatus()
	return len(offline) == 0
}

func (ps *PeerStatus) getPeersStatus() ([]peer.ID, []peer.ID) {
	var online []peer.ID
	var offline []peer.ID
	ps.peerStatusLock.RLock()
	defer ps.peerStatusLock.RUnlock()
	for peerNode, val := range ps.peersResponse {
		if val {
			online = append(online, peerNode)
		} else {
			offline = append(offline, peerNode)
		}
	}

	return online, offline
}

func (ps *PeerStatus) getProtocol() (protocol.ID, error) {
	ps.peerStatusLock.Lock()
	defer ps.peerStatusLock.Unlock()
	protocols := make(map[string]string)
	for k, v := range ps.peerProtocol {
		for idx, el := range v {
			protocols[k.String()+string(idx)] = el
		}
	}
	ret, _, err := conversion.GetHighestFreq(protocols)
	if err != nil {
		return "", err
	}
	proto := protocol.ConvertFromStrings([]string{ret})[0]
	for _, el := range TssProtocols {
		if proto == el {
			return proto, nil
		}
	}
	return "", errors.New("unknown protocol")
}

func (ps *PeerStatus) updatePeer(peerNode peer.ID, protos []string) (bool, error) {
	ps.peerStatusLock.Lock()
	defer ps.peerStatusLock.Unlock()
	val, ok := ps.peersResponse[peerNode]
	if !ok {
		return false, errors.New("key not found")
	}
	if !val {
		ps.peersResponse[peerNode] = true
		ps.peerProtocol[peerNode] = protos
		return true, nil
	}
	return false, nil
}
