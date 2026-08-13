package toolenv

import (
	"context"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// fakeInfo is a minimal fs.FileInfo: only the mode bits are consulted.
type fakeInfo struct {
	mode fs.FileMode
}

func (f fakeInfo) Name() string       { return "fake" }
func (f fakeInfo) Size() int64        { return 0 }
func (f fakeInfo) Mode() fs.FileMode  { return f.mode }
func (f fakeInfo) ModTime() time.Time { return time.Time{} }
func (f fakeInfo) IsDir() bool        { return f.mode.IsDir() }
func (f fakeInfo) Sys() any           { return nil }

// statFor builds a Stat seam where exactly the listed paths exist as
// executable regular files.
func statFor(executable ...string) func(string) (fs.FileInfo, error) {
	set := make(map[string]bool, len(executable))
	for _, p := range executable {
		set[p] = true
	}
	return func(path string) (fs.FileInfo, error) {
		if !set[path] {
			return nil, os.ErrNotExist
		}
		return fakeInfo{mode: 0o755}, nil
	}
}

// TestResolveStrategyOrder pins which strategy answers, because the order is
// the design: what the caller already had beats what the machine's own shell
// configuration says, which beats this package guessing at install locations.
func TestResolveStrategyOrder(t *testing.T) {
	t.Parallel()
	const gvmGo = "/home/u/.gvm/gos/go1.25.12/bin/go"
	cases := []struct {
		name       string
		lookPath   func(string) (string, error)
		loginShell func(context.Context, string) (string, error)
		exists     []string
		wantPath   string
		wantSource Source
	}{
		{
			name:       "inherited PATH wins when it has the tool",
			lookPath:   func(string) (string, error) { return "/usr/local/bin/go", nil },
			loginShell: func(context.Context, string) (string, error) { return gvmGo, nil },
			exists:     []string{"/usr/local/bin/go", gvmGo},
			wantPath:   "/usr/local/bin/go", wantSource: SourceInherited,
		},
		{
			// The launchd case: no PATH to inherit, and the tool lives where
			// only the operator's shell configuration knows about.
			name:       "login shell answers when PATH does not",
			lookPath:   func(string) (string, error) { return "", errors.New("not found") },
			loginShell: func(context.Context, string) (string, error) { return gvmGo + "\n", nil },
			exists:     []string{gvmGo},
			wantPath:   gvmGo, wantSource: SourceLoginShell,
		},
		{
			name:       "standard location is the last resort",
			lookPath:   func(string) (string, error) { return "", errors.New("not found") },
			loginShell: func(context.Context, string) (string, error) { return "", errors.New("no shell") },
			exists:     []string{"/opt/homebrew/bin/go"},
			wantPath:   "/opt/homebrew/bin/go", wantSource: SourceStandard,
		},
		{
			// gvm and friends can expose a tool as a shell function; the
			// answer is then not a path and must not reach a child's PATH.
			name:       "a non-path shell answer is rejected",
			lookPath:   func(string) (string, error) { return "", errors.New("not found") },
			loginShell: func(context.Context, string) (string, error) { return "go () { ... }", nil },
			exists:     []string{"/usr/local/bin/go"},
			wantPath:   "/usr/local/bin/go", wantSource: SourceStandard,
		},
		{
			// A version manager that switched releases leaves the old path
			// behind in a stale answer. Stat follows the link and refuses.
			name:       "a stale shell answer that no longer exists is rejected",
			lookPath:   func(string) (string, error) { return "", errors.New("not found") },
			loginShell: func(context.Context, string) (string, error) { return "/home/u/.gvm/gos/go1.25.11/bin/go", nil },
			exists:     []string{"/usr/local/bin/go"},
			wantPath:   "/usr/local/bin/go", wantSource: SourceStandard,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			r := Resolver{
				LookPath:   tc.lookPath,
				Stat:       statFor(tc.exists...),
				LoginShell: tc.loginShell,
				Getenv:     func(string) string { return "/home/u" },
			}
			got, err := r.Resolve(context.Background(), ToolGo)
			if err != nil {
				t.Fatalf("Resolve: %v", err)
			}
			if len(got) != 1 {
				t.Fatalf("got %d resolutions, want 1", len(got))
			}
			if got[0].Path != tc.wantPath || got[0].Source != tc.wantSource {
				t.Errorf("resolved %s via %s, want %s via %s",
					got[0].Path, got[0].Source, tc.wantPath, tc.wantSource)
			}
		})
	}
}

// TestResolveReportsEveryMissingTool keeps the failure actionable: an operator
// fixing a fresh machine should learn about both tools from one run.
func TestResolveReportsEveryMissingTool(t *testing.T) {
	t.Parallel()
	r := Resolver{
		LookPath:   func(string) (string, error) { return "", errors.New("not found") },
		Stat:       statFor(),
		LoginShell: func(context.Context, string) (string, error) { return "", errors.New("no shell") },
		Getenv:     func(string) string { return "" },
	}
	_, err := r.Resolve(context.Background(), ToolGo, ToolGit)
	if err == nil {
		t.Fatal("Resolve with nothing installed returned nil error")
	}
	for _, want := range []string{ToolGo, ToolGit} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error %q does not name %q", err, want)
		}
	}
}

// TestResolvePartialSucceedsForWhatItFound lets a caller act on the tools that
// did resolve while still seeing the error for the one that did not.
func TestResolvePartialSucceedsForWhatItFound(t *testing.T) {
	t.Parallel()
	r := Resolver{
		LookPath: func(tool string) (string, error) {
			if tool == ToolGit {
				return "/usr/bin/git", nil
			}
			return "", errors.New("not found")
		},
		Stat:       statFor("/usr/bin/git"),
		LoginShell: func(context.Context, string) (string, error) { return "", errors.New("no shell") },
		Getenv:     func(string) string { return "" },
	}
	got, err := r.Resolve(context.Background(), ToolGo, ToolGit)
	if err == nil {
		t.Fatal("expected an error naming the missing tool")
	}
	if len(got) != 1 || got[0].Tool != ToolGit {
		t.Fatalf("got %+v, want the one resolution for git", got)
	}
}

func TestChildEnv(t *testing.T) {
	t.Parallel()
	sep := string(os.PathListSeparator)
	res := []Resolution{
		{Tool: ToolGo, Path: "/opt/go/bin/go", Source: SourceLoginShell},
		{Tool: ToolGit, Path: "/usr/bin/git", Source: SourceInherited},
	}

	t.Run("resolved directories lead, inherited entries follow once", func(t *testing.T) {
		t.Parallel()
		env := ChildEnv("/usr/bin"+sep+"/bin", res, nil)
		want := "PATH=/opt/go/bin" + sep + "/usr/bin" + sep + "/bin"
		if env[0] != want {
			t.Errorf("PATH = %q, want %q", env[0], want)
		}
	})

	t.Run("an empty inherited PATH falls back to the system default", func(t *testing.T) {
		t.Parallel()
		env := ChildEnv("", res, nil)
		if !strings.HasSuffix(env[0], fallbackPath) {
			t.Errorf("PATH = %q, want it to end with %q", env[0], fallbackPath)
		}
		if !strings.HasPrefix(env[0], "PATH=/opt/go/bin") {
			t.Errorf("PATH = %q, want the resolved directory first", env[0])
		}
	})

	t.Run("extras are sorted and empty values are dropped", func(t *testing.T) {
		t.Parallel()
		env := ChildEnv("/usr/bin", nil, map[string]string{
			"GOTOOLCHAIN": "auto",
			"GOPATH":      "/home/u/go",
			"GOFLAGS":     "",
		})
		want := []string{"PATH=/usr/bin", "GOPATH=/home/u/go", "GOTOOLCHAIN=auto"}
		if len(env) != len(want) {
			t.Fatalf("env = %v, want %v", env, want)
		}
		for i := range want {
			if env[i] != want[i] {
				t.Errorf("env[%d] = %q, want %q", i, env[i], want[i])
			}
		}
	})
}

// TestVarRejectsAnUnsafeName guards the one place caller-shaped text would
// otherwise reach a shell command string.
func TestVarRejectsAnUnsafeName(t *testing.T) {
	t.Parallel()
	called := false
	r := Resolver{LoginShell: func(context.Context, string) (string, error) {
		called = true
		return "", nil
	}}
	if got := r.Var(context.Background(), "GOPATH; rm -rf /"); got != "" {
		t.Errorf("Var = %q, want empty", got)
	}
	if called {
		t.Error("an unsafe variable name reached the login shell")
	}
}

// TestResolutionDir is the value the PATH is built from.
func TestResolutionDir(t *testing.T) {
	t.Parallel()
	r := Resolution{Path: filepath.Join("/opt", "go", "bin", "go")}
	if got := r.Dir(); got != "/opt/go/bin" {
		t.Errorf("Dir() = %q, want /opt/go/bin", got)
	}
}
