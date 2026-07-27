package main

import (
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/0xmhha/knowledge-system/internal/system/daemon"
)

// runDaemon dispatches `system-mcp daemon <start|stop|restart|status|list>`.
// It supervises HTTP instances of this same binary; each managed instance is a
// child `system-mcp --config <cfg> --name <name> [--http-addr <addr>]` process.
// Instance registration into an MCP client, LAN-IP autodetect, and --advertise
// stay in shell/plugin glue and are intentionally not ported here.
func runDaemon(args []string, stdout io.Writer) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: daemon <start|stop|restart|status|list> [flags]")
	}
	sub := args[0]

	fs := flag.NewFlagSet("daemon "+sub, flag.ContinueOnError)
	name := fs.String("name", "", "instance name (pidfile/log key)")
	config := fs.String("config", "", "cks config passed to the instance (start/restart)")
	addr := fs.String("http-addr", "", "override listen http_addr for the instance")
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
	}
	fmt.Fprintf(w, "[%s] %s\n", i.Name, state)
}

func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
