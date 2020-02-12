package common

import (
	"fmt"

	btss "github.com/binance-chain/tss-lib/tss"
	"github.com/libp2p/go-libp2p-core/peer"
)

// PartyInfo the information used by tss key gen and key sign
type PartyInfo struct {
	MessageID string
	Party     btss.Party
	PartiesID []*btss.PartyID
}

//  GetPeers get all peers with in current keygen/keysign party, but exclude the given peers
func (pi *PartyInfo) GetPeers(exclude []peer.ID) ([]peer.ID, error) {
	var output []peer.ID
	for _, item := range pi.PartiesID {
		shouldExclude := false
		for _, p := range exclude {
			if item.Moniker == p.String() {
				shouldExclude = true
				break
			}
		}
		if !shouldExclude {
			id, err := peer.IDB58Decode(item.Moniker)
			if err != nil {
				return nil, fmt.Errorf("fail to decode peer id: %w", err)
			}
			output = append(output, id)
		}
	}
	return output, nil
}
func (pi *PartyInfo) GetPeersFromParty(parties []*btss.PartyID) ([]peer.ID, error) {
	var output []peer.ID
	for _, item := range pi.PartiesID {
		for _, p := range parties {
			if p.Id == item.Id && p.Moniker == item.Moniker {
				id, err := peer.IDB58Decode(item.Moniker)
				if err != nil {
					return nil, fmt.Errorf("fail to decode peer id: %w", err)

				}
				output = append(output, id)
			}
		}
	}
	return output, nil
}
