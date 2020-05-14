package go_tss

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"gitlab.com/thorchain/tss/go-tss/keygen"
	"gitlab.com/thorchain/tss/go-tss/keysign"
	"gitlab.com/thorchain/tss/go-tss/messages"

	"gitlab.com/thorchain/tss/go-tss/tss"

	btsskeygen "github.com/binance-chain/tss-lib/ecdsa/keygen"
	"github.com/libp2p/go-libp2p-core/peer"
	maddr "github.com/multiformats/go-multiaddr"
	"gitlab.com/thorchain/tss/go-tss/conversion"
	. "gopkg.in/check.v1"

	"gitlab.com/thorchain/tss/go-tss/common"
)

const (
	partyNum         = 6
	testFileLocation = "./test_data"
	preParamTestFile = "preParam_test.data"
	testSharesFile   = "16Uiu2HAmAWKWf5vnpiAhfdSQebTbbB3Bg35qtyG7Hr4ce23VFA8V.data" // this is the test node1 share
)

var (
	testPubKeys = []string{
		"thorpub1addwnpepqtdklw8tf3anjz7nn5fly3uvq2e67w2apn560s4smmrt9e3x52nt2svmmu3",
		"thorpub1addwnpepqtspqyy6gk22u37ztra4hq3hdakc0w0k60sfy849mlml2vrpfr0wvm6uz09",
		"thorpub1addwnpepq2ryyje5zr09lq7gqptjwnxqsy2vcdngvwd6z7yt5yjcnyj8c8cn559xe69",
		"thorpub1addwnpepqfjcw5l4ay5t00c32mmlky7qrppepxzdlkcwfs2fd5u73qrwna0vzag3y4j",
		"thorpub1addwnpepqd4ghum330y9mjhf384h66gnw77nfpl2q3f9fxzx87ktqjc36h7dc57z7y7",
		"thorpub1addwnpepqwhcp0catgy9a4vm0ymzjxnhpc3fy08ar0qtspu7trc6tjut3xdk5sq7tz7",
	}
	testPriKeyArr = []string{
		"MjQ1MDc2MmM4MjU5YjRhZjhhNmFjMmI0ZDBkNzBkOGE1ZTBmNDQ5NGI4NzM4OTYyM2E3MmI0OWMzNmE1ODZhNw==",
		"YmNiMzA2ODU1NWNjMzk3NDE1OWMwMTM3MDU0NTNjN2YwMzYzZmVhZDE5NmU3NzRhOTMwOWIxN2QyZTQ0MzdkNg==",
		"ZThiMDAxOTk2MDc4ODk3YWE0YThlMjdkMWY0NjA1MTAwZDgyNDkyYzdhNmMwZWQ3MDBhMWIyMjNmNGMzYjVhYg==",
		"ZTc2ZjI5OTIwOGVlMDk2N2M3Yzc1MjYyODQ0OGUyMjE3NGJiOGRmNGQyZmVmODg0NzQwNmUzYTk1YmQyODlmNA==",
		"NjM3ZWIwMGU5NTY4OTBmNmQwNmFlZDEzMGQ4Y2EyZjQ3ZjVlMmEzOWE5YzkzZThhNmMzZTJkNmE0MmMzZjFjMg==",
		"OTFmMzhjZDkyMzY2MWJhZTU3NGNlZGNiMTkxZDFhN2ZhZDgzMjkxNWI1OTM1YjEyYTljNzJhZWFjZThkY2MzOA==",
	}
	testPeersIDs = []string{
		"16Uiu2HAm2FzqoUdS6Y9Esg2EaGcAG5rVe1r6BFNnmmQr2H3bqafa",
		"16Uiu2HAm4TmEzUqy3q3Dv7HvdoSboHk5sFj2FH3npiN5vDbJC6gh",
		"16Uiu2HAmACG5DtqmQsHtXg4G2sLS65ttv84e7MrL4kapkjfmhxAp",
		"16Uiu2HAmAWKWf5vnpiAhfdSQebTbbB3Bg35qtyG7Hr4ce23VFA8V",
		"16Uiu2HAmQU5ZYFVuy78V2jTdBjWwGZwHCKjNs84ZVRSFAxMMwcY9",
		"16Uiu2HAmKpuLfkS3A2Mk4xU3dHbuMPwibNpkz19KjF5VEimLqXWb",
	}
)

func TestPackage(t *testing.T) {
	TestingT(t)
}

type SixNodeTestSuite struct {
	servers        []*tss.TssServer
	ports          []int
	preParams      []*btsskeygen.LocalPreParams
	bootstrapPeer  string
	keyGenPeersID  []peer.ID
	keySignPeersID []peer.ID
}

var _ = Suite(&SixNodeTestSuite{})

// setup Six nodes for test
func (s *SixNodeTestSuite) SetUpTest(c *C) {
	common.InitLog("info", true, "Six_nodes_test")
	conversion.SetupBech32Prefix()
	s.ports = []int{
		16666, 16667, 16668, 16669, 16670, 16671,
	}
	s.bootstrapPeer = "/ip4/127.0.0.1/tcp/16666/p2p/16Uiu2HAmACG5DtqmQsHtXg4G2sLS65ttv84e7MrL4kapkjfmhxAp"
	s.preParams = getPreparams(c)
	s.servers = make([]*tss.TssServer, partyNum)
	conf := common.TssConfig{
		KeyGenTimeout:   15 * time.Second,
		KeySignTimeout:  30 * time.Second,
		PreParamTimeout: 5 * time.Second,
	}
	var peersID []peer.ID
	for i := 0; i < partyNum; i++ {
		node, err := peer.Decode(testPeersIDs[i])
		c.Assert(err, IsNil)
		peersID = append(peersID, node)
		if i == 0 {
			s.servers[i] = s.getTssServer(c, i, conf, "")
		} else {
			s.servers[i] = s.getTssServer(c, i, conf, s.bootstrapPeer)
		}
		// time.Sleep(time.Millisecond * 500)
	}
	s.keyGenPeersID = peersID
	s.keySignPeersID = peersID
	for i := 0; i < partyNum; i++ {
		c.Assert(s.servers[i].Start(), IsNil)
	}
}

func hash(payload []byte) []byte {
	h := sha256.New()
	h.Write(payload)
	return h.Sum(nil)
}

//func (s *SixNodeTestSuite) TestApplyWrongShareNotFail(c *C) {
//	if testing.Short() {
//		c.Skip("skipping test in short mode.")
//	}
//	req := keygen.NewRequest(testPubKeys, "", nil, nil, nil)
//	// we apply the second broadcast message to all peers
//	req1 := keygen.NewRequest(testPubKeys, messages.KEYGEN3, testPeersIDs, nil, nil)
//	wg := sync.WaitGroup{}
//	keygenResult := make(map[int]keygen.Response)
//	lock := &sync.Mutex{}
//	for i := 0; i < partyNum; i++ {
//		wg.Add(1)
//		go func(idx int) {
//			defer wg.Done()
//			if idx == 1 || idx == 2 {
//				resp, err := s.servers[idx].Keygen(req1)
//				c.Assert(err, IsNil)
//				c.Assert(resp.Blame.BlameNodes, HasLen, 0)
//				lock.Lock()
//				defer lock.Unlock()
//				keygenResult[idx] = resp
//
//			} else {
//				resp, err := s.servers[idx].Keygen(req)
//				c.Assert(resp.Blame.BlameNodes, HasLen, 0)
//				c.Assert(err, IsNil)
//				lock.Lock()
//				defer lock.Unlock()
//				keygenResult[idx] = resp
//			}
//		}(i)
//	}
//	wg.Wait()
//
//	var poolPubKey string
//	for _, item := range keygenResult {
//		if len(poolPubKey) == 0 {
//			poolPubKey = item.PubKey
//		} else {
//			c.Assert(poolPubKey, Equals, item.PubKey)
//		}
//	}
//}
//
//// this tigger binance Tss bug
//func (s *SixNodeTestSuite) TestKeygenAttackSendWrongShareNotFail(c *C) {
//	if testing.Short() {
//		c.Skip("skipping test in short mode.")
//	}
//	shares, err := getTestShares(c)
//	c.Assert(err, IsNil)
//	req := keygen.NewRequest(testPubKeys, "", nil, nil, nil)
//	req1 := keygen.NewRequest(testPubKeys, messages.KEYGEN2b, nil, testPeersIDs[1:2], shares[2])
//	wg := sync.WaitGroup{}
//	keygenResult := make(map[int]keygen.Response)
//	lock := &sync.Mutex{}
//	for i := 0; i < partyNum; i++ {
//		wg.Add(1)
//		go func(idx int) {
//			defer wg.Done()
//			if idx == 1 {
//				resp, err := s.servers[idx].Keygen(req1)
//				c.Assert(err, IsNil)
//				c.Assert(resp.Blame.BlameNodes, HasLen, 0)
//				lock.Lock()
//				defer lock.Unlock()
//				keygenResult[idx] = resp
//			} else {
//				resp, err := s.servers[idx].Keygen(req)
//				c.Assert(resp.Blame.BlameNodes, HasLen, 0)
//				c.Assert(err, IsNil)
//				lock.Lock()
//				defer lock.Unlock()
//				keygenResult[idx] = resp
//			}
//		}(i)
//	}
//	wg.Wait()
//	var poolPubKey string
//	for _, item := range keygenResult {
//		if len(poolPubKey) == 0 {
//			poolPubKey = item.PubKey
//		} else {
//			c.Assert(poolPubKey, Equals, item.PubKey)
//		}
//	}
//}
//
//// this tigger binance Tss bug
//func (s *SixNodeTestSuite) TestKeygenAttackSendWrongShareToAll(c *C) {
//	if testing.Short() {
//		c.Skip("skipping test in short mode.")
//	}
//	shares, err := getTestShares(c)
//	c.Assert(err, IsNil)
//	req := keygen.NewRequest(testPubKeys, "", nil, nil, nil)
//	req1 := keygen.NewRequest(testPubKeys, messages.KEYGEN2b, testPeersIDs, testPeersIDs, shares[2])
//	wg := sync.WaitGroup{}
//	for i := 0; i < partyNum; i++ {
//		wg.Add(1)
//		go func(idx int) {
//			defer wg.Done()
//			if idx == 1 {
//				resp, err := s.servers[idx].Keygen(req1)
//				c.Assert(err, NotNil)
//				fmt.Printf("------->%v", resp.Blame.BlameNodes)
//
//			} else {
//				resp, err := s.servers[idx].Keygen(req)
//				c.Assert(err, NotNil)
//				fmt.Printf("------->%v", resp.Blame.BlameNodes)
//			}
//		}(i)
//	}
//	wg.Wait()
//}
//
//func (s *SixNodeTestSuite) TestKeygenAttackOnePeerFail(c *C) {
//	if testing.Short() {
//		c.Skip("skipping test in short mode.")
//	}
//	req := keygen.NewRequest(testPubKeys, "", nil, nil, nil)
//	req1 := keygen.NewRequest(testPubKeys, messages.KEYGEN3, testPeersIDs[:2], nil, nil)
//	wg := sync.WaitGroup{}
//	var expected []string
//	for i := 1; i < 3; i++ {
//		sk := s.servers[i].PrivateKey
//		blameKey, err := sdk.Bech32ifyAccPub(sk.PubKey())
//		c.Assert(err, IsNil)
//		expected = append(expected, blameKey)
//	}
//	sort.Strings(expected)
//	for i := 0; i < partyNum; i++ {
//		wg.Add(1)
//		go func(idx int) {
//			defer wg.Done()
//			if idx == 1 || idx == 2 {
//				s.servers[idx].Keygen(req1)
//			} else {
//				resp, err := s.servers[idx].Keygen(req)
//				c.Assert(err, NotNil)
//				var blames []string
//				for _, el := range resp.Blame.BlameNodes {
//					blames = append(blames, el.Pubkey)
//				}
//				sort.Strings(blames)
//				c.Assert(blames, DeepEquals, expected)
//			}
//		}(i)
//	}
//	wg.Wait()
//}

func (s *SixNodeTestSuite) TestKeygenAndKeySign(c *C) {
	req := keygen.NewRequest(testPubKeys, "", nil, nil, nil)
	wg := sync.WaitGroup{}
	lock := &sync.Mutex{}
	keygenResult := make(map[int]keygen.Response)
	for i := 0; i < partyNum; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			res, err := s.servers[idx].Keygen(req)
			c.Assert(err, IsNil)
			lock.Lock()
			defer lock.Unlock()
			keygenResult[idx] = res
		}(i)
	}
	wg.Wait()
	var poolPubKey string
	for _, item := range keygenResult {
		if len(poolPubKey) == 0 {
			poolPubKey = item.PubKey
		} else {
			c.Assert(poolPubKey, Equals, item.PubKey)
		}
	}
	var signersKey []string
	signersKey = make([]string, 6)
	for i, el := range testPubKeys[:6] {
		signersKey[i] = el
	}
	var attackerKey []string
	signersKey = make([]string, 6)
	for i, el := range testPubKeys[:6] {
		signersKey[i] = el
	}
	keySignReq := keysign.NewRequest(poolPubKey, base64.StdEncoding.EncodeToString(hash([]byte("helloworld"))), signersKey, "", nil, nil, nil)
	keySignReqErr := keysign.NewRequest(poolPubKey, base64.StdEncoding.EncodeToString(hash([]byte("helloworld"))), signersKey, messages.KEYSIGN7, attackerKey, nil, nil)
	keySignResult := make(map[int]keysign.Response)
	for i := 0; i < partyNum; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			var res keysign.Response
			if idx == 1 {
				res, _ = s.servers[idx].KeySign(keySignReqErr)
			} else {
				res, _ = s.servers[idx].KeySign(keySignReq)
			}
			lock.Lock()
			defer lock.Unlock()
			keySignResult[idx] = res
		}(i)
	}
	wg.Wait()
	// var signature string
	for _, item := range keySignResult {
		fmt.Printf("------%v\n", item.Blame)
		//if len(signature) == 0 {
		//	signature = item.S + item.R
		//	continue
		//}
		//c.Assert(signature, Equals, item.S+item.R)
	}
	// c.Assert(len(signature), Equals, 88)
}

func (s *SixNodeTestSuite) TearDownTest(c *C) {
	// give a second before we shutdown the network
	time.Sleep(time.Millisecond * 200)
	//if !s.isBlameTest {
	//	s.servers[0].Stop()
	//}
	for i := 0; i < partyNum; i++ {
		s.servers[i].Stop()
	}
}

func (s *SixNodeTestSuite) getTssServer(c *C, index int, conf common.TssConfig, bootstrap string) *tss.TssServer {
	priKey, err := conversion.GetPriKey(testPriKeyArr[index])
	c.Assert(err, IsNil)
	baseHome := path.Join(os.TempDir(), strconv.Itoa(index))
	if _, err := os.Stat(baseHome); os.IsNotExist(err) {
		err := os.Mkdir(baseHome, os.ModePerm)
		c.Assert(err, IsNil)
	}
	var peerIDs []maddr.Multiaddr
	if len(bootstrap) > 0 {
		multiAddr, err := maddr.NewMultiaddr(bootstrap)
		c.Assert(err, IsNil)
		peerIDs = []maddr.Multiaddr{multiAddr}
	} else {
		peerIDs = nil
	}
	instance, err := tss.NewTss(peerIDs, s.ports[index], priKey, "Asgard", baseHome, conf, s.preParams[index])
	c.Assert(err, IsNil)
	return instance
}

func getPreparams(c *C) []*btsskeygen.LocalPreParams {
	var preParamArray []*btsskeygen.LocalPreParams
	buf, err := ioutil.ReadFile(path.Join(testFileLocation, preParamTestFile))
	c.Assert(err, IsNil)
	preParamsStr := strings.Split(string(buf), "\n")
	for _, item := range preParamsStr {
		var preParam btsskeygen.LocalPreParams
		val, err := hex.DecodeString(item)
		c.Assert(err, IsNil)
		c.Assert(json.Unmarshal(val, &preParam), IsNil)
		preParamArray = append(preParamArray, &preParam)
	}
	return preParamArray
}

func getTestShares(c *C) ([][]byte, error) {
	buf, err := ioutil.ReadFile(path.Join(testFileLocation, testSharesFile))
	if err != nil {
		return nil, err
	}
	shares := strings.Split(string(buf), "\n")
	var rawShares [][]byte
	for _, el := range shares {
		val, err := hex.DecodeString(el)
		c.Assert(err, IsNil)
		rawShares = append(rawShares, val)
	}
	return rawShares, nil
}
