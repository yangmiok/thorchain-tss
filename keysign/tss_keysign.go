package keysign

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"sync"
	"time"

	bc "github.com/binance-chain/tss-lib/common"
	"github.com/binance-chain/tss-lib/ecdsa/signing"
	btss "github.com/binance-chain/tss-lib/tss"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	tcrypto "github.com/tendermint/tendermint/crypto"

	"gitlab.com/thorchain/tss/go-tss/common"
	"gitlab.com/thorchain/tss/go-tss/messages"
	"gitlab.com/thorchain/tss/go-tss/p2p"
	"gitlab.com/thorchain/tss/go-tss/storage"
)

type TssKeySign struct {
	logger          zerolog.Logger
	tssCommonStruct *common.TssCommon
	stopChan        chan struct{} // channel to indicate whether we should stop
	syncMsg         chan *p2p.Message
	localParty      *btss.PartyID
	commStopChan    chan struct{}
	lastMsg         btss.Message
}

func NewTssKeySign(localP2PID string,
	conf common.TssConfig,
	broadcastChan chan *messages.BroadcastMsgChan,
	stopChan chan struct{}, msgID string, privKey tcrypto.PrivKey) *TssKeySign {
	logItems := []string{"keySign", msgID}
	return &TssKeySign{
		logger:          log.With().Strs("module", logItems).Logger(),
		tssCommonStruct: common.NewTssCommon(localP2PID, broadcastChan, conf, msgID, privKey),
		stopChan:        stopChan,
		localParty:      nil,
		commStopChan:    make(chan struct{}),
		lastMsg:         nil,
	}
}

func (tKeySign *TssKeySign) GetTssKeySignChannels() chan *p2p.Message {
	return tKeySign.tssCommonStruct.TssMsg
}

func (tKeySign *TssKeySign) GetTssCommonStruct() *common.TssCommon {
	return tKeySign.tssCommonStruct
}

// signMessage
func (tKeySign *TssKeySign) SignMessage(msgToSign []byte, localStateItem storage.KeygenLocalState, parties []string) (*bc.SignatureData, error) {
	partiesID, localPartyID, err := common.GetParties(parties, localStateItem.LocalPartyKey)
	tKeySign.localParty = localPartyID
	if err != nil {
		return nil, fmt.Errorf("fail to form key sign party: %w", err)
	}
	if !common.Contains(partiesID, localPartyID) {
		tKeySign.logger.Info().Msgf("we are not in this rounds key sign")
		return nil, nil
	}
	threshold, err := common.GetThreshold(len(localStateItem.ParticipantKeys))
	if err != nil {
		return nil, errors.New("fail to get threshold")
	}

	tKeySign.logger.Debug().Msgf("local party: %+v", localPartyID)
	ctx := btss.NewPeerContext(partiesID)
	params := btss.NewParameters(ctx, localPartyID, len(partiesID), threshold)
	outCh := make(chan btss.Message, len(partiesID))
	endCh := make(chan bc.SignatureData, len(partiesID))
	errCh := make(chan struct{})
	m, err := common.MsgToHashInt(msgToSign)
	if err != nil {
		return nil, fmt.Errorf("fail to convert msg to hash int: %w", err)
	}
	keySignParty := signing.NewLocalParty(m, params, localStateItem.LocalData, outCh, endCh)
	partyIDMap := common.SetupPartyIDMap(partiesID)
	err = common.SetupIDMaps(partyIDMap, tKeySign.tssCommonStruct.PartyIDtoP2PID)
	if err != nil {
		tKeySign.logger.Error().Msgf("error in creating mapping between partyID and P2P ID")
		return nil, err
	}
	tKeySign.tssCommonStruct.SetPartyInfo(&common.PartyInfo{
		Party:      keySignParty,
		PartyIDMap: partyIDMap,
	})

	tKeySign.tssCommonStruct.P2PPeers = common.GetPeersID(tKeySign.tssCommonStruct.PartyIDtoP2PID, tKeySign.tssCommonStruct.GetLocalPeerID())
	var keySignWg sync.WaitGroup
	keySignWg.Add(2)
	// start the key sign
	go func() {
		defer keySignWg.Done()
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
	go tKeySign.tssCommonStruct.ProcessInboundMessages(tKeySign.commStopChan, &keySignWg)
	result, err := tKeySign.processKeySign(errCh, outCh, endCh)
	if err != nil {
		close(tKeySign.commStopChan)
		return nil, fmt.Errorf("fail to process key sign: %w", err)
	}

	select {
	case <-time.After(time.Second * 5):
		close(tKeySign.commStopChan)
		break
	case <-tKeySign.tssCommonStruct.GetTaskDone():
		break
	}

	keySignWg.Wait()

	tKeySign.logger.Info().Msg("successfully sign the message")
	return result, nil
}

func (tKeySign *TssKeySign) processKeySign(errChan chan struct{}, outCh <-chan btss.Message, endCh <-chan bc.SignatureData) (*bc.SignatureData, error) {
	defer tKeySign.logger.Info().Msg("key sign finished")
	tKeySign.logger.Info().Msg("start to read messages from local party")
	tssConf := tKeySign.tssCommonStruct.GetConf()
	for {
		select {
		case <-errChan: // when key sign return
			tKeySign.logger.Error().Msg("key sign failed")
			return nil, errors.New("error channel closed fail to start local party")
		case <-tKeySign.stopChan: // when TSS processor receive signal to quit
			return nil, errors.New("received exit signal")
		case <-time.After(tssConf.KeySignTimeout):
			// we bail out after KeySignTimeoutSeconds
			tKeySign.logger.Error().Msgf("fail to sign message with %s", tssConf.KeySignTimeout.String())
			tssCommonStruct := tKeySign.GetTssCommonStruct()
			lastMsg := tKeySign.lastMsg

			var blamePeers []common.BlameNode
			var err error
			if lastMsg.IsBroadcast() == false {
				blamePeers, err = tssCommonStruct.GetUnicastBlame(lastMsg.Type())
				if err != nil {
					tKeySign.logger.Error().Err(err).Msg("error in get unicast blame")
				}
			} else {
				blamePeers, err = tssCommonStruct.GetBroadcastBlame(lastMsg.Type())
				if err != nil {
					tKeySign.logger.Error().Err(err).Msg("error in get broadcast blame")
				}
			}
			tssCommonStruct.BlamePeers.SetBlame(common.BlameTssTimeout, blamePeers)
			return nil, common.ErrTssTimeOut
		case msg := <-outCh:
			tKeySign.logger.Debug().Msgf(">>>>>>>>>>key sign msg: %s", msg.String())
			tKeySign.lastMsg = msg
			err := tKeySign.tssCommonStruct.ProcessOutCh(msg, messages.TSSKeySignMsg)
			if err != nil {
				return nil, err
			}

		case msg := <-endCh:
			tKeySign.logger.Debug().Msg("we have done the key sign")
			err := tKeySign.tssCommonStruct.NotifyTaskDone()
			if err != nil {
				tKeySign.logger.Error().Err(err).Msg("fail to broadcast the keysign done")
			}
			return &msg, nil
		}
	}
}

func (tKeySign *TssKeySign) WriteKeySignResult(w http.ResponseWriter, R, S string, status common.Status) {
	signResp := Response{
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
