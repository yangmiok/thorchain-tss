package tss

import (
	"fmt"
	"sync"

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
)

type KeygenService struct {
	priKey    cryptokey.PrivKey
	preParams *bkg.LocalPreParams
	logger    zerolog.Logger
	lock      *sync.Mutex
	host      host.Host
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

	ctx := btss.NewPeerContext(partyIDs)
	params := btss.NewParameters(ctx, localPartyID, len(partyIDs), threshold)
	outCh := make(chan btss.Message, len(partiesID))
	endCh := make(chan bkeygen.LocalPartySaveData, len(partiesID))
	errChan := make(chan struct{})
	if tKeyGen.preParams == nil {
		tKeyGen.logger.Error().Err(err).Msg("error, empty pre-parameters")
		return nil, errors.New("error, empty pre-parameters")
	}
	keyGenParty := bkeygen.NewLocalParty(params, outCh, endCh, *tKeyGen.preParams)
	partyIDMap := common.SetupPartyIDMap(partiesID)
	err = common.SetupIDMaps(partyIDMap, tKeyGen.tssCommonStruct.PartyIDtoP2PID)
	if err != nil {
		tKeyGen.logger.Error().Msgf("error in creating mapping between partyID and P2P ID")
		return nil, err
	}
}
