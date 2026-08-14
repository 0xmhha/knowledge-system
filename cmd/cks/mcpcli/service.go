package mcpcli

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/spf13/cobra"

	"github.com/0xmhha/knowledge-system/internal/system/daemon"
	cksmcp "github.com/0xmhha/knowledge-system/internal/system/mcp"
	"github.com/0xmhha/knowledge-system/internal/system/service"
)

// installReadyTimeout bounds how long `service install` waits for the freshly
// bootstrapped instance to serve before reporting what it sees.
const installReadyTimeout = 120 * time.Second

// ollamaEndpointEnv carries the embedding daemon's address to the instance,
// and is where recovery reads it back from.
const ollamaEndpointEnv = "CKV_OLLAMA_ENDPOINT"

// defaultOllamaEndpoint matches the engine's own default, so recovery probes
// the daemon the server would have used when the config named none.
const defaultOllamaEndpoint = "http://localhost:11434"

// embedderDependency describes the embedding daemon to the recovery ladder.
//
// It is composed here rather than inside the service package: supervision is
// generic, and which daemon an instance depends on is a property of this
// deployment. The package takes probe/start seams and knows nothing about
// Ollama.
//
// Start prefers the app bundle because that is what the documented install
// produces — the brew formula ships without llama-server on Apple Silicon, so
// the cask is what an operator has — and falls back to the CLI.
func embedderDependency(endpoint string) *service.Dependency {
	if endpoint == "" {
		endpoint = defaultOllamaEndpoint
	}
	return &service.Dependency{
		Name: "the embedding daemon",
		Probe: func(ctx context.Context) bool {
			req, err := http.NewRequestWithContext(ctx, http.MethodGet,
				strings.TrimSuffix(endpoint, "/")+"/api/version", nil)
			if err != nil {
				return false
			}
			resp, err := (&http.Client{Timeout: 5 * time.Second}).Do(req)
			if err != nil {
				return false
			}
			defer func() { _ = resp.Body.Close() }()
			return resp.StatusCode == http.StatusOK
		},
		Start: func(ctx context.Context) error {
			if _, err := os.Stat("/Applications/Ollama.app"); err == nil {
				return exec.CommandContext(ctx, "/usr/bin/open", "-ga", "Ollama").Run()
			}
			bin, err := exec.LookPath("ollama")
			if err != nil {
				return fmt.Errorf("neither /Applications/Ollama.app nor an ollama binary is present: %w", err)
			}
			// Detached: the daemon has to outlive this recovery run.
			cmd := exec.Command(bin, "serve")
			cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
			return cmd.Start()
		},
	}
}

// lsofPath locates the process holding a port during a takeover check.
const lsofPath = "/usr/sbin/lsof"

// serviceFlags is the flag set shared by the service verbs. Config and Name
// resolve the instance; the rest are per-verb and ignored where they do not
// apply.
type serviceFlags struct {
	config           string
	name             string
	runDir           string
	watchdogInterval time.Duration
	linkInterval     time.Duration
	timeout          time.Duration
	force            bool
	once             bool
	disconnectGrace  time.Duration
	quiet            bool
	asJSON           bool
	takeover         bool
}

// newServiceCmd builds `cks mcp service <install|uninstall|status|recover>`:
// the launchd deployment of this server on macOS, and the recovery routine its
// watchdog and a remote operator both call.
func newServiceCmd() *cobra.Command {
	f := &serviceFlags{}
	cmd := &cobra.Command{
		Use:   "service",
		Short: "Run this instance as a launchd agent that survives sleep, crashes and logout",
		Long: "Install the server as a macOS launchd user agent (wrapped in caffeinate so it\n" +
			"asserts no-sleep), plus a watchdog that probes /healthz on a timer. `recover`\n" +
			"is the routine the watchdog runs and a remote operator triggers over SSH.",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error { return cmd.Help() },
	}
	pf := cmd.PersistentFlags()
	pf.StringVar(&f.config, "config", "", "path to cks.yaml — the instance this verb acts on")
	pf.StringVar(&f.name, "name", "", "instance name (defaults to the config's name)")
	pf.StringVar(&f.runDir, "run-dir", "", "directory for launchd stdout/stderr logs (default <config dir>/run)")

	install := &cobra.Command{
		Use:   "install",
		Short: "Write and load the launchd agents for this instance",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runServiceInstall(cmd.Context(), f, cmd.OutOrStdout())
		},
	}
	install.Flags().DurationVar(&f.watchdogInterval, "watchdog-interval", time.Minute,
		"how often the watchdog probes /healthz")
	install.Flags().BoolVar(&f.takeover, "takeover", false,
		"stop a foreign process already holding the port instead of refusing to install")

	uninstall := &cobra.Command{
		Use:   "uninstall",
		Short: "Unload the launchd agents and remove their definitions",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runServiceUninstall(cmd.Context(), f, cmd.OutOrStdout())
		},
	}

	status := &cobra.Command{
		Use:   "status",
		Short: "Report agent load state, serving state and host sleep policy",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runServiceStatus(cmd.Context(), f, cmd.OutOrStdout())
		},
	}

	recoverCmd := &cobra.Command{
		Use:   "recover",
		Short: "Restart the instance if it is not serving, then confirm it came back",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runServiceRecover(cmd.Context(), f, cmd.OutOrStdout())
		},
	}
	recoverCmd.Flags().BoolVar(&f.force, "force", false, "restart even when the instance is serving")
	recoverCmd.Flags().DurationVar(&f.timeout, "timeout", 90*time.Second,
		"how long to wait for the restarted instance to serve")
	recoverCmd.Flags().BoolVar(&f.quiet, "quiet", false,
		"print nothing when the instance is already serving (for the watchdog's log)")
	recoverCmd.Flags().BoolVar(&f.asJSON, "json", false, "print the recovery report as JSON")

	watchNet := &cobra.Command{
		Use:   "watch-network",
		Short: "Republish the instance when the host moves to a different network",
		Long: "Restart the instance and print the URL clients should use when the host's\n" +
			"externally reachable address changes — a different AP, a dock in or out, a\n" +
			"VPN up or down. The listener survives such a move on its own (it is bound to\n" +
			"the wildcard address), so nothing looks wrong; what breaks is every client\n" +
			"still holding the previous URL.\n\n" +
			"Losing the network is NOT a move. Wireless drops and returns constantly, and\n" +
			"almost always at the same address, so an outage never restarts anything — the\n" +
			"published address is kept and compared against on return. Only a genuinely\n" +
			"different address republishes.\n\n" +
			"Runs until interrupted. --once performs a single check and exits, which is\n" +
			"how the installed launchd timer job invokes it.",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runServiceWatchNetwork(cmd.Context(), f, cmd.OutOrStdout())
		},
	}
	watchNet.Flags().DurationVar(&f.linkInterval, "interval", 20*time.Second,
		"how often to re-read connectivity when watching")
	watchNet.Flags().DurationVar(&f.disconnectGrace, "disconnect-grace", 2*time.Minute,
		"how long an outage must last before it is logged; never affects restarts")
	watchNet.Flags().BoolVar(&f.once, "once", false, "check once and exit instead of watching")
	watchNet.Flags().DurationVar(&f.timeout, "timeout", 90*time.Second,
		"how long to wait for the restarted instance to serve")
	watchNet.Flags().BoolVar(&f.asJSON, "json", false, "print each change as JSON")

	cmd.AddCommand(install, uninstall, status, recoverCmd, watchNet)
	return cmd
}

// deployment resolves the flags into the launchd identity of one instance,
// alongside the address its health is probed at. Everything is made absolute
// here: a launchd job inherits no working directory or PATH.
func (f *serviceFlags) deployment() (service.Deployment, string, error) {
	if f.config == "" {
		return service.Deployment{}, "", fmt.Errorf("--config is required: a launchd job cannot fall back to defaults it cannot see")
	}
	cfg, err := loadConfig(f.config)
	if err != nil {
		return service.Deployment{}, "", fmt.Errorf("load config: %w", err)
	}
	instance := f.name
	if instance == "" {
		instance = cfg.Name
	}
	if instance == "" {
		instance = cksmcp.DefaultInstanceName()
	}
	addr := cfg.Listen.HTTPAddr
	if addr == "" {
		return service.Deployment{}, "", fmt.Errorf("config %q has no listen.http_addr — this deployment serves over HTTP", f.config)
	}
	configPath, err := filepath.Abs(f.config)
	if err != nil {
		return service.Deployment{}, "", fmt.Errorf("resolve config path: %w", err)
	}
	binary, err := os.Executable()
	if err != nil {
		return service.Deployment{}, "", fmt.Errorf("resolve own binary: %w", err)
	}
	if resolved, err := filepath.EvalSymlinks(binary); err == nil {
		binary = resolved
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return service.Deployment{}, "", fmt.Errorf("resolve home directory: %w", err)
	}

	workDir := filepath.Dir(configPath)
	logDir := f.runDir
	if logDir == "" {
		logDir = filepath.Join(workDir, "run")
	}
	if logDir, err = filepath.Abs(logDir); err != nil {
		return service.Deployment{}, "", fmt.Errorf("resolve run dir: %w", err)
	}

	env := map[string]string{}
	if v := os.Getenv(ollamaEndpointEnv); v != "" {
		env[ollamaEndpointEnv] = v
	}

	return service.Deployment{
		Instance:         instance,
		Binary:           binary,
		Config:           configPath,
		WorkDir:          workDir,
		LogDir:           logDir,
		HomeDir:          home,
		Env:              env,
		LabelPrefix:      cfg.Service.LabelPrefix,
		WatchdogInterval: f.watchdogInterval,
		LinkInterval:     f.linkInterval,
	}, addr, nil
}

// controller drives launchctl in the invoking user's GUI domain.
func controller() service.Controller {
	return service.Controller{Runner: service.ExecRunner{}, UID: os.Getuid()}
}

func runServiceInstall(ctx context.Context, f *serviceFlags, out io.Writer) error {
	dep, addr, err := f.deployment()
	if err != nil {
		return err
	}
	ctl := controller()

	if err := preflightPort(ctx, ctl, dep, addr, f.takeover, out); err != nil {
		return err
	}
	if err := os.MkdirAll(dep.LogDir, 0o755); err != nil {
		return fmt.Errorf("prepare log dir: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(dep.PlistPath(dep.ServerLabel())), 0o755); err != nil {
		return fmt.Errorf("prepare LaunchAgents dir: %w", err)
	}

	agents := []service.AgentSpec{dep.ServerAgent(), dep.WatchdogAgent(), dep.LinkAgent()}
	for _, agent := range agents {
		body, err := agent.Render()
		if err != nil {
			return err
		}
		path := dep.PlistPath(agent.Label)
		if err := os.WriteFile(path, body, 0o644); err != nil {
			return fmt.Errorf("write %s: %w", path, err)
		}
		// Replace, not bootstrap: install has to work over an existing install,
		// and it must not leave the label unloaded if it does not.
		if err := ctl.Replace(ctx, agent.Label, path); err != nil {
			return err
		}
		fmt.Fprintf(out, "loaded %s (%s)\n", agent.Label, path)
	}

	if daemon.WaitReady(addr, installReadyTimeout, 2*time.Second) {
		fmt.Fprintf(out, "serving at %s\n", addr)
	} else {
		fmt.Fprintf(out, "WARNING: not serving at %s after %s — see %s\n",
			addr, installReadyTimeout, filepath.Join(dep.LogDir, dep.Instance+".launchd.log"))
	}
	reportPower(ctx, out)
	return nil
}

// preflightPort refuses to install over a foreign process already holding the
// port: two servers on one port means launchd restart-loops against a bind
// error. A process belonging to our own (already installed) label is fine —
// bootout will replace it.
func preflightPort(ctx context.Context, ctl service.Controller, dep service.Deployment, addr string, takeover bool, out io.Writer) error {
	if !daemon.Serviceable(addr) {
		return nil
	}
	loaded, err := ctl.Loaded(ctx, dep.ServerLabel())
	if err != nil {
		return err
	}
	if loaded {
		return nil // our own agent — reinstalling replaces it
	}
	pid := listenerPID(ctx, addr)
	if !takeover {
		return fmt.Errorf("%s is already served by a process this agent does not own (pid %d) — "+
			"stop it, or re-run with --takeover", addr, pid)
	}
	if pid <= 0 {
		return fmt.Errorf("--takeover: could not identify the process serving %s", addr)
	}
	if err := syscall.Kill(pid, syscall.SIGTERM); err != nil {
		return fmt.Errorf("--takeover: stop pid %d: %w", pid, err)
	}
	fmt.Fprintf(out, "stopped the unmanaged process on %s (pid %d)\n", addr, pid)
	// Give the port time to be released before launchd binds it.
	for i := 0; i < 20 && daemon.Serviceable(addr); i++ {
		time.Sleep(250 * time.Millisecond)
	}
	return nil
}

// listenerPID returns the pid listening on addr's port, or 0 when it cannot be
// determined. Best-effort: it only refines an error message or a takeover.
func listenerPID(ctx context.Context, addr string) int {
	_, port, err := net.SplitHostPort(addr)
	if err != nil {
		return 0
	}
	out, err := service.ExecRunner{}.Run(ctx, lsofPath, "-ti", "tcp:"+port, "-sTCP:LISTEN")
	if err != nil {
		return 0
	}
	first := strings.Fields(out)
	if len(first) == 0 {
		return 0
	}
	pid, err := strconv.Atoi(first[0])
	if err != nil {
		return 0
	}
	return pid
}

func runServiceUninstall(ctx context.Context, f *serviceFlags, out io.Writer) error {
	dep, _, err := f.deployment()
	if err != nil {
		return err
	}
	ctl := controller()
	for _, label := range []string{dep.LinkLabel(), dep.WatchdogLabel(), dep.ServerLabel()} {
		if err := ctl.Bootout(ctx, label); err != nil {
			return err
		}
		path := dep.PlistPath(label)
		if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("remove %s: %w", path, err)
		}
		fmt.Fprintf(out, "unloaded %s\n", label)
	}
	return nil
}

func runServiceStatus(ctx context.Context, f *serviceFlags, out io.Writer) error {
	dep, addr, err := f.deployment()
	if err != nil {
		return err
	}
	ctl := controller()
	for _, label := range []string{dep.ServerLabel(), dep.WatchdogLabel(), dep.LinkLabel()} {
		loaded, err := ctl.Loaded(ctx, label)
		if err != nil {
			return err
		}
		state := "not loaded"
		if loaded {
			state = "loaded"
		}
		fmt.Fprintf(out, "%-48s %s\n", label, state)
	}
	serving := "NOT serving"
	if daemon.Serviceable(addr) {
		serving = "serving"
	}
	fmt.Fprintf(out, "%-48s %s\n", addr, serving)
	reportPower(ctx, out)
	return nil
}

func runServiceRecover(ctx context.Context, f *serviceFlags, out io.Writer) error {
	dep, addr, err := f.deployment()
	if err != nil {
		return err
	}
	ctl := controller()
	label := dep.ServerLabel()

	loaded, err := ctl.Loaded(ctx, label)
	if err != nil {
		return err
	}
	if !loaded {
		return fmt.Errorf("%s is not loaded — install it first: cks mcp service install --config %s", label, dep.Config)
	}

	rec := service.Recoverer{
		Probe:      daemon.Serviceable,
		Restart:    func(ctx context.Context) error { return ctl.Kickstart(ctx, label) },
		Timeout:    f.timeout,
		Dependency: embedderDependency(dep.Env[ollamaEndpointEnv]),
		StatePath:  service.RecoverStatePath(dep.LogDir),
	}
	report, recErr := rec.Recover(ctx, dep.Instance, addr, f.force)
	if f.quiet && report.Outcome == service.OutcomeHealthy {
		return nil
	}
	if f.asJSON {
		enc := json.NewEncoder(out)
		enc.SetIndent("", "  ")
		if err := enc.Encode(report); err != nil {
			return err
		}
		return recErr
	}
	fmt.Fprintf(out, "%s [%s] %s — %s\n",
		time.Now().Format(time.RFC3339), report.Instance, report.Outcome, report.Detail)
	for _, a := range report.Actions {
		fmt.Fprintf(out, "  - %s\n", a)
	}
	return recErr
}

// runServiceWatchNetwork republishes the instance whenever the host's external
// address moves. The restart is forced: the server is still serving on its
// wildcard bind, so a health probe would report "fine" and skip the restart the
// move actually needs. Losing the network is not a move and restarts nothing.
func runServiceWatchNetwork(ctx context.Context, f *serviceFlags, out io.Writer) error {
	dep, addr, err := f.deployment()
	if err != nil {
		return err
	}
	_, port, err := net.SplitHostPort(addr)
	if err != nil {
		return fmt.Errorf("listen address %q has no port: %w", addr, err)
	}
	ctl := controller()
	label := dep.ServerLabel()

	watcher := service.LinkWatcher{
		Instance: dep.Instance,
		Port:     port,
		Addrs:    service.HostAddrs,
		Recoverer: service.Recoverer{
			Probe:   daemon.Serviceable,
			Restart: func(ctx context.Context) error { return ctl.Kickstart(ctx, label) },
			Timeout: f.timeout,
		},
		StatePath:       service.LinkStatePath(dep.LogDir),
		Interval:        f.linkInterval,
		DisconnectGrace: f.disconnectGrace,
	}

	show := func(c service.LinkChange) {
		if f.asJSON {
			enc := json.NewEncoder(out)
			enc.SetIndent("", "  ")
			_ = enc.Encode(c)
			return
		}
		printLinkChange(out, dep.Instance, c)
	}

	// --once returns the failure so the exit code carries it and cobra prints
	// it once. The watch loop cannot return, so it prints its own.
	if f.once {
		change, err := watcher.Check(ctx)
		if change != nil {
			show(*change)
		}
		return err
	}
	return watcher.Run(ctx, func(c service.LinkChange, err error) {
		show(c)
		if err != nil {
			fmt.Fprintf(out, "  ! %v\n", err)
		}
	})
}

// printLinkChange renders one observation. The URL is the point of the whole
// command, so it gets its own line rather than being folded into prose.
func printLinkChange(out io.Writer, instance string, c service.LinkChange) {
	fmt.Fprintf(out, "%s [%s] %s", time.Now().Format(time.RFC3339), instance, c.State)
	switch c.State {
	case service.LinkDisconnected:
		fmt.Fprintf(out, " — off-network for %s; still publishing %s\n", c.DownFor, c.Published)
	case service.LinkMoved:
		fmt.Fprintf(out, " — %s -> %s\n", c.Published, c.Current)
	default:
		if c.DownFor != "" {
			fmt.Fprintf(out, " — back after %s on the same address\n", c.DownFor)
		} else {
			fmt.Fprintf(out, " — publishing %s\n", c.Current)
		}
	}
	if c.Detail != "" {
		fmt.Fprintf(out, "  - %s\n", c.Detail)
	}
	if c.URL != "" {
		fmt.Fprintf(out, "  MCP URL: %s\n", c.URL)
	}
}

// reportPower prints the host sleep policy verdict. A violation is a warning
// rather than a failure: fixing it needs root, which this binary deliberately
// does not ask for.
func reportPower(ctx context.Context, out io.Writer) {
	profile, err := service.ReadPowerProfile(ctx, service.ExecRunner{})
	if err != nil {
		fmt.Fprintf(out, "power policy: could not be read (%v)\n", err)
		return
	}
	violations := service.Violations(profile)
	if len(violations) == 0 {
		fmt.Fprintln(out, "power policy: this host will not sleep")
		return
	}
	fmt.Fprintln(out, "power policy: the host may go unreachable —")
	for _, v := range violations {
		fmt.Fprintf(out, "  - %s is %d, needs %d: %s\n", v.Setting, v.Got, v.Want, v.Why)
	}
	fmt.Fprintf(out, "  fix with: %s\n", service.RemediationCommand(violations))
}
