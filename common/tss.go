package common

import (
	"errors"
	"fmt"
	"math/big"
	"sort"
	"strconv"

	"github.com/binance-chain/go-sdk/common/types"
	"github.com/binance-chain/tss-lib/crypto"
	btss "github.com/binance-chain/tss-lib/tss"
	"github.com/btcsuite/btcd/btcec"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/tendermint/tendermint/crypto/secp256k1"
)

func GetParties(keys []string, localPartyKey string) ([]*btss.PartyID, *btss.PartyID, error) {
	var localPartyID *btss.PartyID
	var unSortedPartiesID []*btss.PartyID
	sort.Strings(keys)
	for idx, item := range keys {
		pk, err := sdk.GetAccPubKeyBech32(item)
		if err != nil {
			return nil, nil, fmt.Errorf("fail to get account pub key address(%s): %w", item, err)
		}
		secpPk := pk.(secp256k1.PubKeySecp256k1)
		key := new(big.Int).SetBytes(secpPk[:])
		peerID, err := GetPeerIDFromSecp256PubKey(secpPk)
		if err != nil {
			return nil, nil, fmt.Errorf("fail to get peer id from pub key")
		}
		// Set up the parameters
		// Note: The `id` and `moniker` fields are for convenience to allow you to easily track participants.
		// The `id` should be a unique string representing this party in the network and `moniker` can be anything (even left blank).
		// The `uniqueKey` is a unique identifying key for this peer (such as its p2p public key) as a big.Int.
		partyID := btss.NewPartyID(strconv.Itoa(idx), peerID.String(), key)
		if item == localPartyKey {
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

func getPeerIDFromPartyID(partyID *btss.PartyID) (peer.ID, error) {
	pkBytes := partyID.KeyInt().Bytes()
	var pk secp256k1.PubKeySecp256k1
	copy(pk[:], pkBytes)
	return GetPeerIDFromSecp256PubKey(pk)
}

//
// func (t *TssCommon) checkDupAndUpdateVerMsg(bMsg *p2p.BroadcastConfirmMessage, peerID string) bool {
// 	localCacheItem := t.TryGetLocalCacheItem(bMsg.Key)
// 	// we check whether this node has already sent the VerMsg message to avoid eclipse of others VerMsg
// 	if localCacheItem == nil {
// 		bMsg.P2PID = peerID
// 		return true
// 	}
//
// 	localCacheItem.lock.Lock()
// 	defer localCacheItem.lock.Unlock()
// 	if _, ok := localCacheItem.ConfirmedList[peerID]; ok {
// 		return false
// 	}
// 	bMsg.P2PID = peerID
// 	return true
// }
//
// func (t *TssCommon) ProcessOneMessage(wrappedMsg *p2p.WrappedMessage, peerID string) error {
// 	t.logger.Debug().Msg("start process one message")
// 	defer t.logger.Debug().Msg("finish processing one message")
// 	if nil == wrappedMsg {
// 		return errors.New("invalid wireMessage")
// 	}
//
// 	switch wrappedMsg.MessageType {
// 	case p2p.TSSKeyGenMsg, p2p.TSSKeySignMsg:
// 		var wireMsg p2p.WireMessage
// 		if err := json.Unmarshal(wrappedMsg.Payload, &wireMsg); nil != err {
// 			return fmt.Errorf("fail to unmarshal wire message: %w", err)
// 		}
// 		return t.processTSSMsg(&wireMsg, wrappedMsg.MessageType)
// 	case p2p.TSSKeyGenVerMsg, p2p.TSSKeySignVerMsg:
// 		var bMsg p2p.BroadcastConfirmMessage
// 		if err := json.Unmarshal(wrappedMsg.Payload, &bMsg); nil != err {
// 			return errors.New("fail to unmarshal broadcast confirm message")
// 		}
// 		// we check whether this peer has already send us the VerMsg before update
// 		ret := t.checkDupAndUpdateVerMsg(&bMsg, peerID)
// 		if ret {
// 			return t.processVerMsg(&bMsg)
// 		}
// 		return nil
// 	}
// 	return nil
// }
//
// func (t *TssCommon) hashCheck(localCacheItem *LocalCacheItem) error {
// 	dataOwner := localCacheItem.Msg.Routing.From
// 	dataOwnerP2PID, ok := t.PartyIDtoP2PID[dataOwner.Id]
// 	if !ok {
// 		t.logger.Warn().Msgf("error in find the data Owner P2PID\n")
// 		return errors.New("error in find the data Owner P2PID")
// 	}
// 	localCacheItem.lock.Lock()
// 	defer localCacheItem.lock.Unlock()
//
// 	targetHashValue := localCacheItem.Hash
// 	for P2PID, hashValue := range localCacheItem.ConfirmedList {
// 		if P2PID == dataOwnerP2PID.String() {
// 			t.logger.Warn().Msgf("we detect that the data owner try to send the hash for his own message\n")
// 			delete(localCacheItem.ConfirmedList, P2PID)
// 			return ErrHashFromOwner
// 		}
// 		if targetHashValue == hashValue {
// 			continue
// 		}
// 		t.logger.Error().Msgf("hash is not in consistency!!")
// 		return ErrHashFromPeer
// 	}
// 	return nil
// }
//

//
// func (t *TssCommon) processVerMsg(broadcastConfirmMsg *p2p.BroadcastConfirmMessage) error {
// 	t.logger.Debug().Msg("process ver msg")
// 	defer t.logger.Debug().Msg("finish process ver msg")
// 	if nil == broadcastConfirmMsg {
// 		return nil
// 	}
// 	partyInfo := t.getPartyInfo()
// 	if nil == partyInfo {
// 		return errors.New("can't process ver msg , local party is not ready")
// 	}
// 	key := broadcastConfirmMsg.Key
// 	localCacheItem := t.TryGetLocalCacheItem(key)
// 	if nil == localCacheItem {
// 		// we didn't receive the TSS Message yet
// 		localCacheItem = NewLocalCacheItem(nil, broadcastConfirmMsg.Hash)
// 		t.updateLocalUnconfirmedMessages(key, localCacheItem)
// 	}
//
// 	localCacheItem.UpdateConfirmList(broadcastConfirmMsg.P2PID, broadcastConfirmMsg.Hash)
// 	t.logger.Debug().Msgf("total confirmed parties:%+v", localCacheItem.ConfirmedList)
// 	if localCacheItem.TotalConfirmParty() == (len(partyInfo.PartyIDMap)-1) && localCacheItem.Msg != nil {
// 		errHashCheck := t.hashCheck(localCacheItem)
//
// 		if errHashCheck != nil {
// 			blamePeers, err := t.getHashCheckBlamePeers(localCacheItem, errHashCheck)
// 			if err != nil {
// 				t.logger.Error().Err(err).Msgf("error in get the blame nodes")
// 				t.BlamePeers.SetBlame(BlameHashCheck, nil)
// 				return fmt.Errorf("error in getting the blame nodes %w", errHashCheck)
// 			}
// 			blamePubKeys, _, err := t.GetBlamePubKeysLists(blamePeers)
// 			if err != nil {
// 				t.logger.Error().Err(err).Msg("fail to get the blame nodes public key")
//
// 				t.BlamePeers.SetBlame(BlameHashCheck, nil)
// 				return fmt.Errorf("fail to get the blame nodes public key %w", errHashCheck)
// 			}
// 			t.BlamePeers.SetBlame(BlameHashCheck, blamePubKeys)
// 			t.logger.Error().Msg("The consistency check failed")
// 			return errHashCheck
// 		}
//
// 		if err := t.updateLocal(localCacheItem.Msg); nil != err {
// 			return fmt.Errorf("fail to update the message to local party: %w", err)
// 		}
// 		// the information had been confirmed by all party , we don't need it anymore
// 		t.logger.Debug().Msgf("remove key: %s", key)
// 		t.removeKey(key)
// 	}
// 	return nil
// }
//
func GetTssPubKey(pubKeyPoint *crypto.ECPoint) (string, types.AccAddress, error) {
	if pubKeyPoint == nil {
		return "", types.AccAddress{}, errors.New("invalid points")
	}
	tssPubKey := btcec.PublicKey{
		Curve: btcec.S256(),
		X:     pubKeyPoint.X(),
		Y:     pubKeyPoint.Y(),
	}
	var pubKeyCompressed secp256k1.PubKeySecp256k1
	copy(pubKeyCompressed[:], tssPubKey.SerializeCompressed())
	pubKey, err := sdk.Bech32ifyAccPub(pubKeyCompressed)
	addr := types.AccAddress(pubKeyCompressed.Address().Bytes())
	return pubKey, addr, err
}
