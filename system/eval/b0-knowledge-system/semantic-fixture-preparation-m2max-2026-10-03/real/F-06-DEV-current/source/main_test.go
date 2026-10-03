package fixture

import "testing"

func TestHealth(t *testing.T) { if 1+1 != 2 { t.Fatal("health") } }
