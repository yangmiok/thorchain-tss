package p2p

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sync"

	"github.com/gogo/protobuf/proto"
	"github.com/libp2p/go-libp2p-core/host"
	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/libp2p/go-libp2p-core/protocol"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	"gitlab.com/thorchain/tss/go-tss/messages"
)

const (
	distributors = 10
)

// MessageConfirmedHandler is callback function , it will be used to notify that a message had been confirmed
type MessageConfirmedHandler func(message *WireMessage)

// MessageValidator is a service design to make sure that all the parties receive the same messages
type MessageValidator struct {
	logger                     zerolog.Logger
	currentProtocolID          protocol.ID
	host                       host.Host
	messenger                  *Messenger
	lock                       *sync.Mutex
	cache                      map[string]*StandbyMessage
	onMessageConfirmedCallback MessageConfirmedHandler
}

// NewMessageValidator create a new message
func NewMessageValidator(host host.Host,
	confirmedCallback MessageConfirmedHandler,
	protocol protocol.ID) (*MessageValidator, error) {

	mv := &MessageValidator{
		logger:                     log.With().Str("module", "message_validator").Logger(),
		host:                       host,
		lock:                       &sync.Mutex{},
		cache:                      make(map[string]*StandbyMessage),
		onMessageConfirmedCallback: confirmedCallback,
		currentProtocolID:          protocol,
	}
	messenger, err := NewMessenger(protocol, host, mv.onReceivedMessage)
	if err != nil {
		return nil, fmt.Errorf("fail to create messenger: %w", err)
	}
	mv.messenger = messenger
	return mv, nil
}

func (mv *MessageValidator) onReceivedMessage(buf []byte, remotePeer peer.ID) {
	var msg messages.ConfirmMessage
	if err := proto.Unmarshal(buf, &msg); err != nil {
		mv.logger.Error().Err(err).Msg("fail to unmarshal confirm message")
		return
	}
	mv.onConfirmMessage(&msg, remotePeer)
}

func (mv *MessageValidator) onConfirmMessage(msg *messages.ConfirmMessage, remotePeer peer.ID) {
	sm := mv.getStandbyMessage(msg.Key)
	if nil == sm {
		// someone confirm the message before we do
		sm := NewStandbyMessage(nil, msg.Hash, -1)
		mv.cache[msg.Key] = sm
		sm.UpdateConfirmList(remotePeer, msg.Hash)
		return
	}
	sm.UpdateConfirmList(remotePeer, msg.Hash)
	if sm.Msg == nil {
		return
	}

	if sm.Threshold != -1 && sm.Threshold == sm.TotalConfirmed() {
		mv.onMessageConfirmedCallback(sm.Msg)
	}
	mv.removeStandbyMessage(msg.Key)
}
func (mv *MessageValidator) removeStandbyMessage(key string) {
	mv.lock.Lock()
	defer mv.lock.Unlock()
	delete(mv.cache, key)
}
func (mv *MessageValidator) getStandbyMessage(key string) *StandbyMessage {
	mv.lock.Lock()
	defer mv.lock.Unlock()
	return mv.cache[key]
}

// msgToHashString , this is required to be here for now, to avoid import cycle
func msgToHashString(msg []byte) (string, error) {
	h := sha256.New()
	_, err := h.Write(msg)
	if err != nil {
		return "", fmt.Errorf("fail to caculate sha256 hash: %w", err)
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// VerifyMessage add a message to local cache, and send messages to all the peers
func (mv *MessageValidator) VerifyMessage(msg *WireMessage, peers []peer.ID) error {
	mv.lock.Lock()
	defer mv.lock.Unlock()
	hash, err := msgToHashString(msg.Message)
	if err != nil {
		return fmt.Errorf("fail to generate hash from msg: %w", err)
	}

	key := fmt.Sprintf("%s-%s-%s", msg.MessageID, msg.Routing.From.Id, msg.RoundInfo)
	sm := mv.getStandbyMessage(key)
	if sm == nil {
		// first one to receive the messsage
		sm := NewStandbyMessage(msg, hash, len(peers)+1)
		mv.cache[key] = sm
		sm.UpdateConfirmList(mv.host.ID(), hash)
		return nil
	}
	if sm.Msg == nil {
		sm.Msg = msg
	}
	buf, err := proto.Marshal(&messages.ConfirmMessage{
		Key:  key,
		Hash: hash,
	})
	if err != nil {
		return fmt.Errorf("fail to marshal confirm message: %w", err)
	}
	mv.messenger.Send(buf, peers)
	// fan out the messages
	sm.UpdateConfirmList(mv.host.ID(), hash)
	if sm.Threshold != -1 && sm.Threshold == sm.TotalConfirmed() {
		mv.onMessageConfirmedCallback(sm.Msg)
	}
	mv.removeStandbyMessage(key)
	return nil
}

// Start the validator
func (mv *MessageValidator) Start() {
	mv.messenger.Start()
}

// Stop the process
func (mv *MessageValidator) Stop() {
	mv.messenger.Stop()
}
