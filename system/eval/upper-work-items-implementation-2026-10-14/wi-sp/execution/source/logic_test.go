package fixture

import "testing"

func TestFixture(t *testing.T) {
	if Seed(3) != 4 || Empty(3) != 6 {
		t.Fatal("fixture")
	}
}
