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
	messageValidator, err := p2p.NewMessageValidator(host, tkeyGen.onMessageValidated, p2p.KeygenVerifyProtocol)
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
	pi := kg.getPartyInfo()
	if pi == nil {
		kg.logger.Info().Msgf("local party is not ready yet, we could only leave a message")
		kg.messageValidator.Park(&msg, remotePeer)
		return
	}
	if !msg.Routing.IsBroadcast {
		kg.logger.Info().Msg("message is not broadcast")
		kg.onMessageValidated(&msg)
	}
	kg.validateMessage(&msg, remotePeer)
}

func (kg *TssKeyGen) validateMessage(msg *p2p.WireMessage, remotePeer peer.ID) {
	pi := kg.getPartyInfo()
	if nil == pi {
		return
	}
	peers, err := pi.GetPeers([]peer.ID{
		remotePeer,
		kg.host.ID(),
	})
	if err != nil {
		kg.logger.Err(err).Msg("fail to get peers")
		return
	}
	if err := kg.messageValidator.VerifyMessage(msg, peers); err != nil {
		kg.logger.Err(err).Msg("fail to verify message")
	}
}

func (kg *TssKeyGen) onMessageValidated(msg *p2p.WireMessage) {
	pi := kg.getPartyInfo()
	kg.logger.Info().
		Str("message-id", msg.MessageID).
		Str("routing", msg.RoundInfo).
		Msg("message validated")
	if _, err := pi.Party.UpdateFromBytes(msg.Message, msg.Routing.From, msg.Routing.IsBroadcast); err != nil {
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
func (kg *TssKeyGen) getPartyInfo() *common.PartyInfo {
	kg.lock.Lock()
	defer kg.lock.Unlock()
	return kg.partyInfo
}

// TODO make keygen timeout calculate based on the number of parties
func (kg *TssKeyGen) getKeygenTimeout() time.Duration {
	return time.Minute * 5
}

func (kg *TssKeyGen) processKeyGen(errChan chan struct{},
	outCh <-chan btss.Message,
	endCh <-chan bkg.LocalPartySaveData,
	keyGenLocalStateItem storage.KeygenLocalState,
	messageID string) (*crypto.ECPoint, error) {
	defer kg.logger.Info().Msg("finished keygen process")
	kg.logger.Info().Msg("start to read messages from local party")
	keyGenTimeout := kg.getKeygenTimeout()
	for {
		select {
		case <-errChan: // when keyGenParty return
			return nil, errors.New("keygen party fail to start")
		case <-kg.stopChan: // when TSS processor receive signal to quit
			return nil, errors.New("received exit signal")

		case <-time.After(keyGenTimeout):

			kg.logger.Error().Msgf("fail to generate message with %s", keyGenTimeout)
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
			pi := kg.getPartyInfo()
			peersAll, err := pi.GetPeers([]peer.ID{
				kg.host.ID(),
			})
			if err != nil {
				return nil, fmt.Errorf("fail to get peers: %w", err)
			}
			if r.To == nil && r.IsBroadcast {
				kg.messenger.Send(jsonBuf, peersAll)
			} else {
				kg.logger.Info().Msg("##########none broadcast messages")
				peersTo, err := pi.GetPeersFromParty(r.To)
				if err != nil {
					return nil, fmt.Errorf("fail to get peers: %w", err)
				}
				kg.messenger.Send(jsonBuf, peersTo)
			}
			if err := kg.messageValidator.VerifyParkedMessages(messageID, peersAll); err != nil {
				return nil, fmt.Errorf("fail to verify parked messages:%w", err)
			}

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
