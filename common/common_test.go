package common

import (
	"io"
	"os"
	"testing"

	btss "github.com/binance-chain/tss-lib/tss"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/tendermint/tendermint/crypto/secp256k1"
	. "gopkg.in/check.v1"

	"gitlab.com/thorchain/thornode/cmd"
)

var (
	testBlamePrivKey = "OWU2YTk1NzdlOTA5NTAxZmI4YjUyODYyMmZkYzBjNzJlMTQ5YTI2YWY5NzkzYTc0MjA3MDBkMWQzMzFiMDNhZg=="
	testPubKeys      = [...]string{"thorpub1addwnpepqtdklw8tf3anjz7nn5fly3uvq2e67w2apn560s4smmrt9e3x52nt2svmmu3", "thorpub1addwnpepqtspqyy6gk22u37ztra4hq3hdakc0w0k60sfy849mlml2vrpfr0wvm6uz09", "thorpub1addwnpepq2ryyje5zr09lq7gqptjwnxqsy2vcdngvwd6z7yt5yjcnyj8c8cn559xe69", "thorpub1addwnpepqfjcw5l4ay5t00c32mmlky7qrppepxzdlkcwfs2fd5u73qrwna0vzag3y4j"}
	testBlamePubKeys = []string{"thorpub1addwnpepqtr5p8tllhp4xaxmu77zhqen24pmrdlnekzevshaqkyzdqljm6rejnnt02t", "thorpub1addwnpepqwz59mn4ae82svm2pfqnycducjjeez0qlf3sum7rclrr8mn44pr5gkeey25", "thorpub1addwnpepqga4nded5hhnwsrwmrns803w7vu9mffp9r6dz4l6smaww2l5useuq6vkttg", "thorpub1addwnpepq28hfdpu3rdgvj8skzhlm8hyt5nlwwc8pjrzvn253j86e4dujj6jsmuf25q", "thorpub1addwnpepqfuq0xc67052h288r6flp67l0ny9mg6u3sxhsrlukyfg0fe9j6q36ysd33y", "thorpub1addwnpepq0jszts80udfl4pkfk6cp93647yl6fhu6pk486uwjdz2sf94qvu0kw0t6ug", "thorpub1addwnpepqw6mmffk69n5taaqhq3wsc8mvdpsrdnx960kujeh4jwm9lj8nuyux9hz5e4", "thorpub1addwnpepq0pdhm2jatzg2vy6fyw89vs6q374zayqd5498wn8ww780grq256ygq7hhjt", "thorpub1addwnpepqggwmlgd8u9t2sx4a0styqwhzrvdhpvdww7sqwnweyrh25rjwwm9q65kx9s", "thorpub1addwnpepqtssltyjvms8pa7k4yg85lnrjqtvvr2ecr36rhm7pa4ztf55tnuzzgvegpk"}
)

type TestParties struct {
	honest    []int
	malicious []int
}

func TestPackage(t *testing.T) { TestingT(t) }

type TssTestSuite struct{}

var _ = Suite(&TssTestSuite{})

func (t *TssTestSuite) SetUpSuite(c *C) {
	InitLog("info", true, "tss_common_test")
	config := sdk.GetConfig()
	config.SetBech32PrefixForAccount(cmd.Bech32PrefixAccAddr, cmd.Bech32PrefixAccPub)
	config.SetBech32PrefixForValidator(cmd.Bech32PrefixValAddr, cmd.Bech32PrefixValPub)
	config.SetBech32PrefixForConsensusNode(cmd.Bech32PrefixConsAddr, cmd.Bech32PrefixConsPub)
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

func senderIDtoPubKey(senderID *btss.PartyID) (string, error) {
	blamePartyKeyBytes := senderID.GetKey()
	var pk secp256k1.PubKeySecp256k1
	copy(pk[:], blamePartyKeyBytes)
	blamedPubKey, err := sdk.Bech32ifyAccPub(pk)
	return blamedPubKey, err
}
