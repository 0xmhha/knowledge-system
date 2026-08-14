package service

import (
	"strings"
	"testing"
	"time"
)

func testDeployment() Deployment {
	return Deployment{
		Instance: "example-project",
		Binary:   "/repo/bin/cks",
		Config:   "/repo/cks.yaml",
		WorkDir:  "/repo",
		LogDir:   "/repo/run",
		HomeDir:  "/Users/op",
		Env:      map[string]string{"CKV_OLLAMA_ENDPOINT": "http://localhost:11434"},
	}
}

func TestDeploymentNaming(t *testing.T) {
	d := testDeployment()
	cases := []struct {
		name string
		got  string
		want string
	}{
		{"server label", d.ServerLabel(), "knowledge-system.example-project"},
		{"watchdog label", d.WatchdogLabel(), "knowledge-system.example-project.watchdog"},
		{"network label", d.LinkLabel(), "knowledge-system.example-project.network"},
		{"plist path", d.PlistPath(d.ServerLabel()),
			"/Users/op/Library/LaunchAgents/knowledge-system.example-project.plist"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if tc.got != tc.want {
				t.Errorf("got %q, want %q", tc.got, tc.want)
			}
		})
	}
}

func TestServerAgentHoldsSleepAssertionAndRestarts(t *testing.T) {
	agent := testDeployment().ServerAgent()

	if got := agent.ProgramArguments[0]; got != caffeinatePath {
		t.Errorf("server is not wrapped in caffeinate: argv[0] = %q", got)
	}
	if !agent.KeepAlive {
		t.Error("server agent must be KeepAlive: a crashed server has to come back on its own")
	}
	if !agent.RunAtLoad {
		t.Error("server agent must be RunAtLoad: it has to start at login without an operator")
	}
	if agent.StartInterval != 0 {
		t.Error("server agent is not a timer job")
	}
	argv := strings.Join(agent.ProgramArguments, " ")
	for _, want := range []string{"/repo/bin/cks", "mcp", "--config /repo/cks.yaml", "--name example-project"} {
		if !strings.Contains(argv, want) {
			t.Errorf("argv %q missing %q", argv, want)
		}
	}
}

func TestWatchdogAgentIsATimerNotAKeepAliveJob(t *testing.T) {
	d := testDeployment()
	d.WatchdogInterval = 30 * time.Second
	agent := d.WatchdogAgent()

	if agent.KeepAlive {
		t.Error("watchdog must not be KeepAlive: it is a one-shot probe, not a daemon")
	}
	if agent.StartInterval != 30*time.Second {
		t.Errorf("StartInterval = %s, want 30s", agent.StartInterval)
	}
	argv := strings.Join(agent.ProgramArguments, " ")
	if !strings.Contains(argv, "service recover") {
		t.Errorf("watchdog must run the recovery routine, got %q", argv)
	}
	if !strings.Contains(argv, "--quiet") {
		t.Errorf("watchdog must be quiet on the normal case or its log grows forever, got %q", argv)
	}
}

func TestWatchdogIntervalDefaults(t *testing.T) {
	if got := testDeployment().WatchdogAgent().StartInterval; got != defaultWatchdogInterval {
		t.Errorf("StartInterval = %s, want the %s default", got, defaultWatchdogInterval)
	}
}

// The network agent catches the one failure the other two cannot see: a host
// that moved networks. The server keeps serving on its wildcard bind and the
// watchdog keeps finding it healthy, while every client holds a dead URL.
func TestLinkAgentIsATimerRunningTheSingleShotCheck(t *testing.T) {
	d := testDeployment()
	d.LinkInterval = 30 * time.Second
	agent := d.LinkAgent()

	if agent.KeepAlive {
		t.Error("network agent must not be KeepAlive: launchd is the scheduler, not the process")
	}
	if agent.StartInterval != 30*time.Second {
		t.Errorf("StartInterval = %s, want 30s", agent.StartInterval)
	}
	argv := strings.Join(agent.ProgramArguments, " ")
	if !strings.Contains(argv, "service watch-network") {
		t.Errorf("network agent must run the link watcher, got %q", argv)
	}
	if !strings.Contains(argv, "--once") {
		t.Errorf("a timer job must check once and exit, not loop forever, got %q", argv)
	}
}

func TestLinkIntervalDefaults(t *testing.T) {
	if got := testDeployment().LinkAgent().StartInterval; got != defaultLinkAgentInterval {
		t.Errorf("StartInterval = %s, want the %s default", got, defaultLinkAgentInterval)
	}
}

func TestRenderProducesLoadablePlist(t *testing.T) {
	body, err := testDeployment().ServerAgent().Render()
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	out := string(body)
	for _, want := range []string{
		`<?xml version="1.0" encoding="UTF-8"?>`,
		"<!DOCTYPE plist PUBLIC",
		"<key>Label</key>\n\t<string>knowledge-system.example-project</string>",
		"<key>KeepAlive</key>\n\t<true/>",
		"<key>ThrottleInterval</key>\n\t<integer>10</integer>",
		"<key>EnvironmentVariables</key>",
		"<key>WorkingDirectory</key>\n\t<string>/repo</string>",
		"</plist>",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("rendered plist missing %q:\n%s", want, out)
		}
	}
}

func TestRenderIsDeterministic(t *testing.T) {
	d := testDeployment()
	d.Env = map[string]string{"B": "2", "A": "1", "C": "3"}
	first, err := d.ServerAgent().Render()
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	for i := 0; i < 5; i++ {
		again, err := d.ServerAgent().Render()
		if err != nil {
			t.Fatalf("render: %v", err)
		}
		if string(again) != string(first) {
			t.Fatal("map iteration leaked into the output: the same spec rendered differently")
		}
	}
	if a, b := strings.Index(string(first), "<key>A</key>"), strings.Index(string(first), "<key>B</key>"); a > b {
		t.Error("environment keys are not sorted")
	}
}

func TestRenderEscapesOperatorSuppliedText(t *testing.T) {
	d := testDeployment()
	d.Instance = "a&b"
	body, err := d.ServerAgent().Render()
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	if !strings.Contains(string(body), "a&amp;b") {
		t.Errorf("instance name was not XML-escaped:\n%s", body)
	}
}

func TestRenderRejectsIncompleteSpecs(t *testing.T) {
	cases := map[string]AgentSpec{
		"no label":   {ProgramArguments: []string{"/bin/true"}},
		"no program": {Label: "x"},
	}
	for name, spec := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := spec.Render(); err == nil {
				t.Error("want an error, got none")
			}
		})
	}
}
