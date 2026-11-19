package golang_test

import (
	"fmt"
	"sort"
	"strings"
	"testing"

	gop "github.com/0xmhha/knowledge-system/internal/graph/parse/golang"
	"github.com/0xmhha/knowledge-system/pkg/graph/types"
)

// TestChannelLinks checks that sends and receives in
// testdata/resolve/coll1/chans.go point at the channel they operate on: a
// field, a package-level variable, or the make site, also when the channel
// arrives as a parameter of a function started as a goroutine.
func TestChannelLinks(t *testing.T) {
	g, err := gop.LoadAndResolve("testdata/resolve")
	if err != nil {
		t.Fatalf("LoadAndResolve: %v", err)
	}
	name := map[string]string{}
	for _, n := range g.Nodes {
		if n.Type == types.NodeChannel {
			if strings.HasSuffix(n.FilePath, "coll1/chans.go") {
				name[n.ID] = fmt.Sprintf("Channel@L%d", n.StartLine)
			}
			continue
		}
		name[n.ID] = n.QualifiedName
	}
	got := map[string]bool{}
	for _, e := range g.Edges {
		if (e.Type == types.EdgeSendsTo || e.Type == types.EdgeRecvsFrom) && e.Confidence == types.ConfInferred && e.Line > 0 {
			got[fmt.Sprintf("%s %s %s %s L%d", name[e.Src], e.Type, name[e.Dst], e.DispatchKind, e.Line)] = true
		}
	}
	want := []string{
		"coll1.pump.push sends_to coll1.pump.out field L10",
		"coll1.pump.pull recvs_from coll1.pump.out field L12",
		"coll1.emit sends_to coll1.events global L17",
		"coll1.listen recvs_from coll1.events global L21",
		"coll1.relay recvs_from Channel@L31 make L28",
		"coll1.relay sends_to Channel@L32 make L28",
		"coll1.runRelay sends_to Channel@L31 make L34",
		"coll1.runRelay recvs_from Channel@L32 make L35",
		// seal's goroutine sends on the parameter its closure captures by reference
		"coll1.seal sends_to Channel@L52 make L44",
	}
	for _, w := range want {
		if !got[w] {
			var have []string
			for k := range got {
				have = append(have, k)
			}
			sort.Strings(have)
			t.Errorf("missing %q; have:\n%s", w, strings.Join(have, "\n"))
		}
	}
	// newPump stores the channel it makes in pump.out: one channel, two names.
	flows := map[string]bool{}
	for _, e := range g.Edges {
		if e.Type == types.EdgeFlowsTo {
			flows[fmt.Sprintf("%s %s %s L%d", name[e.Src], name[e.Dst], e.DispatchKind, e.Line)] = true
		}
	}
	if !flows["Channel@L7 coll1.pump.out chan L7"] {
		t.Errorf("missing flows_to from the make site to pump.out; have %v", flows)
	}
	// relay's in and out are different channels.
	if got["coll1.relay recvs_from Channel@L32 make L28"] || got["coll1.relay sends_to Channel@L31 make L28"] {
		t.Errorf("relay's channels are mixed up: %v", got)
	}
}
