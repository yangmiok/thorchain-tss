package common

import (
	. "gopkg.in/check.v1"
)

type BlameTestSuite struct{}

var _ = Suite(&BlameTestSuite{})

func createNewBlameNode(key string) BlameNode {
	return NewBlameNode(key, nil, nil)
}

func (BlameTestSuite) TestBlame(c *C) {
	b := NewBlame("whatever", []BlameNode{createNewBlameNode("1"), createNewBlameNode("2")})
	c.Assert(b.IsEmpty(), Equals, false)
	c.Logf("%s", b)
	b.AddBlameNodes(createNewBlameNode("3"), createNewBlameNode("4"))
	c.Assert(b.BlameNodes, HasLen, 4)
	b.AddBlameNodes(createNewBlameNode("3"))
	c.Assert(b.BlameNodes, HasLen, 4)
	b.SetBlame("helloworld", nil)
	c.Assert(b.FailReason, Equals, "helloworld")
}

func (t *BlameTestSuite) TestBlameReason(c *C) {
	c.Assert(NoBlame.IsEmpty(), Equals, true)
	blame := Blame{FailReason: "test"}
	c.Assert(blame.String(), Equals, "reason:test\nnodes:[]\n")
}
