package knowledgecli

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/0xmhha/knowledge-system/internal/setup"
	"github.com/0xmhha/knowledge-system/internal/system/knowledgepack"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
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
			for _, name := range []string{"domain", "policies", "decisions", "relations", "reviews", "questions"} {
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
				"policy_count": len(instances.Policies), "decision_count": len(instances.Decisions), "relation_count": len(instances.Relations),
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
				Kind        string                  `json:"kind"`
				ID          string                  `json:"id"`
				Status      string                  `json:"status"`
				SourceRef   knowledgepack.SourceRef `json:"source_ref"`
				ReviewedBy  string                  `json:"reviewed_by,omitempty"`
				ReviewCount int                     `json:"review_count"`
				HoldReason  string                  `json:"hold_reason,omitempty"`
			}
			queue := []item{}
			reviewState := func(kind, id, status string) (string, int) {
				votes := map[string]string{}
				for _, record := range instances.Reviews {
					if record.TargetKind == kind && record.TargetID == id {
						votes[record.Reviewer] = record.Decision
					}
				}
				if status == "rejected" {
					return "rejected_by_reviewer", len(votes)
				}
				if status != "proposed" {
					return "", len(votes)
				}
				if len(votes) == 0 {
					return "awaiting_human_review", 0
				}
				verified, rejected := false, false
				for _, vote := range votes {
					verified = verified || vote == "verified"
					rejected = rejected || vote == "rejected"
				}
				if verified && rejected {
					return "conflicting_reviews", len(votes)
				}
				return "approval_quorum_pending", len(votes)
			}
			for _, p := range instances.Policies {
				reason, count := reviewState("policy", p.ID, p.Status)
				queue = append(queue, item{"policy", p.ID, p.Status, p.SourceRef, p.ReviewedBy, count, reason})
			}
			for _, d := range instances.Decisions {
				reason, count := reviewState("decision", d.ID, d.Status)
				queue = append(queue, item{"decision", d.ID, d.Status, d.SourceRef, d.ReviewedBy, count, reason})
			}
			for _, relation := range instances.Relations {
				reason, count := reviewState("relation", relation.ID, relation.Status)
				queue = append(queue, item{"relation", relation.ID, relation.Status, relation.SourceRef, relation.ReviewedBy, count, reason})
			}
			for _, link := range instances.TraceLinks {
				reason, count := reviewState("trace-link", link.ID, link.Status)
				queue = append(queue, item{"trace-link", link.ID, link.Status, link.SourceRef, link.ReviewedBy, count, reason})
			}
			return json.NewEncoder(c.OutOrStdout()).Encode(map[string]any{"project_id": lock.ProjectID, "items": queue, "conflicts": instances.Conflicts})
		}}
	reviewCmd.PersistentFlags().StringVar(&reviewVersionDir, "version-dir", "", "read policy and ADR records from a pinned retained dataset")
	var reviewKind, reviewID, reviewDecision, reviewer, reviewReason string
	recordCmd := &cobra.Command{Use: "record", Short: "Record a human decision against archived source bytes", Args: cobra.NoArgs,
		RunE: func(c *cobra.Command, _ []string) error {
			if err := requireRoot(); err != nil {
				return err
			}
			if reviewVersionDir == "" || reviewID == "" || reviewer == "" || reviewReason == "" ||
				(reviewDecision != "verified" && reviewDecision != "rejected") {
				return fmt.Errorf("review record requires --version-dir, --kind, --id, --decision, --reviewer and --reason")
			}
			manifest, err := knowledgepack.ReadManifest(root)
			if err != nil {
				return err
			}
			identity, err := setup.InspectVersionIdentity(reviewVersionDir)
			if err != nil || identity == nil || identity.Source.ProjectID != manifest.ProjectID {
				return fmt.Errorf("snapshot_mismatch: review candidate project or identity differs")
			}
			instances, _, err := knowledgepack.LoadRetainedProjectInstances(reviewVersionDir)
			if err != nil {
				return err
			}
			var source knowledgepack.SourceRef
			status := ""
			switch reviewKind {
			case "policy":
				for _, p := range instances.Policies {
					if p.ID == reviewID {
						source, status = p.SourceRef, p.Status
					}
				}
			case "decision":
				for _, d := range instances.Decisions {
					if d.ID == reviewID {
						source, status = d.SourceRef, d.Status
					}
				}
			case "relation":
				for _, relation := range instances.Relations {
					if relation.ID == reviewID {
						source, status = relation.SourceRef, relation.Status
					}
				}
			case "trace-link":
				for _, link := range instances.TraceLinks {
					if link.ID == reviewID {
						source, status = link.SourceRef, link.Status
					}
				}
			default:
				return fmt.Errorf("invalid review kind %q", reviewKind)
			}
			if status != "proposed" || source.OriginID != "repo" {
				return fmt.Errorf("evidence_unverified: review target is missing or no longer proposed")
			}
			archived, _, _, err := setup.ReadRetainedFile(reviewVersionDir, source.OriginID, source.Path)
			if err != nil {
				return err
			}
			live, err := setup.ReadSourceFileNoFollow(root, source.Path, 4<<20)
			if err != nil || !bytes.Equal(live, archived) {
				return fmt.Errorf("snapshot_mismatch: live review target differs from archived candidate")
			}
			sum := sha256.Sum256(archived)
			record := knowledgepack.ReviewRecord{TargetKind: reviewKind, TargetID: reviewID, TargetRef: source,
				TargetSHA256: hex.EncodeToString(sum[:]), SnapshotID: identity.Source.SnapshotID,
				Decision: reviewDecision, Reviewer: reviewer, Reason: reviewReason,
				ReviewedAt: time.Now().UTC().Format(time.RFC3339)}
			if err := knowledgepack.ValidateReviewRecord(record); err != nil {
				return err
			}
			body, err := yaml.Marshal(record)
			if err != nil {
				return err
			}
			dir := filepath.Join(root, ".cks", "knowledge", "reviews")
			if err := os.Mkdir(dir, 0o700); err != nil && !os.IsExist(err) {
				return err
			}
			info, err := os.Lstat(dir)
			if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
				return fmt.Errorf("evidence_unverified: review directory is not a real directory")
			}
			recordHash := sha256.Sum256(body)
			path := filepath.Join(dir, hex.EncodeToString(recordHash[:])+".yaml")
			file, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
			if err != nil {
				return err
			}
			if _, err := file.Write(body); err != nil {
				file.Close()
				os.Remove(path)
				return err
			}
			if err := file.Close(); err != nil {
				return err
			}
			return json.NewEncoder(c.OutOrStdout()).Encode(map[string]any{"status": "review_recorded", "target_id": reviewID,
				"snapshot_id": record.SnapshotID, "target_sha256": record.TargetSHA256, "path": path, "needs_relock": true})
		}}
	recordCmd.Flags().StringVar(&reviewKind, "kind", "", "policy, decision, relation or trace-link")
	recordCmd.Flags().StringVar(&reviewID, "id", "", "knowledge instance ID")
	recordCmd.Flags().StringVar(&reviewDecision, "decision", "", "verified or rejected")
	recordCmd.Flags().StringVar(&reviewer, "reviewer", "", "local reviewer ID")
	recordCmd.Flags().StringVar(&reviewReason, "reason", "", "human review reason")
	reviewCmd.AddCommand(recordCmd)
	cmd.AddCommand(initCmd, validateCmd, lockCmd, digestCmd, reviewCmd)
	return cmd
}
