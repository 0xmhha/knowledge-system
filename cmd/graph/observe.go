package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/0xmhha/knowledge-system/internal/graph/verify/observe"
)

// observeLineNote says what the recorded call line is: the line the Go
// runtime reports for the caller's frame. A deferred call runs when the
// deferring function returns, so its line is where it ran, not where it
// was deferred.
const observeLineNote = "The call line is the line the Go runtime reports for the calling frame. For a call made by a\n" +
	"defer statement that is the line where the deferred call ran: usually the deferring function's closing\n" +
	"line (or the return or panic that ended it), not the line of the defer statement."

// observeRuntimeChannelNote says how channels the program does not make
// show in the comparison.
const observeRuntimeChannelNote = "Channel operations on channels the runtime or the standard library makes (a timer's C, the\n" +
	"channel of time.After) have no channel node in the graph, which holds the channels of the built code\n" +
	"only. They are counted under channel_ops and listed in its missing_samples: read such a sample as an\n" +
	"operation outside the build, not as a link the graph lost."

func newObserveCmd() *cobra.Command {
	var src, scopePath, out string
	cmd := &cobra.Command{
		Use:   "observe",
		Short: "Copy a Go source tree with calls and channel operations instrumented, for ckg verify --observed",
		Long: "Copies --src to --out and instruments the Go files of --scope: every function body records its caller\n" +
			"and call line, every channel operation on a named channel records the channel. Line numbers stay the\n" +
			"same. Run tests in --out with CKGOBS_DIR set to a directory, then compare with\n" +
			"ckg verify --observed <that directory>. The source tree is not changed.\n\n" +
			observeLineNote + "\n\n" + observeRuntimeChannelNote,
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			if out == "" || scopePath == "" {
				return fmt.Errorf("--out and --scope are required")
			}
			if abs, _ := filepath.Abs(out); abs == mustAbs(src) {
				return fmt.Errorf("--out must differ from --src")
			}
			s, _, _, err := loadScope(scopePath)
			if err != nil {
				return err
			}
			var files []string
			for _, f := range s.Files {
				if f.Language == "go" {
					files = append(files, f.Path)
				}
			}
			mod, err := modulePath(filepath.Join(src, "go.mod"))
			if err != nil {
				return err
			}
			st, err := observe.Instrument(src, out, mod, files)
			if err != nil {
				return err
			}
			enc := json.NewEncoder(cmd.OutOrStdout())
			enc.SetIndent("", "  ")
			return enc.Encode(st)
		},
	}
	cmd.Flags().StringVar(&src, "src", ".", "source root")
	cmd.Flags().StringVar(&scopePath, "scope", "", "build scope JSON; its Go files are instrumented")
	cmd.Flags().StringVar(&out, "out", "", "directory for the instrumented copy")
	return cmd
}

func mustAbs(p string) string {
	a, _ := filepath.Abs(p)
	return a
}

// modulePath reads the module line of a go.mod file.
func modulePath(goMod string) (string, error) {
	f, err := os.Open(goMod)
	if err != nil {
		return "", err
	}
	defer func() { _ = f.Close() }()
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		if l := strings.TrimSpace(sc.Text()); strings.HasPrefix(l, "module ") {
			return strings.Trim(strings.TrimSpace(strings.TrimPrefix(l, "module ")), `"`), nil
		}
	}
	return "", fmt.Errorf("no module line in %s", goMod)
}
