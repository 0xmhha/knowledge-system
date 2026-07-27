package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/0xmhha/knowledge-system/internal/system/daemon"
	"github.com/0xmhha/knowledge-system/internal/system/netutil"
)

// runDaemon dispatches `system-mcp daemon <up|down|reload|start|stop|restart|status|list>`.
// It supervises HTTP instances of this same binary; each managed instance is a
// child `system-mcp --config <cfg> --name <name> [--http-addr <addr>]` process.
// `up`/`down` bring a whole registry (instances.yaml) of datasets up or down,
// one port per dataset, and print a LAN-reachable connection URL per instance;
// `up --wait` polls each instance's /healthz until it is serviceable before
// returning, so a caller never gets a URL that is not yet ready.
// `reload` health-gates a blue-green swap (start green on a temp port, verify
// /healthz, then restart on the real port), so a broken new dataset version
// never takes the running instance down.
func runDaemon(args []string, stdout io.Writer) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: daemon <up|down|reload|start|stop|restart|status|list> [flags]")
	}
	sub := args[0]

	fs := flag.NewFlagSet("daemon "+sub, flag.ContinueOnError)
	name := fs.String("name", "", "instance name (pidfile/log key)")
	config := fs.String("config", "", "cks config passed to the instance (start/restart)")
	addr := fs.String("http-addr", "", "override listen http_addr for the instance")
	registry := fs.String("registry", envOr("CKS_REGISTRY", "instances.yaml"), "instance registry file (up/down)")
	wait := fs.Bool("wait", false, "up: poll each instance's /healthz until it is serviceable before returning")
	waitTimeout := fs.Duration("wait-timeout", 60*time.Second, "up --wait: max time to wait per instance for readiness")
	runDir := fs.String("run-dir", envOr("CKS_RUN_DIR", "run"), "directory holding per-instance pidfiles + logs")
	ollamaURL := fs.String("ollama-url", os.Getenv("CKV_OLLAMA_ENDPOINT"), "ollama endpoint exported to the instance")
	if err := fs.Parse(args[1:]); err != nil {
		return err
	}

	self, err := os.Executable()
	if err != nil {
		return fmt.Errorf("resolve own binary: %w", err)
	}
	sup := &daemon.Supervisor{RunDir: *runDir, Binary: self}
	if *ollamaURL != "" {
		sup.Env = []string{"CKV_OLLAMA_ENDPOINT=" + *ollamaURL}
	}

	needName := func() error {
		if *name == "" {
			return fmt.Errorf("daemon %s: --name is required", sub)
		}
		return nil
	}

	switch sub {
	case "up":
		reg, err := daemon.LoadRegistry(*registry)
		if err != nil {
			return err
		}
		started, err := sup.Up(reg, nil)
		for _, st := range started {
			url := netutil.AdvertiseHostPort(st.Addr)
			state := "running"
			if *wait && st.Running {
				if daemon.WaitReady(st.Addr, *waitTimeout, 500*time.Millisecond) {
					state = "ready"
				} else {
					state = fmt.Sprintf("running but NOT serviceable after %s", *waitTimeout)
				}
			}
			fmt.Fprintf(stdout, "[%s] %s (pid %d) — http://%s/mcp\n", st.Name, state, st.PID, url)
		}
		return err
	case "down":
		reg, err := daemon.LoadRegistry(*registry)
		if err != nil {
			return err
		}
		if err := sup.Down(reg); err != nil {
			return err
		}
		fmt.Fprintf(stdout, "stopped %d instance(s) from %s\n", len(reg.Instances), *registry)
		return nil
	case "reload":
		reg, err := daemon.LoadRegistry(*registry)
		if err != nil {
			return err
		}
		return runReload(sup, reg, *name, *runDir, stdout)
	case "start", "restart":
		if err := needName(); err != nil {
			return err
		}
		if *config == "" {
			return fmt.Errorf("daemon %s: --config is required", sub)
		}
		fn := sup.Start
		if sub == "restart" {
			fn = sup.Restart
		}
		inst, err := fn(*name, *config, *addr)
		if err != nil {
			return err
		}
		fmt.Fprintf(stdout, "[%s] running (pid %d)\n", inst.Name, inst.PID)
	case "stop":
		if err := needName(); err != nil {
			return err
		}
		if err := sup.Stop(*name); err != nil {
			return err
		}
		fmt.Fprintf(stdout, "[%s] stopped\n", *name)
	case "status":
		if *name != "" {
			printInstance(stdout, sup.Status(*name))
			return nil
		}
		fallthrough
	case "list":
		list, err := sup.List()
		if err != nil {
			return err
		}
		if len(list) == 0 {
			fmt.Fprintln(stdout, "(no managed instances)")
			return nil
		}
		for _, inst := range list {
			printInstance(stdout, inst)
		}
	default:
		return fmt.Errorf("daemon: unknown subcommand %q (want start|stop|restart|status|list)", sub)
	}
	return nil
}

func printInstance(w io.Writer, i daemon.Instance) {
	state := "stopped"
	if i.Running {
		state = fmt.Sprintf("running (pid %d)", i.PID)
		if i.Addr != "" {
			state += fmt.Sprintf(" — http://%s/mcp", netutil.AdvertiseHostPort(i.Addr))
		}
	}
	fmt.Fprintf(w, "[%s] %s\n", i.Name, state)
}

// runReload health-gated blue-green reloads registry instances (all, or the one
// named by nameFilter), reusing each running instance's recorded address and its
// config (explicit, or the one generated under runDir). It reports per instance
// and returns an error if any reload did not come up healthy.
func runReload(sup *daemon.Supervisor, reg *daemon.Registry, nameFilter, runDir string, stdout io.Writer) error {
	var firstErr error
	matched := false
	for _, e := range reg.Instances {
		if nameFilter != "" && e.Name != nameFilter {
			continue
		}
		matched = true
		cur := sup.Status(e.Name)
		if !cur.Running || cur.Addr == "" {
			fmt.Fprintf(stdout, "[%s] not running — start it with `daemon up`\n", e.Name)
			if firstErr == nil {
				firstErr = fmt.Errorf("daemon reload: %s is not running", e.Name)
			}
			continue
		}
		cfg := e.Config
		if cfg == "" {
			cfg = filepath.Join(runDir, e.Name+".yaml")
		}
		inst, err := sup.Reload(e.Name, cfg, cur.Addr, daemon.ReloadOptions{})
		if err != nil {
			fmt.Fprintf(stdout, "[%s] %v\n", e.Name, err)
			if firstErr == nil {
				firstErr = err
			}
			continue
		}
		fmt.Fprintf(stdout, "[%s] reloaded (pid %d) — http://%s/mcp\n",
			inst.Name, inst.PID, netutil.AdvertiseHostPort(cur.Addr))
	}
	if nameFilter != "" && !matched {
		return fmt.Errorf("daemon reload: no instance named %q in the registry", nameFilter)
	}
	return firstErr
}

func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
