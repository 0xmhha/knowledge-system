package service

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"path/filepath"
	"sort"
	"strconv"
	"time"
)

const (
	// DefaultLabelPrefix namespaces this deployment's launchd agents. It names
	// the software, not the project being served: this engine serves any pack,
	// and which one an instance serves is already carried by the instance name
	// the label ends with — a name the registry keeps unique per host.
	//
	// It is a constant rather than something injected per distribution because
	// the alternative axis does not exist: one build serves several packs, so
	// a build-time stamp could not distinguish them anyway. A deployment that
	// genuinely needs its own prefix — two distributions of this software on
	// one host — sets Deployment.LabelPrefix from its config.
	DefaultLabelPrefix = "knowledge-system"

	// caffeinatePath wraps the server so the process itself holds a no-sleep
	// assertion for exactly as long as it runs. The host's pmset policy is the
	// primary defence (see power.go); this is the second one, and it is the one
	// that survives someone resetting pmset.
	caffeinatePath = "/usr/bin/caffeinate"

	// launchAgentsDir is where a user agent's definition must live to be
	// bootstrappable into the login session's domain.
	launchAgentsDir = "Library/LaunchAgents"

	// defaultWatchdogInterval is how often the watchdog probes /healthz. A
	// wedged instance is therefore invisible for at most this long.
	defaultWatchdogInterval = time.Minute

	// defaultLinkAgentInterval is how often connectivity is re-read. It is
	// shorter than the watchdog's period because the failure it catches is
	// silent: the instance stays healthy on its wildcard bind while every
	// client holds a URL that no longer routes.
	defaultLinkAgentInterval = 20 * time.Second

	// throttleInterval bounds launchd's restart rate for a server that fails at
	// startup, so a bad config loops slowly enough to be diagnosable.
	throttleInterval = 10 * time.Second
)

// AgentSpec is one launchd user agent. It is rendered, not executed, so the
// rendering is a pure function of this struct and can be asserted in a test.
type AgentSpec struct {
	Label             string
	ProgramArguments  []string
	WorkingDirectory  string
	StandardOutPath   string
	StandardErrorPath string
	Environment       map[string]string
	// RunAtLoad starts the job as soon as it is bootstrapped (and at login).
	RunAtLoad bool
	// KeepAlive restarts the job whenever it exits, for any reason.
	KeepAlive bool
	// StartInterval runs the job on a timer. Zero means "not a timer job".
	StartInterval time.Duration
	// ThrottleInterval is launchd's minimum respawn spacing. Zero omits the key.
	ThrottleInterval time.Duration
}

// Deployment is one instance's launchd identity: which binary serves which
// config under which name, and where its files live. It is the single owner of
// the label and path conventions — nothing else in the tree spells them out.
type Deployment struct {
	// Instance is the MCP instance name; it also suffixes the launchd labels.
	Instance string
	// LabelPrefix overrides DefaultLabelPrefix. Empty uses the default, which
	// is the case every deployment should be in; set it only when one host
	// runs two distributions of this software and their instance names could
	// collide. It lands in a filename, so it must be filesystem-safe.
	LabelPrefix string
	// Binary, Config, WorkDir, LogDir and HomeDir are absolute paths. launchd
	// jobs inherit no shell, so a relative path here would resolve against "/".
	Binary  string
	Config  string
	WorkDir string
	LogDir  string
	HomeDir string
	// Env is extra environment for the server (e.g. CKV_OLLAMA_ENDPOINT).
	Env map[string]string
	// WatchdogInterval overrides how often the watchdog probes. Zero picks
	// defaultWatchdogInterval.
	WatchdogInterval time.Duration
	// LinkInterval overrides how often connectivity is re-read. Zero picks
	// defaultLinkAgentInterval.
	LinkInterval time.Duration
}

// ServerLabel is the launchd label of the served instance.
func (d Deployment) ServerLabel() string { return d.labelPrefix() + "." + d.Instance }

// labelPrefix is the configured prefix, or the software's default.
func (d Deployment) labelPrefix() string {
	if d.LabelPrefix != "" {
		return d.LabelPrefix
	}
	return DefaultLabelPrefix
}

// WatchdogLabel is the launchd label of the timer job that probes the server.
func (d Deployment) WatchdogLabel() string { return d.ServerLabel() + ".watchdog" }

// LinkLabel is the launchd label of the timer job that republishes the
// instance when the host moves to a different network.
func (d Deployment) LinkLabel() string { return d.ServerLabel() + ".network" }

// PlistPath is where a label's definition file belongs for this user.
func (d Deployment) PlistPath(label string) string {
	return filepath.Join(d.HomeDir, launchAgentsDir, label+".plist")
}

// ServerAgent is the always-on job: `caffeinate -s -i cks mcp --config ...`,
// restarted by launchd whenever it exits. caffeinate is the parent so the
// no-sleep assertion is released the moment the server stops, and launchd
// supervises the pair as one job.
func (d Deployment) ServerAgent() AgentSpec {
	return AgentSpec{
		Label: d.ServerLabel(),
		ProgramArguments: []string{
			caffeinatePath, "-s", "-i",
			d.Binary, "mcp", "--config", d.Config, "--name", d.Instance,
		},
		WorkingDirectory:  d.WorkDir,
		StandardOutPath:   filepath.Join(d.LogDir, d.Instance+".launchd.log"),
		StandardErrorPath: filepath.Join(d.LogDir, d.Instance+".launchd.log"),
		Environment:       d.Env,
		RunAtLoad:         true,
		KeepAlive:         true,
		ThrottleInterval:  throttleInterval,
	}
}

// WatchdogAgent is the timer job that runs `cks mcp service recover` on an
// interval. KeepAlive covers a server that exits; this covers the one that
// stays alive without serving — a wedged process, a dataset that stopped being
// serviceable — which launchd cannot see.
//
// It runs --quiet because it runs every minute forever: a log that records the
// normal case grows without bound and buries the one line that matters.
func (d Deployment) WatchdogAgent() AgentSpec {
	interval := d.WatchdogInterval
	if interval == 0 {
		interval = defaultWatchdogInterval
	}
	return AgentSpec{
		Label: d.WatchdogLabel(),
		ProgramArguments: []string{
			d.Binary, "mcp", "service", "recover",
			"--config", d.Config, "--name", d.Instance, "--quiet",
		},
		WorkingDirectory:  d.WorkDir,
		StandardOutPath:   filepath.Join(d.LogDir, d.Instance+".watchdog.log"),
		StandardErrorPath: filepath.Join(d.LogDir, d.Instance+".watchdog.log"),
		Environment:       d.Env,
		StartInterval:     interval,
	}
}

// LinkAgent is the timer job that republishes the instance when the host moves
// to a different network. KeepAlive covers a server that exits and the
// watchdog covers one that stops serving; neither sees this failure, because
// the server is bound to the wildcard address and goes on serving perfectly
// well at an address no client was told about.
//
// It is a timer job rather than a long-lived loop for the same reason the
// watchdog is: launchd is already a supervisor, so a crashed check is retried
// on the next tick instead of leaving the host unwatched. The comparison
// survives across ticks because the watcher records what it last published.
func (d Deployment) LinkAgent() AgentSpec {
	interval := d.LinkInterval
	if interval == 0 {
		interval = defaultLinkAgentInterval
	}
	return AgentSpec{
		Label: d.LinkLabel(),
		ProgramArguments: []string{
			d.Binary, "mcp", "service", "watch-network",
			"--config", d.Config, "--name", d.Instance, "--once",
		},
		WorkingDirectory:  d.WorkDir,
		StandardOutPath:   filepath.Join(d.LogDir, d.Instance+".network.log"),
		StandardErrorPath: filepath.Join(d.LogDir, d.Instance+".network.log"),
		Environment:       d.Env,
		StartInterval:     interval,
	}
}

// Render writes the agent as a launchd property list. Environment keys are
// emitted in sorted order so the same spec always produces the same bytes —
// an installer can compare against what is on disk.
func (s AgentSpec) Render() ([]byte, error) {
	if s.Label == "" {
		return nil, fmt.Errorf("service: agent spec has no label")
	}
	if len(s.ProgramArguments) == 0 {
		return nil, fmt.Errorf("service: agent %s has no program arguments", s.Label)
	}

	var b bytes.Buffer
	b.WriteString(xml.Header)
	b.WriteString("<!DOCTYPE plist PUBLIC \"-//Apple//DTD PLIST 1.0//EN\" \"http://www.apple.com/DTDs/PropertyList-1.0.dtd\">\n")
	b.WriteString("<plist version=\"1.0\">\n<dict>\n")

	writeStringKey(&b, "Label", s.Label)

	b.WriteString("\t<key>ProgramArguments</key>\n\t<array>\n")
	for _, a := range s.ProgramArguments {
		b.WriteString("\t\t<string>")
		writeEscaped(&b, a)
		b.WriteString("</string>\n")
	}
	b.WriteString("\t</array>\n")

	writeOptionalStringKey(&b, "WorkingDirectory", s.WorkingDirectory)
	writeOptionalStringKey(&b, "StandardOutPath", s.StandardOutPath)
	writeOptionalStringKey(&b, "StandardErrorPath", s.StandardErrorPath)

	if len(s.Environment) > 0 {
		b.WriteString("\t<key>EnvironmentVariables</key>\n\t<dict>\n")
		keys := make([]string, 0, len(s.Environment))
		for k := range s.Environment {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			b.WriteString("\t\t<key>")
			writeEscaped(&b, k)
			b.WriteString("</key>\n\t\t<string>")
			writeEscaped(&b, s.Environment[k])
			b.WriteString("</string>\n")
		}
		b.WriteString("\t</dict>\n")
	}

	writeBoolKey(&b, "RunAtLoad", s.RunAtLoad)
	writeBoolKey(&b, "KeepAlive", s.KeepAlive)
	if s.StartInterval > 0 {
		writeIntKey(&b, "StartInterval", int(s.StartInterval.Seconds()))
	}
	if s.ThrottleInterval > 0 {
		writeIntKey(&b, "ThrottleInterval", int(s.ThrottleInterval.Seconds()))
	}

	b.WriteString("</dict>\n</plist>\n")
	return b.Bytes(), nil
}

func writeStringKey(b *bytes.Buffer, key, value string) {
	b.WriteString("\t<key>")
	writeEscaped(b, key)
	b.WriteString("</key>\n\t<string>")
	writeEscaped(b, value)
	b.WriteString("</string>\n")
}

func writeOptionalStringKey(b *bytes.Buffer, key, value string) {
	if value == "" {
		return
	}
	writeStringKey(b, key, value)
}

func writeBoolKey(b *bytes.Buffer, key string, value bool) {
	if !value {
		return
	}
	b.WriteString("\t<key>")
	writeEscaped(b, key)
	b.WriteString("</key>\n\t<true/>\n")
}

func writeIntKey(b *bytes.Buffer, key string, value int) {
	b.WriteString("\t<key>")
	writeEscaped(b, key)
	b.WriteString("</key>\n\t<integer>")
	b.WriteString(strconv.Itoa(value))
	b.WriteString("</integer>\n")
}

// writeEscaped writes s XML-escaped. Paths and instance names come from an
// operator's flags, so they are not assumed to be markup-safe.
func writeEscaped(b *bytes.Buffer, s string) {
	_ = xml.EscapeText(b, []byte(s))
}
