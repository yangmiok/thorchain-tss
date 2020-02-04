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
	"time"

	"github.com/binance-chain/tss-lib/ecdsa/signing"
	btss "github.com/binance-chain/tss-lib/tss"
	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
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
func (tKeySign *TssKeySign) SignMessage(req KeySignReq) (*signing.SignatureData, error) {
	if len(req.PoolPubKey) == 0 {
		return nil, errors.New("empty pool pub key")
	}
	localFileName := fmt.Sprintf("localstate-%s.json", req.PoolPubKey)
	if len(tKeySign.homeBase) > 0 {
		localFileName = filepath.Join(tKeySign.homeBase, localFileName)
	}
	storedKeyGenLocalStateItem, err := common.LoadLocalState(localFileName)
	if nil != err {
		return nil, fmt.Errorf("fail to read local state file: %w", err)
	}
	var msgsBigInt []*big.Int
	fmt.Printf(">>>>>1111111111111111111>>>+%v\n", req.Messages)
	for _, m := range req.Messages {
		msgToSign, err := base64.StdEncoding.DecodeString(m)
		if nil != err {
			return nil, fmt.Errorf("fail to decode message(%s): %w", m, err)
		}
		m, err := common.MsgToHashInt(msgToSign)
		if nil != err {
			return nil, fmt.Errorf("fail to convert msg to hash int: %w", err)
		}
		msgsBigInt = append(msgsBigInt, m)
	}
	threshold, err := common.GetThreshold(len(storedKeyGenLocalStateItem.ParticipantKeys))
	if nil != err {
		return nil, err
	}
	tKeySign.logger.Debug().Msgf("keysign threshold: %d", threshold)
	partiesIDFromKeyFile, localPartyID, err := common.GetParties(storedKeyGenLocalStateItem.ParticipantKeys, storedKeyGenLocalStateItem.LocalPartyKey)
	if nil != err {
		return nil, fmt.Errorf("fail to form key sign party: %w", err)
	}
	partyIDMapFromKeyFile := common.SetupPartyIDMap(partiesIDFromKeyFile)
	tempPartyIDtoP2PID := make(map[string]peer.ID)
	err = common.SetupIDMaps(partyIDMapFromKeyFile, tempPartyIDtoP2PID)
	if nil != err {
		tKeySign.logger.Error().Msgf("error in creating mapping between partyID and P2P ID")
		return nil, err
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
		return nil, err
	}

	signers, isMember := tKeySign.getSigningPeers(standbyPeers, threshold)
	if !isMember {
		fmt.Println("we are not in this round !!!!!!!!!!!!!!!!!!!!")
		return nil, nil
	}

	partiesIDInSigning := tKeySign.tssCommonStruct.GetPartiesIDFromPeerID(signers[:], partyIDMapFromKeyFile, tempPartyIDtoP2PID)
	partyIDMap := common.SetupPartyIDMap(partiesIDInSigning)
	err = common.SetupIDMaps(partyIDMap, tKeySign.tssCommonStruct.PartyIDtoP2PID)
	if nil != err {
		tKeySign.logger.Error().Msgf("error in creating mapping between partyID and P2P ID")
		return nil, err
	}

	//now we confirm and update the signing peers
	tKeySign.tssCommonStruct.P2PPeers = common.GetPeersID(tKeySign.tssCommonStruct.PartyIDtoP2PID, tKeySign.tssCommonStruct.GetLocalPeerID())
	localKeyData, partiesIDInSigning := common.ProcessStateFile(storedKeyGenLocalStateItem, partiesIDInSigning)
	// Set up the parameters
// Note: The `id` and `moniker` fields are for convenience to allow you to easily track participants.
	// The `id` should be a unique string representing this party in the network and `moniker` can be anything (even left blank).
	// The `uniqueKey` is a unique identifying key for this peer (such as its p2p public key) as a big.Int.
	tKeySign.logger.Debug().Msgf("local party: %+v", localPartyID)

	outCh := make(chan btss.Message, len(partiesIDInSigning))
	endCh := make(chan signing.SignatureData, len(partiesIDInSigning))
	errCh := make(chan struct{})

	keySignPartyMap := make(map[string]*btss.Party, 0)
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
	el := msgsBigInt[0]
	moiker:= el.String() + ":" + strconv.Itoa(0)
	ctx := btss.NewPeerContext(partiesIDInSigning)
	eachLocalPartyID := btss.NewPartyID(localPartyID.GetId(), moiker, localPartyID.KeyInt())
	//eachLocalPartyID := amino.DeepCopy(localPartyID).(*btss.PartyID)
	//eachLocalPartyID.Moniker = index
	eachLocalPartyID.Index = localPartyID.Index
	tKeySign.localParty = append(tKeySign.localParty,eachLocalPartyID)
	params := btss.NewParameters(ctx, eachLocalPartyID, len(partiesIDInSigning), threshold)
	keySignParty := signing.NewLocalParty(el, params, localKeyData, outCh, endCh)
	keySignPartyMap[moiker] = &keySignParty


	el = msgsBigInt[1]
	moiker= el.String() + ":" + strconv.Itoa(1)
	ctx = btss.NewPeerContext(partiesIDInSigning)
	eachLocalPartyID = btss.NewPartyID(localPartyID.GetId(), moiker, localPartyID.KeyInt())
	//eachLocalPartyID := amino.DeepCopy(localPartyID).(*btss.PartyID)
	//eachLocalPartyID.Moniker = index
	localKeyData, partiesIDInSigning := common.ProcessStateFile(storedKeyGenLocalStateItem, partiesIDInSigning)
	eachLocalPartyID.Index = localPartyID.Index
	tKeySign.localParty = append(tKeySign.localParty,eachLocalPartyID)
	params = btss.NewParameters(ctx, eachLocalPartyID, len(partiesIDInSigning), threshold)
	keySignParty = signing.NewLocalParty(el, params, localKeyData, outCh, endCh)
	keySignPartyMap[moiker] = &keySignParty

	tKeySign.tssCommonStruct.SetPartyInfo(&common.PartyInfo{
		PartyMap:   keySignPartyMap,
		PartyIDMap: partyIDMap,
	})

	//start the key sign
	for _, eachParty := range keySignPartyMap {
		runparty := *eachParty
		go func() {
			if err := runparty.Start(); nil != err {
				tKeySign.logger.Error().Err(err).Msg("fail to start key sign party")
				//close(errCh)
			}
			tKeySign.logger.Info().Msg("local party is ready")
		}()
	}
	result, err := tKeySign.processKeySign(errCh, outCh, endCh)
	if nil != err {
		return nil, fmt.Errorf("fail to process key sign: %w", err)
	}
	tKeySign.logger.Info().Msg("successfully sign the message")
	return result, nil
}

func (tKeySign *TssKeySign) processKeySign(errChan chan struct{}, outCh <-chan btss.Message, endCh <-chan signing.SignatureData) (*signing.SignatureData, error) {
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

		case msg := <-endCh:
			tKeySign.logger.Debug().Msg("we have done the key sign")
			fmt.Printf(">>>>>>>>>>>>>>>>>>%s\n", msg.String())
			return &msg, nil
		}
	}
}

func (tKeySign *TssKeySign) WriteKeySignResult(w http.ResponseWriter, R, S string, status common.Status) {

	//blame := common.NewBlame()
	//blame.SetBlame(tKeySign.tssCommonStruct.Blame.FailReason, tKeySign.tssCommonStruct.Blame.BlameNodes)
	signResp := Signature{
		R:      R,
		S:      S,
		Status: status,
		Blame:  tKeySign.tssCommonStruct.BlamePeers,
	}
	jsonResult, err := json.MarshalIndent(signResp, "", "	")
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
