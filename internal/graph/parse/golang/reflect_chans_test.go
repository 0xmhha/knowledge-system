package golang_test

import (
	"fmt"
	"sort"
	"strings"
	"testing"

	gop "github.com/0xmhha/knowledge-system/internal/graph/parse/golang"
	"github.com/0xmhha/knowledge-system/pkg/graph/types"
)

// TestReflectChannelsAndHeldPrimitives checks, on
// testdata/concurrency/reflect_chans.go: a channel subscribed to a feed
// that delivers by reflect.Select gets a send from the feed's Send (and a
// trigger to its receive); a context's Done channel kept in a field makes
// the receive on the field a done_recv of that context; a cancel function
// kept in a field of type func() is a cancel when called through it; a
// channel received from a carrier inside a select is followed.
func TestReflectChannelsAndHeldPrimitives(t *testing.T) {
	g, err := gop.LoadAndResolveWith("testdata/concurrency", gop.LoadOptions{})
	if err != nil {
		t.Fatalf("LoadAndResolveWith: %v", err)
	}
	label := func(n types.Node) string {
		switch n.Type {
		case types.NodeCallSite, types.NodeChannel:
			return fmt.Sprintf("%s:%d", n.Type, n.StartLine)
		}
		return strings.TrimPrefix(n.QualifiedName, "mutex_fixture.")
	}
	byID := map[string]types.Node{}
	for _, n := range g.Nodes {
		byID[n.ID] = n
	}
	got := map[string]bool{}
	for _, e := range g.Edges {
		src, dst := byID[e.Src], byID[e.Dst]
		if !strings.HasSuffix(src.FilePath, "reflect_chans.go") && !strings.HasSuffix(dst.FilePath, "reflect_chans.go") {
			continue
		}
		kind := e.DispatchKind
		if i := strings.LastIndex(kind, ":"); e.Type == types.EdgeTriggers && i >= 0 {
			kind = kind[i:]
		}
		got[fmt.Sprintf("%s %s %s %s", label(src), e.Type, label(dst), kind)] = true
	}
	for _, want := range []string{
		"xferFeed.Send sends_to Channel:30 reflect",
		"xferFeed.Send triggers CallSite:32 :30",
		"CallSite:44 flows_to xferWatch.done sync",
		"CallSite:47 operates_on newXferWatch.ctx done_recv",
		"CallSite:49 operates_on xferWatch.cancel cancel",
		"xferPick triggers CallSite:65 :63~aliasing",
	} {
		if !got[want] {
			t.Errorf("missing %q", want)
		}
	}
	if t.Failed() {
		keys := make([]string, 0, len(got))
		for k := range got {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			t.Log(k)
		}
	}
}
