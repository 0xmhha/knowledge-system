package setup

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/0xmhha/knowledge-system/internal/toolenv"
)

// Event is one machine-readable progress record. The setup CLI serializes
// these as one JSON object per line under --progress=json; the MCP job
// surface returns the tail of the same stream.
type Event struct {
	Time    time.Time `json:"time"`
	Step    string    `json:"step"`
	Type    string    `json:"type"` // "start" | "output" | "warning" | "done" | "error"
	Message string    `json:"message,omitempty"`
}

// Runner executes one step. The default is SubprocessRunner; tests substitute
// their own to exercise plan orchestration without real engine binaries.
type Runner interface {
	Run(ctx context.Context, s Step, emit func(Event)) error
}

// SubprocessRunner runs command steps as child processes, streaming combined
// output line-by-line as Events, and runs internal steps in-process.
//
// Children get a PATH that is guaranteed to contain the build toolchain,
// resolved from this machine rather than inherited — see toolchainEnv. Without
// it, a server launchd started has no `go` to hand the graph build, and the
// failure surfaces deep inside go/packages instead of here.
type SubprocessRunner struct{}

// toolchainTools are what a build subprocess shells out to: go/packages runs
// `go list`, and indexing records each chunk's commit.
var toolchainTools = []string{toolenv.ToolGo, toolenv.ToolGit}

// toolchainEnv resolves the build toolchain once per process. Resolution can
// cost a login-shell round trip, and a plan runs many steps, so the answer is
// computed on first use and reused; nothing about it changes mid-run.
var toolchainEnv = sync.OnceValue(func() toolchainResolution {
	ctx, cancel := context.WithTimeout(context.Background(), toolchainResolveTimeout)
	defer cancel()

	r := toolenv.New()
	res, err := r.Resolve(ctx, toolchainTools...)

	// Values that are machine state rather than tool locations. Both are set
	// only when the inherited environment leaves them unset, so an operator
	// who chose one keeps it: GOPATH points a child at the module cache the
	// operator's own builds already filled instead of defaulting to ~/go, and
	// GOTOOLCHAIN=auto lets a go.mod that names a newer release fetch it.
	extra := map[string]string{}
	if os.Getenv("GOPATH") == "" {
		extra["GOPATH"] = r.Var(ctx, "GOPATH")
	}
	if os.Getenv("GOTOOLCHAIN") == "" {
		extra["GOTOOLCHAIN"] = "auto"
	}
	return toolchainResolution{
		env:     toolenv.ChildEnv(os.Getenv("PATH"), res, extra),
		summary: strings.Join(toolenv.Describe(res), " "),
		err:     err,
	}
})

// toolchainResolveTimeout bounds the whole resolution, login shell included.
const toolchainResolveTimeout = 15 * time.Second

type toolchainResolution struct {
	env     []string
	summary string
	err     error
}

// reportToolchain emits the resolution once per process. A missing tool is a
// warning here rather than a hard failure: not every step needs the toolchain,
// and the step that does will fail with its own message. Naming it up front is
// what turns that later failure into something an operator can act on.
var reportToolchain = func() func(emit func(Event)) {
	var once sync.Once
	return func(emit func(Event)) {
		once.Do(func() {
			t := toolchainEnv()
			if t.err != nil {
				emit(Event{Time: time.Now().UTC(), Step: "toolchain", Type: "warning",
					Message: t.err.Error()})
			}
			if t.summary != "" {
				emit(Event{Time: time.Now().UTC(), Step: "toolchain", Type: "output",
					Message: "resolved " + t.summary})
			}
		})
	}
}()

func (SubprocessRunner) Run(ctx context.Context, s Step, emit func(Event)) error {
	if s.Cmd == nil {
		if s.Verify == nil {
			return fmt.Errorf("step %s: neither command nor verify", s.ID)
		}
		return s.Verify(emit)
	}
	reportToolchain(emit)
	cmd := exec.CommandContext(ctx, s.Cmd[0], s.Cmd[1:]...)
	// Toolchain first, then the step's own env: a step that names a variable
	// outranks the machine-wide resolution (exec keeps the last duplicate).
	cmd.Env = append(cmd.Environ(), toolchainEnv().env...)
	cmd.Env = append(cmd.Env, s.Env...)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("step %s: %w", s.ID, err)
	}
	cmd.Stderr = cmd.Stdout // combined stream, in order
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("step %s: start %s: %w", s.ID, s.Cmd[0], err)
	}
	sc := bufio.NewScanner(stdout)
	sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for sc.Scan() {
		emit(Event{Time: time.Now().UTC(), Step: s.ID, Type: "output", Message: sc.Text()})
	}
	if err := cmd.Wait(); err != nil {
		return fmt.Errorf("step %s: %s: %w", s.ID, s.Cmd[0], err)
	}
	return nil
}

// Execute runs the plan sequentially, emitting start/done/error markers
// around each step and stopping at the first failure.
func Execute(ctx context.Context, p Plan, r Runner, emit func(Event)) error {
	if emit == nil {
		emit = func(Event) {}
	}
	var mu sync.Mutex
	safeEmit := func(e Event) { mu.Lock(); defer mu.Unlock(); emit(e) }
	for _, s := range p.Steps {
		safeEmit(Event{Time: time.Now().UTC(), Step: s.ID, Type: "start", Message: s.Title})
		if err := r.Run(ctx, s, safeEmit); err != nil {
			safeEmit(Event{Time: time.Now().UTC(), Step: s.ID, Type: "error", Message: err.Error()})
			return err
		}
		safeEmit(Event{Time: time.Now().UTC(), Step: s.ID, Type: "done"})
	}
	return nil
}
