package valueflow

import (
	"fmt"
	"go/ast"
	"go/token"
)

type branchValidator struct {
	loops, breaks int
	labels        map[string]ast.Stmt
	active        map[string]ast.Stmt
	failure       *error
}

func validateBranches(body *ast.BlockStmt) error {
	var failure error
	labels := map[string]ast.Stmt{}
	ast.Inspect(body, func(node ast.Node) bool {
		if _, ok := node.(*ast.FuncLit); ok {
			return false
		}
		if label, ok := node.(*ast.LabeledStmt); ok {
			if labels[label.Label.Name] != nil {
				failure = fmt.Errorf("duplicate label")
			}
			labels[label.Label.Name] = label.Stmt
		}
		return true
	})
	if failure != nil {
		return failure
	}
	ast.Walk(branchValidator{labels: labels, active: map[string]ast.Stmt{}, failure: &failure}, body)
	return failure
}
func (v branchValidator) Visit(node ast.Node) ast.Visitor {
	if *v.failure != nil {
		return nil
	}
	switch n := node.(type) {
	case *ast.FuncLit:
		return nil
	case *ast.ForStmt, *ast.RangeStmt:
		v.loops++
		v.breaks++
	case *ast.SwitchStmt, *ast.TypeSwitchStmt, *ast.SelectStmt:
		v.breaks++
	case *ast.LabeledStmt:
		active := map[string]ast.Stmt{}
		for name, statement := range v.active {
			active[name] = statement
		}
		active[n.Label.Name] = n.Stmt
		v.active = active
	case *ast.BranchStmt:
		valid := true
		switch n.Tok {
		case token.BREAK:
			valid = v.breaks > 0
			if n.Label != nil {
				switch v.active[n.Label.Name].(type) {
				case *ast.ForStmt, *ast.RangeStmt, *ast.SwitchStmt, *ast.TypeSwitchStmt, *ast.SelectStmt:
					valid = true
				default:
					valid = false
				}
			}
		case token.CONTINUE:
			valid = v.loops > 0
			if n.Label != nil {
				switch v.active[n.Label.Name].(type) {
				case *ast.ForStmt, *ast.RangeStmt:
					valid = true
				default:
					valid = false
				}
			}
		case token.GOTO:
			valid = n.Label != nil && v.labels[n.Label.Name] != nil
		}
		if !valid {
			*v.failure = fmt.Errorf("branch has no valid lexical target")
			return nil
		}
	}
	return v
}
