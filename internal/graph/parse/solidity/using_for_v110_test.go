package solidity_test

import (
	"testing"
)

// Struct-field receiver dispatch tests.
//
// Shape: `<obj>.<field>.<method>(...)`. obj is a state-var or
// parameter whose type is a struct; field is a member of that struct.
// Resolver chains obj → objType → struct field → fieldType → binding
// lookup.
//
// The `this.<field>` predicate runs first (inner.object = "this"); this
// one catches all other identifier-receiver shapes. It only fires when
// objType is a known struct in the structFieldTypes index.
//
// Not covered here: cross-file struct definitions, nested
// struct field receivers (`obj.outer.inner.method()`), multi-return
// tuple destructuring.

// TestUsingForV110_StructFieldBasic — canonical struct-field case.
// `info.amount.add(1)` resolves through info's UserInfo struct type
// to amount's uint256 field type to StructLib.add.
func TestUsingForV110_StructFieldBasic(t *testing.T) {
	nodes, edges := parseResolveOneSol(t, "testdata/using_for_v110", "struct_field_basic.sol")
	got := collectUsingForCalls(nodes, edges)
	want := []callWant{
		{caller: "Vault.run", target: "StructLib.add"},
	}
	if !equalCallWants(got, want) {
		t.Errorf("EdgeCalls (using-for struct-field) mismatch:\n got=%v\nwant=%v", got, want)
	}
}

// TestUsingForV110_StructFieldUnknown — `data.missingField.tag()`. obj
// is a known struct state-var, but the field doesn't exist on the
// struct. The resolver must drop without false-positive emission.
func TestUsingForV110_StructFieldUnknown(t *testing.T) {
	nodes, edges := parseResolveOneSol(t, "testdata/using_for_v110", "struct_field_unknown.sol")
	got := collectUsingForCalls(nodes, edges)
	if len(got) != 0 {
		t.Errorf("unexpected V1.10 EdgeCalls when struct field missing: %v", got)
	}
}

// TestUsingForV110_StructFieldParam — struct receiver is a function
// parameter (not state-var). Resolver falls back to paramTypes via the
// state-var/param idiom shared by the other using-for branches.
func TestUsingForV110_StructFieldParam(t *testing.T) {
	nodes, edges := parseResolveOneSol(t, "testdata/using_for_v110", "struct_field_param.sol")
	got := collectUsingForCalls(nodes, edges)
	want := []callWant{
		{caller: "ParamUser.process", target: "ParamLib.bump"},
	}
	if !equalCallWants(got, want) {
		t.Errorf("EdgeCalls (using-for struct-field param) mismatch:\n got=%v\nwant=%v", got, want)
	}
}
