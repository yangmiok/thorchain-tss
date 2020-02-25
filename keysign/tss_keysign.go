package keysign

import (
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"net/http"
	"path/filepath"
	"time"

	"github.com/binance-chain/tss-lib/ecdsa/signing"
	btss "github.com/binance-chain/tss-lib/tss"
	sdk "github.com/cosmos/cosmos-sdk/types"
	p2peer "github.com/libp2p/go-libp2p-core/peer"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	cryptokey "github.com/tendermint/tendermint/crypto"
	"github.com/tendermint/tendermint/crypto/secp256k1"

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
	signatureChan   chan *p2p.Message
	localParty      *btss.PartyID
	keySignCurrent  *string
	committeeID     string
}

func NewTssKeySign(homeBase, localP2PID string, conf common.TssConfig, privKey cryptokey.PrivKey, broadcastChan chan *p2p.BroadcastMsgChan, stopChan *chan struct{}, keySignCurrent *string, msgID, committeeID string) TssKeySign {
	return TssKeySign{
		logger:          log.With().Str("module", "keySign").Logger(),
		priKey:          privKey,
		tssCommonStruct: common.NewTssCommon(localP2PID, broadcastChan, conf, msgID),
		stopChan:        stopChan,
		homeBase:        homeBase,
		syncMsg:         make(chan *p2p.Message),
		signatureChan:   make(chan *p2p.Message),
		localParty:      nil,
		keySignCurrent:  keySignCurrent,
		committeeID: committeeID,
	}
}

func (tKeySign *TssKeySign) GetTssKeySignChannels() (chan *p2p.Message, chan *p2p.Message, chan *p2p.Message) {
	return tKeySign.tssCommonStruct.TssMsg, tKeySign.syncMsg, tKeySign.signatureChan
}

func (tKeySign *TssKeySign) GetTssCommonStruct() *common.TssCommon {
	return tKeySign.tssCommonStruct
}

// signMessage
func (tKeySign *TssKeySign) SignMessage(req KeySignReq) (*signing.SignatureData,string, error) {
	if len(req.PoolPubKey) == 0 {
		return nil, req.Message, errors.New("empty pool pub key")
	}
	localFileName := fmt.Sprintf("localstate-%s.json", req.PoolPubKey)
	if len(tKeySign.homeBase) > 0 {
		localFileName = filepath.Join(tKeySign.homeBase, localFileName)
	}
	storedKeyGenLocalStateItem, err := common.LoadLocalState(localFileName)
	if err != nil {
		return nil, req.Message, fmt.Errorf("fail to read local state file: %w", err)
	}
	msgToSign, err := base64.StdEncoding.DecodeString(req.Message)
	if err != nil {
		return nil, req.Message,fmt.Errorf("fail to decode message(%s): %w", req.Message, err)
	}
	threshold := len(req.SignersPubKey) - 1
	tKeySign.logger.Debug().Msgf("keysign threshold: %d", threshold)

	selectedKey := make([]*big.Int, len(req.SignersPubKey))
	for i, item := range req.SignersPubKey {
		pk, err := sdk.GetAccPubKeyBech32(item)
		if err != nil {
			return nil, req.Message,fmt.Errorf("fail to get account pub key address(%s): %w", item, err)
		}
		secpPk := pk.(secp256k1.PubKeySecp256k1)
		key := new(big.Int).SetBytes(secpPk[:])
		selectedKey[i] = key
	}

	partiesID, nonSignParties, localPartyID, err := common.GetParties(storedKeyGenLocalStateItem.ParticipantKeys, selectedKey, storedKeyGenLocalStateItem.LocalPartyKey)
	if err != nil {
		return nil, req.Message,fmt.Errorf("fail to form key sign party: %w", err)
	}
	var nonSignerPeers [] p2peer.ID
	for _, each := range nonSignParties{
		peerID, err := common.GetPeerIDFromPartyID(each)
		nonSignerPeers= append(nonSignerPeers, peerID)
		if err != nil {
			return nil, req.Message, fmt.Errorf("fail to get nonsigner's peer ID: %w", err)
		}
	}

	if !common.Contains(partiesID, localPartyID) {
		tKeySign.logger.Info().Msgf("we are not in this rounds key sign")
		sharedSignature, err:=tKeySign.tssCommonStruct.WaitForSignature(threshold,req.PoolPubKey,tKeySign.signatureChan)
		if err != nil{
			tKeySign.tssCommonStruct.BlamePeers.SetBlame(common.BlameFailSigRecv, nil)
			return nil, "",fmt.Errorf("fail to process key sign: %w", err)
		}
		return &sharedSignature.Sig, sharedSignature.Msg, nil
	}

	tKeySign.localParty = localPartyID
	localKeyData, partiesID := common.ProcessStateFile(storedKeyGenLocalStateItem, partiesID)
	// Set up the parameters
	// Note: The `id` and `moniker` fields are for convenience to allow you to easily track participants.
	// The `id` should be a unique string representing this party in the network and `moniker` can be anything (even left blank).
	// The `uniqueKey` is a unique identifying key for this peer (such as its p2p public key) as a big.Int.
	tKeySign.logger.Debug().Msgf("local party: %+v", localPartyID)
	ctx := btss.NewPeerContext(partiesID)
	params := btss.NewParameters(ctx, localPartyID, len(partiesID), threshold)
	outCh := make(chan btss.Message, len(partiesID))
	endCh := make(chan signing.SignatureData, len(partiesID))
	errCh := make(chan struct{})
	m, err := common.MsgToHashInt(msgToSign)
	if err != nil {
		return nil, req.Message,fmt.Errorf("fail to convert msg to hash int: %w", err)
	}
	keySignParty := signing.NewLocalParty(m, params, localKeyData, outCh, endCh)
	partyIDMap := common.SetupPartyIDMap(partiesID)
	err = common.SetupIDMaps(partyIDMap, tKeySign.tssCommonStruct.PartyIDtoP2PID)
	if err != nil {
		tKeySign.logger.Error().Msgf("error in creating mapping between partyID and P2P ID")
		return nil, req.Message,err
	}
	tKeySign.tssCommonStruct.SetPartyInfo(&common.PartyInfo{
		Party:      keySignParty,
		PartyIDMap: partyIDMap,
	})

	tKeySign.tssCommonStruct.P2PPeers = common.GetPeersID(tKeySign.tssCommonStruct.PartyIDtoP2PID, tKeySign.tssCommonStruct.GetLocalPeerID())

	// we set the coordinator of the keygen
	tKeySign.tssCommonStruct.Coordinator, err = tKeySign.tssCommonStruct.GetCoordinator(hex.EncodeToString(msgToSign))
	if err != nil {
		tKeySign.logger.Error().Err(err).Msg("error in get the coordinator")
		tKeySign.tssCommonStruct.SendSignature(tKeySign.committeeID,p2p.TSSSignature,req.Message, nil, nonSignerPeers)
		return nil, req.Message,err
	}
	standbyPeers, errNodeSync := tKeySign.tssCommonStruct.NodeSync(tKeySign.syncMsg, p2p.TSSKeySignSync)
	if errNodeSync != nil {
		tKeySign.tssCommonStruct.SendSignature(tKeySign.committeeID,p2p.TSSSignature,req.Message, nil, nonSignerPeers)
		tKeySign.logger.Error().Err(err).Msgf("the nodes online are +%v", standbyPeers)
		standbyPeers = common.RemoveCoordinator(standbyPeers, tKeySign.GetTssCommonStruct().Coordinator)
		_, blamePubKeys, err := tKeySign.tssCommonStruct.GetBlamePubKeysLists(standbyPeers)
		if err != nil {
			tKeySign.logger.Error().Err(err).Msgf("error in get blame node pubkey +%v\n", errNodeSync)
			return nil, req.Message,errNodeSync
		}
		tKeySign.tssCommonStruct.BlamePeers.SetBlame(common.BlameNodeSyncCheck, blamePubKeys)

		return nil, req.Message,errNodeSync
	}
	// start the key sign
	go func() {
		if err := keySignParty.Start(); nil != err {
			tKeySign.logger.Error().Err(err).Msg("fail to start key sign party")
			close(errCh)
		}
		tKeySign.tssCommonStruct.SetPartyInfo(&common.PartyInfo{
			Party:      keySignParty,
			PartyIDMap: partyIDMap,
		})
		tKeySign.logger.Debug().Msg("local party is ready")
	}()



	result, err := tKeySign.processKeySign(errCh, outCh, endCh)
	if err != nil {
		tKeySign.tssCommonStruct.SendSignature(tKeySign.committeeID,p2p.TSSSignature,req.Message, nil, nonSignerPeers)
		return nil, req.Message,fmt.Errorf("fail to process key sign: %w", err)
	}
	tKeySign.logger.Info().Msg("successfully sign the message")
	tKeySign.tssCommonStruct.SendSignature(tKeySign.committeeID, p2p.TSSSignature,req.Message, result, nonSignerPeers)
	return result,req.Message, nil
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
			tKeySign.logger.Debug().Msgf(">>>>>>>>>>key sign msg: %s", msg.String())
			// for the sake of performance, we do not lock the status update
			// we report a rough status of current round
			*tKeySign.keySignCurrent = msg.Type()
			err := tKeySign.tssCommonStruct.ProcessOutCh(msg, p2p.TSSKeySignMsg)
			if err != nil {
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
			return &msg, nil
		}
	}
}

func (tKeySign *TssKeySign) WriteKeySignResult(w http.ResponseWriter, R, S string, status common.Status) {
	// blame := common.NewBlame()
	// blame.SetBlame(tKeySign.tssCommonStruct.Blame.FailReason, tKeySign.tssCommonStruct.Blame.BlameNodes)
	signResp := KeySignResp{
		R:      R,
		S:      S,
		Status: status,
		Blame:  tKeySign.tssCommonStruct.BlamePeers,
	}
	jsonResult, err := json.MarshalIndent(signResp, "", "	")
	if err != nil {
		tKeySign.logger.Error().Err(err).Msg("fail to marshal response to json message")
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	_, err = w.Write(jsonResult)
	if err != nil {
		tKeySign.logger.Error().Err(err).Msg("fail to write response")
	}
}
