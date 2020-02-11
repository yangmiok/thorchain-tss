package p2p

import (
	"sync"

	"github.com/libp2p/go-libp2p-core/host"
	"github.com/libp2p/go-libp2p-core/network"
	"github.com/libp2p/go-libp2p-core/protocol"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

var (
	// Keygen
	KeygenVerifyProtocol  protocol.ID = "/p2p/keygen-verify"
	KeysignVerifyProtocol protocol.ID = "/p2p/keysign-verify"
)

type MessageConfirmedHandler func(message *WireMessage)

// MessageValidator is a service design to make sure that all the parties receive the same messages
type MessageValidator struct {
	logger                     zerolog.Logger
	host                       host.Host
	lock                       *sync.Mutex
	cache                      map[string]*StandbyMessage
	onMessageConfirmedCallback MessageConfirmedHandler
}

// NewMessageValidator create a new message
func NewMessageValidator(host host.Host, confirmedCallback MessageConfirmedHandler, protocol protocol.ID) (*MessageValidator, error) {
	mv := &MessageValidator{
		logger:                     log.With().Str("module", "message_validator").Logger(),
		host:                       host,
		lock:                       &sync.Mutex{},
		cache:                      make(map[string]*StandbyMessage),
		onMessageConfirmedCallback: confirmedCallback,
	}
	host.SetStreamHandler(protocol, mv.onConfirmMessage)
	return mv, nil
}

func (mv *MessageValidator) handleStream(stream network.Stream) {
	defer func() {
		if err := stream.Close(); err != nil {
			mv.logger.Err(err).Msg("fail to close stream")
		}
	}()
	l, err := ReadLength(stream)
	if err != nil {
		mv.logger.Err(err).Msg("fail to read length")
		return
	}
	_ = l
}

func (mv *MessageValidator) onConfirmMessage() {

}
