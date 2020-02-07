package p2p

import (
	"sync"

	"github.com/gogo/protobuf/proto"
	"github.com/libp2p/go-libp2p-core/network"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	"gitlab.com/thorchain/tss/go-tss/messages"
)

type JoinPartyCallback func(msg *messages.JoinPartyRequest) error

type PartyCoordinator struct {
	logger       zerolog.Logger
	ceremonyLock *sync.Mutex
	ceremonies   map[string]*Ceremony
}

// NewPartyCoordinator create a new instance of PartyCoordinator
func NewPartyCoordinator() *PartyCoordinator {
	return &PartyCoordinator{
		logger:       log.With().Str("module", "party_coordinator").Logger(),
		ceremonyLock: &sync.Mutex{},
		ceremonies:   make(map[string]*Ceremony),
	}
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
	length, err := ReadLength(stream)
	if err != nil {
		logger.Err(err).Msg("fail to read length header from stream")
		return
	}
	payload, err := ReadPayload(stream, length)
	if err != nil {
		logger.Err(err).Msgf("fail to read payload from stream")
		return
	}
	var msg messages.JoinPartyRequest
	if err := proto.Unmarshal(payload, &msg); err != nil {
		logger.Err(err).Msg("fail to unmarshal join party request")
		return
	}

	if err := pc.onJoinParty(msg); err != nil {

	}
}

func (pc *PartyCoordinator) onJoinParty(msg *messages.JoinPartyRequest) error {
	return nil
}
