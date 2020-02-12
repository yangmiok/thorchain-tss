package p2p

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sync"
	"time"

	"github.com/gogo/protobuf/proto"
	"github.com/libp2p/go-libp2p-core/host"
	"github.com/libp2p/go-libp2p-core/network"
	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/libp2p/go-libp2p-core/protocol"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	"gitlab.com/thorchain/tss/go-tss/messages"
)

const (
	distributors = 10
)

var (
	// KeygenVerifyProtocol the protocol we used to send keygen verify messages
	KeygenVerifyProtocol protocol.ID = "/p2p/keygen-verify"
	// KeysignVerifyProtocol the protocol we used send keysign verify messages
	KeysignVerifyProtocol protocol.ID = "/p2p/keysign-verify"
)

type task struct {
	msg *messages.ConfirmMessage
	pId peer.ID
}

// MessageConfirmedHandler is callback function , it will be used to notify that a message had been confirmed
type MessageConfirmedHandler func(message *WireMessage)

// MessageValidator is a service design to make sure that all the parties receive the same messages
type MessageValidator struct {
	logger                     zerolog.Logger
	currentProtocolID          protocol.ID
	host                       host.Host
	lock                       *sync.Mutex
	cache                      map[string]*StandbyMessage
	onMessageConfirmedCallback MessageConfirmedHandler
	wg                         *sync.WaitGroup
	stopChan                   chan struct{}
	tasksChan                  chan *task
}

// NewMessageValidator create a new message
func NewMessageValidator(host host.Host, confirmedCallback MessageConfirmedHandler, protocol protocol.ID) (*MessageValidator, error) {
	mv := &MessageValidator{
		logger:                     log.With().Str("module", "message_validator").Logger(),
		host:                       host,
		lock:                       &sync.Mutex{},
		cache:                      make(map[string]*StandbyMessage),
		onMessageConfirmedCallback: confirmedCallback,
		currentProtocolID:          protocol,
		wg:                         &sync.WaitGroup{},
		stopChan:                   make(chan struct{}),
		tasksChan:                  make(chan *task),
	}
	host.SetStreamHandler(protocol, mv.handleStream)
	return mv, nil
}

// Start the message validator
func (mv *MessageValidator) Start() {
	for i := 1; i <= distributors; i++ {
		mv.wg.Add(1)
		idx := i
		go mv.messageDistributor(idx)
	}
}

// Stop the processor
func (mv *MessageValidator) Stop() {
	close(mv.stopChan)
	mv.wg.Wait()
	mv.host.RemoveStreamHandler(mv.currentProtocolID)
}

func (mv *MessageValidator) handleStream(stream network.Stream) {
	defer func() {
		if err := stream.Close(); err != nil {
			mv.logger.Err(err).Msg("fail to close stream")
		}
	}()
	remotePeer := stream.Conn().RemotePeer()
	logger := mv.logger.With().Str("remote-peer", remotePeer.String()).Logger()
	l, err := ReadLength(stream)
	if err != nil {
		logger.Err(err).Msg("fail to read length")
		return
	}
	buf, err := ReadPayload(stream, l)
	if err != nil {
		logger.Err(err).Msg("fail to read message body")
		return
	}
	var msg messages.ConfirmMessage
	if err := proto.Unmarshal(buf, &msg); err != nil {
		logger.Err(err).Msg("")
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

func (mv *MessageValidator) messageDistributor(idx int) {
	logger := mv.logger.With().
		Int("idx", idx).
		Str("protocol", string(mv.currentProtocolID)).
		Logger()
	logger.Info().Msg("message distributor started")
	defer logger.Info().Msg("message distributor stopped")
	for {
		select {
		case <-mv.stopChan:
			return
		case t, more := <-mv.tasksChan:
			if !more {
				return
			}
			if err := mv.sendConfirmMessagesToPeer(t); err != nil {
				logger.Error().Err(err).Msg("fail to send message to peer")
			}
		}
	}
}

func (mv *MessageValidator) sendConfirmMessagesToPeer(t *task) error {
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	stream, err := mv.host.NewStream(ctx, t.pId, mv.currentProtocolID)
	if err != nil {
		return err
	}
	defer func() {
		if err := stream.Close(); err != nil {
			mv.logger.Err(err).Msg("fail to close stream")
		}
	}()
	buf, err := proto.Marshal(t.msg)
	if err != nil {
		return err
	}
	if err := WriteLength(stream, uint32(len(buf))); err != nil {
		return fmt.Errorf("fail to write message length:%w", err)
	}
	_, err = stream.Write(buf)
	if err != nil {
		if errReset := stream.Reset(); errReset != nil {
			return errReset
		}
		return fmt.Errorf("fail to write message to stream: %w", err)
	}
	return nil
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
func (mv *MessageValidator) VerifyMessage(msg *WireMessage, messageID string, peers []peer.ID) error {
	mv.lock.Lock()
	defer mv.lock.Unlock()
	hash, err := msgToHashString(msg.Message)
	if err != nil {
		return fmt.Errorf("fail to generate hash from msg: %w", err)
	}

	key := fmt.Sprintf("%s-%s-%s", messageID, msg.Routing.From.Id, msg.RoundInfo)
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
	mv.sendMessagesToPeers(&messages.ConfirmMessage{
		Key:  key,
		Hash: hash,
	}, peers)
	// fan out the messages
	sm.UpdateConfirmList(mv.host.ID(), hash)
	if sm.Threshold != -1 && sm.Threshold == sm.TotalConfirmed() {
		mv.onMessageConfirmedCallback(sm.Msg)
	}
	mv.removeStandbyMessage(key)
	return nil
}
func (mv *MessageValidator) sendMessagesToPeers(msg *messages.ConfirmMessage, peers []peer.ID) {
	for _, p := range peers {
		t := &task{
			msg: msg,
			pId: p,
		}
		select {
		case <-mv.stopChan:
			// get stop signal , let's bail out now
			return
		case mv.tasksChan <- t:
		}
	}
}
