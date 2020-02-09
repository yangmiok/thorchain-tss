package tss

import (
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/binance-chain/tss-lib/crypto"
	bkg "github.com/binance-chain/tss-lib/ecdsa/keygen"
	btss "github.com/binance-chain/tss-lib/tss"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/libp2p/go-libp2p-core/host"
	"github.com/libp2p/go-libp2p-core/network"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	cryptokey "github.com/tendermint/tendermint/crypto"

	"gitlab.com/thorchain/tss/go-tss/common"
	"gitlab.com/thorchain/tss/go-tss/keygen"
	"gitlab.com/thorchain/tss/go-tss/p2p"
)

// KeygenService is a service that will be able to work with other parties to generate a new key
// all keygen related logic stays in here
type KeygenService struct {
	priKey    cryptokey.PrivKey
	preParams *bkg.LocalPreParams
	logger    zerolog.Logger
	host      host.Host
	lock      *sync.Mutex
	partyInfo *common.PartyInfo
	wg        *sync.WaitGroup
	stopChan  chan struct{}
}

// NewKeygenService create a new instance of KeygenService which will perform keygen
func NewKeygenService(host host.Host, priKey cryptokey.PrivKey, preParams *bkg.LocalPreParams) (*KeygenService, error) {
	return &KeygenService{
		priKey:    priKey,
		preParams: preParams,
		logger:    log.With().Str("module", "keygen").Logger(),
		lock:      &sync.Mutex{},
		host:      host,
		wg:        &sync.WaitGroup{},
		stopChan:  make(chan struct{}),
	}, nil
}

// HandleStream is to handle keygen related messages
// this method will be mapped to the stream handler of keygenProtocolID
func (ks *KeygenService) HandleStream(stream network.Stream) {
	remotePeer := stream.Conn().RemotePeer()
	logger := ks.logger.With().Str("remote-peer", remotePeer.String()).Logger()
	defer func() {
		if err := stream.Close(); err != nil {
			logger.Error().Err(err).Msg("fail to close stream")
		}
	}()
	// read
}

// GenerateNewKey generate a new key
func (ks *KeygenService) GenerateNewKey(req keygen.Request) (*keygen.Response, error) {
	pubKey, err := sdk.Bech32ifyAccPub(ks.priKey.PubKey())
	if err != nil {
		return nil, fmt.Errorf("fail to genearte the key: %w", err)
	}
	partyIDs, localPartyID, err := getParties(req.Keys, pubKey)
	if err != nil {
		return nil, fmt.Errorf("fail to get keygen parties: %w", err)
	}
	keyGenLocalStateItem := common.KeygenLocalStateItem{
		ParticipantKeys: req.Keys,
		LocalPartyKey:   pubKey,
	}
	threshold, err := getThreshold(len(partyIDs))
	if err != nil {
		return nil, fmt.Errorf("fail to get keygen threshold: %w", err)
	}

	ctx := btss.NewPeerContext(partyIDs)
	params := btss.NewParameters(ctx, localPartyID, len(partyIDs), threshold)
	outCh := make(chan btss.Message, len(partyIDs))
	endCh := make(chan bkg.LocalPartySaveData, len(partyIDs))
	errChan := make(chan struct{})
	if ks.preParams == nil {
		ks.logger.Error().Err(err).Msg("empty pre-parameters")
		return nil, errors.New("error, empty pre-parameters")
	}
	partyInfo := &common.PartyInfo{
		Party:      bkg.NewLocalParty(params, outCh, endCh, *ks.preParams),
		PartyIDMap: getPartyIDMap(partyIDs),
	}
	ks.setLocalPartyInfo(partyInfo)
	// start keygen
	go func() {
		defer ks.logger.Info().Msg("keygen party finished")
		if err := partyInfo.Party.Start(); nil != err {
			ks.logger.Error().Err(err).Msg("fail to start keygen party")
			close(errChan)
		}
	}()
	result, err := ks.processKeygen(errChan, outCh, endCh, keyGenLocalStateItem)
	if err != nil {
		ks.logger.Error().Err(err).Msg("fail to process keygen")
		return nil, fmt.Errorf("fail to process keygen")
	}
	_ = result
}
func (ks *KeygenService) setLocalPartyInfo(partyInfo *common.PartyInfo) {
	ks.lock.Lock()
	defer ks.lock.Unlock()
	ks.partyInfo = partyInfo
}

// getKeygenTimeout return a duration indicate how long it should wait before timeout the keygen process
// the value for keygen timeout varies based on the numbers of keygen parties
func (ks *KeygenService) getKeygenTimeout() time.Duration {
	// TODO update this to a dynamic value calculate based on the number of keygen parties.
	return time.Minute
}
func (ks *KeygenService) processKeygen(errChan chan struct{},
	outCh <-chan btss.Message,
	endCh <-chan bkg.LocalPartySaveData,
	keyGenLocalStateItem common.KeygenLocalStateItem) (*crypto.ECPoint, error) {
	defer ks.logger.Info().Msg("keygen process finished")
	ks.logger.Info().Msg("start to process keygen messages")
	timeout := ks.getKeygenTimeout()
	for {
		select {
		case <-errChan: // when keyGenParty return
			return nil, errors.New("keygen failed")
		case <-ks.stopChan: // when TSS processor receive signal to quit
			return nil, errors.New("received exit signal")
		case <-time.After(timeout):
			// add blame
			return nil, common.ErrTssTimeOut
		case msg := <-outCh:
			ks.logger.Debug().Msgf(">>>>>>>>>>msg: %s", msg.String())
			// these are the messages we need to send to other parties

		case m, ok := <-tKeyGen.tssCommonStruct.TssMsg:
			if !ok {
				return nil, nil
			}

		case msg := <-endCh:
			ks.logger.Debug().Msgf("we have done the keygen %s", msg.ECDSAPub.Y().String())
			if err := tKeyGen.AddLocalPartySaveData(tKeyGen.homeBase, msg, keyGenLocalStateItem); nil != err {
				return nil, fmt.Errorf("fail to save key gen result to local store: %w", err)
			}
			return msg.ECDSAPub, nil
		}
	}
}
