package mcphandlers

import (
	"testing"

	"github.com/0xmhha/knowledge-system/internal/graph/concfacts"
	"github.com/0xmhha/knowledge-system/pkg/graph/types"
)

// TestChannelRow: a triggers edge's aliasing mark is a field of the row,
// never part of the channel's name.
func TestChannelRow(t *testing.T) {
	sure, unsure := map[string]any{}, map[string]any{}
	channelRow(sure, "chan@a.go:3")
	channelRow(unsure, "chan@a.go:3~aliasing")
	if sure["channel"] != "chan@a.go:3" || sure["certain"] != nil {
		t.Errorf("certain row %v", sure)
	}
	if unsure["channel"] != "chan@a.go:3" || unsure["certain"] != false || unsure["reason"] != "aliasing" {
		t.Errorf("aliasing row %v", unsure)
	}
}

// TestLegendExplainsSchema145: an answer with the facts of schema 1.45
// explains each in its legend, and an answer without them has none.
func TestLegendExplainsSchema145(t *testing.T) {
	out := map[string]any{}
	addLegend(out, []types.Node{{Type: types.NodeFuncLit}, {Type: types.NodeCaseClause},
		{Type: types.NodeCallSite, SyncUnknown: []types.SyncUnknown{{}}}},
		[]types.Edge{{Type: types.EdgeJumpsTo}, {Type: types.EdgeLiteralPassedTo}, {Type: types.EdgeTriggers},
			{Type: types.EdgeFlowsTo, DispatchKind: concfacts.SyncAliasKind}, {Type: types.EdgeEncloses, DispatchKind: clauseKind}})
	l, _ := out[legendKey].(map[string]string)
	for _, k := range []string{"FuncLit", "CaseClause", "sync_unknown", "jumps_to", "literal_passed_to", "triggers", "flows_to sync", "encloses clause"} {
		if l[k] == "" {
			t.Errorf("legend lacks %q: %v", k, l)
		}
	}
	none := map[string]any{}
	addLegend(none, []types.Node{{Type: types.NodeFunction}}, []types.Edge{{Type: types.EdgeCalls}})
	if none[legendKey] != nil {
		t.Errorf("legend without the facts: %v", none)
	}
}
