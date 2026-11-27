package golang

import (
	"go/token"
	"testing"

	"github.com/0xmhha/knowledge-system/pkg/graph/types"
)

// TestSiteIndexFileIsTheLongestMatch: a position's file is the node path
// that ends its file name and is the longest such path. eth/handler_test.go
// also ends /src/eth/protocols/eth/handler_test.go; taking the first match
// of a map made the receives of that file come and go between two builds.
func TestSiteIndexFileIsTheLongestMatch(t *testing.T) {
	ix := newSiteIndex([]types.Node{
		{ID: "a", Type: types.NodeCallSite, FilePath: "eth/handler_test.go"},
		{ID: "b", Type: types.NodeCallSite, FilePath: "eth/protocols/eth/handler_test.go"},
	})
	for i := 0; i < 20; i++ {
		ix.files = map[string]string{}
		if f := ix.file(token.Position{Filename: "/src/eth/protocols/eth/handler_test.go"}); f != "eth/protocols/eth/handler_test.go" {
			t.Fatalf("file = %q", f)
		}
	}
}
