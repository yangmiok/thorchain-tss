package keysign

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/binance-chain/tss-lib/ecdsa/signing"
	btss "github.com/binance-chain/tss-lib/tss"
	"github.com/libp2p/go-libp2p-core/host"
	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	"gitlab.com/thorchain/tss/go-tss/common"
	"gitlab.com/thorchain/tss/go-tss/p2p"
	"gitlab.com/thorchain/tss/go-tss/storage"
)

type TssKeySign struct {
	logger           zerolog.Logger
	host             host.Host
	messageValidator *p2p.MessageValidator
	messenger        *p2p.Messenger
	lock             *sync.Mutex
	partyInfo        *common.PartyInfo
	stopChan         chan struct{} // channel to indicate whether we should stop
}

func NewTssKeySign(host host.Host) (*TssKeySign, error) {

	tKeySign := &TssKeySign{
		logger:   log.With().Str("module", "keysign").Logger(),
		host:     host,
		lock:     &sync.Mutex{},
		stopChan: make(chan struct{}),
	}
	messageValidator, err := p2p.NewMessageValidator(host, tKeySign.onMessageValidated, p2p.KeysignVerifyProtocol)
	if err != nil {
		return nil, fmt.Errorf("fail to create message validator")
	}
	tKeySign.messageValidator = messageValidator
	messenger, err := p2p.NewMessenger(p2p.KeysignProtocol, host, tKeySign.onMessageReceived)
	if err != nil {
		return nil, fmt.Errorf("fail to create messenger: %w", err)
	}
	tKeySign.messenger = messenger
	return tKeySign, nil
}
func (ks *TssKeySign) onMessageReceived(buf []byte, remotePeer peer.ID) {
	var msg p2p.WireMessage
	if err := json.Unmarshal(buf, &msg); err != nil {
		ks.logger.Error().Err(err).Msg("fail to unmarshal keygen message")
		return
	}
	pi := ks.getPartyInfo()
	if pi == nil {
		ks.logger.Info().Msgf("local party is not ready yet, we could only leave a message")
		ks.messageValidator.Park(&msg, remotePeer)
		return
	}
	ks.validateMessage(&msg, remotePeer)
}

func (ks *TssKeySign) validateMessage(msg *p2p.WireMessage, remotePeer peer.ID) {
	pi := ks.getPartyInfo()
	if nil == pi {
		return
	}
	peers, err := pi.GetPeers([]peer.ID{
		remotePeer,
		ks.host.ID(),
	})
	if err != nil {
		ks.logger.Err(err).Msg("fail to get peers")
		return
	}
	if err := ks.messageValidator.VerifyMessage(msg, peers); err != nil {
		ks.logger.Err(err).Msg("fail to verify message")
	}
}

func (ks *TssKeySign) onMessageValidated(msg *p2p.WireMessage) {
	ks.lock.Lock()
	defer ks.lock.Unlock()
	if _, err := ks.partyInfo.Party.UpdateFromBytes(msg.Message, msg.Routing.From, msg.Routing.IsBroadcast); err != nil {
		ks.logger.Error().Err(err).Msg("fail to update local party")
		// get who to blame
	}
}

// signMessage
func (ks *TssKeySign) SignMessage(msgToSign []byte, localStateItem storage.KeygenLocalState, parties []string, messageID string) (*signing.SignatureData, error) {
	partiesID, localPartyID, err := common.GetParties(parties, localStateItem.LocalPartyKey)

	if !common.Contains(partiesID, localPartyID) {
		ks.logger.Info().Msgf("we are not in this rounds key sign")
		return nil, nil
	}
	threshold, err := common.GetThreshold(len(localStateItem.ParticipantKeys))
	if err != nil {
		return nil, errors.New("fail to get threshold")
	}

	ks.logger.Debug().Msgf("local party: %+v", localPartyID)
	ctx := btss.NewPeerContext(partiesID)
	params := btss.NewParameters(ctx, localPartyID, len(partiesID), threshold)
	outCh := make(chan btss.Message, len(partiesID))
	endCh := make(chan signing.SignatureData, len(partiesID))
	errCh := make(chan struct{})
	m, err := common.MsgToHashInt(msgToSign)
	if err != nil {
		return nil, fmt.Errorf("fail to convert msg to hash int: %w", err)
	}
	keySignParty := signing.NewLocalParty(m, params, localStateItem.LocalData, outCh, endCh)

	ks.setPartyInfo(&common.PartyInfo{
		Party:     keySignParty,
		PartiesID: partiesID,
	})

	// start the key sign
	go func() {
		if err := keySignParty.Start(); nil != err {
			ks.logger.Error().Err(err).Msg("fail to start key sign party")
			close(errCh)
		}

		ks.logger.Debug().Msg("local party is ready")
	}()

	result, err := ks.processKeySign(errCh, outCh, endCh, messageID)
	if err != nil {
		return nil, fmt.Errorf("fail to process key sign: %w", err)
	}
	ks.logger.Info().Msg("successfully sign the message")
	return result, nil
}

func (ks *TssKeySign) setPartyInfo(partyInfo *common.PartyInfo) {
	ks.lock.Lock()
	defer ks.lock.Unlock()
	ks.partyInfo = partyInfo
}
func (ks *TssKeySign) getPartyInfo() *common.PartyInfo {
	ks.lock.Lock()
	defer ks.lock.Unlock()
	return ks.partyInfo
}

// TODO make keygen timeout calculate based on the number of parties
func (ks *TssKeySign) getKeysignTimeout() time.Duration {
	return time.Minute * 5
}

func (ks *TssKeySign) processKeySign(errChan chan struct{}, outCh <-chan btss.Message, endCh <-chan signing.SignatureData, messageID string) (*signing.SignatureData, error) {
	defer ks.logger.Info().Msg("key sign finished")
	ks.logger.Info().Msg("start to read messages from local party")
	keysignTimeout := ks.getKeysignTimeout()
	for {
		select {
		case <-errChan: // when key sign return
			ks.logger.Error().Msg("key sign failed")
			return nil, errors.New("error channel closed fail to start local party")
		case <-ks.stopChan: // when TSS processor receive signal to quit
			return nil, errors.New("received exit signal")
		case <-time.After(keysignTimeout):
			// we bail out after KeySignTimeoutSeconds
			// tKeySign.logger.Error().Msgf("fail to sign message with %s", tssConf.KeySignTimeout.String())
			// tssCommonStruct := tKeySign.GetTssCommonStruct()
			// localCachedItems := tssCommonStruct.TryGetAllLocalCached()
			// blamePeers, err := tssCommonStruct.TssTimeoutBlame(localCachedItems)
			// if err != nil {
			// 	tKeySign.logger.Error().Err(err).Msg("fail to get the blamed peers")
			// 	tssCommonStruct.BlamePeers.SetBlame(common.BlameTssTimeout, nil)
			// 	return nil, fmt.Errorf("fail to get the blamed peers %w", common.ErrTssTimeOut)
			// }
			// tssCommonStruct.BlamePeers.SetBlame(common.BlameTssTimeout, blamePeers)
			return nil, common.ErrTssTimeOut
		case msg := <-outCh:
			ks.logger.Debug().Msgf(">>>>>>>>>>key sign msg: %s", msg.String())
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
			pi := ks.getPartyInfo()
			peersAll, err := pi.GetPeers([]peer.ID{
				ks.host.ID(),
			})
			if err != nil {
				return nil, fmt.Errorf("fail to get peers: %w", err)
			}
			if r.To == nil && r.IsBroadcast {
				ks.messenger.Send(jsonBuf, peersAll)
				continue
			}
			peersTo, err := pi.GetPeersFromParty(r.To)
			if err != nil {
				return nil, fmt.Errorf("fail to get peers: %w", err)
			}
			ks.messenger.Send(jsonBuf, peersTo)
			if err := ks.messageValidator.VerifyParkedMessages(messageID, peersAll); err != nil {
				return nil, fmt.Errorf("fail to verify parked messages:%w", err)
			}
		case msg := <-endCh:
			ks.logger.Debug().Msg("we have done the key sign")
			return &msg, nil
		}
	}
}

func (ks *TssKeySign) WriteKeySignResult(w http.ResponseWriter, R, S string, status common.Status) {
	// blame := common.NewBlame()
	// blame.SetBlame(tKeySign.tssCommonStruct.Blame.FailReason, tKeySign.tssCommonStruct.Blame.BlameNodes)
	signResp := Response{
		R:      R,
		S:      S,
		Status: status,
		Blame:  common.NoBlame,
	}
	jsonResult, err := json.MarshalIndent(signResp, "", "	")
	if err != nil {
		ks.logger.Error().Err(err).Msg("fail to marshal response to json message")
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	_, err = w.Write(jsonResult)
	if err != nil {
		ks.logger.Error().Err(err).Msg("fail to write response")
	}
}

func (ks *TssKeySign) Start() {
	ks.messenger.Start()
	ks.messageValidator.Start()
}
func (ks *TssKeySign) Stop() {
	ks.messenger.Stop()
	ks.messageValidator.Stop()
	close(ks.stopChan)
}
