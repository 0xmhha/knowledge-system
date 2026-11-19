package coll1

import "testing"

func TestClockArm(t *testing.T) {
	c := &clock{}
	c.arm(0)
	c.disarm()
}
