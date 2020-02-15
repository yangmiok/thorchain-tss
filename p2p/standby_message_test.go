package p2p

import (
	"testing"

	pt "github.com/libp2p/go-libp2p-core/test"
	"github.com/stretchr/testify/assert"
)

func TestStandbyMessage(t *testing.T) {
	wireMsg := GetRandomWireMessage(true)
	hash := RandStringBytesMask(10)
	standbyMsg := NewStandbyMessage(wireMsg, hash, 2)
	assert.NotNil(t, standbyMsg)
	for i := 0; i < 3; i++ {
		peerID := pt.RandPeerIDFatal(t)
		standbyMsg.UpdateConfirmList(peerID, hash)
	}
	assert.Equal(t, standbyMsg.TotalConfirmed(), 3)
}

func TestStandbyMessage_Validated(t *testing.T) {
	wireMsg := GetRandomWireMessage(true)
	hash := RandStringBytesMask(10)
	standbyMsg := NewStandbyMessage(wireMsg, hash, 10)
	for i := 0; i < 5; i++ {
		peerID := pt.RandPeerIDFatal(t)
		standbyMsg.UpdateConfirmList(peerID, hash)
	}
	assert.Equal(t, false, standbyMsg.Validated())
	for i := 0; i < 4; i++ {
		peerID := pt.RandPeerIDFatal(t)
		standbyMsg.UpdateConfirmList(peerID, hash)
	}
	assert.Equal(t, false, standbyMsg.Validated())
	peerID := pt.RandPeerIDFatal(t)
	standbyMsg.UpdateConfirmList(peerID, "rubbish")
	assert.Equal(t, false, standbyMsg.Validated())

	// happy path
	wireMsg1 := GetRandomWireMessage(true)
	hash1 := RandStringBytesMask(10)
	standbyMsg1 := NewStandbyMessage(wireMsg1, hash1, 10)
	for i := 0; i < 10; i++ {
		peerID := pt.RandPeerIDFatal(t)
		standbyMsg1.UpdateConfirmList(peerID, hash)
	}
	assert.Equal(t, 10, standbyMsg1.TotalConfirmed())
	assert.Equal(t, true, standbyMsg1.Validated())
}
