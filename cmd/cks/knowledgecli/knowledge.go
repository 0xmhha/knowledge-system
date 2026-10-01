package knowledgecli

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/0xmhha/knowledge-system/internal/system/knowledgepack"
	"github.com/spf13/cobra"
)

func NewCmd() *cobra.Command {
	var root string
	cmd := &cobra.Command{Use: "knowledge", Short: "Validate and lock local domain knowledge packs"}
	cmd.PersistentFlags().StringVar(&root, "project-root", "", "project root containing .cks/knowledge")
	requireRoot := func() error {
		if root == "" {
			return fmt.Errorf("--project-root is required")
		}
		return nil
	}
	var projectID string
	initCmd := &cobra.Command{Use: "init", Short: "Create a core-only local knowledge template without overwriting files", Args: cobra.NoArgs,
		RunE: func(c *cobra.Command, _ []string) error {
			if err := requireRoot(); err != nil {
				return err
			}
			if projectID == "" {
				return fmt.Errorf("--project-id is required")
			}
			dir := filepath.Join(root, ".cks", "knowledge")
			if err := os.MkdirAll(dir, 0o700); err != nil {
				return err
			}
			for _, name := range []string{"domain", "policies", "decisions", "questions"} {
				if err := os.MkdirAll(filepath.Join(dir, name), 0o700); err != nil {
					return err
				}
			}
			path := filepath.Join(dir, "manifest.yaml")
			file, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
			if err != nil {
				return fmt.Errorf("knowledge manifest already exists or cannot be created: %w", err)
			}
			content := fmt.Sprintf("schema_version: 1\nproject_id: %q\nselected_packs: []\noverlay_root: .cks/knowledge\nreview_policy:\n  min_approvals: 1\n", projectID)
			if _, err := file.WriteString(content); err != nil {
				file.Close()
				os.Remove(path)
				return err
			}
			if err := file.Close(); err != nil {
				return err
			}
			return json.NewEncoder(c.OutOrStdout()).Encode(map[string]any{"status": "created", "manifest": path, "project_id": projectID})
		}}
	initCmd.Flags().StringVar(&projectID, "project-id", "", "stable CKS project ID")
	validateCmd := &cobra.Command{Use: "validate", Short: "Check selected local packs, dependency graph and saved lock", Args: cobra.NoArgs,
		RunE: func(c *cobra.Command, _ []string) error {
			if err := requireRoot(); err != nil {
				return err
			}
			lock, err := knowledgepack.BuildLock(root)
			if err != nil {
				return err
			}
			status := "unlocked"
			if _, err := os.Lstat(filepath.Join(root, ".cks", "knowledge", "knowledge.lock.json")); err == nil {
				if _, err := knowledgepack.VerifyLock(root); err != nil {
					return err
				}
				status = "locked"
			} else if !os.IsNotExist(err) {
				return err
			}
			instances, err := knowledgepack.LoadProjectInstances(root)
			if err != nil {
				return err
			}
			return json.NewEncoder(c.OutOrStdout()).Encode(map[string]any{"status": status, "project_id": lock.ProjectID,
				"pack_count": len(lock.Packs), "lock_digest": lock.LockDigest,
				"policy_count": len(instances.Policies), "decision_count": len(instances.Decisions),
				"conflict_count": len(instances.Conflicts)})
		}}
	lockCmd := &cobra.Command{Use: "lock", Short: "Write the exact local pack and overlay byte lock", Args: cobra.NoArgs,
		RunE: func(c *cobra.Command, _ []string) error {
			if err := requireRoot(); err != nil {
				return err
			}
			lock, err := knowledgepack.BuildLock(root)
			if err != nil {
				return err
			}
			path := filepath.Join(root, ".cks", "knowledge", "knowledge.lock.json")
			data, err := json.MarshalIndent(lock, "", "  ")
			if err != nil {
				return err
			}
			data = append(data, '\n')
			tmp, err := os.CreateTemp(filepath.Dir(path), ".knowledge-lock-*")
			if err != nil {
				return err
			}
			defer os.Remove(tmp.Name())
			if err := tmp.Chmod(0o600); err != nil {
				tmp.Close()
				return err
			}
			if _, err := tmp.Write(data); err != nil {
				tmp.Close()
				return err
			}
			if err := tmp.Close(); err != nil {
				return err
			}
			if err := os.Rename(tmp.Name(), path); err != nil {
				return err
			}
			return json.NewEncoder(c.OutOrStdout()).Encode(map[string]any{"status": "locked", "project_id": lock.ProjectID,
				"pack_count": len(lock.Packs), "lock_digest": lock.LockDigest, "path": path})
		}}
	var source string
	digestCmd := &cobra.Command{Use: "digest", Short: "Calculate one registered local pack's exact file digest", Args: cobra.NoArgs,
		RunE: func(c *cobra.Command, _ []string) error {
			if err := requireRoot(); err != nil {
				return err
			}
			if source == "" {
				return fmt.Errorf("--source is required")
			}
			loaded, err := knowledgepack.LoadRegisteredPack(root, source)
			if err != nil {
				return err
			}
			return json.NewEncoder(c.OutOrStdout()).Encode(map[string]any{"pack_id": loaded.Pack.PackID,
				"version": loaded.Pack.Version, "sha256": loaded.Digest})
		}}
	digestCmd.Flags().StringVar(&source, "source", "", "project-relative pack directory")
	var reviewVersionDir string
	reviewCmd := &cobra.Command{Use: "review", Short: "List source-backed policy and ADR records for human review", Args: cobra.NoArgs,
		RunE: func(c *cobra.Command, _ []string) error {
			var lock knowledgepack.Lock
			var instances knowledgepack.Instances
			var err error
			if reviewVersionDir != "" {
				instances, lock, err = knowledgepack.LoadRetainedProjectInstances(reviewVersionDir)
			} else {
				if err := requireRoot(); err != nil {
					return err
				}
				lock, err = knowledgepack.BuildLock(root)
				if err == nil {
					instances, err = knowledgepack.LoadProjectInstances(root)
				}
			}
			if err != nil {
				return err
			}
			type item struct {
				Kind       string                  `json:"kind"`
				ID         string                  `json:"id"`
				Status     string                  `json:"status"`
				SourceRef  knowledgepack.SourceRef `json:"source_ref"`
				ReviewedBy string                  `json:"reviewed_by,omitempty"`
				HoldReason string                  `json:"hold_reason,omitempty"`
			}
			queue := []item{}
			for _, p := range instances.Policies {
				reason := ""
				if p.Status == "proposed" {
					reason = "awaiting_human_review"
				} else if p.Status == "rejected" {
					reason = "rejected_by_reviewer"
				}
				queue = append(queue, item{"policy", p.ID, p.Status, p.SourceRef, p.ReviewedBy, reason})
			}
			for _, d := range instances.Decisions {
				reason := ""
				if d.Status == "proposed" {
					reason = "awaiting_human_review"
				} else if d.Status == "rejected" {
					reason = "rejected_by_reviewer"
				}
				queue = append(queue, item{"decision", d.ID, d.Status, d.SourceRef, d.ReviewedBy, reason})
			}
			return json.NewEncoder(c.OutOrStdout()).Encode(map[string]any{"project_id": lock.ProjectID, "items": queue, "conflicts": instances.Conflicts})
		}}
	reviewCmd.Flags().StringVar(&reviewVersionDir, "version-dir", "", "read policy and ADR records from a pinned retained dataset")
	cmd.AddCommand(initCmd, validateCmd, lockCmd, digestCmd, reviewCmd)
	return cmd
}
