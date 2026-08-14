package service

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

// scriptedRunner replays a canned result per launchctl verb and records every
// argv it was given.
type scriptedRunner struct {
	results map[string]struct {
		out string
		err error
	}
	calls [][]string
}

func (s *scriptedRunner) Run(_ context.Context, name string, args ...string) (string, error) {
	s.calls = append(s.calls, append([]string{name}, args...))
	verb := ""
	if len(args) > 0 {
		verb = args[0]
	}
	r := s.results[verb]
	return r.out, r.err
}

func newScriptedRunner() *scriptedRunner {
	return &scriptedRunner{results: map[string]struct {
		out string
		err error
	}{}}
}

func (s *scriptedRunner) on(verb, out string, err error) *scriptedRunner {
	s.results[verb] = struct {
		out string
		err error
	}{out, err}
	return s
}

func testController(r Runner) Controller { return Controller{Runner: r, UID: 501} }

func TestControllerTargetsTheUserDomain(t *testing.T) {
	r := newScriptedRunner()
	if err := testController(r).Kickstart(context.Background(), "com.example.svc"); err != nil {
		t.Fatalf("kickstart: %v", err)
	}
	got := strings.Join(r.calls[0], " ")
	want := "/bin/launchctl kickstart -k gui/501/com.example.svc"
	if got != want {
		t.Errorf("ran %q, want %q", got, want)
	}
}

func TestLoaded(t *testing.T) {
	cases := []struct {
		name    string
		out     string
		err     error
		want    bool
		wantErr bool
	}{
		{name: "loaded", want: true},
		{
			name: "absent is an answer, not a failure",
			out:  "Could not find service \"com.example.svc\" in domain for login item",
			err:  errors.New("exit status 113"),
		},
		{
			name:    "a real failure is reported",
			out:     "Bootstrap failed: 5: Input/output error",
			err:     errors.New("exit status 5"),
			wantErr: true,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c := testController(newScriptedRunner().on("print", tc.out, tc.err))
			got, err := c.Loaded(context.Background(), "com.example.svc")
			if got != tc.want {
				t.Errorf("loaded = %v, want %v", got, tc.want)
			}
			if (err != nil) != tc.wantErr {
				t.Errorf("err = %v, want error: %v", err, tc.wantErr)
			}
		})
	}
}

func TestBootoutIsIdempotent(t *testing.T) {
	c := testController(newScriptedRunner().on("bootout",
		"Boot-out failed: 3: No such process", errors.New("exit status 3")))
	if err := c.Bootout(context.Background(), "com.example.svc"); err != nil {
		t.Errorf("unloading a label that is not loaded must succeed, got %v", err)
	}
}

func TestBootoutPropagatesRealFailures(t *testing.T) {
	c := testController(newScriptedRunner().on("bootout",
		"Boot-out failed: 1: Operation not permitted", errors.New("exit status 1")))
	if err := c.Bootout(context.Background(), "com.example.svc"); err == nil {
		t.Error("want an error for a failure that is not 'already unloaded'")
	}
}

// queuedRunner replays a queue of results per launchctl verb, so a test can
// model launchd changing its answer between polls.
type queuedRunner struct {
	queues map[string][]struct {
		out string
		err error
	}
	calls [][]string
}

func (q *queuedRunner) Run(_ context.Context, name string, args ...string) (string, error) {
	q.calls = append(q.calls, append([]string{name}, args...))
	verb := ""
	if len(args) > 0 {
		verb = args[0]
	}
	queue := q.queues[verb]
	if len(queue) == 0 {
		return "", nil
	}
	head := queue[0]
	if len(queue) > 1 {
		q.queues[verb] = queue[1:]
	}
	return head.out, head.err
}

func (q *queuedRunner) verbCount(verb string) int {
	n := 0
	for _, c := range q.calls {
		if len(c) > 1 && c[1] == verb {
			n++
		}
	}
	return n
}

const notLoadedOutput = `Could not find service "com.example.svc" in domain`

// bootstrapRace is what launchd answers while it is still tearing a job down.
var bootstrapRace = errors.New("exit status 5")

func TestReplaceWaitsOutTheTeardownRace(t *testing.T) {
	// print says "still loaded" twice, then "gone"; the first bootstrap hits
	// the I/O error launchd returns mid-teardown, the second succeeds.
	r := &queuedRunner{queues: map[string][]struct {
		out string
		err error
	}{
		"print": {
			{out: "state = running"},
			{out: "state = running"},
			{out: notLoadedOutput, err: errors.New("exit status 113")},
		},
		"bootstrap": {
			{out: "Bootstrap failed: 5: Input/output error", err: bootstrapRace},
			{out: ""},
		},
	}}
	c := Controller{Runner: r, UID: 501, Sleep: func(time.Duration) {}}

	if err := c.Replace(context.Background(), "com.example.svc", "/tmp/x.plist"); err != nil {
		t.Fatalf("replace: %v", err)
	}
	if got := r.verbCount("bootstrap"); got != 2 {
		t.Errorf("bootstrap attempted %d times, want 2 (the first one races)", got)
	}
	if got := r.verbCount("print"); got != 3 {
		t.Errorf("polled %d times, want 3 (it must wait for the label to go)", got)
	}
}

func TestReplaceFailsLoudlyWhenTheLoadNeverTakes(t *testing.T) {
	r := &queuedRunner{queues: map[string][]struct {
		out string
		err error
	}{
		"print":     {{out: notLoadedOutput, err: errors.New("exit status 113")}},
		"bootstrap": {{out: "Bootstrap failed: 5: Input/output error", err: bootstrapRace}},
	}}
	c := Controller{Runner: r, UID: 501, Sleep: func(time.Duration) {}}

	err := c.Replace(context.Background(), "com.example.svc", "/tmp/x.plist")
	if err == nil {
		t.Fatal("want an error: a silent failure here leaves the server down")
	}
	if got := r.verbCount("bootstrap"); got != bootstrapAttempts {
		t.Errorf("bootstrap attempted %d times, want %d", got, bootstrapAttempts)
	}
}

func TestReplaceGivesUpIfTheLabelNeverUnloads(t *testing.T) {
	r := &queuedRunner{queues: map[string][]struct {
		out string
		err error
	}{
		"print": {{out: "state = running"}}, // never goes away
	}}
	c := Controller{Runner: r, UID: 501, Sleep: func(time.Duration) {}, UnloadTimeout: 20 * time.Millisecond}

	if err := c.Replace(context.Background(), "com.example.svc", "/tmp/x.plist"); err == nil {
		t.Fatal("want an error rather than bootstrapping over a job that is still there")
	}
	if got := r.verbCount("bootstrap"); got != 0 {
		t.Errorf("bootstrapped %d times while the old job was still loaded, want 0", got)
	}
}

func TestBootstrapCarriesTheDefinitionPath(t *testing.T) {
	r := newScriptedRunner()
	if err := testController(r).Bootstrap(context.Background(), "/Users/op/Library/LaunchAgents/x.plist"); err != nil {
		t.Fatalf("bootstrap: %v", err)
	}
	got := strings.Join(r.calls[0], " ")
	want := "/bin/launchctl bootstrap gui/501 /Users/op/Library/LaunchAgents/x.plist"
	if got != want {
		t.Errorf("ran %q, want %q", got, want)
	}
}
