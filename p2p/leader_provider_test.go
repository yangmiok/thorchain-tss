package p2p

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestLeaderNode(t *testing.T) {
	buf := []byte("helloworld")
	result, err := LeaderNode(buf, 10)
	assert.Nil(t, err)
	assert.Equal(t, int32(3), result)
	// do it again , it should return the same result
	result, err = LeaderNode(buf, 10)
	assert.Nil(t, err)
	assert.Equal(t, int32(3), result)
}
