package mcp

import "testing"

func TestRoot_Precedence(t *testing.T) {
	// explicit wins over everything.
	t.Setenv(EnvRoot, "from-env")
	if got := Root("explicit", "fallback"); got != "explicit" {
		t.Errorf("Root explicit = %q, want explicit", got)
	}
	// env wins over build-time and fallback.
	oldBuild := BuildRoot
	BuildRoot = "from-build"
	t.Cleanup(func() { BuildRoot = oldBuild })
	if got := Root("", "fallback"); got != "from-env" {
		t.Errorf("Root env = %q, want from-env", got)
	}
	// build-time wins over fallback.
	t.Setenv(EnvRoot, "")
	if got := Root("", "fallback"); got != "from-build" {
		t.Errorf("Root build = %q, want from-build", got)
	}
	// fallback last.
	BuildRoot = ""
	if got := Root("", "fallback"); got != "fallback" {
		t.Errorf("Root fallback = %q, want fallback", got)
	}
}

func TestName_Join(t *testing.T) {
	if got := Name("", "context.find_symbol"); got != "context.find_symbol" {
		t.Errorf("bare Name = %q", got)
	}
	if got := Name("cks", "context.find_symbol"); got != "cks.context.find_symbol" {
		t.Errorf("joined Name = %q", got)
	}
}
