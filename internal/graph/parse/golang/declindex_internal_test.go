package golang

import (
	"go/token"
	"testing"

	"github.com/0xmhha/knowledge-system/pkg/graph/types"
)

// A declaration found by its short qualified name must be in the file asked
// for. Two programs both declare `package main` with a function dump; when
// only one is in the build, the other's uses were attached to it (7 edges on
// go-stablenet came from cmd/rlpdump and cmd/clef, which the build leaves
// out, and landed on cmd/gstable functions of the same name).
func TestDeclIndexAt_QualifiedNameNeedsTheFile(t *testing.T) {
	const qname = "main.dump"
	inBuild := types.Node{ID: "a", Type: types.NodeFunction, QualifiedName: qname,
		FilePath: "cmd/gstable/main.go", StartLine: 10, EndLine: 20}
	ix := newDeclIndex([]types.Node{inBuild})
	cases := []struct {
		name string
		pos  token.Position
		want string
	}{
		{"same file", token.Position{Filename: "/src/cmd/gstable/main.go", Line: 12}, "a"},
		{"another program's file", token.Position{Filename: "/src/cmd/rlpdump/main.go", Line: 12}, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := ""
			if n := ix.at("", qname, tc.pos); n != nil {
				got = n.ID
			}
			if got != tc.want {
				t.Errorf("at(%q, %s) = %q, want %q", qname, tc.pos.Filename, got, tc.want)
			}
		})
	}
}
