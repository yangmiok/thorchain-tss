package p2p

import (
	"context"
	"errors"
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

var errJoinPartyTimeout = errors.New("fail to join party, timeout")

type PartyCoordinator struct {
	logger             zerolog.Logger
	host               host.Host
	stopChan           chan struct{}
	timeout            time.Duration
	peersGroup         map[string]*PeerStatus
	joinPartyGroupLock *sync.Mutex
}

// NewPartyCoordinator create a new instance of PartyCoordinator
func NewPartyCoordinator(host host.Host, timeout time.Duration) *PartyCoordinator {
	pc := &PartyCoordinator{
		logger:             log.With().Str("module", "party_coordinator").Logger(),
		host:               host,
		stopChan:           make(chan struct{}),
		timeout:            timeout,
		peersGroup:         make(map[string]*PeerStatus),
		joinPartyGroupLock: &sync.Mutex{},
	}
	host.SetStreamHandler(joinPartyProtocol, pc.HandleStream)
	return pc
}

// Stop the PartyCoordinator rune
func (pc *PartyCoordinator) Stop() {
	defer pc.logger.Info().Msg("stop party coordinator")
	pc.host.RemoveStreamHandler(joinPartyProtocol)
	close(pc.stopChan)
}

// HandleStream handle party coordinate stream
func (pc *PartyCoordinator) HandleStream(stream network.Stream) {
	defer func() {
		if err := stream.Close(); err != nil {
			pc.logger.Err(err).Msg("fail to close the stream")
		}
	}()
	remotePeer := stream.Conn().RemotePeer()
	logger := pc.logger.With().Str("remote peer", remotePeer.String()).Logger()
	logger.Debug().Msg("reading from join party request")
	payload, err := ReadStreamWithBuffer(stream)
	if err != nil {
		logger.Err(err).Msgf("fail to read payload from stream")
		return
	}
	var msg messages.JoinPartyRequest
	if err := proto.Unmarshal(payload, &msg); err != nil {
		logger.Err(err).Msg("fail to unmarshal join party request")
		return
	}
	pc.joinPartyGroupLock.Lock()
	peerGroup, ok := pc.peersGroup[msg.ID]
	pc.joinPartyGroupLock.Unlock()
	if !ok {
		pc.logger.Info().Msg("this party is not ready")
		return
	}
	newFound, err := peerGroup.updatePeer(remotePeer)
	if err != nil {
		pc.logger.Error().Err(err).Msg("receive msg from unknown peer")
		return
	}
	if newFound {
		peerGroup.newFound <- true
	}
}

func (pc *PartyCoordinator) removePeerGroup(messageID string) {
	pc.joinPartyGroupLock.Lock()
	defer pc.joinPartyGroupLock.Unlock()
	delete(pc.peersGroup, messageID)
}

func (pc *PartyCoordinator) createJoinPartyGroups(messageID string, peers []string) (*PeerStatus, error) {
	pIDs, err := pc.getPeerIDs(peers)
	if err != nil {
		pc.logger.Error().Err(err).Msg("fail to parse peer id")
		return nil, err
	}
	pc.joinPartyGroupLock.Lock()
	defer pc.joinPartyGroupLock.Unlock()
	peerStatus := NewPeerStatus(pIDs, pc.host.ID())
	pc.peersGroup[messageID] = peerStatus
	return peerStatus, nil
}

func (pc *PartyCoordinator) getPeerIDs(ids []string) ([]peer.ID, error) {
	result := make([]peer.ID, len(ids))
	for i, item := range ids {
		pid, err := peer.Decode(item)
		if err != nil {
			return nil, fmt.Errorf("fail to decode peer id(%s):%w", item, err)
		}
		result[i] = pid
	}
	return result, nil
}

func (pc *PartyCoordinator) sendRequestToAll(msg *messages.JoinPartyRequest, peers []peer.ID) {
	var wg sync.WaitGroup
	wg.Add(len(peers))
	for _, el := range peers {
		go func(peer peer.ID) {
			defer wg.Done()
			if err := pc.sendRequestToPeer(msg, peer); err != nil {
				pc.logger.Error().Err(err).Msg("error in send the join party request to peer")
			}
		}(el)
	}
	wg.Wait()
}

func (pc *PartyCoordinator) sendRequestToPeer(msg *messages.JoinPartyRequest, remotePeer peer.ID) error {
	msgBuf, err := proto.Marshal(msg)
	if err != nil {
		return fmt.Errorf("fail to marshal msg to bytes: %w", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*30)
	defer cancel()
	var stream network.Stream
	var streamError error
	streamGetChan := make(chan struct{})
	go func() {
		defer close(streamGetChan)

		pc.host.Network().Conns()

		pc.logger.Info().Msgf("try to open stream to (%s) ", remotePeer)
		stream, err = pc.host.NewStream(ctx, remotePeer, joinPartyProtocol)
		if err != nil {
			streamError = fmt.Errorf("fail to create stream to peer(%s):%w", remotePeer, err)
		}
	}()

	select {
	case <-streamGetChan:
		if streamError != nil {
			pc.logger.Error().Err(streamError).Msg("fail to open stream")
			return streamError
		}
	case <-ctx.Done():
		pc.logger.Error().Err(ctx.Err()).Msg("fail to open stream with context timeout")
		// we reset the whole connection of this peer
		err := pc.host.Network().ClosePeer(remotePeer)
		pc.logger.Error().Err(err).Msgf("fail to close the connection to peer %s", remotePeer.String())
		return ctx.Err()
	}

	defer func() {
		if err := stream.Close(); err != nil {
			pc.logger.Error().Err(err).Msg("fail to close stream")
		}
	}()
	pc.logger.Info().Msgf("open stream to (%s) successfully", remotePeer)

	err = WriteStreamWithBuffer(msgBuf, stream)
	if err != nil {
		if errReset := stream.Reset(); errReset != nil {
			return errReset
		}
		return fmt.Errorf("fail to write message to stream:%w", err)
	}

	return nil
}

// JoinPartyWithRetry this method provide the functionality to join party with retry and back off
func (pc *PartyCoordinator) JoinPartyWithRetry(msg *messages.JoinPartyRequest, peers []string) ([]peer.ID, error) {
	peerGroup, err := pc.createJoinPartyGroups(msg.ID, peers)
	if err != nil {
		pc.logger.Error().Err(err).Msg("fail to create the join party group")
		return nil, err
	}
	defer pc.removePeerGroup(msg.ID)
	_, offline := peerGroup.getPeersStatus()
	var wg sync.WaitGroup
	done := make(chan struct{})
	wg.Add(1)
	go func() {
		wg.Done()
		for {
			select {
			case <-done:
				return
			default:
				pc.sendRequestToAll(msg, offline)
			}
			time.Sleep(time.Second)
		}
	}()
	// this is the total time TSS will wait for the party to form
	wg.Add(1)
	go func() {
		defer wg.Done()
		for {
			select {
			case <-peerGroup.newFound:
				pc.logger.Info().Msg("we have found the new peer")
				if peerGroup.getCoordinationStatus() {
					close(done)
					return
				}
			case <-time.After(pc.timeout):
				// timeout
				close(done)
				return
			}
		}
	}()

	wg.Wait()
	onlinePeers, _ := peerGroup.getPeersStatus()
	pc.logger.Info().Msg("----------->start broadcast........")
	pc.sendRequestToAll(msg, onlinePeers)
	pc.logger.Info().Msg("----------->finishing broadcast........")
	// we always set ourselves as online
	onlinePeers = append(onlinePeers, pc.host.ID())
	if len(onlinePeers) == len(peers) {
		return onlinePeers, nil
	}
	return onlinePeers, errJoinPartyTimeout
}

func GetStream(host host.Host, protocolID string, pid peer.ID) (network.Stream, error) {
	nt := host.Network()
	nc := nt.ConnsToPeer(pid)
	// if no connection to the given peer, we need to create one
	if len(nc) == 0 {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second*30)
		defer cancel()
		conn, err := nt.DialPeer(ctx, pid)
		if err != nil {
			return nil, err
		}
		stream, err := conn.NewStream()
		if err != nil {
			return nil, err
		}
		stream.SetProtocol(protocol.ID(protocolID))
		return stream, err
	}
	// we try to get this stream from the connection
	for _, el := range nc {
		streams := el.GetStreams()
		for _, s := range streams {
			if s.Protocol() == protocol.ID(protocolID) {
				return s, nil
			}
		}
	}
	// we cannot find the stream with this protocol, create one
	for _, el := range nc {
		stream, err := el.NewStream()
		if err != nil {
			return nil, err
		}
		stream.SetProtocol(protocol.ID(protocolID))
		return stream, nil
	}
	return nil, errors.New("we can not create a stream over a connection")
}
