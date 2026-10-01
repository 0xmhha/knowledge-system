package patchcli

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/0xmhha/knowledge-system/internal/system/patch"
)

func NewCmd() *cobra.Command {
	var dataset, patchID string
	root := &cobra.Command{Use: "patch", Short: "Record external patch identity, test execution and criterion review"}
	root.PersistentFlags().StringVar(&dataset, "dataset", "", "versioned CKS dataset root")
	root.PersistentFlags().StringVar(&patchID, "patch-id", "", "stable external patch ID")
	require := func() error {
		if dataset == "" || patchID == "" {
			return fmt.Errorf("--dataset and --patch-id are required")
		}
		return nil
	}
	var version string
	register := &cobra.Command{Use: "register", Short: "Derive changed files between an active base and held candidate", Args: cobra.NoArgs,
		RunE: func(c *cobra.Command, _ []string) error {
			if err := require(); err != nil {
				return err
			}
			if version == "" {
				return fmt.Errorf("--version is required")
			}
			a, err := patch.Register(dataset, version, patchID)
			if err != nil {
				return err
			}
			return json.NewEncoder(c.OutOrStdout()).Encode(a)
		}}
	register.Flags().StringVar(&version, "version", "", "held result version")
	var storePath, reportPath string
	record := &cobra.Command{Use: "record-run", Short: "Attach a passing, snapshot-bound semantic test report", Args: cobra.NoArgs,
		RunE: func(c *cobra.Command, _ []string) error {
			if err := require(); err != nil {
				return err
			}
			if storePath == "" || reportPath == "" {
				return fmt.Errorf("--semantic-store and --report are required")
			}
			run, err := patch.RecordRun(c.Context(), dataset, patchID, storePath, reportPath)
			if err != nil {
				return err
			}
			return json.NewEncoder(c.OutOrStdout()).Encode(run)
		}}
	record.Flags().StringVar(&storePath, "semantic-store", "", "semantic projection SQLite path")
	record.Flags().StringVar(&reportPath, "report", "", "existing cks semantic test report JSON")
	var reviewStore, decisionID, criterionID, reviewer, outcome, reason string
	decide := &cobra.Command{Use: "decide", Short: "Record an immutable human judgment for one acceptance criterion", Args: cobra.NoArgs,
		RunE: func(c *cobra.Command, _ []string) error {
			if err := require(); err != nil {
				return err
			}
			if reviewStore == "" {
				return fmt.Errorf("--semantic-store is required")
			}
			d, err := patch.Decide(c.Context(), dataset, patchID, reviewStore, decisionID, criterionID, reviewer, outcome, reason)
			if err != nil {
				return err
			}
			return json.NewEncoder(c.OutOrStdout()).Encode(d)
		}}
	decide.Flags().StringVar(&reviewStore, "semantic-store", "", "candidate semantic projection SQLite path")
	decide.Flags().StringVar(&decisionID, "decision-id", "", "immutable decision ID")
	decide.Flags().StringVar(&criterionID, "criterion-id", "", "acceptance criterion ID")
	decide.Flags().StringVar(&reviewer, "reviewer", "", "recorded reviewer identity")
	decide.Flags().StringVar(&outcome, "outcome", "", "approved or rejected")
	decide.Flags().StringVar(&reason, "reason", "", "human reason grounded in criterion evidence")
	var promoteStore string
	promote := &cobra.Command{Use: "promote", Short: "Promote only after all linked criteria have passing runs and human approvals", Args: cobra.NoArgs,
		RunE: func(c *cobra.Command, _ []string) error {
			if err := require(); err != nil {
				return err
			}
			if promoteStore == "" {
				return fmt.Errorf("--semantic-store is required")
			}
			previous, err := patch.Promote(c.Context(), dataset, patchID, promoteStore)
			if err != nil {
				return err
			}
			return json.NewEncoder(c.OutOrStdout()).Encode(map[string]string{"status": "promoted", "previous_version": previous, "patch_id": patchID})
		}}
	promote.Flags().StringVar(&promoteStore, "semantic-store", "", "candidate semantic projection SQLite path")
	status := &cobra.Command{Use: "status", Short: "Inspect an immutable patch attempt without promoting it", Args: cobra.NoArgs,
		RunE: func(c *cobra.Command, _ []string) error {
			if err := require(); err != nil {
				return err
			}
			a, err := patch.Load(dataset, patchID)
			if err != nil {
				return err
			}
			return json.NewEncoder(c.OutOrStdout()).Encode(a)
		}}
	root.AddCommand(register, record, decide, promote, status)
	return root
}
