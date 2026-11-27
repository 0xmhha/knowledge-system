package a

import "testing"

func TestRun(t *testing.T) {
	if Run() != 3+1+2+2+2 {
		t.Fatal(Run())
	}
	if Hub() != 5 {
		t.Fatal(Hub())
	}
	if Deferred(inc) != 0 {
		t.Fatal("Deferred")
	}
}
