package p2p

import (
	"testing"

	pt "github.com/libp2p/go-libp2p-core/test"
	"github.com/stretchr/testify/assert"
)

func TestMailBoxImp(t *testing.T) {
	peerID, err := pt.RandPeerID()
	assert.Nil(t, err)
	mi, err := NewMailBoxImp()
	assert.Nil(t, err)
	assert.NotNil(t, mi)
	wm := GetRandomWireMessage(true)
	mi.AddMessage(wm.MessageID, wm, peerID)
	messages := mi.GetMessages(wm.MessageID)
	assert.NotNil(t, messages)
	assert.Len(t, messages, 1)
	assert.Equal(t, messages[0].RemotePeer, peerID)
	assert.Equal(t, messages[0].Message.MessageID, wm.MessageID)

	peerID1, err := pt.RandPeerID()
	assert.Nil(t, err)
	wm1 := GetRandomWireMessage(true)
	wm1.MessageID = wm.MessageID
	mi.AddMessage(wm.MessageID, wm1, peerID1)
	messages = mi.GetMessages(wm.MessageID)
	assert.NotNil(t, messages)
	assert.Len(t, messages, 2)
	assert.Equal(t, messages[1].RemotePeer, peerID1)
	assert.Equal(t, messages[1].Message.RoundInfo, wm1.RoundInfo)
	result := mi.GetMessages("whatever")
	assert.Nil(t, result)
	mi.RemoveMessage("whatever")
	mi.RemoveMessage(wm.MessageID)
}
func TestNewMailBoxImpNotOverflow(t *testing.T) {
	mi, err := NewMailBoxImp()
	assert.Nil(t, err)
	assert.NotNil(t, mi)
	for i := 0; i < 250; i++ {
		peerID, err := pt.RandPeerID()
		assert.Nil(t, err)
		wm := GetRandomWireMessage(true)
		mi.AddMessage(wm.MessageID, wm, peerID)
	}
	assert.Equal(t, mi.cache.Len(), 128)
}
