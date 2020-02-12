package keygen

import (
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/binance-chain/tss-lib/crypto"
	bkg "github.com/binance-chain/tss-lib/ecdsa/keygen"
	btss "github.com/binance-chain/tss-lib/tss"
	"github.com/libp2p/go-libp2p-core/host"
	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	"gitlab.com/thorchain/tss/go-tss/common"
	"gitlab.com/thorchain/tss/go-tss/p2p"
	"gitlab.com/thorchain/tss/go-tss/storage"
)

type TssKeyGen struct {
	logger           zerolog.Logger
	localNodePubKey  string
	preParams        *bkg.LocalPreParams
	stopChan         chan struct{} // channel to indicate whether we should stop
	stateManager     storage.LocalStateManager
	host             host.Host
	messageValidator *p2p.MessageValidator
	messenger        *p2p.Messenger
	lock             *sync.Mutex
	partyInfo        *common.PartyInfo
}

func NewTssKeyGen(localNodePubKey string,
	preParam *bkg.LocalPreParams,
	host host.Host,
	stateManager storage.LocalStateManager) (*TssKeyGen, error) {
	tkeyGen := &TssKeyGen{
		host:            host,
		logger:          log.With().Str("module", "keyGen").Logger(),
		localNodePubKey: localNodePubKey,
		preParams:       preParam,
		stopChan:        make(chan struct{}),
		lock:            &sync.Mutex{},
		stateManager:    stateManager,
	}
	messageValidator, err := p2p.NewMessageValidator(host, tkeyGen.onMessageCallback, p2p.KeygenVerifyProtocol)
	if err != nil {
		return nil, fmt.Errorf("fail to create message validator")
	}
	tkeyGen.messageValidator = messageValidator
	messenger, err := p2p.NewMessenger(p2p.KeygenProtocol, host, tkeyGen.onMessageReceived)
	if err != nil {
		return nil, fmt.Errorf("fail to create messenger: %w", err)
	}
	tkeyGen.messenger = messenger
	return tkeyGen, nil
}

func (kg *TssKeyGen) onMessageReceived(buf []byte, remotePeer peer.ID) {
	var msg p2p.WireMessage
	if err := json.Unmarshal(buf, &msg); err != nil {
		kg.logger.Error().Err(err).Msg("fail to unmarshal keygen message")
		return
	}
	kg.logger.Info().Msgf("received message from:%s", remotePeer)
	peers := kg.getPeers([]peer.ID{
		remotePeer,
		kg.host.ID(),
	})
	if err := kg.messageValidator.VerifyMessage(&msg, peers); err != nil {
		kg.logger.Err(err).Msg("fail to verify message")
	}
}

func (kg *TssKeyGen) getPeers(exclude []peer.ID) []peer.ID {
	kg.lock.Lock()
	defer kg.lock.Unlock()
	var output []peer.ID
	for _, item := range kg.partyInfo.PartiesID {
		shouldExclude := false
		for _, p := range exclude {
			if item.Moniker == p.String() {
				shouldExclude = true
				break
			}
		}
		if !shouldExclude {
			id, err := peer.IDB58Decode(item.Moniker)
			if err != nil {
				kg.logger.Err(err).Msg("fail to decode peer id")
				return output
			}
			output = append(output, id)
		}
	}
	return output
}
func (kg *TssKeyGen) getPeersFromParty(parties []*btss.PartyID) []peer.ID {
	kg.lock.Lock()
	defer kg.lock.Unlock()
	var output []peer.ID
	for _, item := range kg.partyInfo.PartiesID {
		for _, p := range parties {
			if p.Id == item.Id && p.Moniker == item.Moniker {
				id, err := peer.IDB58Decode(item.Moniker)
				if err != nil {
					kg.logger.Err(err).Msg("fail to decode peer id")
					return output
				}
				output = append(output, id)
			}
		}
	}
	return output
}

func (kg *TssKeyGen) onMessageCallback(msg *p2p.WireMessage) {
	kg.lock.Lock()
	defer kg.lock.Unlock()
	kg.logger.Info().Msgf("confirmed message:%+v", msg)
	if _, err := kg.partyInfo.Party.UpdateFromBytes(msg.Message, msg.Routing.From, msg.Routing.IsBroadcast); err != nil {
		kg.logger.Error().Err(err).Msg("fail to update local party")
		// get who to blame
	}
}

// GenerateNewKey create a new key
func (kg *TssKeyGen) GenerateNewKey(keygenReq Request, messageID string) (*crypto.ECPoint, error) {
	partiesID, localPartyID, err := common.GetParties(keygenReq.Keys, kg.localNodePubKey)
	if err != nil {
		return nil, fmt.Errorf("fail to get keygen parties: %w", err)
	}
	keyGenLocalStateItem := storage.KeygenLocalState{
		ParticipantKeys: keygenReq.Keys,
		LocalPartyKey:   kg.localNodePubKey,
	}

	threshold, err := common.GetThreshold(len(partiesID))
	if err != nil {
		return nil, err
	}
	ctx := btss.NewPeerContext(partiesID)
	params := btss.NewParameters(ctx, localPartyID, len(partiesID), threshold)
	outCh := make(chan btss.Message, len(partiesID))
	endCh := make(chan bkg.LocalPartySaveData, len(partiesID))
	errChan := make(chan struct{})
	if kg.preParams == nil {
		kg.logger.Error().Err(err).Msg("error, empty pre-parameters")
		return nil, errors.New("error, empty pre-parameters")
	}
	keyGenParty := bkg.NewLocalParty(params, outCh, endCh, *kg.preParams)
	kg.setPartyInfo(&common.PartyInfo{
		Party:     keyGenParty,
		PartiesID: partiesID,
	})

	// start keygen
	go func() {
		defer kg.logger.Info().Msg("keyGenParty finished")
		if err := keyGenParty.Start(); nil != err {
			kg.logger.Error().Err(err).Msg("fail to start keygen party")
			close(errChan)
		}
	}()

	r, err := kg.processKeyGen(errChan, outCh, endCh, keyGenLocalStateItem, messageID)
	return r, err
}

func (kg *TssKeyGen) setPartyInfo(partyInfo *common.PartyInfo) {
	kg.lock.Lock()
	defer kg.lock.Unlock()
	kg.partyInfo = partyInfo
}

// TODO make keygen timeout calculate based on the number of parties
func (kg *TssKeyGen) getKeygenTimeout() time.Duration {
	return time.Minute * 5
}

func (kg *TssKeyGen) processKeyGen(errChan chan struct{},
	outCh <-chan btss.Message,
	endCh <-chan bkg.LocalPartySaveData,
	keyGenLocalStateItem storage.KeygenLocalState, messageID string) (*crypto.ECPoint, error) {
	defer kg.logger.Info().Msg("finished keygen process")
	kg.logger.Info().Msg("start to read messages from local party")
	keyGenTimeout := kg.getKeygenTimeout()
	for {
		select {
		case <-errChan: // when keyGenParty return
			kg.logger.Error().Msg("key gen failed")
			return nil, errors.New("error channel closed fail to start local party")
		case <-kg.stopChan: // when TSS processor receive signal to quit
			return nil, errors.New("received exit signal")

		case <-time.After(keyGenTimeout):
			// we bail out after KeyGenTimeoutSeconds
			kg.logger.Error().Msgf("fail to generate message with %s", keyGenTimeout)
			// tssCommonStruct := tKeyGen.GetTssCommonStruct()
			// localCachedItems := tssCommonStruct.TryGetAllLocalCached()
			// blamePeers, err := tssCommonStruct.TssTimeoutBlame(localCachedItems)
			// if err != nil {
			// 	tKeyGen.logger.Error().Err(err).Msg("fail to get the blamed peers")
			// 	tssCommonStruct.BlamePeers.SetBlame(common.BlameTssTimeout, nil)
			// 	return nil, fmt.Errorf("fail to get the blamed peers %w", common.ErrTssTimeOut)
			// }
			// tssCommonStruct.BlamePeers.SetBlame(common.BlameTssTimeout, blamePeers)
			return nil, common.ErrTssTimeOut

		case msg := <-outCh:
			kg.logger.Debug().Msgf(">>>>>>>>>>msg: %s", msg.String())
			// for the sake of performance, we do not lock the status update
			// we report a rough status of current round
			buf, r, err := msg.WireBytes()
			// if we cannot get the wire share, the tss keygen will fail, we just quit.
			if err != nil {
				return nil, fmt.Errorf("fail to get wire bytes: %w", err)
			}
			wireMsg := p2p.WireMessage{
				Routing:   r,
				RoundInfo: msg.Type(),
				Message:   buf,
				MessageID: messageID,
			}
			jsonBuf, err := json.Marshal(wireMsg)
			if err != nil {
				return nil, fmt.Errorf("fail to marshal wire message: %w", err)
			}
			if r.To == nil && r.IsBroadcast {
				peers := kg.getPeers([]peer.ID{
					kg.host.ID(),
				})
				kg.messenger.Send(jsonBuf, peers)
				continue
			}
			peers := kg.getPeersFromParty(r.To)
			kg.messenger.Send(jsonBuf, peers)

		case msg := <-endCh:
			kg.logger.Debug().Msgf("keygen finished successfully: %s", msg.ECDSAPub.Y().String())
			pubKey, _, err := common.GetTssPubKey(msg.ECDSAPub)
			if err != nil {
				return nil, fmt.Errorf("fail to get thorchain pubkey: %w", err)
			}
			keyGenLocalStateItem.LocalData = msg
			keyGenLocalStateItem.PubKey = pubKey
			if err := kg.stateManager.SaveLocalState(keyGenLocalStateItem); err != nil {
				return nil, fmt.Errorf("fail to save keygen result to storage: %w", err)
			}

			return msg.ECDSAPub, nil
		}
	}
}

func (kg *TssKeyGen) Start() {
	kg.messenger.Start()
	kg.messageValidator.Start()
}
func (kg *TssKeyGen) Stop() {
	kg.messenger.Stop()
	kg.messageValidator.Stop()
	close(kg.stopChan)
}
