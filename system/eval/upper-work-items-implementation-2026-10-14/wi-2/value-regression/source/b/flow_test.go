package p

import "testing"

func TestSeparatePackageSameName(t *testing.T) {
	if Same(3) != 6 {
		t.Fatal("different function")
	}
}
