package p2p

import (
	"context"
	"testing"
	"time"

	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/libp2p/go-libp2p-core/protocol"
	tnet "github.com/libp2p/go-libp2p-testing/net"
	mocknet "github.com/libp2p/go-libp2p/p2p/net/mock"
	"github.com/stretchr/testify/assert"
)

const messageConfirmProtocolID protocol.ID = "/p2p/test/confirm/1.0.0"

type messageConfirmCallbackType struct {
	receivedMessages map[string]*WireMessage
}

func (mc *messageConfirmCallbackType) onReceived(msg *WireMessage) {
	mc.receivedMessages[msg.MessageID] = msg
}

func TestMessageValidator(t *testing.T) {
	applyDeadline = false
	id1 := tnet.RandIdentityOrFatal(t)
	id2 := tnet.RandIdentityOrFatal(t)
	id3 := tnet.RandIdentityOrFatal(t)
	id4 := tnet.RandIdentityOrFatal(t)
	mn := mocknet.New(context.Background())
	// add peers to mock net

	a1 := tnet.RandLocalTCPAddress()
	a2 := tnet.RandLocalTCPAddress()
	a3 := tnet.RandLocalTCPAddress()
	a4 := tnet.RandLocalTCPAddress()

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
	defer closeHost(t, h3)
	p3 := h3.ID()
	h4, err := mn.AddPeer(id4.PrivateKey(), a4)
	if err != nil {
		t.Fatal(err)
	}
	defer closeHost(t, h4)

	p4 := h4.ID()

	if err := mn.LinkAll(); err != nil {
		t.Error(err)
	}
	if err := mn.ConnectAllButSelf(); err != nil {
		t.Error(err)
	}
	mc1 := &messageConfirmCallbackType{
		receivedMessages: make(map[string]*WireMessage),
	}
	mc2 := &messageConfirmCallbackType{
		receivedMessages: make(map[string]*WireMessage),
	}
	mc3 := &messageConfirmCallbackType{
		receivedMessages: make(map[string]*WireMessage),
	}
	mc4 := &messageConfirmCallbackType{
		receivedMessages: make(map[string]*WireMessage),
	}
	wireMsg := GetRandomWireMessage(true)
	mv1, err := NewMessageValidator(h1, mc1.onReceived, messageConfirmProtocolID)
	assert.Nil(t, err)
	assert.NotNil(t, mv1)

	mv2, err := NewMessageValidator(h2, mc2.onReceived, messageConfirmProtocolID)
	assert.Nil(t, err)
	assert.NotNil(t, mv2)

	mv3, err := NewMessageValidator(h3, mc3.onReceived, messageConfirmProtocolID)
	assert.Nil(t, err)
	assert.NotNil(t, mv3)
	mv4, err := NewMessageValidator(h4, mc4.onReceived, messageConfirmProtocolID)
	assert.Nil(t, err)
	assert.NotNil(t, mv4)

	mv1.Start()
	defer mv1.Stop()
	mv2.Start()
	defer mv2.Stop()
	mv3.Start()
	defer mv3.Stop()
	mv4.Start()
	defer mv4.Stop()

	assert.Nil(t, mv1.VerifyMessage(wireMsg, []peer.ID{
		p3, p4,
	}))
	assert.Nil(t, mv3.VerifyMessage(wireMsg, []peer.ID{
		p1, p4,
	}))
	assert.Nil(t, mv4.VerifyMessage(wireMsg, []peer.ID{
		p1, p3,
	}))
	// give a little bit time for the messaging to finish
	time.Sleep(time.Second)
	assert.NotNil(t, mc1.receivedMessages[wireMsg.MessageID])
	assert.NotNil(t, mc3.receivedMessages[wireMsg.MessageID])
	assert.NotNil(t, mc4.receivedMessages[wireMsg.MessageID])

	wireMsg1 := GetRandomWireMessage(true)
	mv1.Park(wireMsg1, p2)

	assert.Nil(t, mv3.VerifyMessage(wireMsg1, []peer.ID{
		p1, p4,
	}))
	assert.Nil(t, mv4.VerifyMessage(wireMsg1, []peer.ID{
		p1, p3,
	}))
	assert.Nil(t, mv1.VerifyParkedMessages(wireMsg1.MessageID, []peer.ID{
		p3, p4,
	}))
	time.Sleep(time.Second)
	assert.NotNil(t, mc1.receivedMessages[wireMsg1.MessageID])
	assert.NotNil(t, mc3.receivedMessages[wireMsg1.MessageID])
	assert.NotNil(t, mc4.receivedMessages[wireMsg1.MessageID])
}
