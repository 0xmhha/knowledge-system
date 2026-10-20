package setup

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

// ErrPublicationUnresolved blocks another pointer mutation when durable intent
// and observed state cannot be reconciled. It never requests automatic rollback.
var ErrPublicationUnresolved = errors.New("managed publication requires reconciliation")
var managedID = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]{0,127}$`)
var managedHash = regexp.MustCompile(`^[0-9a-f]{64}$`)
var managedCommit = regexp.MustCompile(`^([0-9a-f]{40}|[0-9a-f]{64})$`)

const managedPublicationDirectory = ".maintenance-publications"

type managedPublicationKey struct{}

// ManagedPublication is a durable operation history, separate from which
// version later readers/current adopt. A subsequent writer settles pending
// history before it is permitted to move current again.
type ManagedPublication struct {
	SchemaVersion    int              `json:"schema_version"`
	OperationID      string           `json:"operation_id"`
	ProjectID        string           `json:"project_id"`
	PreviousVersion  string           `json:"previous_version"`
	PreviousIdentity *DatasetIdentity `json:"previous_identity"`
	Version          string           `json:"version"`
	Identity         DatasetIdentity  `json:"identity"`
	BeganAt          string           `json:"began_at"`
	ConfirmedAt      string           `json:"confirmed_at,omitempty"`
	Outcome          string           `json:"outcome"`
}

type managedPublicationEnvelope struct {
	Proof    ManagedPublication `json:"proof"`
	Checksum string             `json:"checksum"`
}

func managedVersion(version string) bool {
	return managedID.MatchString(version) && !strings.EqualFold(version, "current")
}
func managedProject(dataset string) (string, bool, error) {
	var owner struct {
		SchemaVersion int    `json:"schema_version"`
		ProjectID     string `json:"project_id"`
	}
	err := readManagedMetadata(filepath.Join(dataset, ".maintenance-project.json"), &owner)
	if os.IsNotExist(err) {
		return "", false, nil
	}
	if err != nil || owner.SchemaVersion != 1 || !managedID.MatchString(owner.ProjectID) {
		return "", false, ErrPublicationUnresolved
	}
	return owner.ProjectID, true, nil
}
func publicationRoot(dataset string, create bool) (string, error) {
	root := filepath.Join(dataset, managedPublicationDirectory)
	info, err := os.Lstat(root)
	if os.IsNotExist(err) && create {
		if err := os.Mkdir(root, 0o700); err != nil {
			return "", err
		}
		if err := syncDirectory(dataset); err != nil {
			return "", err
		}
		return root, nil
	}
	if err != nil {
		return "", err
	}
	if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return "", ErrPublicationUnresolved
	}
	return root, nil
}
func validManagedIdentity(identity DatasetIdentity, project string) bool {
	s := identity.Source
	if s.ProjectID != project || !managedHash.MatchString(s.SnapshotID) || sourceSnapshotID(s) != s.SnapshotID || !managedHash.MatchString(s.FileManifestDigest) || !managedHash.MatchString(s.CapturePolicyDigest) {
		return false
	}
	switch s.SourceMode {
	case "snapshot-only":
		if s.SourceCommit != "" {
			return false
		}
	case "committed", "working-tree":
		if !managedCommit.MatchString(s.SourceCommit) {
			return false
		}
	default:
		return false
	}
	normalized, err := NewDatasetIdentity(s, identity.EmbeddingIdentity, identity.InputDigest)
	return err == nil && normalized.DatasetID == identity.DatasetID
}
func validManagedProof(p ManagedPublication) bool {
	if p.SchemaVersion != 1 || !managedID.MatchString(p.OperationID) || !managedID.MatchString(p.ProjectID) || !managedVersion(p.Version) || p.Version == p.PreviousVersion || !validManagedIdentity(p.Identity, p.ProjectID) {
		return false
	}
	if (p.PreviousVersion == "") != (p.PreviousIdentity == nil) {
		return false
	}
	if p.PreviousIdentity != nil && (!managedVersion(p.PreviousVersion) || !validManagedIdentity(*p.PreviousIdentity, p.ProjectID)) {
		return false
	}
	began, err := time.Parse(time.RFC3339Nano, p.BeganAt)
	if err != nil {
		return false
	}
	if p.Outcome == "pending" {
		return p.ConfirmedAt == ""
	}
	if p.Outcome != "committed" && p.Outcome != "not_committed" {
		return false
	}
	confirmed, err := time.Parse(time.RFC3339Nano, p.ConfirmedAt)
	return err == nil && !confirmed.Before(began)
}
func proofChecksum(p ManagedPublication) (string, error) {
	raw, err := json.Marshal(p)
	if err != nil {
		return "", err
	}
	return identityHashFields("cks.managed-publication.v1", string(raw)), nil
}
func readManagedProof(path string) (*ManagedPublication, error) {
	var envelope managedPublicationEnvelope
	if err := readManagedMetadata(path, &envelope); err != nil {
		return nil, err
	}
	checksum, err := proofChecksum(envelope.Proof)
	if err != nil || checksum != envelope.Checksum || !validManagedProof(envelope.Proof) {
		return nil, ErrPublicationUnresolved
	}
	return &envelope.Proof, nil
}
func writeManagedProof(path string, p ManagedPublication) error {
	if !validManagedProof(p) {
		return ErrPublicationUnresolved
	}
	checksum, err := proofChecksum(p)
	if err != nil {
		return err
	}
	return writeJSONAtomic(path, managedPublicationEnvelope{p, checksum})
}
func sameManagedIntent(a, b ManagedPublication) bool {
	a.ConfirmedAt, b.ConfirmedAt = "", ""
	a.Outcome, b.Outcome = "pending", "pending"
	left, lerr := json.Marshal(a)
	right, rerr := json.Marshal(b)
	return lerr == nil && rerr == nil && string(left) == string(right)
}

func beginManagedPublication(ctx context.Context, dataset, version, previous string, target *DatasetIdentity) (*ManagedPublication, error) {
	project, enabled, err := managedProject(dataset)
	if err != nil {
		return nil, err
	}
	operationID, _ := ctx.Value(managedPublicationKey{}).(string)
	if !enabled {
		if operationID != "" {
			return nil, ErrPublicationUnresolved
		}
		return nil, nil
	}
	if target == nil || target.Source.ProjectID != project || !managedVersion(version) {
		return nil, ErrPublicationUnresolved
	}
	if previous == version {
		if operationID != "" {
			return nil, ErrPublicationUnresolved
		}
		return nil, nil
	}
	if operationID == "" {
		var random [16]byte
		if _, err := rand.Read(random[:]); err != nil {
			return nil, err
		}
		operationID = "cli-" + hex.EncodeToString(random[:])
	}
	if !managedID.MatchString(operationID) || strings.EqualFold(operationID, "pending") {
		return nil, ErrPublicationUnresolved
	}
	root, err := publicationRoot(dataset, true)
	if err != nil {
		return nil, err
	}
	if _, err := os.Lstat(filepath.Join(root, operationID+".json")); !os.IsNotExist(err) {
		return nil, ErrPublicationUnresolved
	}
	if _, err := os.Lstat(filepath.Join(root, "pending.json")); !os.IsNotExist(err) {
		return nil, ErrPublicationUnresolved
	}
	var prev *DatasetIdentity
	if previous != "" {
		prev, err = InspectVersionIdentityContext(ctx, filepath.Join(dataset, previous))
		if err != nil || prev == nil {
			return nil, ErrPublicationUnresolved
		}
	}
	p := ManagedPublication{SchemaVersion: 1, OperationID: operationID, ProjectID: project, PreviousVersion: previous, PreviousIdentity: prev, Version: version, Identity: *target, BeganAt: time.Now().UTC().Format(time.RFC3339Nano), Outcome: "pending"}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if err := writeManagedProof(filepath.Join(root, "pending.json"), p); err != nil {
		return nil, err
	}
	if err := durabilityStep("publication-intent", filepath.Join(root, "pending.json")); err != nil {
		return nil, err
	}
	return &p, nil
}

func finishManagedPublication(dataset string, p ManagedPublication, outcome string) (*ManagedPublication, error) {
	root, err := publicationRoot(dataset, false)
	if err != nil {
		return nil, err
	}
	history := filepath.Join(root, p.OperationID+".json")
	if old, err := readManagedProof(history); err == nil {
		if !sameManagedIntent(*old, p) || old.Outcome != outcome {
			return nil, ErrPublicationUnresolved
		}
		p = *old
	} else if !os.IsNotExist(err) {
		return nil, err
	} else {
		p.Outcome, p.ConfirmedAt = outcome, time.Now().UTC().Format(time.RFC3339Nano)
		if err := writeManagedProof(history, p); err != nil {
			return nil, err
		}
	}
	if err := durabilityStep("publication-history", history); err != nil {
		return nil, err
	}
	if err := os.Remove(filepath.Join(root, "pending.json")); err != nil && !os.IsNotExist(err) {
		return nil, err
	}
	if err := syncDirectory(root); err != nil {
		return nil, err
	}
	return &p, nil
}

// Called only while the dataset and pointer locks are held, before any next
// pointer action. Historical proof is immutable even after a later rollback/GC.
func reconcileManagedPublicationLocked(ctx context.Context, dataset string) error {
	project, enabled, err := managedProject(dataset)
	if err != nil || !enabled {
		return err
	}
	root, err := publicationRoot(dataset, false)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	pending, err := readManagedProof(filepath.Join(root, "pending.json"))
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	if pending.ProjectID != project || pending.Outcome != "pending" {
		return ErrPublicationUnresolved
	}
	if history, err := readManagedProof(filepath.Join(root, pending.OperationID+".json")); err == nil {
		if !sameManagedIntent(*history, *pending) || history.Outcome == "pending" {
			return ErrPublicationUnresolved
		}
		_, err = finishManagedPublication(dataset, *pending, history.Outcome)
		return err
	} else if !os.IsNotExist(err) {
		return err
	}
	current, err := os.Readlink(filepath.Join(dataset, "current"))
	if os.IsNotExist(err) {
		current = ""
	} else if err != nil {
		return err
	}
	switch current {
	case pending.Version:
		actual, err := InspectVersionIdentityContext(ctx, filepath.Join(dataset, current))
		if err != nil || actual == nil {
			return ErrPublicationUnresolved
		}
		expected, err := json.Marshal(pending.Identity)
		if err != nil {
			return err
		}
		observed, err := json.Marshal(actual)
		if err != nil || string(expected) != string(observed) {
			return ErrPublicationUnresolved
		}
		if err := flushCandidateContext(ctx, filepath.Join(dataset, current)); err != nil {
			return err
		}
		if err := syncCurrentParent(dataset); err != nil {
			return err
		}
		if err := recordRollbackTargets(dataset, pending.PreviousVersion, false); err != nil {
			return err
		}
		_, err = finishManagedPublication(dataset, *pending, "committed")
		return err
	case pending.PreviousVersion:
		if current != "" {
			actual, err := InspectVersionIdentityContext(ctx, filepath.Join(dataset, current))
			if err != nil || actual == nil {
				return ErrPublicationUnresolved
			}
			a, _ := json.Marshal(actual)
			b, _ := json.Marshal(pending.PreviousIdentity)
			if string(a) != string(b) {
				return ErrPublicationUnresolved
			}
		}
		_, err = finishManagedPublication(dataset, *pending, "not_committed")
		return err
	default:
		return ErrPublicationUnresolved
	}
}

// ReadManagedPublication is a read-only immutable history lookup. A missing
// result is not by itself proof that an incomplete build's children stopped.
func ReadManagedPublication(dataset, operationID string) (*ManagedPublication, error) {
	if !managedID.MatchString(operationID) {
		return nil, ErrPublicationUnresolved
	}
	project, enabled, err := managedProject(dataset)
	if err != nil {
		return nil, err
	}
	if !enabled {
		return nil, ErrPublicationUnresolved
	}
	root, err := publicationRoot(dataset, false)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	p, err := readManagedProof(filepath.Join(root, operationID+".json"))
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if p.OperationID != operationID || p.ProjectID != project || p.Outcome == "pending" {
		return nil, ErrPublicationUnresolved
	}
	return p, nil
}

// ReconcileManagedPublication shares the same mutation/pointer exclusion as
// all managed pointer writers and never moves current while recovering proof.
func ReconcileManagedPublication(ctx context.Context, dataset, operationID string) (*ManagedPublication, error) {
	var result *ManagedPublication
	err := WithReconciledManagedPublication(ctx, dataset, operationID, func(proof *ManagedPublication) error {
		result = proof
		return nil
	})
	return result, err
}

// WithReconciledManagedPublication keeps ownership while a trusted service
// checks retained candidate evidence and records its recovered outcome. The
// action must not recursively acquire the dataset/pointer locks.
func WithReconciledManagedPublication(ctx context.Context, dataset, operationID string, action func(*ManagedPublication) error) error {
	err := WithDatasetMutationLock(ctx, dataset, func(context.Context) error {
		_, err := withPointerLockContext(ctx, dataset, func() (string, error) {
			proof, err := ReadManagedPublication(dataset, operationID)
			if err != nil {
				return "", err
			}
			return "", action(proof)
		})
		return err
	})
	if err != nil {
		return fmt.Errorf("%w: %w", ErrPublicationUnresolved, err)
	}
	return nil
}
