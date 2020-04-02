package p2p

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/cenkalti/backoff/v4"
	"github.com/gogo/protobuf/proto"
	"github.com/libp2p/go-libp2p-core/host"
	"github.com/libp2p/go-libp2p-core/network"
	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	"gitlab.com/thorchain/tss/go-tss/messages"
)

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

	peerGroup, ok := pc.peersGroup[msg.ID]
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

	return
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
	peerStatus := NewPeerStatus(pIDs, pc.host.ID())
	pc.joinPartyGroupLock.Lock()
	pc.peersGroup[messageID] = &peerStatus
	pc.joinPartyGroupLock.Unlock()
	return &peerStatus, nil
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
			_, err := pc.sendRequestToPeer(msg, peer)
			if err != nil {
				pc.logger.Error().Err(err).Msg("error in send the join party request to peer")
			}
		}(el)
	}
	wg.Wait()
}

func (pc *PartyCoordinator) sendRequestToPeer(msg *messages.JoinPartyRequest, remotePeer peer.ID) (bool, error) {

	msgBuf, err := proto.Marshal(msg)
	if err != nil {
		return false, fmt.Errorf("fail to marshal msg to bytes: %w", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*30)
	defer cancel()
	stream, err := pc.host.NewStream(ctx, remotePeer, joinPartyProtocol)
	if err != nil {
		return false, fmt.Errorf("fail to create stream to peer(%s):%w", remotePeer, err)
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
			return false, errReset
		}
		return false, fmt.Errorf("fail to write message to stream:%w", err)
	}

	return false, nil
}

// JoinPartyWithRetry this method provide the functionality to join party with retry and backoff
func (pc *PartyCoordinator) JoinPartyWithRetry(msg *messages.JoinPartyRequest, peers []string) ([]peer.ID, error) {
	peerGroup, err := pc.createJoinPartyGroups(msg.ID, peers)
	if err != nil {
		pc.logger.Error().Err(err).Msg("fail to create the join party group")
		return nil, err
	}
	defer pc.removePeerGroup(msg.ID)
	_, offline := peerGroup.getPeersStatus()

	bf := backoff.NewExponentialBackOff()
	bf.MaxElapsedTime = pc.timeout
	var wg sync.WaitGroup
	go func() {
		wg.Add(1)
		defer wg.Done()
		pc.sendRequestToAll(msg, offline)
	}()

	err = backoff.Retry(func() error {
		ret := peerGroup.getCoordinationStatus()
		if ret {
			return nil
		}
		select {
		case <-peerGroup.newFound:
			pc.logger.Info().Msg("we have found the new peer, reset the backoff timer")
			bf.Reset()
		default:
			pc.logger.Debug().Msg("no new peer found")
		}
		return errors.New("not all party are ready")
	}, bf)

	wg.Wait()
	onlinePeers, _ := peerGroup.getPeersStatus()
	pc.sendRequestToAll(msg, onlinePeers)

	//we always set ourselves as online
	onlinePeers = append(onlinePeers, pc.host.ID())
	if len(onlinePeers) == len(peers) {

		return onlinePeers, nil
	}
	return onlinePeers, err
}
