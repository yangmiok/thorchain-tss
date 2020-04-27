package common

import (
	"github.com/tendermint/tendermint/crypto/secp256k1"
	. "gopkg.in/check.v1"
)

type tssHelpSuite struct {
	tssCommon *TssCommon
}

var _ = Suite(&tssHelpSuite{})

func (t *tssHelpSuite) TestGetHashToBroadcast(c *C) {
	testMap := make(map[string]string)
	val, freq, err := getHighestFreq(testMap)
	c.Assert(err, NotNil)
	val, freq, err = getHighestFreq(nil)
	c.Assert(err, NotNil)
	testMap["1"] = "aa"
	testMap["2"] = "aa"
	testMap["3"] = "aa"
	testMap["4"] = "ab"
	testMap["5"] = "bb"
	testMap["6"] = "bb"
	testMap["7"] = "bc"
	testMap["8"] = "cd"
	val, freq, err = getHighestFreq(testMap)
	c.Assert(err, IsNil)
	c.Assert(val, Equals, "aa")
	c.Assert(freq, Equals, 3)
}

func (t *tssHelpSuite) TestMsgSignAndVerification(c *C) {
	msg := []byte("hello")
	msgID := "123"
	sk := secp256k1.GenPrivKey()
	sig, err := generateSignature(msg, msgID, sk)
	c.Assert(err, IsNil)
	ret := verifySignature(sk.PubKey(), msg, sig, msgID)
	c.Assert(ret, Equals, true)
}
