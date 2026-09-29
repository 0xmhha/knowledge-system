package semanticcli

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/0xmhha/knowledge-system/internal/system/semantic"
	"github.com/spf13/cobra"
)

func newTestCmd() *cobra.Command {
	var project, repo, graph, vector, storePath, criterion, output string
	cmd := &cobra.Command{
		Use:   "test --project-id ID --repo DIR --graph DIR --vector DIR --store DB --criterion-id ID --out FILE -- COMMAND [ARGS...]",
		Short: "Execute a command against a reviewed trace and record its result",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, argv []string) error {
			store, err := semantic.OpenStore(storePath)
			if err != nil {
				return err
			}
			defer store.Close()
			active, err := store.CurrentAligned(cmd.Context(), project, repo, graph, vector)
			if err != nil {
				return err
			}
			report, err := active.ExecuteLinkedTest(cmd.Context(), repo, criterion, argv)
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
			if !report.CommandPassed || !report.SnapshotConsistent {
				return fmt.Errorf("test run failed or source changed; report: %s", output)
			}
			return nil
		},
	}
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
		{"out", &output, "new JSON report path; existing files are never overwritten"},
	} {
		cmd.Flags().StringVar(flag.target, flag.name, "", flag.usage)
		_ = cmd.MarkFlagRequired(flag.name)
	}
	return cmd
}
