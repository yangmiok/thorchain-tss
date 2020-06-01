package conversion

import (
	"github.com/libp2p/go-libp2p-core/protocol"
	. "gopkg.in/check.v1"
)

type TssHelper struct{}

var _ = Suite(&TssHelper{})

func (*TssHelper) TestGetHighestFreq(c *C) {
	testMap := make(map[string]string)
	_, _, err := GetHighestFreq(testMap)
	c.Assert(err, NotNil)
	_, _, err = GetHighestFreq(nil)
	c.Assert(err, NotNil)
	testMap["1"] = "aa"
	testMap["2"] = "aa"
	testMap["3"] = "aa"
	testMap["4"] = "ab"
	testMap["5"] = "bb"
	testMap["6"] = "bb"
	testMap["7"] = "bc"
	testMap["8"] = "cd"
	val, freq, err := GetHighestFreq(testMap)
	c.Assert(err, IsNil)
	c.Assert(val, Equals, "aa")
	c.Assert(freq, Equals, 3)
}

func (*TssHelper) TestSupportedVersion(c *C) {
	protos := []string{"/p2p/tss/18", "/p2p/tss/20"}
	version, err := SupportedVersion(protos)
	c.Assert(err, IsNil)
	c.Assert(version["/p2p/tss/18"].String(), Equals, "18.0.0")
	c.Assert(version["/p2p/tss/20"].String(), Equals, "20.0.0")

	_, err = SupportedVersion([]string{"ss"})
	c.Assert(err, ErrorMatches, "error parsing version: Invalid Semantic Version")

	_, err = SupportedVersion([]string{"33#12"})
	c.Assert(err, ErrorMatches, "error parsing version: Invalid Semantic Version")

	_, err = SupportedVersion(nil)
	c.Assert(err, ErrorMatches, "empty input")
}

func (*TssHelper) TestGetProtocol(c *C) {
	protos := []string{"/p2p/tss/18", "/p2p/tss/20"}
	version, err := SupportedVersion(protos)
	c.Assert(err, IsNil)
	_, err = GetProtocol("18.2.0", version)
	c.Assert(err, ErrorMatches, "unsupported version format")

	ret, err := GetProtocol("gg18.2.0", version)
	c.Assert(err, IsNil)
	proto := protocol.ConvertToStrings([]protocol.ID{ret})[0]
	c.Assert(proto, Equals, "/p2p/tss/18")

	ret, err = GetProtocol("gg18.3.4", version)
	c.Assert(err, IsNil)
	proto = protocol.ConvertToStrings([]protocol.ID{ret})[0]
	c.Assert(proto, Equals, "/p2p/tss/18")

	ret, err = GetProtocol("gg20.2.0", version)
	c.Assert(err, IsNil)
	proto = protocol.ConvertToStrings([]protocol.ID{ret})[0]
	c.Assert(proto, Equals, "/p2p/tss/20")
}
