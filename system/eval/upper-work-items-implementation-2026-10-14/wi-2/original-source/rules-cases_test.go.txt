package cases

import "testing"

func TestIndependentRuntimeContracts(t *testing.T) {
	input := "/fixture/input-command"
	if Compound(input).Path != input+"suffix" {
		t.Fatal("compound input propagation lost")
	}
	for _, cmd := range []string{Parameter(input).Path, Copy(input).Path, Branch(input, true).Path, Loop(input, true).Path, CommandContext(input).Path, Alias(input).Path, Renamed(input).Path, Closure(input)().Path} {
		if cmd != input {
			t.Fatal("input-dependent name lost", cmd)
		}
	}
	for _, cmd := range []string{Kill(input).Path, Argument(input).Path, Branch(input, false).Path, Loop(input, false).Path, Shadow(input).Path, Parallel(input).Path, Sanitizer(input).Path, Spoof(input, Fake{})} {
		if cmd != "fixed" {
			t.Fatal("fixed command contract", cmd)
		}
	}
	if Discard("17") != 17 || Discard("bad") != 0 || DiscardCopy("17") != 17 || Checked("bad") != 0 || ErrorKill("bad") != 3 || ErrorUnused("bad") != 3 || ErrorBranch("17", true) != 17 || ErrorBranch("17", false) != 0 || ErrorRenamed("17") != 17 {
		t.Fatal("conversion contract")
	}
	Multiple(input)
}
