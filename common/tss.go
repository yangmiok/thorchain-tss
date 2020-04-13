package common

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"sort"
	"strconv"
	"sync"

	"github.com/binance-chain/go-sdk/common/types"
	bcrypto "github.com/binance-chain/tss-lib/crypto"
	btss "github.com/binance-chain/tss-lib/tss"
	"github.com/btcsuite/btcd/btcec"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	tcrypto "github.com/tendermint/tendermint/crypto"
	"github.com/tendermint/tendermint/crypto/secp256k1"

	"gitlab.com/thorchain/tss/go-tss"
	"gitlab.com/thorchain/tss/go-tss/messages"
	"gitlab.com/thorchain/tss/go-tss/p2p"
)

// PartyInfo the information used by tss key gen and key sign
type PartyInfo struct {
	Party      btss.Party
	PartyIDMap map[string]*btss.PartyID
}

type TssCommon struct {
	conf                TssConfig
	logger              zerolog.Logger
	partyLock           *sync.Mutex
	partyInfo           *PartyInfo
	PartyIDtoP2PID      map[string]peer.ID
	unConfirmedMsgLock  *sync.Mutex
	unConfirmedMessages map[string]*LocalCacheItem
	localPeerID         string
	broadcastChannel    chan *messages.BroadcastMsgChan
	TssMsg              chan *p2p.Message
	P2PPeers            []peer.ID // most of tss message are broadcast, we store the peers ID to avoid iterating
	BlamePeers          Blame
	msgID               string
	lastUnicastPeer     map[string][]peer.ID
	privateKey          tcrypto.PrivKey
	msgStored           TssMsgStored
}

func NewTssCommon(peerID string, broadcastChannel chan *messages.BroadcastMsgChan, conf TssConfig, msgID string, privateKey tcrypto.PrivKey) *TssCommon {
	return &TssCommon{
		conf:                conf,
		logger:              log.With().Str("module", "tsscommon").Logger(),
		partyLock:           &sync.Mutex{},
		partyInfo:           nil,
		PartyIDtoP2PID:      make(map[string]peer.ID),
		unConfirmedMsgLock:  &sync.Mutex{},
		unConfirmedMessages: make(map[string]*LocalCacheItem),
		broadcastChannel:    broadcastChannel,
		TssMsg:              make(chan *p2p.Message),
		P2PPeers:            nil,
		BlamePeers:          Blame{},
		msgID:               msgID,
		lastUnicastPeer:     make(map[string][]peer.ID),
		privateKey:          privateKey,
		msgStored:           NewTssMsgStored(),
	}
}

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
		// Set up the parameters
		// Note: The `id` and `moniker` fields are for convenience to allow you to easily track participants.
		// The `id` should be a unique string representing this party in the network and `moniker` can be anything (even left blank).
		// The `uniqueKey` is a unique identifying key for this peer (such as its p2p public key) as a big.Int.
		partyID := btss.NewPartyID(strconv.Itoa(idx), "", key)
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

func (t *TssCommon) renderToP2P(broadcastMsg *messages.BroadcastMsgChan) {
	if t.broadcastChannel == nil {
		t.logger.Warn().Msg("broadcast channel is not set")
		return
	}
	t.broadcastChannel <- broadcastMsg
}

func (t *TssCommon) sendMsg(message messages.WrappedMessage, peerIDs []peer.ID) {
	t.renderToP2P(&messages.BroadcastMsgChan{
		WrappedMessage: message,
		PeersID:        peerIDs,
	})
}

func getPeerIDFromPartyID(partyID *btss.PartyID) (peer.ID, error) {
	pkBytes := partyID.KeyInt().Bytes()
	var pk secp256k1.PubKeySecp256k1
	copy(pk[:], pkBytes)
	return go_tss.GetPeerIDFromSecp256PubKey(pk)
}

// GetConf get current configuration for Tss
func (t *TssCommon) GetConf() TssConfig {
	return t.conf
}

func (t *TssCommon) getLastUnicastPeers(key string) ([]peer.ID, bool) {
	ret, ok := t.lastUnicastPeer[key]
	return ret, ok
}

func (t *TssCommon) SetPartyInfo(partyInfo *PartyInfo) {
	t.partyLock.Lock()
	defer t.partyLock.Unlock()
	t.partyInfo = partyInfo
}

func (t *TssCommon) getPartyInfo() *PartyInfo {
	t.partyLock.Lock()
	defer t.partyLock.Unlock()
	return t.partyInfo
}

// it is only used for debuging
func (t *TssCommon) QueryRequestMsgLenDebug() int {
	return len(t.msgStored.requested)
}

func (t *TssCommon) GetLocalPeerID() string {
	return t.localPeerID
}

func (t *TssCommon) SetLocalPeerID(peerID string) {
	t.localPeerID = peerID
}

func BytesToHashString(msg []byte) (string, error) {
	h := sha256.New()
	_, err := h.Write(msg)
	if err != nil {
		return "", fmt.Errorf("fail to caculate sha256 hash: %w", err)
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// updateLocal will apply the wireMsg to local keygen/keysign party
func (t *TssCommon) updateLocal(wireMsg *messages.WireMessage) error {
	if nil == wireMsg {
		t.logger.Warn().Msg("wire msg is nil")
	}
	partyInfo := t.getPartyInfo()
	if partyInfo == nil {
		return nil
	}
	partyID, ok := partyInfo.PartyIDMap[wireMsg.Routing.From.Id]
	if !ok {
		return fmt.Errorf("get message from unknown party %s", partyID.Id)
	}

	dataOwnerPeerID, ok := t.PartyIDtoP2PID[wireMsg.Routing.From.Id]
	if !ok {
		t.logger.Error().Msg("fail to find the peer ID of this party")
		return errors.New("fail to find the peer")
	}
	//here we log down this peer
	l, ok := t.lastUnicastPeer[wireMsg.RoundInfo]
	if !ok {
		peerList := []peer.ID{dataOwnerPeerID}
		t.lastUnicastPeer[wireMsg.RoundInfo] = peerList
	} else {
		l = append(l, dataOwnerPeerID)
		t.lastUnicastPeer[wireMsg.RoundInfo] = l
	}

	if _, err := partyInfo.Party.UpdateFromBytes(wireMsg.Message, partyID, wireMsg.Routing.IsBroadcast); nil != err {
		pk := t.TssWrongShareBlame(wireMsg)
		blameNode := NewBlameNode(pk, wireMsg.Message, wireMsg.Sig)
		t.BlamePeers.SetBlame(BlameHashCheck, []BlameNode{blameNode})
		return fmt.Errorf("fail to set bytes to local party: %w", err)
	}
	return nil
}

func (t *TssCommon) isLocalPartyReady() bool {
	partyInfo := t.getPartyInfo()
	if nil == partyInfo {
		return false
	}
	return true
}

func (t *TssCommon) checkDupAndUpdateVerMsg(bMsg *messages.BroadcastConfirmMessage, peerID string) bool {
	localCacheItem := t.TryGetLocalCacheItem(bMsg.Key)
	// we check whether this node has already sent the VerMsg message to avoid eclipse of others VerMsg
	if localCacheItem == nil {
		bMsg.P2PID = peerID
		return true
	}

	localCacheItem.lock.Lock()
	defer localCacheItem.lock.Unlock()
	if _, ok := localCacheItem.ConfirmedList[peerID]; ok {
		return false
	}
	bMsg.P2PID = peerID
	return true
}

func (t *TssCommon) ProcessOneMessage(wrappedMsg *messages.WrappedMessage, peerIDStr string) error {
	t.logger.Debug().Msg("start process one message")
	defer t.logger.Debug().Msg("finish processing one message")
	if nil == wrappedMsg {
		return errors.New("invalid wireMessage")
	}

	switch wrappedMsg.MessageType {
	case messages.TSSKeyGenMsg, messages.TSSKeySignMsg:
		var wireMsg messages.WireMessage
		if err := json.Unmarshal(wrappedMsg.Payload, &wireMsg); nil != err {
			return fmt.Errorf("fail to unmarshal wire message: %w", err)
		}
		return t.processTSSMsg(&wireMsg, wrappedMsg.MessageType)
	case messages.TSSKeyGenVerMsg, messages.TSSKeySignVerMsg:
		var bMsg messages.BroadcastConfirmMessage
		if err := json.Unmarshal(wrappedMsg.Payload, &bMsg); nil != err {
			return errors.New("fail to unmarshal broadcast confirm message")
		}
		// we check whether this peer has already send us the VerMsg before update
		ret := t.checkDupAndUpdateVerMsg(&bMsg, peerIDStr)
		if ret {
			return t.processVerMsg(&bMsg, wrappedMsg.MessageType)
		}
	case messages.TSSMsgBody:
		var wireMsg messages.RequestMsgBodyMessage
		if err := json.Unmarshal(wrappedMsg.Payload, &wireMsg); nil != err {
			return fmt.Errorf("fail to unmarshal wire message: %w", err)
		}
		if wireMsg.Msg == nil {
			peerID, err := peer.Decode(peerIDStr)
			if err != nil {
				t.logger.Error().Err(err).Msg("error in decode the peer")
				return err
			}
			return t.processRequestMsgFromPeer([]peer.ID{peerID}, &wireMsg, false)
		}
		exist := t.msgStored.queryAndDelete(wireMsg.ReqHash)
		if !exist {
			t.logger.Debug().Msg("this request does not exit, maybe already processed")
			return nil
		}
		return t.processTSSMsg(wireMsg.Msg, wireMsg.RequestType)

	}
	return nil
}

func (t *TssCommon) getMsgHash(localCacheItem *LocalCacheItem, threshold int) (string, error) {
	hash, freq, err := getHighestFreq(localCacheItem.ConfirmedList)
	if err != nil {
		t.logger.Error().Err(err).Msg("fail to get the hash freq")
		return "", ErrHashCheck
	}
	if freq < threshold {
		t.logger.Error().Msgf("fail to have more than 2/3 peers agree on the received message threshold(%d)--total confirmed(%d)\n", threshold, freq)
		return "", ErrHashInconsistency
	}
	return hash, nil
}

func (t *TssCommon) hashCheck(localCacheItem *LocalCacheItem, threshold int) error {
	dataOwner := localCacheItem.Msg.Routing.From
	dataOwnerP2PID, ok := t.PartyIDtoP2PID[dataOwner.Id]
	if !ok {
		t.logger.Warn().Msgf("error in find the data Owner P2PID\n")
		return errors.New("error in find the data Owner P2PID")
	}
	localCacheItem.lock.Lock()
	defer localCacheItem.lock.Unlock()

	targetHashValue := localCacheItem.Hash

	for P2PID, _ := range localCacheItem.ConfirmedList {
		if P2PID == dataOwnerP2PID.String() {
			t.logger.Warn().Msgf("we detect that the data owner try to send the hash for his own message\n")
			delete(localCacheItem.ConfirmedList, P2PID)
			return ErrHashFromOwner
		}
	}
	hash, err := t.getMsgHash(localCacheItem, threshold)
	if err != nil {
		return err
	}
	if targetHashValue == hash {
		t.logger.Debug().Msgf("hash check complete for messageID: %v", t.msgID)
		return nil
	}

	return ErrMsgHashCheck

}

func (t *TssCommon) ProcessOutCh(msg btss.Message, msgType messages.THORChainTSSMessageType) error {
	buf, r, err := msg.WireBytes()
	// if we cannot get the wire share, the tss keygen will fail, we just quit.
	if err != nil {
		return fmt.Errorf("fail to get wire bytes: %w", err)
	}
	sig, err := t.privateKey.Sign(buf)

	wireMsg := messages.WireMessage{
		Routing:   r,
		RoundInfo: msg.Type(),
		Message:   buf,
		Sig:       sig,
	}
	wireMsgBytes, err := json.Marshal(wireMsg)
	if err != nil {
		return fmt.Errorf("fail to convert tss msg to wire bytes: %w", err)
	}
	wrappedMsg := messages.WrappedMessage{
		MessageType: msgType,
		MsgID:       t.msgID,
		Payload:     wireMsgBytes,
	}
	peerIDs := make([]peer.ID, 0)
	if len(r.To) == 0 {
		peerIDs = t.P2PPeers
	} else {
		for _, each := range r.To {
			peerID, ok := t.PartyIDtoP2PID[each.Id]
			if !ok {
				t.logger.Error().Msg("error in find the P2P ID")
				continue
			}
			peerIDs = append(peerIDs, peerID)
		}
	}
	t.renderToP2P(&messages.BroadcastMsgChan{
		WrappedMessage: wrappedMsg,
		PeersID:        peerIDs,
	})

	return nil
}

func (t *TssCommon) processBlameVerMsg(broadcastConfirmMsg *messages.BroadcastConfirmMessage) error {
	t.logger.Debug().Msg("process ver msg blame")
	defer t.logger.Debug().Msg("finish process ver msg")
	if nil == broadcastConfirmMsg {
		return nil
	}
	partyInfo := t.getPartyInfo()
	if nil == partyInfo {
		return errors.New("can't process ver msg , local party is not ready")
	}
	key := broadcastConfirmMsg.Key
	localCacheItem := t.TryGetLocalCacheItem(key)
	if nil == localCacheItem {
		// we didn't receive the TSS Message yet
		t.logger.Info().Msgf("we(%v) receive the hash that we do not have any message matched with ", t.GetLocalPeerID())
		return nil
	}
	localCacheItem.UpdateConfirmList(broadcastConfirmMsg.P2PID, broadcastConfirmMsg.Hash)
	return nil
}

func (t *TssCommon) processRequestMsgFromPeer(peersID []peer.ID, msg *messages.RequestMsgBodyMessage, requester bool) error {
	// we need to send msg to the peer
	if !requester {
		reqKey := msg.ReqKey
		storedMsg := t.msgStored.getTssMsgStored(reqKey)
		if msg == nil {
			t.logger.Debug().Msg("we do not have this message either")
			return nil
		}
		msg.Msg = storedMsg
	}

	data, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("fail to marshal the request body %w", err)
	}
	wrappedMsg := messages.WrappedMessage{
		MessageType: messages.TSSMsgBody,
		MsgID:       t.msgID,
		Payload:     data,
	}

	t.renderToP2P(&messages.BroadcastMsgChan{
		WrappedMessage: wrappedMsg,
		PeersID:        peersID,
	})
	return nil
}

func (t *TssCommon) processVerMsg(broadcastConfirmMsg *messages.BroadcastConfirmMessage, msgType messages.THORChainTSSMessageType) error {
	t.logger.Debug().Msg("process ver msg")
	defer t.logger.Debug().Msg("finish process ver msg")
	if nil == broadcastConfirmMsg {
		return nil
	}
	partyInfo := t.getPartyInfo()
	if nil == partyInfo {
		return errors.New("can't process ver msg , local party is not ready")
	}
	key := broadcastConfirmMsg.Key
	localCacheItem := t.TryGetLocalCacheItem(key)
	if nil == localCacheItem {
		// we didn't receive the TSS Message yet
		localCacheItem = NewLocalCacheItem(nil, broadcastConfirmMsg.Hash)
		t.updateLocalUnconfirmedMessages(key, localCacheItem)
	}

	localCacheItem.UpdateConfirmList(broadcastConfirmMsg.P2PID, broadcastConfirmMsg.Hash)
	t.logger.Debug().Msgf("total confirmed parties:%+v", localCacheItem.ConfirmedList)

	threshold, err := GetThreshold(len(partyInfo.PartyIDMap))
	if err != nil {
		return err
	}
	if localCacheItem.Msg == nil {
		// in acquiring the message, we do not need to have the threshold check
		targetHash, err := t.getMsgHash(localCacheItem, 0)
		if err != nil {
			return err
		}
		var peersIDs []peer.ID
		for thisPeerStr, hash := range localCacheItem.ConfirmedList {
			if hash == targetHash {
				thisPeer, err := peer.Decode(thisPeerStr)
				if err != nil {
					t.logger.Error().Err(err).Msg("fail to convert the p2p id")
					return err
				}
				peersIDs = append(peersIDs, thisPeer)
			}
		}

		msg := &messages.RequestMsgBodyMessage{
			ReqHash:     targetHash,
			ReqKey:      key,
			RequestType: 0,
			Msg:         nil,
		}
		t.msgStored.setRequest(targetHash)
		switch msgType {
		case messages.TSSKeyGenVerMsg:
			msg.RequestType = messages.TSSKeyGenMsg
			return t.processRequestMsgFromPeer(peersIDs, msg, true)
		case messages.TSSKeySignVerMsg:
			msg.RequestType = messages.TSSKeySignMsg
			return t.processRequestMsgFromPeer(peersIDs, msg, true)
		default:
			t.logger.Debug().Msg("unknown message type for request")
			return nil
		}
	} else {
		t.msgStored.storeTssMsg(key, localCacheItem.Msg)
		err = t.hashCheck(localCacheItem, threshold)
		if err != nil {
			if err == ErrMsgHashCheck {
				blamePk, err := partyIDtoPubKey(partyInfo.PartyIDMap[localCacheItem.Msg.Routing.From.Id])
				if err != nil {
					t.logger.Error().Err(err).Msgf("error in get the blame nodes")
					t.BlamePeers.SetBlame(BlameHashCheck, nil)
					return fmt.Errorf("error in getting the blame nodes %w", ErrMsgHashCheck)
				}
				blameNode := NewBlameNode(blamePk, localCacheItem.Msg.Message, localCacheItem.Msg.Sig)
				t.BlamePeers.SetBlame(BlameHashCheck, []BlameNode{blameNode})
				return ErrMsgHashCheck
			}
			//fixme add blame
			return ErrMsgHashCheck
		}

		if err := t.updateLocal(localCacheItem.Msg); nil != err {
			return fmt.Errorf("fail to update the message to local party: %w", err)
		}
		// the information had been confirmed by all party , we don't need it anymore
		t.logger.Debug().Msgf("remove key: %s", key)
		t.removeKey(key)
	}

	return nil
}

func (t *TssCommon) broadcastHashToPeers(key, msgHash string, peerIDs []peer.ID, msgType messages.THORChainTSSMessageType) error {

	if len(peerIDs) == 0 {
		t.logger.Error().Msg("fail to get any peer ID")
		return errors.New("fail to get any peer ID")
	}

	broadcastConfirmMsg := &messages.BroadcastConfirmMessage{
		// P2PID will be filled up by the receiver.
		P2PID: "",
		Key:   key,
		Hash:  msgHash,
	}
	buf, err := json.Marshal(broadcastConfirmMsg)
	if err != nil {
		return fmt.Errorf("fail to marshal borad cast confirm message: %w", err)
	}
	t.logger.Debug().Msg("broadcast VerMsg to all other parties")

	p2pWrappedMSg := messages.WrappedMessage{
		MessageType: msgType,
		MsgID:       t.msgID,
		Payload:     buf,
	}
	t.renderToP2P(&messages.BroadcastMsgChan{
		WrappedMessage: p2pWrappedMSg,
		PeersID:        peerIDs,
	})

	return nil
}

// processTSSMsg
func (t *TssCommon) processTSSMsg(wireMsg *messages.WireMessage, msgType messages.THORChainTSSMessageType) error {
	t.logger.Debug().Msg("process wire message")
	defer t.logger.Debug().Msg("finish process wire message")
	// we only update it local party
	if !wireMsg.Routing.IsBroadcast {
		t.logger.Debug().Msgf("msg from %s to %+v", wireMsg.Routing.From, wireMsg.Routing.To)
		return t.updateLocal(wireMsg)
	}

	partyIDMap := t.getPartyInfo().PartyIDMap
	dataOwner, ok := partyIDMap[wireMsg.Routing.From.Id]
	if !ok {
		t.logger.Error().Msg("error in find the data owner")
		return errors.New("error in find the data owner")
	}

	keyBytes := dataOwner.GetKey()
	var pk secp256k1.PubKeySecp256k1
	copy(pk[:], keyBytes)
	//out, _ := sdk.Bech32ifyAccPub(pk)

	//for id, el := range partyIDMap {
	//	var pk secp256k1.PubKeySecp256k1
	//	copy(pk[:], el.Key)
	//	out, _ := sdk.Bech32ifyAccPub(pk)
	//	fmt.Printf("-----%v---->%v\n", id, out)
	//}

	ok = pk.VerifyBytes(wireMsg.Message, wireMsg.Sig)
	if !ok {
		t.logger.Error().Msg("fail to verify the signature")
		return errors.New("signature verification failed")
	}
	// broadcast message , we save a copy locally , and then tell all others what we got
	msgHash, err := BytesToHashString(wireMsg.Message)
	if err != nil {
		return fmt.Errorf("fail to calculate hash of the wire message: %w", err)
	}
	partyInfo := t.getPartyInfo()
	key := wireMsg.GetCacheKey()

	localCacheItem := t.TryGetLocalCacheItem(key)
	t.msgStored.storeTssMsg(key, wireMsg)
	if nil == localCacheItem {
		t.logger.Debug().Msgf("++%s doesn't exist yet,add a new one", key)
		localCacheItem = NewLocalCacheItem(wireMsg, msgHash)
		t.updateLocalUnconfirmedMessages(key, localCacheItem)
	} else {
		// this means we received the broadcast confirm message from other party first
		t.logger.Debug().Msgf("==%s exist", key)
		if localCacheItem.Msg == nil {
			t.logger.Debug().Msgf("==%s exist, set message", key)
			localCacheItem.Msg = wireMsg
			localCacheItem.Hash = msgHash
		}
	}
	localCacheItem.UpdateConfirmList(t.localPeerID, msgHash)

	threshold, err := GetThreshold(len(partyInfo.PartyIDMap))
	if err != nil {
		return err
	}
	if t.hashCheck(localCacheItem, threshold) == nil {
		if err := t.updateLocal(localCacheItem.Msg); nil != err {
			return fmt.Errorf("fail to update the message to local party: %w", err)
		}
		t.logger.Debug().Msgf("remove key: %s", key)
		t.removeKey(key)
	}
	var peerIDs []peer.ID
	dataOwnerPartyID := wireMsg.Routing.From.Id
	dataOwnerPeerID, ok := t.PartyIDtoP2PID[dataOwnerPartyID]
	if !ok {
		return errors.New("error in find the data owner peerID")
	}
	for _, el := range t.P2PPeers {
		if el == dataOwnerPeerID {
			continue
		}
		peerIDs = append(peerIDs, el)
	}
	msgVerType := getBroadcastMessageType(msgType)
	err = t.broadcastHashToPeers(key, msgHash, peerIDs, msgVerType)
	if err != nil {
		t.logger.Error().Err(err).Msg("fail to broadcast the hash to peers")
		return err
	}
	return nil
}

func getBroadcastMessageType(msgType messages.THORChainTSSMessageType) messages.THORChainTSSMessageType {
	switch msgType {
	case messages.TSSKeyGenMsg:
		return messages.TSSKeyGenVerMsg
	case messages.TSSKeySignMsg:
		return messages.TSSKeySignVerMsg
	default:
		return messages.Unknown // this should not happen
	}
}

func (t *TssCommon) TryGetLocalCacheItem(key string) *LocalCacheItem {
	t.unConfirmedMsgLock.Lock()
	defer t.unConfirmedMsgLock.Unlock()
	localCacheItem, ok := t.unConfirmedMessages[key]
	if !ok {
		return nil
	}
	return localCacheItem
}

func (t *TssCommon) TryGetAllLocalCached() []*LocalCacheItem {
	var localCachedItems []*LocalCacheItem
	t.unConfirmedMsgLock.Lock()
	defer t.unConfirmedMsgLock.Unlock()
	for _, value := range t.unConfirmedMessages {
		localCachedItems = append(localCachedItems, value)
	}
	return localCachedItems
}

func (t *TssCommon) updateLocalUnconfirmedMessages(key string, cacheItem *LocalCacheItem) {
	t.unConfirmedMsgLock.Lock()
	defer t.unConfirmedMsgLock.Unlock()
	t.unConfirmedMessages[key] = cacheItem
}

func (t *TssCommon) removeKey(key string) {
	t.unConfirmedMsgLock.Lock()
	defer t.unConfirmedMsgLock.Unlock()
	delete(t.unConfirmedMessages, key)
}

func GetTssPubKey(pubKeyPoint *bcrypto.ECPoint) (string, types.AccAddress, error) {
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

func (t *TssCommon) ProcessInboundMessages(finishChan chan struct{}, wg *sync.WaitGroup) {
	t.logger.Info().Msg("start processing inbound messages")
	defer wg.Done()
	defer t.logger.Info().Msg("stop processing inbound messages")
	for {
		select {
		case <-finishChan:
			return
		case m, ok := <-t.TssMsg:
			if !ok {
				return
			}
			var wrappedMsg messages.WrappedMessage
			if err := json.Unmarshal(m.Payload, &wrappedMsg); nil != err {
				t.logger.Error().Err(err).Msg("fail to unmarshal wrapped message bytes")
				continue
			}

			if err := t.ProcessOneMessage(&wrappedMsg, m.PeerID.String()); err != nil {
				t.logger.Error().Err(err).Msg("fail to process the received message")
			}
		}
	}
}
