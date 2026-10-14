package fixture

import "testing"

func TestIndependentCallAndValueContracts(t *testing.T) {
	if Seed(2) != 5 || Seed(-2) != 3 || Cycle(-1) != 3 || Transform(1) != 4 || Normalize(1) != 3 || Noise() != 99 || Isolated(3) != 6 {
		t.Fatal("fixture contract")
	}
}
