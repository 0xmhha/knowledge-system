package twice

import "testing"

func TestUse(t *testing.T) {
	if Use() == 0 {
		t.Fatal("Use returned zero")
	}
}
