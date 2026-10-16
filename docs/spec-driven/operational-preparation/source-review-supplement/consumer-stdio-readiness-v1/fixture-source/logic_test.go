package fixture

import "testing"

func TestFixture(t *testing.T) {
	if Helper(2) != 3 || Empty(3) != 6 || Field(&Box{}, 4) != 4 || Branch(1, 2, true) != 2 {
		t.Fatal("fixture contract")
	}
	if Seed("owned-nonexecuted-command", []int{1, 2}, false).Path != "owned-nonexecuted-command" {
		t.Fatal("command construction")
	}
}
