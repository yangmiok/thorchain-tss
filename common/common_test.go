package common

import (
	"bytes"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"sort"
	"testing"

	btsskeygen "github.com/binance-chain/tss-lib/ecdsa/keygen"
	btss "github.com/binance-chain/tss-lib/tss"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	tcrypto "github.com/tendermint/tendermint/crypto"
	"github.com/tendermint/tendermint/crypto/secp256k1"
	. "gopkg.in/check.v1"

	"gitlab.com/thorchain/tss/go-tss/blame"
	"gitlab.com/thorchain/tss/go-tss/conversion"
	"gitlab.com/thorchain/tss/go-tss/messages"
)

var (
	testBlamePrivKey = "YmNiMzA2ODU1NWNjMzk3NDE1OWMwMTM3MDU0NTNjN2YwMzYzZmVhZDE5NmU3NzRhOTMwOWIxN2QyZTQ0MzdkNg=="
	testSenderPubKey = "thorpub1addwnpepqtspqyy6gk22u37ztra4hq3hdakc0w0k60sfy849mlml2vrpfr0wvm6uz09"
	testPubKeys      = [...]string{"thorpub1addwnpepqtdklw8tf3anjz7nn5fly3uvq2e67w2apn560s4smmrt9e3x52nt2svmmu3", "thorpub1addwnpepqtspqyy6gk22u37ztra4hq3hdakc0w0k60sfy849mlml2vrpfr0wvm6uz09", "thorpub1addwnpepq2ryyje5zr09lq7gqptjwnxqsy2vcdngvwd6z7yt5yjcnyj8c8cn559xe69", "thorpub1addwnpepqfjcw5l4ay5t00c32mmlky7qrppepxzdlkcwfs2fd5u73qrwna0vzag3y4j"}
	testBlamePubKeys = []string{"thorpub1addwnpepqtr5p8tllhp4xaxmu77zhqen24pmrdlnekzevshaqkyzdqljm6rejnnt02t", "thorpub1addwnpepqtspqyy6gk22u37ztra4hq3hdakc0w0k60sfy849mlml2vrpfr0wvm6uz09", "thorpub1addwnpepqga4nded5hhnwsrwmrns803w7vu9mffp9r6dz4l6smaww2l5useuq6vkttg", "thorpub1addwnpepq28hfdpu3rdgvj8skzhlm8hyt5nlwwc8pjrzvn253j86e4dujj6jsmuf25q", "thorpub1addwnpepqfuq0xc67052h288r6flp67l0ny9mg6u3sxhsrlukyfg0fe9j6q36ysd33y", "thorpub1addwnpepq0jszts80udfl4pkfk6cp93647yl6fhu6pk486uwjdz2sf94qvu0kw0t6ug", "thorpub1addwnpepqw6mmffk69n5taaqhq3wsc8mvdpsrdnx960kujeh4jwm9lj8nuyux9hz5e4", "thorpub1addwnpepq0pdhm2jatzg2vy6fyw89vs6q374zayqd5498wn8ww780grq256ygq7hhjt", "thorpub1addwnpepqggwmlgd8u9t2sx4a0styqwhzrvdhpvdww7sqwnweyrh25rjwwm9q65kx9s", "thorpub1addwnpepqtssltyjvms8pa7k4yg85lnrjqtvvr2ecr36rhm7pa4ztf55tnuzzgvegpk"}
)

type TestParties struct {
	honest    []int
	malicious []int
}

func TestPackage(t *testing.T) { TestingT(t) }

type TssTestSuite struct {
	privKey tcrypto.PrivKey
}

var _ = Suite(&TssTestSuite{})

func (t *TssTestSuite) SetUpSuite(c *C) {
	InitLog("info", true, "tss_common_test")
	conversion.SetupBech32Prefix()
	priHexBytes, err := base64.StdEncoding.DecodeString(testBlamePrivKey)
	c.Assert(err, IsNil)
	rawBytes, err := hex.DecodeString(string(priHexBytes))
	c.Assert(err, IsNil)
	var keyBytesArray [32]byte
	copy(keyBytesArray[:], rawBytes[:32])
	priKey := secp256k1.PrivKeySecp256k1(keyBytesArray)
	t.privKey = priKey
}

func initLog(level string, pretty bool) {
	l, err := zerolog.ParseLevel(level)
	if err != nil {
		log.Warn().Msgf("%s is not a valid log-level, falling back to 'info'", level)
	}
	var out io.Writer = os.Stdout
	if pretty {
		out = zerolog.ConsoleWriter{Out: os.Stdout}
	}
	zerolog.SetGlobalLevel(l)
	log.Logger = log.Output(out).With().Str("service", "go-tss-test").Logger()
}

func (t *TssTestSuite) TestGetThreshold(c *C) {
	_, err := GetThreshold(-2)
	c.Assert(err, NotNil)
	output, err := GetThreshold(4)
	c.Assert(err, IsNil)
	c.Assert(output, Equals, 2)
	output, err = GetThreshold(9)
	c.Assert(err, IsNil)
	c.Assert(output, Equals, 5)
	output, err = GetThreshold(10)
	c.Assert(err, IsNil)
	c.Assert(output, Equals, 6)
	output, err = GetThreshold(99)
	c.Assert(err, IsNil)
	c.Assert(output, Equals, 65)
}

func (t *TssTestSuite) TestMsgToHashInt(c *C) {
	input := []byte("whatever")
	result, err := MsgToHashInt(input)
	c.Assert(err, IsNil)
	c.Assert(result, NotNil)
}

func (t *TssTestSuite) TestContains(c *C) {
	t1 := btss.PartyID{
		Index: 1,
	}
	ret := Contains(nil, &t1)
	c.Assert(ret, Equals, false)

	t2 := btss.PartyID{
		Index: 2,
	}
	t3 := btss.PartyID{
		Index: 3,
	}
	testParties := []*btss.PartyID{&t2, &t3}
	ret = Contains(testParties, &t1)
	c.Assert(ret, Equals, false)
	testParties = append(testParties, &t1)
	ret = Contains(testParties, &t1)
	c.Assert(ret, Equals, true)
	ret = Contains(testParties, nil)
	c.Assert(ret, Equals, false)
}

func (t *TssTestSuite) TestTssProcessOutCh(c *C) {
	conf := TssConfig{}
	localTestPubKeys := make([]string, len(testPubKeys))
	copy(localTestPubKeys, testPubKeys[:])
	partiesID, localPartyID, err := conversion.GetParties(localTestPubKeys, testPubKeys[0])
	c.Assert(err, IsNil)
	messageRouting := btss.MessageRouting{
		From:                    localPartyID,
		To:                      partiesID[3:],
		IsBroadcast:             true,
		IsToOldCommittee:        false,
		IsToOldAndNewCommittees: false,
	}
	testFill := []byte("TEST")
	testContent := &btsskeygen.KGRound1Message{
		Commitment: testFill,
	}
	msg := btss.NewMessageWrapper(messageRouting, testContent)
	tssMsg := btss.NewMessage(messageRouting, testContent, msg)
	tssCommonStruct := NewTssCommon("", nil, conf, "test", t.privKey)
	err = tssCommonStruct.ProcessOutCh(tssMsg, messages.TSSKeyGenMsg)
	c.Assert(err, IsNil)
}

func fabricateTssMsg(c *C, privKey tcrypto.PrivKey, partyID *btss.PartyID, roundInfo, msg, msgID string) *messages.WrappedMessage {
	routingInfo := btss.MessageRouting{
		From:                    partyID,
		To:                      nil,
		IsBroadcast:             true,
		IsToOldCommittee:        false,
		IsToOldAndNewCommittees: false,
	}
	var dataForSign bytes.Buffer
	dataForSign.WriteString(msg)
	dataForSign.WriteString(msgID)
	sig, err := privKey.Sign(dataForSign.Bytes())
	c.Assert(err, IsNil)
	wiredMessage := messages.WireMessage{
		Routing:   &routingInfo,
		RoundInfo: roundInfo,
		Message:   []byte(msg),
		Sig:       sig,
	}

	marshaledMsg, err := json.Marshal(wiredMessage)
	c.Assert(err, IsNil)
	wrappedMsg := messages.WrappedMessage{
		MessageType: messages.TSSKeyGenMsg,
		Payload:     marshaledMsg,
	}
	return &wrappedMsg
}

func fabricateVerMsg(c *C, hash, hashKey string) *messages.WrappedMessage {
	broadcastConfirmMsg := &messages.BroadcastConfirmMessage{
		P2PID: "",
		Key:   hashKey,
		Hash:  hash,
	}
	marshaledMsg, err := json.Marshal(broadcastConfirmMsg)
	c.Assert(err, IsNil)
	wrappedMsg := messages.WrappedMessage{
		MessageType: messages.TSSKeyGenVerMsg,
		Payload:     marshaledMsg,
	}
	return &wrappedMsg
}

func senderIDtoPubKey(senderID *btss.PartyID) (string, error) {
	blamePartyKeyBytes := senderID.GetKey()
	var pk secp256k1.PubKeySecp256k1
	copy(pk[:], blamePartyKeyBytes)
	blamedPubKey, err := sdk.Bech32ifyAccPub(pk)
	return blamedPubKey, err
}

func (t *TssTestSuite) testVerMsgDuplication(c *C, privKey tcrypto.PrivKey, tssCommonStruct *TssCommon, senderID *btss.PartyID, partiesID []*btss.PartyID) {
	testMsg := "testVerMsgDuplication"
	roundInfo := "round testVerMsgDuplication"
	tssCommonStruct.msgID = "123"
	msgKey := fmt.Sprintf("%s-%s", senderID.Id, roundInfo)
	wrappedMsg := fabricateTssMsg(c, privKey, senderID, roundInfo, testMsg, tssCommonStruct.msgID)
	err := tssCommonStruct.ProcessOneMessage(wrappedMsg, tssCommonStruct.PartyIDtoP2PID[partiesID[1].Id].String())
	c.Assert(err, IsNil)
	localItem := tssCommonStruct.TryGetLocalCacheItem(msgKey)
	c.Assert(localItem.ConfirmedList, HasLen, 1)
	err = tssCommonStruct.ProcessOneMessage(wrappedMsg, tssCommonStruct.PartyIDtoP2PID[partiesID[1].Id].String())
	c.Assert(err, IsNil)
	c.Assert(localItem.ConfirmedList, HasLen, 1)
}

func setupProcessVerMsgEnv(c *C, privKey tcrypto.PrivKey, keyPool []string, partyNum int) (*TssCommon, []*btss.PartyID, []*btss.PartyID) {
	conf := TssConfig{}
	tssCommonStruct := NewTssCommon("", nil, conf, "test", privKey)
	localTestPubKeys := make([]string, partyNum)
	copy(localTestPubKeys, keyPool[:partyNum])
	// for the test, we choose the first pubic key as the test instance public key
	partiesID, localPartyID, err := conversion.GetParties(localTestPubKeys, keyPool[0])
	c.Assert(err, IsNil)
	partyIDMap := conversion.SetupPartyIDMap(partiesID)
	conversion.SetupIDMaps(partyIDMap, tssCommonStruct.PartyIDtoP2PID)
	ctx := btss.NewPeerContext(partiesID)
	params := btss.NewParameters(ctx, localPartyID, len(partiesID), 2)
	outCh := make(chan btss.Message, len(partiesID))
	endCh := make(chan btsskeygen.LocalPartySaveData, len(partiesID))
	keyGenParty := btsskeygen.NewLocalParty(params, outCh, endCh)
	tssCommonStruct.SetPartyInfo(&PartyInfo{
		Party:      keyGenParty,
		PartyIDMap: partyIDMap,
	})
	err = conversion.SetupIDMaps(partyIDMap, tssCommonStruct.blameMgr.PartyIDtoP2PID)
	c.Assert(err, IsNil)
	tssCommonStruct.SetLocalPeerID("fakeID")
	err = conversion.SetupIDMaps(partyIDMap, tssCommonStruct.PartyIDtoP2PID)
	c.Assert(err, IsNil)
	tssCommonStruct.blameMgr.SetPartyInfo(keyGenParty, partyIDMap)
	peerPartiesID := append(partiesID[:localPartyID.Index], partiesID[localPartyID.Index+1:]...)
	tssCommonStruct.P2PPeers = conversion.GetPeersID(tssCommonStruct.PartyIDtoP2PID, tssCommonStruct.GetLocalPeerID())
	return tssCommonStruct, peerPartiesID, partiesID
}

func (t *TssTestSuite) testDropMsgOwner(c *C, privKey tcrypto.PrivKey, tssCommonStruct *TssCommon, senderID *btss.PartyID, peerPartiesID []*btss.PartyID) {
	testMsg := "testDropMsgOwner"
	roundInfo := "round testDropMsgOwner"
	msgHash, err := BytesToHashString([]byte(testMsg))
	c.Assert(err, IsNil)
	msgKey := fmt.Sprintf("%s-%s", senderID.Id, roundInfo)
	senderMsg := fabricateTssMsg(c, privKey, senderID, roundInfo, testMsg, "123")

	senderPeer, err := conversion.GetPeerIDFromPartyID(senderID)
	c.Assert(err, IsNil)
	// you can pass any p2pID in Tss message
	err = tssCommonStruct.ProcessOneMessage(senderMsg, senderPeer.String())
	c.Assert(err, IsNil)
	localItem := tssCommonStruct.TryGetLocalCacheItem(msgKey)
	c.Assert(localItem.ConfirmedList, HasLen, 1)

	wrappedVerMsg := fabricateVerMsg(c, msgHash, msgKey)
	// -2 here as we exclude the last node (simulating that the data owner send the verMsg before the last node) and the
	// receiver itself.
	expected := len(peerPartiesID) - 1
	for i := 1; i < expected; i++ {
		err = tssCommonStruct.ProcessOneMessage(wrappedVerMsg, tssCommonStruct.PartyIDtoP2PID[peerPartiesID[i].Id].String())
		c.Assert(err, IsNil)
		c.Assert(localItem.ConfirmedList, HasLen, i+1)
	}
	// the data owner's message should be raise an error
	err = tssCommonStruct.ProcessOneMessage(senderMsg, senderPeer.String())
	c.Assert(err, Equals, blame.ErrHashFromOwner)
	c.Assert(tssCommonStruct.blameMgr.GetBlame().FailReason, Equals, blame.HashCheckFail)
	blamedPubKey, err := senderIDtoPubKey(senderID)
	c.Assert(err, IsNil)
	c.Assert(tssCommonStruct.blameMgr.GetBlame().BlameNodes, DeepEquals, []string{blamedPubKey})
}

func (t *TssTestSuite) testVerMsgAndUpdate(c *C, tssCommonStruct *TssCommon, senderID *btss.PartyID, partiesID []*btss.PartyID) {
	testMsg := "testVerMsgAndUpdate"
	roundInfo := "round testVerMsgAndUpdate"
	msgHash, err := BytesToHashString([]byte(testMsg))
	c.Assert(err, IsNil)
	msgKey := fmt.Sprintf("%s-%s", senderID.Id, roundInfo)
	wrappedMsg := fabricateTssMsg(c, t.privKey, senderID, roundInfo, testMsg, "123")
	// you can pass any p2pID in Tss message
	err = tssCommonStruct.ProcessOneMessage(wrappedMsg, tssCommonStruct.PartyIDtoP2PID[senderID.Id].String())
	c.Assert(err, IsNil)
	localItem := tssCommonStruct.TryGetLocalCacheItem(msgKey)
	c.Assert(localItem.ConfirmedList, HasLen, 1)

	// we send the verify message from the the same sender, Tss should only accept the first verify message
	wrappedVerMsg := fabricateVerMsg(c, msgHash, msgKey)
	err = tssCommonStruct.ProcessOneMessage(wrappedVerMsg, tssCommonStruct.PartyIDtoP2PID[partiesID[1].Id].String())
	c.Assert(localItem.ConfirmedList, HasLen, 2)
	// this error message indicates the message share is accepted by the this system.
	err = tssCommonStruct.ProcessOneMessage(wrappedVerMsg, tssCommonStruct.PartyIDtoP2PID[partiesID[0].Id].String())
	c.Assert(err, ErrorMatches, "fail to update the message to local party: fail to set bytes to local party: task , party <nil>, round -1: proto: can't skip unknown wire type 4")
}

func (t *TssTestSuite) testVerMsgWrongHash(c *C, tssCommonStruct *TssCommon, senderID *btss.PartyID, peerParties []*btss.PartyID, testParties TestParties, senderMsg *messages.WrappedMessage, peerMsgMap map[int]*messages.WrappedMessage, msgKey string, blameOwner bool) {
	err := tssCommonStruct.ProcessOneMessage(senderMsg, senderID.Id)
	c.Assert(err, IsNil)
	localItem := tssCommonStruct.TryGetLocalCacheItem(msgKey)
	c.Assert(localItem.ConfirmedList, HasLen, 1)

	// we firstly load the host node message
	for i, nodeIndex := range testParties.honest {
		msg := peerMsgMap[nodeIndex]
		err = tssCommonStruct.ProcessOneMessage(msg, tssCommonStruct.PartyIDtoP2PID[peerParties[nodeIndex].Id].String())
		c.Assert(err, IsNil)
		c.Assert(localItem.ConfirmedList, HasLen, 2+i)
	}

	// we now load the malicious nodes message
	currentLength := len(localItem.ConfirmedList)
	for i, nodeIndex := range testParties.malicious {

		msg := peerMsgMap[nodeIndex]
		err = tssCommonStruct.ProcessOneMessage(msg, tssCommonStruct.PartyIDtoP2PID[peerParties[nodeIndex].Id].String())
		if err == nil {
			c.Assert(localItem.ConfirmedList, HasLen, currentLength+i+1)
			continue
		}
		c.Assert(err, ErrorMatches, "hashcheck error from peer")

		dataOwner, err := senderIDtoPubKey(senderID)
		c.Assert(err, IsNil)
		expected := []string{dataOwner}
		if blameOwner == false {
			for _, nodeIndex := range testParties.malicious {
				pubKey, err := senderIDtoPubKey(peerParties[nodeIndex])
				c.Assert(err, IsNil)
				expected = append(expected, pubKey)
			}
		}

		// sort.Strings(tssCommonStruct.BlamePeers.BlameNodes)
		sort.Strings(expected)
		c.Assert(tssCommonStruct.blameMgr.GetBlame().BlameNodes, DeepEquals, expected)
	}
}

func findSender(arr []*btss.PartyID) *btss.PartyID {
	for _, el := range arr {
		var pk secp256k1.PubKeySecp256k1
		copy(pk[:], el.GetKey())
		out, _ := sdk.Bech32ifyAccPub(pk)
		if out == testSenderPubKey {
			return el
		}
	}
	return nil
}

// TestProcessVerMessage is the tests for processing the verified message
func (t *TssTestSuite) TestProcessVerMessage(c *C) {
	tssCommonStruct, peerPartiesID, partiesID := setupProcessVerMsgEnv(c, t.privKey, testBlamePubKeys, 4)
	sender := findSender(partiesID)
	t.testVerMsgDuplication(c, t.privKey, tssCommonStruct, sender, peerPartiesID)
	t.testVerMsgAndUpdate(c, tssCommonStruct, sender, partiesID)
}
