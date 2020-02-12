package p2p

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/libp2p/go-libp2p-core/host"
	"github.com/libp2p/go-libp2p-core/network"
	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/libp2p/go-libp2p-core/protocol"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

type distributeTask struct {
	msg []byte
	pId peer.ID
}
type MessageReceivedHandler func(msg []byte, remotePeer peer.ID)

type Messenger struct {
	logger            zerolog.Logger
	currentProtocolID protocol.ID
	host              host.Host
	wg                *sync.WaitGroup
	stopChan          chan struct{}
	tasksChan         chan *distributeTask
	onReceived        MessageReceivedHandler
}

// NewMessenger create a new instance of Messenger
func NewMessenger(pID protocol.ID,
	host host.Host,
	onReceived MessageReceivedHandler) (*Messenger, error) {
	m := &Messenger{
		logger:            log.With().Str("module", "messenger").Str("protocol", string(pID)).Logger(),
		host:              host,
		onReceived:        onReceived,
		currentProtocolID: pID,
		wg:                &sync.WaitGroup{},
		stopChan:          make(chan struct{}),
		tasksChan:         make(chan *distributeTask),
	}
	host.SetStreamHandler(pID, m.handleStream)
	return m, nil
}

func (m *Messenger) handleStream(stream network.Stream) {
	defer func() {
		if err := stream.Close(); err != nil {
			m.logger.Err(err).Msg("fail to close stream")
		}
	}()
	remotePeer := stream.Conn().RemotePeer()
	logger := m.logger.With().Str("remote-peer", remotePeer.String()).Logger()
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
	if m.onReceived != nil {
		m.onReceived(buf, remotePeer)
	}
}

func (m *Messenger) messageDistributor(idx int) {
	logger := m.logger.With().
		Int("idx", idx).
		Str("protocol", string(m.currentProtocolID)).
		Logger()
	logger.Info().Msg("message distributor started")
	defer logger.Info().Msg("message distributor stopped")
	for {
		select {
		case <-m.stopChan:
			return
		case t, more := <-m.tasksChan:
			if !more {
				return
			}
			if err := m.messagesToPeer(t); err != nil {
				logger.Error().Err(err).Msg("fail to send message to peer")
			}
		}
	}
}

func (m *Messenger) messagesToPeer(t *distributeTask) error {
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	stream, err := m.host.NewStream(ctx, t.pId, m.currentProtocolID)
	if err != nil {
		return err
	}
	defer func() {
		if err := stream.Close(); err != nil {
			m.logger.Err(err).Msg("fail to close stream")
		}
	}()

	if err := WriteLength(stream, uint32(len(t.msg))); err != nil {
		return fmt.Errorf("fail to write message length:%w", err)
	}
	_, err = stream.Write(t.msg)
	if err != nil {
		if errReset := stream.Reset(); errReset != nil {
			return errReset
		}
		return fmt.Errorf("fail to write message to stream: %w", err)
	}
	return nil
}
func (m *Messenger) Send(buf []byte, peers []peer.ID) {
	for _, p := range peers {
		t := &distributeTask{
			msg: buf,
			pId: p,
		}
		select {
		case <-m.stopChan:
			// get stop signal , let's bail out now
			return
		case m.tasksChan <- t:
		}
	}
}

// Start the message validator
func (m *Messenger) Start() {
	for i := 1; i <= distributors; i++ {
		m.wg.Add(1)
		idx := i
		go m.messageDistributor(idx)
	}
}

// Stop the processor
func (m *Messenger) Stop() {
	close(m.stopChan)
	m.wg.Wait()
	m.host.RemoveStreamHandler(m.currentProtocolID)
}
