package tss

import (
	"crypto/sha256"
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

	"gitlab.com/thorchain/tss/go-tss/conversion"

	btsskeygen "github.com/binance-chain/tss-lib/ecdsa/keygen"
	"github.com/libp2p/go-libp2p-core/peer"
	maddr "github.com/multiformats/go-multiaddr"
	. "gopkg.in/check.v1"

	"gitlab.com/thorchain/tss/go-tss/common"
	"gitlab.com/thorchain/tss/go-tss/keygen"
	"gitlab.com/thorchain/tss/go-tss/messages"
)

const (
	partyNum         = 6
	testFileLocation = "../test_data"
	preParamTestFile = "preParam_test.data"
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
	servers       []*TssServer
	ports         []int
	preParams     []*btsskeygen.LocalPreParams
	bootstrapPeer string
	isBlameTest   bool
	peersID       []peer.ID
}

var _ = Suite(&SixNodeTestSuite{})

// setup Six nodes for test
func (s *SixNodeTestSuite) SetUpTest(c *C) {
	s.isBlameTest = false
	common.InitLog("info", true, "Six_nodes_test")
	conversion.SetupBech32Prefix()
	s.ports = []int{
		16666, 16667, 16668, 16669, 16670, 16671,
	}
	s.bootstrapPeer = "/ip4/127.0.0.1/tcp/16666/p2p/16Uiu2HAmACG5DtqmQsHtXg4G2sLS65ttv84e7MrL4kapkjfmhxAp"
	s.preParams = getPreparams(c)
	s.servers = make([]*TssServer, partyNum)
	conf := common.TssConfig{
		KeyGenTimeout:   15 * time.Second,
		KeySignTimeout:  15 * time.Second,
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
		time.Sleep(time.Second)
	}
	s.peersID = peersID
	for i := 0; i < partyNum; i++ {
		c.Assert(s.servers[i].Start(), IsNil)
	}
}

func hash(payload []byte) []byte {
	h := sha256.New()
	h.Write(payload)
	return h.Sum(nil)
}

// generate a new key
//func (s *SixNodeTestSuite) TestKeygenAttacks(c *C) {
//	req := keygen.NewRequest(testPubKeys, "", nil)
//	req1 := keygen.NewRequest(testPubKeys, messages.KEYGEN3, nil)
//	wg := sync.WaitGroup{}
//	for i := 0; i < partyNum; i++ {
//		wg.Add(1)
//		go func(idx int) {
//			defer wg.Done()
//			if idx == 1 {
//				s.servers[idx].Keygen(req1)
//			} else {
//				resp, err := s.servers[idx].Keygen(req)
//				c.Assert(err, NotNil)
//				c.Assert(resp.Blame.BlameNodes, HasLen, 1)
//				sk := s.servers[1].privateKey
//				blameKey, err := sdk.Bech32ifyAccPub(sk.PubKey())
//				c.Assert(err, IsNil)
//				c.Assert(resp.Blame.BlameNodes[0].Pubkey, Equals, blameKey)
//			}
//		}(i)
//	}
//	wg.Wait()
//}

func (s *SixNodeTestSuite) TestKeygenAttackOnePeer(c *C) {
	req := keygen.NewRequest(testPubKeys, "", nil)
	req1 := keygen.NewRequest(testPubKeys, messages.KEYGEN3, testPeersIDs[:2])
	wg := sync.WaitGroup{}
	keygenResult := make(map[int]keygen.Response)
	lock := &sync.Mutex{}
	for i := 0; i < partyNum; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			if idx == 1 {
				resp, err := s.servers[idx].Keygen(req1)
				c.Assert(err, NotNil)
				fmt.Printf("%d---------%v\n", idx, resp.Blame.BlameNodes)
				// c.Assert(err, IsNil)
				lock.Lock()
				defer lock.Unlock()
				// keygenResult[idx] = resp
			} else {
				resp, err := s.servers[idx].Keygen(req)
				c.Assert(err, NotNil)
				fmt.Printf("%d---------%v\n", idx, resp.Blame.BlameNodes)
				// c.Assert(err, IsNil)
				// lock.Lock()
				// defer lock.Unlock()
				// keygenResult[idx] = resp
			}
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
}

// generate a new key
//func (s *SixNodeTestSuite) TestKeygenAndKeySign(c *C) {
//	req := keygen.NewRequest(testPubKeys, "", nil)
//	wg := sync.WaitGroup{}
//	lock := &sync.Mutex{}
//	keygenResult := make(map[int]keygen.Response)
//	for i := 0; i < partyNum; i++ {
//		wg.Add(1)
//		go func(idx int) {
//			defer wg.Done()
//			res, err := s.servers[idx].Keygen(req)
//			c.Assert(err, IsNil)
//			lock.Lock()
//			defer lock.Unlock()
//			keygenResult[idx] = res
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
//	keysignReqWithErr := keysign.NewRequest(poolPubKey, "helloworld", testPubKeys)
//	resp, err := s.servers[0].KeySign(keysignReqWithErr)
//	c.Assert(err, NotNil)
//	c.Assert(resp.S, Equals, "")
//	keysignReqWithErr1 := keysign.NewRequest(poolPubKey, base64.StdEncoding.EncodeToString(hash([]byte("helloworld"))), testPubKeys[:1])
//	resp, err = s.servers[0].KeySign(keysignReqWithErr1)
//	c.Assert(err, NotNil)
//	c.Assert(resp.S, Equals, "")
//	keysignReqWithErr2 := keysign.NewRequest(poolPubKey, base64.StdEncoding.EncodeToString(hash([]byte("helloworld"))), nil)
//	resp, err = s.servers[0].KeySign(keysignReqWithErr2)
//	c.Assert(err, NotNil)
//	c.Assert(resp.S, Equals, "")
//	keysignReq := keysign.NewRequest(poolPubKey, base64.StdEncoding.EncodeToString(hash([]byte("helloworld"))), testPubKeys)
//	keysignResult := make(map[int]keysign.Response)
//	for i := 0; i < partyNum; i++ {
//		wg.Add(1)
//		go func(idx int) {
//			defer wg.Done()
//			res, err := s.servers[idx].KeySign(keysignReq)
//			c.Assert(err, IsNil)
//			lock.Lock()
//			defer lock.Unlock()
//			keysignResult[idx] = res
//		}(i)
//	}
//	wg.Wait()
//	var signature string
//	for _, item := range keysignResult {
//		if len(signature) == 0 {
//			signature = item.S + item.R
//			continue
//		}
//		c.Assert(signature, Equals, item.S+item.R)
//	}
//	payload := base64.StdEncoding.EncodeToString(hash([]byte("helloworld+xyz")))
//	keysignReq = keysign.NewRequest(poolPubKey, payload, testPubKeys[:3])
//	keysignResult1 := make(map[int]keysign.Response)
//	for i := 0; i < partyNum; i++ {
//		wg.Add(1)
//		go func(idx int) {
//			defer wg.Done()
//			res, err := s.servers[idx].KeySign(keysignReq)
//			c.Assert(err, IsNil)
//			lock.Lock()
//			defer lock.Unlock()
//			keysignResult1[idx] = res
//		}(i)
//	}
//	wg.Wait()
//	signature = ""
//	for _, item := range keysignResult1 {
//		if len(signature) == 0 {
//			signature = item.S + item.R
//			continue
//		}
//		c.Assert(signature, Equals, item.S+item.R)
//	}
//	// make sure we sign
//}

//func (s *SixNodeTestSuite) TestFailJoinParty(c *C) {
//	// JoinParty should fail if there is a node that suppose to be in the keygen , but we didn't send request in
//	req := keygen.NewRequest(testPubKeys, "", nil)
//	wg := sync.WaitGroup{}
//	lock := &sync.Mutex{}
//	keygenResult := make(map[int]keygen.Response)
//	// here we skip the first node
//	for i := 1; i < partyNum; i++ {
//		wg.Add(1)
//		go func(idx int) {
//			defer wg.Done()
//			res, err := s.servers[idx].Keygen(req)
//			c.Assert(err, IsNil)
//			lock.Lock()
//			defer lock.Unlock()
//			keygenResult[idx] = res
//		}(i)
//	}
//	// if we shutdown one server during keygen , he should be blamed
//
//	wg.Wait()
//	c.Logf("result:%+v", keygenResult)
//	for idx, item := range keygenResult {
//		if idx == 0 {
//			continue
//		}
//		c.Assert(item.PubKey, Equals, "")
//		c.Assert(item.Status, Equals, common.Fail)
//	}
//}

//
//func (s *SixNodeTestSuite) TestBlame(c *C) {
//	s.isBlameTest = true
//	expectedFailNode := testPubKeys[0]
//	req := keygen.NewRequest(testPubKeys, "", nil)
//	wg := sync.WaitGroup{}
//	lock := &sync.Mutex{}
//	keygenResult := make(map[int]keygen.Response)
//	for i := 0; i < partyNum; i++ {
//		wg.Add(1)
//		go func(idx int) {
//			defer wg.Done()
//			res, err := s.servers[idx].Keygen(req)
//			c.Assert(err, NotNil)
//			lock.Lock()
//			defer lock.Unlock()
//			keygenResult[idx] = res
//		}(i)
//	}
//	// if we shutdown one server during keygen , he should be blamed
//
//	time.Sleep(time.Millisecond * 100)
//	s.servers[0].Stop()
//	wg.Wait()
//	c.Logf("result:%+v", keygenResult)
//	for idx, item := range keygenResult {
//		if idx == 0 {
//			continue
//		}
//		c.Assert(item.PubKey, Equals, "")
//		c.Assert(item.Status, Equals, common.Fail)
//		c.Assert(item.Blame.BlameNodes, HasLen, 1)
//		c.Assert(item.Blame.BlameNodes[0].Pubkey, Equals, expectedFailNode)
//	}
//}

func (s *SixNodeTestSuite) TearDownTest(c *C) {
	// give a second before we shutdown the network
	time.Sleep(time.Second)
	if !s.isBlameTest {
		s.servers[0].Stop()
	}
	for i := 1; i < partyNum; i++ {
		s.servers[i].Stop()
	}
}

func (s *SixNodeTestSuite) getTssServer(c *C, index int, conf common.TssConfig, bootstrap string) *TssServer {
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
	instance, err := NewTss(peerIDs, s.ports[index], priKey, "Asgard", baseHome, conf, s.preParams[index])
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
