package fixture

import "testing"

func TestIndependentStateAndGuard(t *testing.T) {
	Repeat("/fixture/command", []int{1, 2})
	box := &Box{}
	cmd := Process(box, "/fixture/command", []int{1, -2}, true)
	if box.Total != -1 || cmd.Path != "fixed" {
		t.Fatal("negative strict guard/state")
	}
	cmd = Process(box, "/fixture/command", []int{1, -2}, false)
	if box.Total != -1 || cmd.Path != "/fixture/command" {
		t.Fatal("unrestricted branch/state")
	}
	cmd = Process(box, "/fixture/command", []int{1, 2}, true)
	if box.Total != 3 || cmd.Path != "/fixture/command" {
		t.Fatal("positive branch/state")
	}
}
