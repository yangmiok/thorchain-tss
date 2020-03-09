package keysign

import (
	"encoding/hex"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"sync"
	"time"

	bc "github.com/binance-chain/tss-lib/common"
	"github.com/binance-chain/tss-lib/ecdsa/signing"
	btss "github.com/binance-chain/tss-lib/tss"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

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
	localParties    []*btss.PartyID
	commStopChan    chan struct{}
	sigMsgMap       map[string][]byte
}

func NewTssKeySign(localP2PID string,
	conf common.TssConfig,
	broadcastChan chan *messages.BroadcastMsgChan,
	stopChan chan struct{},
	msgID string, msgNum uint32) *TssKeySign {
	logItems := []string{"keySign", msgID}
	return &TssKeySign{
		logger:          log.With().Strs("module", logItems).Logger(),
		tssCommonStruct: common.NewTssCommon(localP2PID, broadcastChan, conf, msgID, msgNum),
		stopChan:        stopChan,
		localParties:    make([]*btss.PartyID, 0),
		commStopChan:    make(chan struct{}),
		sigMsgMap:       make(map[string][]byte),
	}
}

func (tKeySign *TssKeySign) GetTssKeySignChannels() chan *p2p.Message {
	return tKeySign.tssCommonStruct.TssMsg
}

func (tKeySign *TssKeySign) GetTssCommonStruct() *common.TssCommon {
	return tKeySign.tssCommonStruct
}

// signMessage
func (tKeySign *TssKeySign) SignMessage(msgsToSign [][]byte, localStateItem storage.KeygenLocalState, parties []string) ([]*bc.SignatureData, error) {
	partiesID, localPartyID, err := common.GetParties(parties, localStateItem.LocalPartyKey)
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
	outCh := make(chan btss.Message, len(partiesID)*len(msgsToSign))
	endCh := make(chan bc.SignatureData, len(partiesID)*len(msgsToSign))
	errCh := make(chan struct{})

	keySignPartyMap := make(map[string]btss.Party, 0)
	for i, val := range msgsToSign {
		m, err := common.MsgToHashInt(val)
		if err != nil {
			return nil, fmt.Errorf("fail to convert msg to hash int: %w", err)
		}
		tKeySign.sigMsgMap[hex.EncodeToString(m.Bytes())] = val
		moniker := m.String() + ":" + strconv.Itoa(i)
		ctx := btss.NewPeerContext(partiesID)
		_, eachLocalPartyID, err := common.GetParties(parties, localStateItem.LocalPartyKey)
		if err != nil {
			return nil, fmt.Errorf("error to create parties in batch signging %w\n", err)
		}
		eachLocalPartyID.Moniker = moniker
		tKeySign.localParties = append(tKeySign.localParties, eachLocalPartyID)
		params := btss.NewParameters(ctx, eachLocalPartyID, len(partiesID), threshold)
		keySignParty := signing.NewLocalParty(m, params, localStateItem.LocalData, outCh, endCh)
		keySignPartyMap[moniker] = keySignParty
	}
	partyIDMap := common.SetupPartyIDMap(partiesID)
	err = common.SetupIDMaps(partyIDMap, tKeySign.tssCommonStruct.PartyIDtoP2PID)
	if err != nil {
		tKeySign.logger.Error().Msgf("error in creating mapping between partyID and P2P ID")
		return nil, err
	}
	tKeySign.tssCommonStruct.SetPartyInfo(&common.PartyInfo{
		PartyMap:   keySignPartyMap,
		PartyIDMap: partyIDMap,
	})

	tKeySign.tssCommonStruct.P2PPeers = common.GetPeersID(tKeySign.tssCommonStruct.PartyIDtoP2PID, tKeySign.tssCommonStruct.GetLocalPeerID())

	//start the key sign
	var keySignWg sync.WaitGroup
	keySignWg.Add(1 + len(keySignPartyMap))
	for _, eachParty := range keySignPartyMap {
		runParty := eachParty
		go func() {
			defer keySignWg.Done()
			if err := runParty.Start(); nil != err {
				tKeySign.logger.Error().Err(err).Msg("fail to start key sign party")
				close(errCh)
			}
			tKeySign.logger.Info().Msg("local party is ready")
		}()
	}

	go tKeySign.tssCommonStruct.ProcessInboundMessages(tKeySign.commStopChan, &keySignWg)
	results, err := tKeySign.processKeySign(len(msgsToSign), errCh, outCh, endCh)
	if err != nil {
		return nil, fmt.Errorf("fail to process key sign: %w", err)
	}
	keySignWg.Wait()

	tKeySign.logger.Info().Msg("successfully sign the message")
	sort.SliceStable(results, func(i, j int) bool {
		a := hex.EncodeToString(results[i].Signature)
		b := hex.EncodeToString(results[j].Signature)
		return a < b
	})
	return results, nil
}

func (tKeySign *TssKeySign) processKeySign(requestNum int, errChan chan struct{}, outCh <-chan btss.Message, endCh <-chan bc.SignatureData) ([]*bc.SignatureData, error) {
	var signatures []*bc.SignatureData
	defer tKeySign.logger.Info().Msg("key sign finished")
	tKeySign.logger.Info().Msg("start to read messages from local party")
	tssConf := tKeySign.tssCommonStruct.GetConf()
	defer close(tKeySign.commStopChan)
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
			err := tKeySign.tssCommonStruct.ProcessOutCh(msg, messages.TSSKeySignMsg)
			if err != nil {
				return nil, err
			}

		case msg := <-endCh:
			orgSignMsg, ok := tKeySign.sigMsgMap[hex.EncodeToString(msg.GetM())]
			if !ok {
				tKeySign.logger.Error().Msg("error in find the original msg")
				orgSignMsg = []byte("unknown Msg")
			}
			msg.M = orgSignMsg
			signatures = append(signatures, &msg)
			if len(signatures) == requestNum {
				tKeySign.logger.Debug().Msg("we have done the key sign")
				return signatures, nil
			}
		}
	}
}
