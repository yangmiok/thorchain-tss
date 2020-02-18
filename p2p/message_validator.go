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
	messageBox                 MailBox
	callbackLock               *sync.Mutex
}

// NewMessageValidator create a new message
func NewMessageValidator(host host.Host,
	confirmedCallback MessageConfirmedHandler,
	protocol protocol.ID) (*MessageValidator, error) {
	messageBox, err := NewMailBoxImp()
	if err != nil {
		return nil, fmt.Errorf("fail to create local in memory cache: %w", err)
	}
	mv := &MessageValidator{
		logger:                     log.With().Str("module", "message_validator").Logger(),
		host:                       host,
		lock:                       &sync.Mutex{},
		cache:                      make(map[string]*StandbyMessage),
		callbackLock:               &sync.Mutex{},
		onMessageConfirmedCallback: confirmedCallback,
		currentProtocolID:          protocol,
		messageBox:                 messageBox,
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
	mv.lock.Lock()
	defer mv.lock.Unlock()
	sm := mv.getStandbyMessage(msg.Key)
	if nil == sm {
		// someone confirm the message before we do
		sm = NewStandbyMessage(nil, msg.Hash, -1)
		mv.setStandbyMessage(msg.Key, sm)
	}
	sm.UpdateConfirmList(remotePeer, msg.Hash)
	if sm.Msg == nil {
		return
	}

	if sm.Threshold != -1 && sm.Validated() {
		mv.fireCallback(sm, msg.Key)
	}
}
func (mv *MessageValidator) removeStandbyMessage(key string) {

	delete(mv.cache, key)
}
func (mv *MessageValidator) getStandbyMessage(key string) *StandbyMessage {

	return mv.cache[key]
}
func (mv *MessageValidator) setStandbyMessage(key string, msg *StandbyMessage) {

	mv.cache[key] = msg
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
		sm = NewStandbyMessage(msg, hash, len(peers)+1)
		mv.setStandbyMessage(key, sm)
	}
	sm.UpdateConfirmList(mv.host.ID(), hash)
	if sm.Msg == nil {
		sm.Msg = msg
		sm.Threshold = len(peers) + 1
	}
	buf, err := proto.Marshal(&messages.ConfirmMessage{
		Key:  key,
		Hash: hash,
	})
	if err != nil {
		return fmt.Errorf("fail to marshal confirm message: %w", err)
	}
	// fan out the messages
	go mv.messenger.Send(buf, peers)

	if sm.Threshold != -1 && sm.Validated() {
		mv.fireCallback(sm, key)
	}
	return nil
}
func (mv *MessageValidator) fireCallback(sm *StandbyMessage, key string) {
	sm.callback.Do(func() {
		mv.onMessageConfirmedCallback(sm.Msg)
		mv.removeStandbyMessage(key)
		sm.Msg = nil
	})
}

// Park a message into MailBox usually it means the local party is not ready
func (mv *MessageValidator) Park(msg *WireMessage, remotePeer peer.ID) {
	mv.messageBox.AddMessage(msg.MessageID, msg, remotePeer)
}

// VerifyParkedMessages when local party is ready , check whether they are messages left in the message box that can be verify now
func (mv *MessageValidator) VerifyParkedMessages(messageID string, peers []peer.ID) error {
	cachedMessages := mv.messageBox.GetMessages(messageID)
	defer mv.messageBox.RemoveMessage(messageID)
	for _, item := range cachedMessages {
		// exclude the node who originally send messages to us
		var peerIDs []peer.ID
		for _, p := range peers {
			if p.String() != item.RemotePeer.String() {
				peerIDs = append(peerIDs, p)
			}
		}
		if err := mv.VerifyMessage(item.Message, peerIDs); err != nil {
			return err
		}
	}
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
