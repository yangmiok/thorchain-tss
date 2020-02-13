package keysign

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"net/http"
	"path/filepath"
	"sort"
	"strconv"
	"sync"
	"time"

	"github.com/binance-chain/tss-lib/ecdsa/signing"
	btss "github.com/binance-chain/tss-lib/tss"
	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/tendermint/go-amino"
	cryptokey "github.com/tendermint/tendermint/crypto"

	"gitlab.com/thorchain/tss/go-tss/common"
	"gitlab.com/thorchain/tss/go-tss/p2p"
)

type TssKeySign struct {
	logger          zerolog.Logger
	priKey          cryptokey.PrivKey
	tssCommonStruct *common.TssCommon
	stopChan        *chan struct{} // channel to indicate whether we should stop
	homeBase        string
	syncMsg         chan *p2p.Message
	localParty      []*btss.PartyID
	keySignCurrent  *string
}

func NewTssKeySign(homeBase, localP2PID string, conf common.TssConfig, privKey cryptokey.PrivKey, broadcastChan chan *p2p.BroadcastMsgChan, stopChan *chan struct{}, keySignCurrent *string) TssKeySign {
	return TssKeySign{
		logger:          log.With().Str("module", "keySign").Logger(),
		priKey:          privKey,
		tssCommonStruct: common.NewTssCommon(localP2PID, broadcastChan, conf),
		stopChan:        stopChan,
		homeBase:        homeBase,
		syncMsg:         make(chan *p2p.Message),
		localParty:      make([]*btss.PartyID, 0),
		keySignCurrent:  keySignCurrent,
	}
}

func (tKeySign *TssKeySign) GetTssKeySignChannels() (chan *p2p.Message, chan *p2p.Message) {
	return tKeySign.tssCommonStruct.TssMsg, tKeySign.syncMsg
}

func (tKeySign *TssKeySign) GetTssCommonStruct() *common.TssCommon {
	return tKeySign.tssCommonStruct
}

func (tKeySign *TssKeySign) getSigningPeers(peers []string, threshold int) ([]string, bool) {
	var candidates []string
	localPeer := tKeySign.GetTssCommonStruct().GetLocalPeerID()
	candidates = append(peers, localPeer)
	sort.Strings(candidates)
	if common.Contains(candidates[:threshold+1], localPeer) {
		return candidates[:threshold+1], true
	}
	return nil, false
}

// signMessage
func (tKeySign *TssKeySign) SignMessage(req KeySignReq) ([]*signing.SignatureData, map[string]string,error) {
	if len(req.PoolPubKey) == 0 {
		return nil, nil, errors.New("empty pool pub key")
	}
	localFileName := fmt.Sprintf("localstate-%s.json", req.PoolPubKey)
	if len(tKeySign.homeBase) > 0 {
		localFileName = filepath.Join(tKeySign.homeBase, localFileName)
	}
	storedKeyGenLocalStateItem, err := common.LoadLocalState(localFileName)
	if nil != err {
		return nil, nil, fmt.Errorf("fail to read local state file: %w", err)
	}
	msgsBigInt := make(map[string] string)
	for _, reqMsg := range req.Messages {
		msgToSign, err := base64.StdEncoding.DecodeString(reqMsg)
		if nil != err {
			return nil, nil, fmt.Errorf("fail to decode message(%s): %w", reqMsg, err)
		}
		m, err := common.MsgToHashInt(msgToSign)
		if nil != err {
			return nil, nil, fmt.Errorf("fail to convert msg to hash int: %w", err)
		}
		msgsBigInt[m.String()] = reqMsg
	}
	threshold, err := common.GetThreshold(len(storedKeyGenLocalStateItem.ParticipantKeys))
	if nil != err {
		return nil, nil,err
	}
	tKeySign.logger.Debug().Msgf("keysign threshold: %d", threshold)
	partiesIDFromKeyFile, localPartyID, err := common.GetParties(storedKeyGenLocalStateItem.ParticipantKeys, storedKeyGenLocalStateItem.LocalPartyKey)
	if nil != err {
		return nil, nil, fmt.Errorf("fail to form key sign party: %w", err)
	}
	partyIDMapFromKeyFile := common.SetupPartyIDMap(partiesIDFromKeyFile)
	tempPartyIDtoP2PID := make(map[string]peer.ID)
	err = common.SetupIDMaps(partyIDMapFromKeyFile, tempPartyIDtoP2PID)
	if nil != err {
		tKeySign.logger.Error().Msgf("error in creating mapping between partyID and P2P ID")
		return nil,nil, err
	}

	// now we do the node sync, we have to wait for the timeout to allow all the nodes that want to be involved in the
	// keysign to express their interest.
	tKeySign.tssCommonStruct.P2PPeers = common.GetPeersID(tempPartyIDtoP2PID, tKeySign.tssCommonStruct.GetLocalPeerID())
	standbyPeers, err := tKeySign.tssCommonStruct.NodeSync(tKeySign.syncMsg, p2p.TSSKeySignSync)
	if err != nil && len(standbyPeers) < threshold {
		tKeySign.logger.Error().Err(err).Msg("node sync error")
		if err == common.ErrNodeSync {
			tKeySign.logger.Info().Msgf("Not Enough signers for the keysign, the nodes online are +%v", standbyPeers)
			tKeySign.tssCommonStruct.BlamePeers.SetBlame("not enough signers", nil)
		}
		return nil, nil, err
	}

	signers, isMember := tKeySign.getSigningPeers(standbyPeers, threshold)
	if !isMember {
		return nil, nil, nil
	}

	partiesIDInSigning := tKeySign.tssCommonStruct.GetPartiesIDFromPeerID(signers[:], partyIDMapFromKeyFile, tempPartyIDtoP2PID)
	partyIDMap := common.SetupPartyIDMap(partiesIDInSigning)
	err = common.SetupIDMaps(partyIDMap, tKeySign.tssCommonStruct.PartyIDtoP2PID)
	if nil != err {
		tKeySign.logger.Error().Msgf("error in creating mapping between partyID and P2P ID")
		return nil, nil, err
	}

	//now we confirm and update the signing peers
	tKeySign.tssCommonStruct.P2PPeers = common.GetPeersID(tKeySign.tssCommonStruct.PartyIDtoP2PID, tKeySign.tssCommonStruct.GetLocalPeerID())
	localKeyData, partiesIDInSigning := common.ProcessStateFile(storedKeyGenLocalStateItem, partiesIDInSigning)
	// Set up the parameters
	// Note: The `id` and `moniker` fields are for convenience to allow you to easily track participants.
	// The `id` should be a unique string representing this party in the network and `moniker` can be anything (even left blank).
	// The `uniqueKey` is a unique identifying key for this peer (such as its p2p public key) as a big.Int.
	tKeySign.logger.Debug().Msgf("local party: %+v", localPartyID)

	outCh := make(chan btss.Message, 100*len(partiesIDInSigning))
	endCh := make(chan signing.SignatureData, 100*len(partiesIDInSigning))
	errCh := make(chan struct{})

	keySignPartyMap := make(map[string]btss.Party, 0)
	//for i, el := range msgsBigInt {
	//	moiker:= el.String() + ":" + strconv.Itoa(i)
	//	ctx := btss.NewPeerContext(partiesIDInSigning)
	//	eachLocalPartyID := btss.NewPartyID(localPartyID.GetId(), moiker, localPartyID.KeyInt())
	//	//eachLocalPartyID := amino.DeepCopy(localPartyID).(*btss.PartyID)
	//	//eachLocalPartyID.Moniker = index
	//	eachLocalPartyID.Index = localPartyID.Index
	//	tKeySign.localParty = append(tKeySign.localParty,eachLocalPartyID)
	//	params := btss.NewParameters(ctx, eachLocalPartyID, len(partiesIDInSigning), threshold)
	//	keySignParty := signing.NewLocalParty(el, params, localKeyData, outCh, endCh)
	//	keySignPartyMap[moiker] = &keySignParty
	//}

	for msgBigstring, _ := range msgsBigInt{
		el ,ok:= new(big.Int).SetString(msgBigstring, 10)
		if !ok{
			tKeySign.logger.Error().Err(err).Msg("error to convert the msg to big int")
			return nil, nil, err
		}
		moiker := el.String() + ":" + strconv.Itoa(0)
		ctx := btss.NewPeerContext(partiesIDInSigning)
		eachLocalPartyID := amino.DeepCopy(localPartyID).(*btss.PartyID)
		eachLocalPartyID.Moniker = moiker
		tKeySign.localParty = append(tKeySign.localParty, eachLocalPartyID)
		params := btss.NewParameters(ctx, eachLocalPartyID, len(partiesIDInSigning), threshold)
		keySignParty := signing.NewLocalParty(el, params, localKeyData, outCh, endCh)
		keySignPartyMap[moiker] = keySignParty
	}

	//el := msgsBigInt[req.Messages[0]]
	//moiker := el.String() + ":" + strconv.Itoa(0)
	//ctx := btss.NewPeerContext(partiesIDInSigning)
	//eachLocalPartyID := amino.DeepCopy(localPartyID).(*btss.PartyID)
	//eachLocalPartyID.Moniker = moiker
	//tKeySign.localParty = append(tKeySign.localParty, eachLocalPartyID)
	//params := btss.NewParameters(ctx, eachLocalPartyID, len(partiesIDInSigning), threshold)
	//keySignParty := signing.NewLocalParty(el, params, localKeyData, outCh, endCh)
	//keySignPartyMap[moiker] = keySignParty
	//
	//el = msgsBigInt[req.Messages[1]]
	//moiker = el.String() + ":" + strconv.Itoa(1)
	//ctx = btss.NewPeerContext(partiesIDInSigning)
	////eachLocalPartyID := btss.NewPartyID(localPartyID.GetId(), moiker, localPartyID.KeyInt())
	//eachLocalPartyID = amino.DeepCopy(localPartyID).(*btss.PartyID)
	//eachLocalPartyID.Moniker = moiker
	////eachLocalPartyID.Index = eachLocalPartyID.Index+1
	////eachLocalPartyID.Index = localPartyID.Index
	//tKeySign.localParty = append(tKeySign.localParty, eachLocalPartyID)
	//params = btss.NewParameters(ctx, eachLocalPartyID, len(partiesIDInSigning), threshold)
	//keySignParty = signing.NewLocalParty(el, params, localKeyData, outCh, endCh)
	//keySignPartyMap[moiker] = keySignParty

	tKeySign.tssCommonStruct.SetPartyInfo(&common.PartyInfo{
		PartyMap:   keySignPartyMap,
		PartyIDMap: partyIDMap,
	})
	//start the key sign
	var wg sync.WaitGroup
	for _, eachParty := range keySignPartyMap {
		runparty := eachParty
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := runparty.Start(); nil != err {
				tKeySign.logger.Error().Err(err).Msg("fail to start key sign party")
				//close(errCh)
			}
			tKeySign.logger.Info().Msg("local party is ready")
		}()
	}
	wg.Wait()
	signatures, err := tKeySign.processKeySign(len(msgsBigInt),errCh, outCh, endCh)
	if nil != err {
		return nil, nil, fmt.Errorf("fail to process key sign: %w", err)
	}
	tKeySign.logger.Info().Msg("successfully sign the messages")
	return signatures, msgsBigInt,nil
}

func (tKeySign *TssKeySign) processKeySign(requestNum int, errChan chan struct{}, outCh <-chan btss.Message, endCh <-chan signing.SignatureData) ([]*signing.SignatureData, error) {
	defer tKeySign.logger.Info().Msg("key sign finished")
	tKeySign.logger.Info().Msg("start to read messages from local party")
	tssConf := tKeySign.tssCommonStruct.GetConf()
	for {
		select {
		case <-errChan: // when key sign return
			tKeySign.logger.Error().Msg("key sign failed")
			return nil, errors.New("error channel closed fail to start local party")
		case <-*tKeySign.stopChan: // when TSS processor receive signal to quit
			return nil, errors.New("received exit signal")
		case <-time.After(tssConf.KeySignTimeout):
			// we bail out after KeySignTimeoutSeconds
			tKeySign.logger.Error().Msgf("fail to sign message with %s", tssConf.KeySignTimeout.String())
			tssCommonStruct := tKeySign.GetTssCommonStruct()
			localCachedItems := tssCommonStruct.TryGetAllLocalCached()
			blamePeers, err := tssCommonStruct.TssTimeoutBlame(localCachedItems)
			if err != nil {
				tKeySign.logger.Error().Err(err).Msg("fail to get the blamed peers")
				tssCommonStruct.BlamePeers.SetBlame(common.BlameTssTimeout, nil)
				return nil, fmt.Errorf("fail to get the blamed peers %w", common.ErrTssTimeOut)
			}
			tssCommonStruct.BlamePeers.SetBlame(common.BlameTssTimeout, blamePeers)
			return nil, common.ErrTssTimeOut
		case msg := <-outCh:
			// for the sake of performance, we do not lock the status update
			// we report a rough status of current round
			*tKeySign.keySignCurrent = msg.Type()
			err := tKeySign.tssCommonStruct.ProcessOutCh(msg, p2p.TSSKeySignMsg)
			if nil != err {
				return nil, err
			}
		case m, ok := <-tKeySign.tssCommonStruct.TssMsg:
			if !ok {
				return nil, nil
			}
			var wrappedMsg p2p.WrappedMessage
			if err := json.Unmarshal(m.Payload, &wrappedMsg); nil != err {
				tKeySign.logger.Error().Err(err).Msg("fail to unmarshal wrapped message bytes")
				return nil, err
			}

			// create timeout func so we can ensure TSS doesn't get locked up and frozen
			errChan := make(chan error, 1)
			go func() {
				err := tKeySign.tssCommonStruct.ProcessOneMessage(&wrappedMsg, m.PeerID.String())
				errChan <- err
			}()

			select {
			case err := <-errChan:
				if err != nil {
					tKeySign.logger.Error().Err(err).Msg("fail to process the received message")
					return nil, err
				}
			case <-time.After(tKeySign.tssCommonStruct.GetConf().KeySignTimeout):
				err := errors.New("timeout")
				tKeySign.logger.Error().Err(err).Msg("fail to process the received message")
			}

		case sig := <-endCh:
			tKeySign.logger.Debug().Msg("we have done the key sign")

			var sigs[]*signing.SignatureData
			sigs = append(sigs, &sig)
			if len(sigs) == requestNum {
				return sigs, nil
			}
		}
	}
}

func (tKeySign *TssKeySign) WriteKeySignResult(w http.ResponseWriter, signatures []*signing.SignatureData, reqMsgMap map[string] string, status common.Status) {

	var keySignResps []KeySignResp
	for _, eachSig := range signatures{
		r := base64.StdEncoding.EncodeToString(eachSig.R)
		s := base64.StdEncoding.EncodeToString(eachSig.S)

	signResp := Signature{
		R:      r,
		S:      s,
		Status: status,
		Blame:  tKeySign.tssCommonStruct.BlamePeers,
	}
	key := new(big.Int).SetBytes(eachSig.M).String()
	keySignResp := KeySignResp{
		Msg: reqMsgMap[key],
		Sig:signResp,
	}
	keySignResps = append(keySignResps, keySignResp)
	}

	signBatch := KeySignRespBatch{KeySignResp:keySignResps}

	jsonResult, err := json.MarshalIndent(signBatch, "", "	")
	if nil != err {
		tKeySign.logger.Error().Err(err).Msg("fail to marshal response to json message")
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	_, err = w.Write(jsonResult)
	if nil != err {
		tKeySign.logger.Error().Err(err).Msg("fail to write response")
	}
}
