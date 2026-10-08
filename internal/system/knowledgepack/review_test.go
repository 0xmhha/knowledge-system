package knowledgepack

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestReviewRecordsBindRawTargetAndCountDistinctReviewers(t *testing.T) {
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, "policies"), 0o700); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(root, "policies", "BR-1.yaml")
	original := []byte("id: BR-1\nstatus: proposed\n")
	if err := os.WriteFile(path, original, 0o600); err != nil {
		t.Fatal(err)
	}
	hash := sha256.Sum256(original)
	ref := SourceRef{OriginID: "repo", Path: ".cks/knowledge/policies/BR-1.yaml"}
	makeRecord := func(reviewer, decision string) ReviewRecord {
		return ReviewRecord{TargetKind: "policy", TargetID: "BR-1", TargetRef: ref,
			TargetSHA256: hex.EncodeToString(hash[:]), SnapshotID: strings.Repeat("a", 64),
			Decision: decision, Reviewer: reviewer, Reason: "Compared with source.", ReviewedAt: "2026-10-01T00:00:00Z"}
	}
	newInstances := func() Instances {
		return Instances{Policies: []Policy{{ID: "BR-1", Status: "proposed", SourceRef: ref}}}
	}
	instances := newInstances()
	instances.Reviews = []ReviewRecord{makeRecord("alice", "verified"), makeRecord("alice", "verified")}
	if err := applyReviewRecords(root, &instances, 2); err != nil || instances.Policies[0].Status != "proposed" {
		t.Fatalf("duplicate reviewer counted: %+v, %v", instances.Policies[0], err)
	}
	instances = newInstances()
	instances.Reviews = []ReviewRecord{makeRecord("bob", "verified"), makeRecord("alice", "verified")}
	if err := applyReviewRecords(root, &instances, 2); err != nil ||
		instances.Policies[0].Status != "verified" || instances.Policies[0].ReviewedBy != "alice,bob" {
		t.Fatalf("two distinct reviewers not accepted: %+v, %v", instances.Policies[0], err)
	}
	instances = newInstances()
	instances.Reviews = []ReviewRecord{makeRecord("alice", "verified"), makeRecord("bob", "rejected")}
	if err := applyReviewRecords(root, &instances, 1); err != nil || instances.Policies[0].Status != "proposed" {
		t.Fatalf("opposing votes were not held: %+v, %v", instances.Policies[0], err)
	}
	if err := os.WriteFile(path, []byte("id: BR-1\nstatus: changed\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	instances = newInstances()
	instances.Reviews = []ReviewRecord{makeRecord("alice", "verified")}
	if err := applyReviewRecords(root, &instances, 1); err == nil || !strings.Contains(err.Error(), "bytes changed") {
		t.Fatalf("stale review accepted: %v", err)
	}
}
