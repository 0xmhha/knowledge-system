package p

import "testing"

func TestNormalContracts(t *testing.T) {
	if Same(3) != 4 || Branch(1, 2, false) != 1 || Branch(1, 2, true) != 2 || Kill(1, 2) != 2 || Loop(0) != 0 || Loop(3) != 0 || Named(false) != 0 || Named(true) != 1 || Closure(3) != 3 {
		t.Fatal("scalar contracts")
	}
	box := &Box{}
	if Field(box, 4) != 4 || box.Value != 4 {
		t.Fatal("field contract")
	}
	if Const() != 3 || *Pointer(new(int)) != 0 || box.Scalar(3) != 3 || Generic(3) != 3 {
		t.Fatal("constant/reference/method/generic contracts")
	}
	Nonexit(false)
}
