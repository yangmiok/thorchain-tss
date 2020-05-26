package conversion

import (
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
