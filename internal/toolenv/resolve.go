package toolenv

import (
	"context"
	"fmt"
	"io/fs"
	"maps"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"time"
)

// Tool names this package knows how to look for. They are the two the build
// subprocesses invoke: go/packages runs `go list`, and indexing records the
// commit each chunk came from.
const (
	ToolGo  = "go"
	ToolGit = "git"
)

// Source records which strategy found a tool. It is reported to the operator
// because the three carry different confidence: an inherited path is what the
// caller was already using, a login-shell path reflects the machine's own
// configuration, and a standard location is this package's guess.
type Source string

const (
	// SourceInherited means the tool was already on the process PATH.
	SourceInherited Source = "inherited_path"
	// SourceLoginShell means the operator's login shell resolved it.
	SourceLoginShell Source = "login_shell"
	// SourceStandard means it was found at a well-known install location.
	SourceStandard Source = "standard_location"
)

// fallbackPath is what a child gets when nothing inherited a PATH at all. It
// matches launchd's own default rather than inventing one.
const fallbackPath = "/usr/bin:/bin:/usr/sbin:/sbin"

// loginShellTimeout bounds the login-shell query. Dotfiles run arbitrary code
// and some of them are slow or interactive; a build must not hang behind one.
const loginShellTimeout = 5 * time.Second

// toolNamePattern constrains what may be interpolated into the login shell's
// command string. The names are compiled in, not caller data, but the value
// crosses into a shell either way, so the guard is stated rather than assumed.
var toolNamePattern = regexp.MustCompile(`^[a-z][a-z0-9_-]*$`)

// standardLocations lists install locations to probe when neither the
// inherited PATH nor the login shell answers, newest-convention first. They
// are patterns evaluated against this machine, never recorded paths.
func standardLocations(tool, home string) []string {
	locations := []string{
		filepath.Join("/opt/homebrew/bin", tool),
		filepath.Join("/usr/local/bin", tool),
	}
	if tool == ToolGo {
		locations = append(locations, "/usr/local/go/bin/go")
	}
	if home != "" {
		locations = append(locations, filepath.Join(home, "go", "bin", tool))
	}
	return append(locations, filepath.Join("/usr/bin", tool))
}

// Resolution is one tool's discovered location and how it was found.
type Resolution struct {
	Tool   string `json:"tool"`
	Path   string `json:"path"`
	Source Source `json:"source"`
}

// Dir is the directory to put on PATH so a child process finds this tool.
func (r Resolution) Dir() string { return filepath.Dir(r.Path) }

// Resolver locates tools. Every interaction with the machine goes through a
// field, so the strategy order is testable without a shell, a filesystem or a
// particular host.
type Resolver struct {
	// LookPath searches the inherited PATH. Defaults to exec.LookPath.
	LookPath func(string) (string, error)
	// Stat reports whether a candidate exists. Defaults to os.Stat, which
	// follows symlinks — a version manager's dangling link is not a hit.
	Stat func(string) (fs.FileInfo, error)
	// LoginShell runs one command through the operator's login shell and
	// returns its stdout. Defaults to execLoginShell.
	LoginShell func(ctx context.Context, command string) (string, error)
	// Getenv reads the process environment. Defaults to os.Getenv.
	Getenv func(string) string
}

// New returns a Resolver wired to the real machine.
func New() Resolver {
	return Resolver{
		LookPath:   exec.LookPath,
		Stat:       os.Stat,
		LoginShell: execLoginShell,
		Getenv:     os.Getenv,
	}
}

// Resolve locates each tool, in the order given. It returns the resolutions it
// managed and an error naming every tool no strategy found, so a caller can
// report the whole gap at once instead of one tool per run.
func (r Resolver) Resolve(ctx context.Context, tools ...string) ([]Resolution, error) {
	out := make([]Resolution, 0, len(tools))
	var missing []string
	for _, tool := range tools {
		res, ok := r.resolveOne(ctx, tool)
		if !ok {
			missing = append(missing, tool)
			continue
		}
		out = append(out, res)
	}
	if len(missing) > 0 {
		return out, fmt.Errorf("toolenv: not found on this machine: %s "+
			"(install them, or start the server from a shell that has them)",
			strings.Join(missing, ", "))
	}
	return out, nil
}

func (r Resolver) resolveOne(ctx context.Context, tool string) (Resolution, bool) {
	if path, err := r.lookPath(tool); err == nil && path != "" {
		if abs, ok := r.usable(path); ok {
			return Resolution{Tool: tool, Path: abs, Source: SourceInherited}, true
		}
	}
	if path, ok := r.fromLoginShell(ctx, tool); ok {
		return Resolution{Tool: tool, Path: path, Source: SourceLoginShell}, true
	}
	for _, candidate := range standardLocations(tool, r.getenv("HOME")) {
		if abs, ok := r.usable(candidate); ok {
			return Resolution{Tool: tool, Path: abs, Source: SourceStandard}, true
		}
	}
	return Resolution{}, false
}

// fromLoginShell asks the login shell where the tool is. The answer is
// untrusted text from a file the operator owns: it is accepted only when it
// names an absolute path to an executable regular file.
func (r Resolver) fromLoginShell(ctx context.Context, tool string) (string, bool) {
	if !toolNamePattern.MatchString(tool) {
		return "", false
	}
	shell := r.LoginShell
	if shell == nil {
		return "", false
	}
	out, err := shell(ctx, "command -v "+tool)
	if err != nil {
		return "", false
	}
	line := strings.TrimSpace(firstLine(out))
	if !filepath.IsAbs(line) {
		// A shell function or builtin resolves to something that is not a
		// path; there is nothing to put on a child's PATH.
		return "", false
	}
	return r.usable(line)
}

// usable reports whether path is an executable regular file, and returns it.
func (r Resolver) usable(path string) (string, bool) {
	stat := r.Stat
	if stat == nil {
		stat = os.Stat
	}
	info, err := stat(path)
	if err != nil || info.IsDir() || info.Mode().Perm()&0o111 == 0 {
		return "", false
	}
	return path, true
}

func (r Resolver) lookPath(tool string) (string, error) {
	if r.LookPath == nil {
		return exec.LookPath(tool)
	}
	return r.LookPath(tool)
}

func (r Resolver) getenv(key string) string {
	if r.Getenv == nil {
		return os.Getenv(key)
	}
	return r.Getenv(key)
}

// Var returns the value the operator's login shell gives name, or "" when the
// shell does not define it. Used for values that are machine state rather than
// tool locations — GOPATH being the one that matters, since a child that
// defaults it to ~/go reaches for a different module cache than the one the
// operator's builds already filled.
func (r Resolver) Var(ctx context.Context, name string) string {
	if !toolNamePattern.MatchString(strings.ToLower(name)) || r.LoginShell == nil {
		return ""
	}
	out, err := r.LoginShell(ctx, "printf %s \"${"+name+":-}\"")
	if err != nil {
		return ""
	}
	return strings.TrimSpace(firstLine(out))
}

// execLoginShell runs command through the operator's login shell. `-lc` is
// what loads their profile; the timeout is what keeps a slow profile from
// becoming a stuck build.
func execLoginShell(ctx context.Context, command string) (string, error) {
	shell := os.Getenv("SHELL")
	if shell == "" {
		shell = "/bin/sh"
	}
	ctx, cancel := context.WithTimeout(ctx, loginShellTimeout)
	defer cancel()
	out, err := exec.CommandContext(ctx, shell, "-lc", command).Output()
	if err != nil {
		return "", fmt.Errorf("toolenv: login shell %q: %w", shell, err)
	}
	return string(out), nil
}

func firstLine(s string) string {
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		return s[:i]
	}
	return s
}

// ChildEnv returns the environment assignments a child process needs so the
// resolved tools are on its PATH: every resolution's directory, in resolution
// order, ahead of whatever PATH was inherited. Directories already present are
// not repeated, and an empty inherited PATH falls back to the system default
// rather than to nothing.
//
// extra is merged last, so a caller can pin values (GOPATH, GOTOOLCHAIN) that
// are not tool locations. An entry whose value is empty is dropped: "unset" and
// "set to empty" mean different things to the go command.
func ChildEnv(inheritedPath string, res []Resolution, extra map[string]string) []string {
	if inheritedPath == "" {
		inheritedPath = fallbackPath
	}
	seen := make(map[string]bool)
	var dirs []string
	for _, r := range res {
		d := r.Dir()
		if d == "" || seen[d] {
			continue
		}
		seen[d] = true
		dirs = append(dirs, d)
	}
	for _, d := range strings.Split(inheritedPath, string(os.PathListSeparator)) {
		if d == "" || seen[d] {
			continue
		}
		seen[d] = true
		dirs = append(dirs, d)
	}

	env := []string{"PATH=" + strings.Join(dirs, string(os.PathListSeparator))}
	// Sorted so the same inputs always produce the same environment — a child
	// process's env is part of what makes a build reproducible.
	for _, key := range slices.Sorted(maps.Keys(extra)) {
		if extra[key] == "" {
			continue
		}
		env = append(env, key+"="+extra[key])
	}
	return env
}

// Describe renders one line per resolution for an operator-facing log. The
// source matters as much as the path: "login_shell" says the value came from
// this machine's configuration and will follow it when it changes.
func Describe(res []Resolution) []string {
	out := make([]string, 0, len(res))
	for _, r := range res {
		out = append(out, fmt.Sprintf("%s=%s (%s)", r.Tool, r.Path, r.Source))
	}
	return out
}
