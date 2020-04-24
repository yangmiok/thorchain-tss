package common

import (
	. "gopkg.in/check.v1"

	"gitlab.com/thorchain/tss/go-tss/messages"
)

type ShareMgrSuite struct{}

var _ = Suite(&ShareMgrSuite{})

func (ShareMgrSuite) TestNewTssMsgStored(c *C) {
	msgmrg := NewTssShareMgr()
	w1 := messages.WireMessage{
		Routing:   nil,
		RoundInfo: "test1",
		Message:   nil,
		Sig:       nil,
	}

	msgmrg.storeTssMsg("test1", &w1)
	w2 := messages.WireMessage{
		Routing:   nil,
		RoundInfo: "test2",
		Message:   nil,
		Sig:       nil,
	}

	msgmrg.storeTssMsg("test2", &w2)
	w3 := messages.WireMessage{
		Routing:   nil,
		RoundInfo: "test3",
		Message:   nil,
		Sig:       nil,
	}
	msgmrg.storeTssMsg("test3", &w3)
	ret := msgmrg.getTssMsgStored("test4")
	c.Assert(ret, IsNil)

	ret = msgmrg.getTssMsgStored("test2")
	c.Assert(ret.RoundInfo, Equals, "test2")
}
