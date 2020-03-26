package p2p

import (
	"context"
	"errors"
	"fmt"
	"io/ioutil"
	"sync"
	"time"

	"github.com/cenkalti/backoff/v4"
	"github.com/gogo/protobuf/proto"
	"github.com/libp2p/go-libp2p-core/host"
	"github.com/libp2p/go-libp2p-core/network"
	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/libp2p/go-yamux"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	"gitlab.com/thorchain/tss/go-tss/messages"
)

type PeerStatus struct {
	peersResponse  map[peer.ID]bool
	peerStatusLock *sync.RWMutex
}

func NewPeerStatus(peers []peer.ID, mypeerID peer.ID) PeerStatus {
	dat := make(map[peer.ID]bool)
	for _, el := range peers {
		if el == mypeerID {
			dat[el] = true
			continue
		}
		dat[el] = false
	}
	peerStatus := PeerStatus{
		peersResponse:  dat,
		peerStatusLock: &sync.RWMutex{},
	}
	return peerStatus
}

func (ps *PeerStatus) getPeersStatus() ([]peer.ID, []peer.ID) {
	var online []peer.ID
	var offline []peer.ID
	ps.peerStatusLock.RLock()
	defer ps.peerStatusLock.RUnlock()
	for peer, val := range ps.peersResponse {
		if val {
			online = append(online, peer)
		} else {
			offline = append(offline, peer)
		}
	}

	return online, offline
}

func (ps *PeerStatus) updatePeer(peer peer.ID) (bool, error) {
	ps.peerStatusLock.Lock()
	defer ps.peerStatusLock.Unlock()
	val, ok := ps.peersResponse[peer]
	if !ok {
		return false, errors.New("key not found")
	}
	if !val {
		ps.peersResponse[peer] = true
		fmt.Printf("###########%v\n", ps.peersResponse)
		return true, nil
	}
	fmt.Printf("###########%v\n", ps.peersResponse)
	return false, nil
}

type PartyCoordinator struct {
	logger             zerolog.Logger
	host               host.Host
	ceremonyLock       *sync.Mutex
	ceremonies         map[string]*Ceremony
	stopChan           chan struct{}
	timeout            time.Duration
	peersGroup         map[string]PeerStatus
	joinPartyGroupLock *sync.Mutex
	threshold          int32
	newFound           bool
}

// NewPartyCoordinator create a new instance of PartyCoordinator
func NewPartyCoordinator(host host.Host, timeout time.Duration) *PartyCoordinator {
	pc := &PartyCoordinator{
		logger:             log.With().Str("module", "party_coordinator").Logger(),
		host:               host,
		ceremonyLock:       &sync.Mutex{},
		ceremonies:         make(map[string]*Ceremony),
		stopChan:           make(chan struct{}),
		timeout:            timeout,
		peersGroup:         make(map[string]PeerStatus),
		joinPartyGroupLock: &sync.Mutex{},
		threshold:          0,
		newFound:           false,
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
	//payload, err := ReadStreamWithBuffer(stream)
	payload, err := ioutil.ReadAll(stream)
	if err != nil {
		logger.Err(err).Msgf("fail to read payload from stream")
		return
	}
	var msg messages.JoinPartyRequest
	if err := proto.Unmarshal(payload, &msg); err != nil {
		logger.Err(err).Msg("fail to unmarshal join party request")
		return
	}

	resp := &messages.JoinPartyResponse{
		ID:   msg.ID,
		Type: messages.JoinPartyResponse_LeaderNotReady,
	}

	pc.joinPartyGroupLock.Lock()
	peerGroup, ok := pc.peersGroup[msg.ID]
	if !ok {
		pc.logger.Info().Msg("this party is not ready")
		if err := pc.writeResponse(stream, resp); err != nil {
			logger.Error().Err(err).Msg("fail to write response to stream")
		}
		return
	}
	pc.joinPartyGroupLock.Unlock()
	newFound, err := peerGroup.updatePeer(remotePeer)
	if err != nil {
		pc.logger.Error().Err(err).Msg("receive msg from unknown peer")
		if err := pc.writeResponse(stream, resp); err != nil {
			logger.Error().Err(err).Msg("fail to write response to stream")
		}
		return
	}
	pc.joinPartyGroupLock.Lock()
	pc.newFound = newFound
	pc.joinPartyGroupLock.Unlock()

	resp.Type = messages.JoinPartyResponse_Success
	if err := pc.writeResponse(stream, resp); err != nil {
		logger.Error().Err(err).Msg("fail to write response to stream")
	}
	return

}

func (pc *PartyCoordinator) processJoinPartyRequest(remotePeer peer.ID, msg *messages.JoinPartyRequest) (*messages.JoinPartyResponse, error) {
	joinParty := NewJoinParty(msg, remotePeer)
	c, err := pc.onJoinParty(joinParty)
	if err != nil {
		if errors.Is(err, errLeaderNotReady) {
			// leader node doesn't have request yet, so don't know how to handle the join party request
			return &messages.JoinPartyResponse{
				ID:   msg.ID,
				Type: messages.JoinPartyResponse_LeaderNotReady,
			}, nil
		}
		if errors.Is(err, errUnknownPeer) {
			return &messages.JoinPartyResponse{
				ID:   msg.ID,
				Type: messages.JoinPartyResponse_UnknownPeer,
			}, nil
		}
		pc.logger.Error().Err(err).Msg("fail to join party")
	}
	if c == nil {
		// it only happen when the node was request to exit gracefully
		return &messages.JoinPartyResponse{
			ID:   msg.ID,
			Type: messages.JoinPartyResponse_Unknown,
		}, nil
	}

	select {
	case r := <-joinParty.Resp:
		return r, nil
	case onlinePeers := <-c.TimeoutChan:
		// make sure the ceremony get removed when there is timeout
		defer pc.ensureRemoveCeremony(msg.ID)
		return &messages.JoinPartyResponse{
			ID:      msg.ID,
			Type:    messages.JoinPartyResponse_Timeout,
			PeerIDs: onlinePeers,
		}, nil
	}
}

// writeResponse write the joinPartyResponse
func (pc *PartyCoordinator) writeResponse(stream network.Stream, resp *messages.JoinPartyResponse) error {
	buf, err := proto.Marshal(resp)
	if err != nil {
		return fmt.Errorf("fail to marshal resp to byte: %w", err)
	}
	_, err = stream.Write(buf)
	if err != nil {
		// when fail to write to the stream we shall reset it
		if resetErr := stream.Reset(); resetErr != nil {
			return fmt.Errorf("fail to reset the stream: %w", err)
		}
		return fmt.Errorf("fail to write response to stream: %w", err)
	}
	return nil
}

var (
	errLeaderNotReady = errors.New("leader node is not ready")
	errUnknownPeer    = errors.New("unknown peer trying to join party")
)

// onJoinParty is a call back function
func (pc *PartyCoordinator) onJoinParty(joinParty *JoinParty) (*Ceremony, error) {
	pc.logger.Info().
		Str("ID", joinParty.Msg.ID).
		Str("remote peer", joinParty.Peer.String()).
		Msgf("get join party request")
	pc.ceremonyLock.Lock()
	defer pc.ceremonyLock.Unlock()
	c, ok := pc.ceremonies[joinParty.Msg.ID]
	if !ok {
		return nil, errLeaderNotReady
	}
	if !c.ValidPeer(joinParty.Peer) {
		return nil, errUnknownPeer
	}
	if c.IsPartyExist(joinParty.Peer) {
		return nil, errUnknownPeer
	}
	c.JoinPartyRequests = append(c.JoinPartyRequests, joinParty)
	if !c.IsReady() {
		// Ceremony is not ready , still waiting for more party to join
		return c, nil
	}
	resp := &messages.JoinPartyResponse{
		ID:      c.ID,
		Type:    messages.JoinPartyResponse_Success,
		PeerIDs: c.GetParties(),
	}
	pc.logger.Info().Msgf("party formed: %+v", resp.PeerIDs)
	for _, item := range c.JoinPartyRequests {
		select {
		case <-pc.stopChan: // receive request to exit
			return nil, nil
		case item.Resp <- resp:
		}
	}
	delete(pc.ceremonies, c.ID)
	return c, nil
}

func (pc *PartyCoordinator) ensureRemoveCeremony(messageID string) {
	pc.ceremonyLock.Lock()
	defer pc.ceremonyLock.Unlock()
	delete(pc.ceremonies, messageID)
}

func (pc *PartyCoordinator) removePeerGroup(messageID string) {
	pc.joinPartyGroupLock.Lock()
	defer pc.joinPartyGroupLock.Unlock()
	delete(pc.peersGroup, messageID)
}

func (pc *PartyCoordinator) createJoinPartyGroups(messageID string, peers []string, threshold int32) {

	pc.joinPartyGroupLock.Lock()
	defer pc.joinPartyGroupLock.Unlock()
	pIDs, err := pc.getPeerIDs(peers)
	if err != nil {
		pc.logger.Error().Err(err).Msg("fail to parse peer id")
	}
	pc.threshold = threshold
	peerStatus := NewPeerStatus(pIDs, pc.host.ID())
	pc.peersGroup[messageID] = peerStatus
}

func (pc *PartyCoordinator) createCeremony(messageID string, peers []string, threshold int32) {
	pc.ceremonyLock.Lock()
	defer pc.ceremonyLock.Unlock()
	pIDs, err := pc.getPeerIDs(peers)
	if err != nil {
		pc.logger.Error().Err(err).Msg("fail to parse peer id")
	}
	pc.ceremonies[messageID] = NewCeremony(messageID, uint32(threshold), pIDs, pc.timeout)
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

func (pc *PartyCoordinator) joinParty2(msg *messages.JoinPartyRequest) bool {
	pc.joinPartyGroupLock.Lock()
	defer pc.joinPartyGroupLock.Unlock()
	peerGroup := pc.peersGroup[msg.ID]
	_, offline := peerGroup.getPeersStatus()
	var wg sync.WaitGroup
	wg.Add(len(offline))
	for _, el := range offline {
		go func(peer peer.ID) {
			defer wg.Done()
			ret, err := pc.getConfirmFromPeer(msg, peer)
			if err != nil {
				pc.logger.Error().Err(err).Msg("fail to get join party confirmation from this peer")
				return
			}
			if ret {
				fmt.Println("33323233232332")
				newFound, err := peerGroup.updatePeer(el)
				if err != nil {
					pc.logger.Error().Err(err).Msg("cannot fine the peer")
					return
				}
				pc.newFound = newFound
			}
		}(el)
	}
	wg.Wait()
	fmt.Println("wwwwwwwwwwwwwwwWWWWWWWWWWWWWWWWWWWWWW")
	if len(offline) == 0 {
		return true
	}
	return false
}

func (pc *PartyCoordinator) getConfirmFromPeer(msg *messages.JoinPartyRequest, remotePeer peer.ID) (bool, error) {

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
	pc.logger.Info().Msgf("open stream to (%s) successfully", remotePeer)
	defer func() {
		if err := stream.Close(); err != nil {
			pc.logger.Error().Err(err).Msg("fail to close stream")
		}
	}()
	_, err = stream.Write(msgBuf)
	if err != nil {
		if errReset := stream.Reset(); errReset != nil {
			return false, errReset
		}
		return false, fmt.Errorf("fail to write message to stream:%w", err)
	}
	// because the test stream doesn't support deadline
	if ApplyDeadline {
		//set a read deadline here , in case the coordinator doesn't timeout appropriately , and keep client hanging there
		//timeout := pc.timeout + time.Second
		if err := stream.SetReadDeadline(time.Now().Add(time.Second)); err != nil {
			return false, fmt.Errorf("fail to set read deadline")
		}
	}
	// read response
	//respBuf, err := ReadStreamWithBuffer(stream)
	respBuf, err := ioutil.ReadAll(stream)
	if err != nil {
		if err != yamux.ErrConnectionReset {
			return false, fmt.Errorf("fail to read response: %w", err)
		}
	}
	if len(respBuf) == 0 {
		return false, errors.New("fail to get response")
	}
	var resp messages.JoinPartyResponse
	if err := proto.Unmarshal(respBuf, &resp); err != nil {
		return false, fmt.Errorf("fail to unmarshal JoinGameResp: %w", err)
	}
	if resp.Type == messages.JoinPartyResponse_Success {
		return true, nil
	}
	pc.logger.Error().Msgf("fail to get response from this peer, response type is %v", resp.Type.String())
	return false, errors.New("p2p fail unknown")
}

// JoinParty join a ceremony , it could be keygen or key sign
func (pc *PartyCoordinator) JoinParty(remotePeer peer.ID, msg *messages.JoinPartyRequest, peers []string, threshold int32) (*messages.JoinPartyResponse, error) {
	if remotePeer == pc.host.ID() {
		pc.logger.Info().
			Str("message-id", msg.ID).
			Str("peerid", remotePeer.String()).
			Int32("threshold", threshold).
			Msg("we are the leader, create ceremony")
		pc.createCeremony(msg.ID, peers, threshold)
		return pc.processJoinPartyRequest(remotePeer, msg)
	}
	msgBuf, err := proto.Marshal(msg)
	if err != nil {
		return nil, fmt.Errorf("fail to marshal msg to bytes: %w", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*30)
	defer cancel()
	stream, err := pc.host.NewStream(ctx, remotePeer, joinPartyProtocol)
	if err != nil {
		return nil, fmt.Errorf("fail to create stream to peer(%s):%w", remotePeer, err)
	}
	pc.logger.Info().Msgf("open stream to (%s) successfully", remotePeer)
	defer func() {
		if err := stream.Close(); err != nil {
			pc.logger.Error().Err(err).Msg("fail to close stream")
		}
	}()
	err = WriteStreamWithBuffer(msgBuf, stream)
	if err != nil {
		if errReset := stream.Reset(); errReset != nil {
			return nil, errReset
		}
		return nil, fmt.Errorf("fail to write message to stream:%w", err)
	}
	// because the test stream doesn't support deadline
	if ApplyDeadline {
		// set a read deadline here , in case the coordinator doesn't timeout appropriately , and keep client hanging there
		timeout := pc.timeout + time.Second
		if err := stream.SetReadDeadline(time.Now().Add(timeout)); err != nil {
			return nil, fmt.Errorf("fail to set read deadline")
		}
	}
	// read response
	respBuf, err := ioutil.ReadAll(stream)
	if err != nil {
		if err != yamux.ErrConnectionReset {
			return nil, fmt.Errorf("fail to read response: %w", err)
		}
	}
	if len(respBuf) == 0 {
		return nil, errors.New("fail to get response")
	}
	var resp messages.JoinPartyResponse
	if err := proto.Unmarshal(respBuf, &resp); err != nil {
		return nil, fmt.Errorf("fail to unmarshal JoinGameResp: %w", err)
	}
	return &resp, nil
}

// JoinPartyWithRetry this method provide the functionality to join party with retry and backoff
func (pc *PartyCoordinator) JoinPartyWithRetry(msg *messages.JoinPartyRequest, peers []string, threshold int32) (*messages.JoinPartyResponse, error) {

	pc.createJoinPartyGroups(msg.ID, peers, threshold)
	defer pc.removePeerGroup(msg.ID)
	bf := backoff.NewExponentialBackOff()
	bf.MaxElapsedTime = time.Second * 5
	resp := &messages.JoinPartyResponse{
		Type: messages.JoinPartyResponse_Unknown,
	}
	err := backoff.Retry(func() error {
		ret := pc.joinParty2(msg)
		if !ret {
			pc.logger.Info().Msg("some peer are not ready")
			pc.joinPartyGroupLock.Lock()
			newFound := pc.newFound
			if newFound {
				fmt.Println("new we reset!!!!!!!!!!!!!!!!!1")
				bf.Reset()
			}
			pc.newFound = false
			pc.joinPartyGroupLock.Unlock()
			return errors.New("peer not ready")
		}
		return nil
	}, bf)
	if err == nil {
		fmt.Println("done!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!111111111111111111111111")
		resp.Type = messages.JoinPartyResponse_Success
	}
	return resp, err
}
