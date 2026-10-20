package valueflow

import "testing"

func TestSelectedReceiveDoesNotKillDefaultValue(t *testing.T) {
	r := analyze(t, "func F(ch chan int) int { x:=1; select { case x=<-ch: return x; default: return x } }")
	initial := idWithText(t, r, "x:=1")
	var selected string
	for _, block := range r.Blocks {
		if block.Kind == "SelectCaseBody" {
			for _, node := range r.Nodes {
				if node.Block == block.ID && node.Kind == "Ident" && node.Text == "x" {
					selected = node.ID
				}
			}
		}
	}
	if selected == "" {
		t.Fatal("missing selected-case owner")
	}
	returns := 0
	for _, block := range r.Blocks {
		for _, node := range r.Nodes {
			if node.Block != block.ID || node.Kind != "ReturnStmt" {
				continue
			}
			returns++
			if block.Kind == "SelectCaseBody" {
				if !relation(r, "data_dependence", selected, node.ID) || relation(r, "data_dependence", initial, node.ID) {
					t.Fatal("selected receive value lost", r.Edges)
				}
			} else if !relation(r, "data_dependence", initial, node.ID) || relation(r, "data_dependence", selected, node.ID) {
				t.Fatal("unselected receive killed default value", r.Edges)
			}
		}
	}
	if returns != 2 {
		t.Fatal("return denominator", returns)
	}
	for _, occurrence := range r.Occurrences {
		if occurrence.Role != "definition" {
			continue
		}
		for _, node := range r.Nodes {
			if node.ID == occurrence.Node && node.Text == "x=<-ch" {
				t.Fatal("definition before case selection", occurrence)
			}
		}
	}
}

func TestSelectedReceiveShortDeclarationAndTwoValueRemainBounded(t *testing.T) {
	for _, code := range []string{
		"func F(ch chan int) int { select { case x:=<-ch: return x; default: return 0 } }",
		"func F(ch chan int) int { select { case x,ok:=<-ch: if ok { return x }; return 0; default: return 0 } }",
		"func F(a,b chan int) int { for { select { case x:=<-a: if x>0{return x}; case y:=<-b: return y } } }",
	} {
		r := analyze(t, code)
		definitions := 0
		for _, occurrence := range r.Occurrences {
			if occurrence.Role != "definition" || occurrence.Node == "entry" {
				continue
			}
			definitions++
			for _, node := range r.Nodes {
				if node.ID == occurrence.Node && node.Kind != "Ident" {
					t.Fatal("receive definition belongs outside selected LHS", node)
				}
			}
		}
		if definitions == 0 || !diag(r, "select_unsupported") {
			t.Fatal("missing receive definition or unsupported scheduling boundary", r)
		}
	}
}
