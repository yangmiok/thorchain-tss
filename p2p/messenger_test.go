package p2p

import (
	"context"
	"testing"
	"time"

	"github.com/libp2p/go-libp2p-core/host"
	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/libp2p/go-libp2p-core/protocol"
	tnet "github.com/libp2p/go-libp2p-testing/net"
	mocknet "github.com/libp2p/go-libp2p/p2p/net/mock"
	"github.com/stretchr/testify/assert"
)

var messengerTestProtocolID protocol.ID = "/p2p/messenger/test"

type messengerCallbackType struct {
	receivedMessages map[peer.ID][]string
}

func (mc *messengerCallbackType) onReceived(msg []byte, remotePeer peer.ID) {
	messages, ok := mc.receivedMessages[remotePeer]
	if !ok {
		mc.receivedMessages[remotePeer] = []string{
			string(msg),
		}
	}
	mc.receivedMessages[remotePeer] = append(messages, string(msg))
}

func closeHost(t *testing.T, host host.Host) {
	assert.Nil(t, host.Close())
}
func TestMessenger(t *testing.T) {
	applyDeadline = false
	id1 := tnet.RandIdentityOrFatal(t)
	id2 := tnet.RandIdentityOrFatal(t)
	id3 := tnet.RandIdentityOrFatal(t)
	mn := mocknet.New(context.Background())
	// add peers to mock net

	a1 := tnet.RandLocalTCPAddress()
	a2 := tnet.RandLocalTCPAddress()
	a3 := tnet.RandLocalTCPAddress()

	h1, err := mn.AddPeer(id1.PrivateKey(), a1)
	if err != nil {
		t.Fatal(err)
	}
	defer closeHost(t, h1)
	p1 := h1.ID()
	h2, err := mn.AddPeer(id2.PrivateKey(), a2)
	if err != nil {
		t.Fatal(err)
	}
	defer closeHost(t, h2)
	p2 := h2.ID()
	h3, err := mn.AddPeer(id3.PrivateKey(), a3)
	if err != nil {
		t.Fatal(err)
	}
	p3 := h3.ID()
	if err := mn.LinkAll(); err != nil {
		t.Error(err)
	}
	defer closeHost(t, h3)
	if err := mn.ConnectAllButSelf(); err != nil {
		t.Error(err)
	}

	callBack1 := &messengerCallbackType{
		receivedMessages: make(map[peer.ID][]string),
	}
	callBack2 := &messengerCallbackType{
		receivedMessages: make(map[peer.ID][]string),
	}
	callBack3 := &messengerCallbackType{
		receivedMessages: make(map[peer.ID][]string),
	}
	messenger1, err := NewMessenger(messengerTestProtocolID, h1, callBack1.onReceived)
	assert.Nil(t, err)
	assert.NotNil(t, messenger1)

	messenger2, err := NewMessenger(messengerTestProtocolID, h2, callBack2.onReceived)
	assert.Nil(t, err)
	assert.NotNil(t, messenger2)

	messenger3, err := NewMessenger(messengerTestProtocolID, h3, callBack3.onReceived)
	assert.Nil(t, err)
	assert.NotNil(t, messenger1)
	messenger1.Start()
	messenger2.Start()
	messenger3.Start()
	defer messenger1.Stop()
	defer messenger2.Stop()
	defer messenger3.Stop()
	messenger1.Send([]byte("helloworld"), []peer.ID{
		p2, p3,
	})
	messenger2.Send([]byte("what's up"), []peer.ID{
		p1, p3,
	})
	time.Sleep(time.Second)
	assert.NotNil(t, callBack2.receivedMessages[p1])
	assert.Equal(t, "helloworld", callBack2.receivedMessages[p1][0])
	assert.NotNil(t, callBack1.receivedMessages[p2])
	assert.Equal(t, "what's up", callBack1.receivedMessages[p2][0])
	assert.Len(t, callBack3.receivedMessages, 2)
	assert.NotNil(t, callBack3.receivedMessages[p1])
	assert.NotNil(t, callBack3.receivedMessages[p2])
	assert.Equal(t, "helloworld", callBack3.receivedMessages[p1][0])
	assert.Equal(t, "what's up", callBack3.receivedMessages[p2][0])

	m, err := NewMessenger(messengerTestProtocolID, nil, nil)
	assert.NotNil(t, err)
	assert.Nil(t, m)
	m, err = NewMessenger(messengerTestProtocolID, h1, nil)
	assert.NotNil(t, err)
	assert.Nil(t, m)

}
