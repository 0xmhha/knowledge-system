package detect

import (
	"testing"

	"golang.org/x/tools/go/packages"
)

func TestValidateLoadedPackagesRejectsFilelessListError(t *testing.T) {
	pkgs := []*packages.Package{{
		ID:     "example.test/fixture",
		Errors: []packages.Error{{Kind: packages.ListError, Msg: "go list failed: cache unavailable"}},
	}}
	if err := validateLoadedPackages("/fixture", pkgs); err == nil {
		t.Fatal("fileless package list error was accepted as an empty index")
	}
}

func TestValidateLoadedPackagesAllowsTypeErrorWithFiles(t *testing.T) {
	pkgs := []*packages.Package{{
		ID:      "example.test/fixture",
		GoFiles: []string{"/fixture/a.go"},
		Errors:  []packages.Error{{Kind: packages.TypeError, Msg: "undefined: Missing"}},
	}}
	if err := validateLoadedPackages("/fixture", pkgs); err != nil {
		t.Fatalf("type-check error must not hide discoverable files: %v", err)
	}
}
