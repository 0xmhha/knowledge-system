package coll1

import (
	"context"
	"time"
)

type clock struct {
	timer *time.Timer
}

func (c *clock) arm(d time.Duration) {
	c.timer = time.AfterFunc(d, func() {
		c.fire()
	})
}

func (c *clock) disarm() {
	if c.timer != nil {
		c.timer.Stop()
	}
}

func (c *clock) fire() {}

func waitDone(ctx context.Context) {
	<-ctx.Done()
}

// stamped embeds an external type; its promoted methods (Unix, ...) are
// declared outside the build and get no Method node.
type stamped struct {
	time.Time
}

func (s stamped) year() int { return s.Year() }
