package semanticcli

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/0xmhha/knowledge-system/internal/system/semantic"
	"github.com/spf13/cobra"
)

func newTestCmd() *cobra.Command {
	var project, repo, graph, vector, storePath, criterion, testCanonicalID, output, datasetID, versionDir string
	var goTestExact bool
	cmd := &cobra.Command{
		Use:   "test --project-id ID --repo DIR --graph DIR --vector DIR --store DB --criterion-id ID --out FILE [--go-test-exact | -- COMMAND [ARGS...]]",
		Short: "Execute a command against a reviewed trace and record its result",
		Args:  cobra.ArbitraryArgs,
		RunE: func(cmd *cobra.Command, argv []string) error {
			if goTestExact && len(argv) != 0 {
				return fmt.Errorf("--go-test-exact derives its command from the reviewed Go test; do not pass COMMAND")
			}
			if !goTestExact && len(argv) == 0 {
				return fmt.Errorf("a command after -- or --go-test-exact is required")
			}
			store, err := semantic.OpenStore(storePath)
			if err != nil {
				return err
			}
			defer store.Close()
			var active semantic.ActiveProjection
			if datasetID != "" || versionDir != "" {
				if datasetID == "" || versionDir == "" {
					return fmt.Errorf("--dataset-id and --version-dir must be used together")
				}
				active, err = store.LoadAlignedRetained(cmd.Context(), project, datasetID, repo, graph, vector, versionDir)
			} else {
				active, err = store.CurrentAligned(cmd.Context(), project, repo, graph, vector)
			}
			if err != nil {
				return err
			}
			var report semantic.TestRun
			if goTestExact {
				report, err = active.ExecuteLinkedGoTestFor(cmd.Context(), repo, criterion, testCanonicalID)
			} else {
				report, err = active.ExecuteLinkedTestFor(cmd.Context(), repo, criterion, testCanonicalID, argv)
			}
			if err != nil {
				return err
			}
			data, err := json.MarshalIndent(report, "", "  ")
			if err != nil {
				return err
			}
			file, err := os.OpenFile(output, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
			if err != nil {
				return fmt.Errorf("create test report: %w", err)
			}
			if _, err = file.Write(append(data, '\n')); err != nil {
				file.Close()
				return err
			}
			if err = file.Close(); err != nil {
				return err
			}
			if _, err = fmt.Fprintln(cmd.OutOrStdout(), output); err != nil {
				return err
			}
			if !report.CommandPassed || !report.SnapshotConsistent || goTestExact && !report.TestPassed {
				return fmt.Errorf("test run failed or source changed; report: %s", output)
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&goTestExact, "go-test-exact", false, "derive one Go test from the reviewed CKG anchor and require its go test -json run/pass events")
	cmd.Flags().StringVar(&datasetID, "dataset-id", "", "reviewed candidate dataset ID (requires --version-dir)")
	cmd.Flags().StringVar(&versionDir, "version-dir", "", "held candidate with retained source archive")
	for _, flag := range []struct {
		name   string
		target *string
		usage  string
	}{
		{"project-id", &project, "active project identifier"},
		{"repo", &repo, "Git repository containing the cited commit"},
		{"graph", &graph, "CKG data directory"},
		{"vector", &vector, "CKV data directory"},
		{"store", &storePath, "CKS semantic SQLite path"},
		{"criterion-id", &criterion, "reviewed acceptance criterion ID"},
		{"test-canonical-id", &testCanonicalID, "reviewed CKG test symbol; required when a criterion links multiple tests"},
		{"out", &output, "new JSON report path; existing files are never overwritten"},
	} {
		cmd.Flags().StringVar(flag.target, flag.name, "", flag.usage)
		if flag.name != "test-canonical-id" {
			_ = cmd.MarkFlagRequired(flag.name)
		}
	}
	return cmd
}
