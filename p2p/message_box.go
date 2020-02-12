package p2p

import (
	lru "github.com/hashicorp/golang-lru"
	"github.com/libp2p/go-libp2p-core/peer"
)

// MessageBox
type MessageBox interface {
	AddMessage(messageID string, msg *WireMessage, remotePeer peer.ID)
	RemoveMessage(messageID string)
	GetMessages(messageID string) []*CachedMessage
}

type MessageBoxImp struct {
	cache *lru.Cache
}

type CachedMessage struct {
	RemotePeer peer.ID
	Message    *WireMessage
}

// NewMessageBoxImp create a new MessageBoxImp
func NewMessageBoxImp() (*MessageBoxImp, error) {
	cache, err := lru.New(128)
	if err != nil {
		return nil, err
	}
	return &MessageBoxImp{cache: cache}, nil
}

// AddMessage will add a message into the mail box
func (m *MessageBoxImp) AddMessage(messageID string, msg *WireMessage, remotePeer peer.ID) {
	// in a real scenario , we might not get multiple messages, but it might happen
	c, ok := m.cache.Get(messageID)
	if !ok {
		m.cache.Add(messageID, []*CachedMessage{
			&CachedMessage{
				RemotePeer: remotePeer,
				Message:    msg,
			},
		})
		return
	}
	cm, ok := c.([]*CachedMessage)
	if ok {
		cm = append(cm, &CachedMessage{
			RemotePeer: remotePeer,
			Message:    msg,
		})
		m.cache.Add(messageID, cm)
	}
}

// RemoveMessage remove the given message from mailbox
func (m *MessageBoxImp) RemoveMessage(messageID string) {
	m.cache.Remove(messageID)
}

// GetMessages return a slice of cached messages
func (m *MessageBoxImp) GetMessages(messageID string) []*CachedMessage {
	if !m.cache.Contains(messageID) {
		return nil
	}
	c, ok := m.cache.Get(messageID)
	if !ok {
		return nil
	}
	cm, ok := c.([]*CachedMessage)
	if !ok {
		return nil
	}
	return cm
}
