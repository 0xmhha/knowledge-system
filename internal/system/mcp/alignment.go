package mcp

// Startup alignment assert (reindex-migration design Q4, 2026-07-10 agreement).
//
// The two backends must be built from the same source commit (and, once CKG
// ships its logical graph_digest, from the exact graph the chunks were aligned
// to) or the canonical_id join silently degrades. cks asserts this ONCE at
// startup and folds the verdict into serviceability, per the agreed two-tier
// severity:
//
//   - ERROR (serviceable=false): ckg/ckv src_commit mismatch, ckg schema
//     < 1.19 (canonical_id values unpopulated), or graph_digest mismatch
//     (when both sides report one).
//   - WARNING (served, surfaced in health): source_root path differs from the
//     indexed checkout at the same commit, source tree HEAD ahead/behind the
//     indexed commit (freshness territory), or the CKV sources ledger not yet
//     present (pre-P1 CKV index — additive rollout).
//
// The interface is manifest-file based by agreement: no new backend API. CKG
// coordinates come from ckgclient.Health (manifest-backed); CKV coordinates
// are parsed from <ckv-data>/manifest.json.

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
)

// AlignmentSource is one engine's coordinates in the alignment report. The
// two engines are named descriptively (graph / vector) on this external
// surface; the values stay per-engine because a divergence is the drift
// signal the report exists to expose.
type AlignmentSource struct {
	SrcCommit     string `json:"src_commit,omitempty"`
	SchemaVersion string `json:"schema_version,omitempty"` // graph only currently
	GraphDigest   string `json:"graph_digest,omitempty"`   // graph: its own; vector: the one it aligned to
}

// AlignmentSources groups the per-engine coordinates.
type AlignmentSources struct {
	Graph  AlignmentSource `json:"graph"`
	Vector AlignmentSource `json:"vector"`
}

// AlignmentReport is the health-facing verdict of the startup assert. OK=false
// makes the instance non-serviceable (fail-loud, 2026-06-15 policy).
type AlignmentReport struct {
	OK bool `json:"ok"`
	// DatasetVersion is the version label of the dataset directory the
	// instance resolved at startup (the "@<ver>" segment when the versioned
	// blue-green layout is in use; empty for legacy flat layouts).
	DatasetVersion string `json:"dataset_version,omitempty"`
	// SrcCommit is the single source commit, populated ONLY when both engines
	// agree. On a mismatch it is empty and OK=false; read Sources for the two
	// diverging values.
	SrcCommit string `json:"src_commit,omitempty"`
	// Sources carries each engine's coordinates for operator diagnosis, keyed
	// by descriptive engine name (graph / vector).
	Sources *AlignmentSources `json:"sources,omitempty"`
	// Deprecated: the suffix-tagged keys below are kept one release for
	// back-compat; new consumers should read SrcCommit / Sources instead.
	SrcCommitCKG  string `json:"src_commit_ckg,omitempty"`
	SrcCommitCKV  string `json:"src_commit_ckv,omitempty"`
	SchemaVersion string `json:"schema_version_ckg,omitempty"`
	// GraphDigest* compare the graph engine's logical digest against the one
	// the vector engine recorded at align time. Empty until both sides ship.
	GraphDigestActual   string `json:"graph_digest_actual,omitempty"`
	GraphDigestExpected string `json:"graph_digest_expected,omitempty"`
	// SourceRootOK is false when the configured source_root path differs from
	// the checkout the index was built from (warning tier: bodies still match
	// when the commit is the same).
	SourceRootOK bool     `json:"source_root_ok"`
	Warnings     []string `json:"warnings,omitempty"`
	// Reason is the error-tier explanation when OK=false.
	Reason string `json:"reason,omitempty"`
}

// AlignmentInputs carries everything ComputeAlignment needs; the caller
// (cmd/cks-mcp) gathers them at startup.
type AlignmentInputs struct {
	// CKG coordinates (from ckgclient.Health / the graph manifest).
	CKGSrcCommit string
	CKGSchema    string
	CKGDigest    string // logical graph digest; "" until CKG ships it
	// CKV manifest (raw bytes of <ckv-data>/manifest.json; nil when missing).
	CKVManifest []byte
	// CKVConfigured is true when a ckv index is part of this deployment
	// (config.Backends.CKV.Path set). When it is, a missing/coordinate-less
	// manifest is an ERROR (the canonical_id join cannot be verified), not a
	// warning — otherwise a mis-aligned index would report serviceable. A
	// ckg-only deployment leaves this false and skips the check.
	CKVConfigured bool
	// Config + environment.
	ConfigSourceRoot string
	SourceHead       string // git HEAD of ConfigSourceRoot; "" when unknown
	DatasetVersion   string // "@<ver>" label from the resolved dataset path
}

// ckvManifest is the subset of CKV's manifest.json the assert reads. The
// sources ledger (§2.2 of the reindex design) is additive: absent on pre-P1
// indexes, in which case top-level src_commit/src_root are the fallback
// coordinates.
type ckvManifest struct {
	SrcCommit   string `json:"src_commit"`
	IndexedHead string `json:"indexed_head"`
	SrcRoot     string `json:"src_root"`
	Sources     struct {
		CKG struct {
			GraphDigest   string `json:"graph_digest"`
			SrcCommit     string `json:"src_commit"`
			SchemaVersion string `json:"schema_version"`
			Path          string `json:"path"`
		} `json:"ckg"`
	} `json:"sources"`
}

// ComputeAlignment evaluates the two-tier assert. Pure function: all I/O is
// the caller's job, so the tier logic is unit-testable.
func ComputeAlignment(in AlignmentInputs) *AlignmentReport {
	rep := &AlignmentReport{
		OK:             true,
		SourceRootOK:   true,
		DatasetVersion: in.DatasetVersion,
		SrcCommitCKG:   in.CKGSrcCommit,
		SchemaVersion:  in.CKGSchema,
	}
	var errs []string

	// --- CKV coordinates -------------------------------------------------
	var m ckvManifest
	haveManifest := len(in.CKVManifest) > 0
	if haveManifest {
		if err := json.Unmarshal(in.CKVManifest, &m); err != nil {
			rep.Warnings = append(rep.Warnings, fmt.Sprintf("ckv manifest unparsable: %v", err))
			haveManifest = false
		}
	} else {
		rep.Warnings = append(rep.Warnings, "ckv manifest missing — alignment coordinates unavailable")
	}

	ckvCommit := ""
	if haveManifest {
		// Prefer the sources ledger; fall back to the top-level fields.
		ckvCommit = m.Sources.CKG.SrcCommit
		if ckvCommit == "" {
			ckvCommit = firstNonEmpty(m.SrcCommit, m.IndexedHead)
			rep.Warnings = append(rep.Warnings,
				"ckv sources.ckg ledger absent (pre-P1 index) — using top-level src_commit")
		}
		rep.SrcCommitCKV = ckvCommit
		rep.GraphDigestExpected = m.Sources.CKG.GraphDigest
	}
	rep.GraphDigestActual = in.CKGDigest

	// --- ERROR tier -------------------------------------------------------
	// Fail closed when ckv is expected but provides no verifiable coordinates:
	// two individually-reachable backends whose join cannot be checked are
	// confidently-wrong, not ok. A pre-P1 index still has a top-level src_commit
	// (ckvCommit non-empty), so only a truly missing/unparsable manifest trips.
	if in.CKVConfigured && ckvCommit == "" {
		errs = append(errs, "ckv index configured but its manifest is missing or has no commit — "+
			"alignment coordinates unavailable, cannot verify the canonical_id join")
	}
	if in.CKGSrcCommit != "" && ckvCommit != "" && in.CKGSrcCommit != ckvCommit {
		errs = append(errs, fmt.Sprintf(
			"ckg/ckv built from different commits (ckg %.9s, ckv %.9s)", in.CKGSrcCommit, ckvCommit))
	}
	if in.CKGSchema != "" && !schemaAtLeast119(in.CKGSchema) {
		errs = append(errs, fmt.Sprintf(
			"ckg schema %s < 1.19 — canonical_id values unpopulated", in.CKGSchema))
	}
	if in.CKGDigest != "" && rep.GraphDigestExpected != "" && in.CKGDigest != rep.GraphDigestExpected {
		errs = append(errs, fmt.Sprintf(
			"graph digest mismatch (ckg %.12s, ckv aligned to %.12s) — ckv canonical_id stale",
			in.CKGDigest, rep.GraphDigestExpected))
	}

	// --- WARNING tier -----------------------------------------------------
	if haveManifest && m.SrcRoot != "" && in.ConfigSourceRoot != "" && m.SrcRoot != in.ConfigSourceRoot {
		rep.SourceRootOK = false
		rep.Warnings = append(rep.Warnings, fmt.Sprintf(
			"config source_root %q differs from indexed checkout %q — bodies match only while both are at the indexed commit",
			in.ConfigSourceRoot, m.SrcRoot))
	}
	indexed := firstNonEmpty(ckvCommit, in.CKGSrcCommit)
	if in.SourceHead != "" && indexed != "" && in.SourceHead != indexed {
		rep.Warnings = append(rep.Warnings, fmt.Sprintf(
			"source_root HEAD %.9s != indexed commit %.9s — stale tree (freshness reports details)",
			in.SourceHead, indexed))
	}

	// Descriptive nested view (preferred) built from the same values as the
	// deprecated suffix keys above. The top-level SrcCommit is exposed only
	// when the two engines agree — a mismatch leaves it empty (Sources shows
	// the divergence and the ERROR tier already flagged it).
	rep.Sources = &AlignmentSources{
		Graph:  AlignmentSource{SrcCommit: in.CKGSrcCommit, SchemaVersion: in.CKGSchema, GraphDigest: in.CKGDigest},
		Vector: AlignmentSource{SrcCommit: ckvCommit, GraphDigest: rep.GraphDigestExpected},
	}
	if in.CKGSrcCommit != "" && in.CKGSrcCommit == ckvCommit {
		rep.SrcCommit = in.CKGSrcCommit
	}

	if len(errs) > 0 {
		rep.OK = false
		rep.Reason = strings.Join(errs, "; ")
	}
	return rep
}

// schemaAtLeast119 reports whether a "major.minor" schema string is >= 1.19.
func schemaAtLeast119(version string) bool {
	parts := strings.SplitN(strings.TrimSpace(version), ".", 3)
	if len(parts) < 2 {
		return false
	}
	major, err1 := strconv.Atoi(parts[0])
	minor, err2 := strconv.Atoi(parts[1])
	if err1 != nil || err2 != nil {
		return false
	}
	return major > 1 || (major == 1 && minor >= 19)
}
