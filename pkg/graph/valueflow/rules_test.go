package valueflow

import (
	"context"
	"encoding/json"
	"reflect"
	"strings"
	"testing"
)

func ruleAnalyze(t *testing.T, code string, o RuleOptions) Result {
	t.Helper()
	r, e := AnalyzeRules(context.Background(), retained(), []byte(code), Options{}, RulePackID, o)
	if e != nil || r.Rules == nil {
		t.Fatal(r, e)
	}
	return r
}
func TestRulePackScalarCommandCorpus(t *testing.T) {
	cases := []struct {
		name, code string
		count      int
	}{
		{"parameter", "func F(cmd string){exec.Command(cmd)}", 1},
		{"copy", "func F(cmd string){name:=cmd;exec.Command(name)}", 1},
		{"compound", "func F(cmd string){cmd+=\"suffix\";exec.Command(cmd)}", 1},
		{"kill", "func F(cmd string){name:=cmd;name=\"fixed\";exec.Command(name)}", 0},
		{"literal", "func F(cmd string){exec.Command(\"fixed\",cmd)}", 0},
		{"branch", "func F(cmd string,flag bool){name:=\"fixed\";if flag{name=cmd};exec.Command(name)}", 1},
		{"loop", "func F(cmd string,flag bool){name:=\"fixed\";for flag{name=cmd;flag=false};exec.Command(name)}", 1},
		{"shadow", "func F(cmd string){name:=\"fixed\";{name:=cmd;_=name};exec.Command(name)}", 0},
		{"parallel_rhs", "func F(cmd string){name,other:=\"fixed\",cmd;_=other;exec.Command(name)}", 0},
		{"command_context", "func F(cmd string){exec.CommandContext(nil,cmd)}", 1},
		{"unknown_sanitizer", "func F(cmd string){name:=sanitize(cmd);exec.Command(name)}", 1},
		{"alias_unknown", "func F(cmd string){ref:=&cmd;exec.Command(*ref)}", 0},
		{"renamed_import_miss", "func F(cmd string){process.Command(cmd)}", 0},
		{"spoofed_target_candidate", "func F(cmd string,exec Fake){exec.Command(cmd)}", 1},
		{"closure_not_inlined", "func F(cmd string){f:=func(){exec.Command(cmd)};_=f}", 0},
		{"unreachable", "func F(cmd string){return;exec.Command(cmd)}", 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r := ruleAnalyze(t, tc.code, RuleOptions{})
			if r.Rules.Status != "matched_bounded" || len(r.Rules.Candidates) != tc.count {
				t.Fatalf("%+v", r.Rules)
			}
			for _, c := range r.Rules.Candidates {
				if c.RuleID != "parameter-command-name" || c.Origin != "automatic" || c.TargetResolution != "syntax_only_unverified" || c.Sanitizer != "not_proven" || len(c.Path) < 2 || c.SourceLocation.StartByte < 400 || c.SinkLocation.StartByte < 400 {
					t.Fatal("false provenance/semantic claim", c)
				}
			}
			if tc.name == "branch" && len(r.Rules.Candidates[0].Conditions) == 0 {
				t.Fatal("branch condition lost")
			}
		})
	}
}
func TestRulePackDiscardedErrorCorpus(t *testing.T) {
	cases := []struct {
		name, code string
		count      int
	}{
		{"discarded", "func F(s string)int{v,_:=strconv.Atoi(s);return v}", 1},
		{"copy", "func F(s string)int{v,_:=strconv.ParseInt(s,10,64);out:=v;return out}", 1},
		{"checked", "func F(s string)int{v,err:=strconv.Atoi(s);if err!=nil{return 0};return v}", 0},
		{"overwritten", "func F(s string)int{v,_:=strconv.Atoi(s);v=3;return v}", 0},
		{"unused", "func F(s string)int{v,_:=strconv.Atoi(s);_=v;return 3}", 0},
		{"branch", "func F(s string,flag bool)int{v,_:=strconv.Atoi(s);if flag{return v};return 0}", 1},
		{"renamed_miss", "func F(s string)int{v,_:=conversion.Atoi(s);return v}", 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r := ruleAnalyze(t, tc.code, RuleOptions{})
			if r.Rules.Status != "matched_bounded" || len(r.Rules.Candidates) != tc.count {
				t.Fatalf("%+v", r.Rules)
			}
			for _, c := range r.Rules.Candidates {
				if c.RuleID != "discarded-conversion-error" {
					t.Fatal(c)
				}
			}
		})
	}
}
func TestRuleBoundsOptInAndDeterministicSourcePins(t *testing.T) {
	code := "func F(cmd string){exec.Command(cmd);exec.Command(cmd)}"
	r := ruleAnalyze(t, code, RuleOptions{MaxCandidates: 1})
	if len(r.Rules.Candidates) != 1 || r.Rules.Status != "incomplete" || r.Rules.StopReason != "candidates" {
		t.Fatal(r.Rules)
	}
	r = ruleAnalyze(t, code, RuleOptions{MaxFlowSteps: 1})
	if r.Rules.Status != "incomplete" || r.Rules.FlowSteps != 1 || r.Rules.StopReason != "flow_steps" {
		t.Fatal(r.Rules)
	}
	r = ruleAnalyze(t, code, RuleOptions{})
	again := ruleAnalyze(t, code, RuleOptions{})
	if !reflect.DeepEqual(r.Rules, again.Rules) {
		t.Fatal("nondeterministic pack/candidates")
	}
	plain, e := Analyze(context.Background(), retained(), []byte(code), Options{})
	if e != nil || plain.Rules != nil || !reflect.DeepEqual(plain.DefUse, r.DefUse) {
		t.Fatal("default enabled or recomputed different base graph")
	}
	for _, pack := range []string{"", "latest", "go-local-review@2.0.0"} {
		if _, e = AnalyzeRules(context.Background(), retained(), []byte(code), Options{}, pack, RuleOptions{}); e == nil {
			t.Fatal("implicit/unknown pack")
		}
	}
	source := retained()
	source.NodeID = "other"
	changed, e := AnalyzeRules(context.Background(), source, []byte(code), Options{}, RulePackID, RuleOptions{})
	if e != nil || changed.Rules.Candidates[0].ID == r.Rules.Candidates[0].ID {
		t.Fatal("candidate mixed source identity", e)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	r, e = AnalyzeRules(ctx, retained(), []byte(code), Options{}, RulePackID, RuleOptions{})
	if e != nil || r.Rules.Status != "incomplete" || r.Rules.StopReason != "base_analysis_incomplete" {
		t.Fatal(r, e)
	}
	r, e = AnalyzeRules(context.Background(), retained(), []byte(code), Options{MaxNodes: 1}, RulePackID, RuleOptions{})
	if e != nil || !r.Truncated || r.Rules.Status != "incomplete" {
		t.Fatal(r, e)
	}
}
func TestRuleQueryStrictInputPinsAndChangingSnapshot(t *testing.T) {
	valid := `{"rule_pack":"go-local-review@1.0.0","function":{"node_id":"1111111111111111"}}`
	q, e := DecodeRuleQueryInput([]byte(valid))
	if e != nil {
		t.Fatal(e)
	}
	for _, raw := range []string{`{}`, strings.Replace(valid, "go-local-review@1.0.0", "latest", 1), strings.Replace(valid, `"node_id":"1111111111111111"`, `"node_id":"1111111111111111","node_id":"x"`, 1), strings.Replace(valid, `"function":{`, `"unknown":true,"function":{`, 1), strings.Replace(valid, `"function":{`, `"max_candidates":0,"function":{`, 1), strings.Replace(valid, `"function":{`, `"max_flow_steps":1.5,"function":{`, 1), strings.Replace(valid, `"function":{"node_id":"1111111111111111"}`, `"function":null`, 1), valid + `{}`} {
		if _, e = DecodeRuleQueryInput([]byte(raw)); e == nil {
			t.Fatal("invalid input accepted", raw)
		}
	}
	s, id := queryFixture(t)
	q.Function.NodeID = id
	out, e := QueryRules(context.Background(), s, q)
	if e != nil || out.Analysis == nil || out.Analysis.Rules == nil {
		t.Fatal(out, e)
	}
	for _, mode := range []string{"manifest", "node", "blob"} {
		out, e = QueryRules(context.Background(), &driftingReader{Reader: s, mode: mode}, q)
		if e != nil || out.Status != "snapshot_changed" || out.Analysis != nil {
			t.Fatal(out, e)
		}
	}
	q.Function.ExpectedSourceSHA256 = strings.Repeat("b", 64)
	out, e = QueryRules(context.Background(), s, q)
	if e != nil || out.Status != "source_mismatch" || out.Analysis != nil {
		t.Fatal(out, e)
	}
	raw, e := json.Marshal(ruleAnalyze(t, "func F(cmd string){exec.Command(cmd)}", RuleOptions{}))
	if e != nil || !strings.Contains(string(raw), `"origin":"automatic"`) {
		t.Fatal(e)
	}
}

func TestRuleRangeControlWithoutASTLocationRemainsExplicitUnknown(t *testing.T) {
	r := ruleAnalyze(t, "func F(cmd string,values []int){for range values{exec.Command(cmd)}}", RuleOptions{})
	if len(r.Rules.Candidates) != 1 {
		t.Fatal(r.Rules)
	}
	unknown := false
	for _, condition := range r.Rules.Candidates[0].Conditions {
		if condition.LocationStatus == "structural_block_unlocated" {
			unknown = true
			if condition.Location != nil {
				t.Fatal("fabricated original location", condition)
			}
		} else if condition.LocationStatus != "source_node" || condition.Location == nil || condition.Location.StartLine < 1 {
			t.Fatal("missing source/unknown distinction", condition)
		}
	}
	if !unknown || r.Rules.EngineVersion != "2" {
		t.Fatal("range block location not explicitly unknown", r.Rules)
	}
}
