package keysign

import (
	"encoding/json"
	bc "github.com/binance-chain/tss-lib/common"
	. "gopkg.in/check.v1"
	"io/ioutil"

	"gitlab.com/thorchain/tss/go-tss"
	"gitlab.com/thorchain/tss/go-tss/common"
)

type NotifierTestSuite struct{}

var _ = Suite(&NotifierTestSuite{})

func (*NotifierTestSuite) SetUpSuite(c *C) {
	common.SetupBech32Prefix()
}

func (NotifierTestSuite) TestNewNotifier(c *C) {
	testMsg := [][]byte{[]byte("hello1"), []byte("hello2")}
	poolPubKey := go_tss.GetRandomPubKey()
	n, err := NewNotifier("", testMsg, poolPubKey)
	c.Assert(err, NotNil)
	c.Assert(n, IsNil)
	n, err = NewNotifier("aasfdasdf", nil, poolPubKey)
	c.Assert(err, NotNil)
	c.Assert(n, IsNil)

	n, err = NewNotifier("hello", testMsg, "")
	c.Assert(err, NotNil)
	c.Assert(n, IsNil)

	n, err = NewNotifier("hello", testMsg, poolPubKey)
	c.Assert(err, IsNil)
	c.Assert(n, NotNil)
	ch := n.GetResponseChannel()
	c.Assert(ch, NotNil)
}

func (NotifierTestSuite) TestNotifierHappyPath(c *C) {
	messageToSign1 := "helloworld-test"
	messageID, err := common.MsgToHashString([]byte(messageToSign1))
	c.Assert(err, IsNil)
	poolPubKey := `thorpub1addwnpepqv6xp3fmm47dfuzglywqvpv8fdjv55zxte4a26tslcezns5czv586u2fw33`
	n, err := NewNotifier(messageID, [][]byte{[]byte(messageToSign1)}, poolPubKey)
	n2, err := NewNotifier(messageID, [][]byte{[]byte(messageToSign1), []byte(messageToSign1)}, poolPubKey)
	c.Assert(err, IsNil)
	c.Assert(n, NotNil)
	sigFile := "../test_data/signature_notify/sig1.json"
	content, err := ioutil.ReadFile(sigFile)
	c.Assert(err, IsNil)
	c.Assert(content, NotNil)
	var signature bc.SignatureData
	err = json.Unmarshal(content, &signature)
	c.Assert(err, IsNil)
	signature.M = []byte(messageToSign1)
	c.Assert(err, IsNil)
	sigInvalidFile := `../test_data/signature_notify/sig_invalid.json`
	contentInvalid, err := ioutil.ReadFile(sigInvalidFile)
	c.Assert(err, IsNil)
	c.Assert(contentInvalid, NotNil)
	var sigInvalid bc.SignatureData
	c.Assert(json.Unmarshal(contentInvalid, &sigInvalid), IsNil)
	// we check if the message num is not equal to the signature we received
	finish, err := n2.ProcessSignature([]*bc.SignatureData{&sigInvalid})
	c.Assert(err, ErrorMatches, "fail to verify signature: message num and signature num does not match")
	c.Assert(finish, Equals, false)

	// valid keysign peer , but invalid signature we should continue to listen
	finish, err = n.ProcessSignature([]*bc.SignatureData{&sigInvalid})
	c.Assert(err, IsNil)
	c.Assert(finish, Equals, false)
	//valid signature from a keysign peer , we should accept it and bail out
	finish, err = n.ProcessSignature([]*bc.SignatureData{&signature})
	c.Assert(err, IsNil)
	c.Assert(finish, Equals, true)

	result := <-n.GetResponseChannel()
	c.Assert(result, NotNil)
	c.Assert([]*bc.SignatureData{&signature}, DeepEquals, result)
}
