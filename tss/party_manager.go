package tss

import (
	"errors"
	"fmt"
	"math/big"
	"sort"

	btss "github.com/binance-chain/tss-lib/tss"
	sdk "github.com/cosmos/cosmos-sdk/types"
	crypto2 "github.com/libp2p/go-libp2p-core/crypto"
	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/tendermint/tendermint/crypto/secp256k1"
)

// getParties
// keys is a slice of node pub key in bech32 format
// localNodePubKeyis the node pub key of local node, in bech32 format
// return values
// a slice of party ids , and local party id, error
func getParties(nodePubKeys []string, localNodePubKey string) ([]*btss.PartyID, *btss.PartyID, error) {
	var localPartyID *btss.PartyID
	var unSortedPartiesID []*btss.PartyID
	sort.Strings(nodePubKeys)
	for _, item := range nodePubKeys {
		pk, err := sdk.GetAccPubKeyBech32(item)
		if err != nil {
			return nil, nil, fmt.Errorf("fail to get account pub key address(%s): %w", item, err)
		}
		secpPk := pk.(secp256k1.PubKeySecp256k1)
		key := new(big.Int).SetBytes(secpPk[:])
		// Set up the parameters
		// Note: The `id` and `moniker` fields are for convenience to allow you to easily track participants.
		// The `id` should be a unique string representing this party in the network and `moniker` can be anything (even left blank).
		// The `uniqueKey` is a unique identifying key for this peer (such as its p2p public key) as a big.Int.
		peerID, err := getPeerIDFromPubKey(secpPk)
		if err != nil {
			return nil, nil, fmt.Errorf("fail to get peer id from pub key: %w", err)
		}
		partyID := btss.NewPartyID(item, peerID.String(), key)
		if item == localNodePubKey {
			localPartyID = partyID
		}
		unSortedPartiesID = append(unSortedPartiesID, partyID)
	}
	if localPartyID == nil {
		return nil, nil, errors.New("local party is not in the list")
	}

	partiesID := btss.SortPartyIDs(unSortedPartiesID)
	return partiesID, localPartyID, nil
}

func getPeerIDFromPubKey(pk secp256k1.PubKeySecp256k1) (peer.ID, error) {
	ppk, err := crypto2.UnmarshalSecp256k1PublicKey(pk[:])
	if err != nil {
		return "", fmt.Errorf("fail to convert pubkey to the crypto pubkey used in libp2p: %w", err)
	}
	return peer.IDFromPublicKey(ppk)
}

// getPartyIDMap , the key of the map will be the node pubkey in bech32 format
// also moniker will be the peer id
func getPartyIDMap(partiesID []*btss.PartyID) map[string]*btss.PartyID {
	partyIDMap := make(map[string]*btss.PartyID)
	for _, id := range partiesID {
		partyIDMap[id.Id] = id
	}
	return partyIDMap
}
