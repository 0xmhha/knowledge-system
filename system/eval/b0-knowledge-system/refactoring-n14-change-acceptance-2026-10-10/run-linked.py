"""Run every reviewed criterion test on the N14 result candidate with the
production CLI (cks semantic test --go-test-exact). F3-T10 is a refusal
control (no reviewed path since the N12 retarget)."""
import json, os, subprocess
from pathlib import Path
E = Path("system/eval/b0-knowledge-system/refactoring-n14-change-acceptance-2026-10-10")
prep = json.loads((E / "preparation.json").read_text()); base = Path(prep["base"])
v = Path(prep["dataset_root"]) / "result"; identity = json.loads((v / "dataset-identity.json").read_text())
runs = [("F3-T03", "ac-filtered-recall", "internal/vector/store/sqlitevec", "TestStoreFilterReturnsKWhenGlobalTopCandidatesAreExcluded"),
        ("F3-T05", "ac-filtered-completeness", "internal/vector/store/sqlitevec", "TestSearchDetailedReportsCandidateLimitWithoutCompleteHits"),
        ("F3-T07", "ac-filtered-completeness", "internal/vector/store/sqlitevec", "TestSearchDetailedCancellationIsNotEmptySuccess"),
        ("F3-R02", "ac-filtered-completeness", "internal/vector/store/sqlitevec", "TestStoreFilterLargeCandidateSetFallsBackToExact"),
        ("F3-R04", "ac-filtered-completeness", "internal/vector/store/sqlitevec", "TestSearchDetailedFilteredCancellationAfterStartIsNotComplete"),
        ("F3-T10", "ac-source-evidence", "internal/system/semantic", "TestPutAlignedRejectsCrossLayerSnapshotAndLeavesStoreEmpty"),
        ("F3-T12", "ac-source-evidence", "internal/system/semantic", "TestForeignCodeAnchorPromotionPreservesActiveDataset"),
        ("F3-T15", "ac-ontology-fallback", "internal/system/composer/stage2", "TestProvidedOntologyFallbackAndAmbiguityDoNotReplaceBaseline"),
        ("F3-T17", "ac-ontology-fallback", "internal/system/composer/stage2", "TestStoredSharedAliasPreservesBaselineAndRetainedEvidence"),
        ("F3-R14", "ac-ontology-fallback", "internal/system/composer/stage2", "TestProvidedOntologyUnverifiedRelationPreservesCKGAndCKVBaseline")]
env = dict(os.environ, GIT_CONFIG_COUNT="1", GIT_CONFIG_KEY_0="commit.gpgsign", GIT_CONFIG_VALUE_0="false")
out_dir = base / "linked-runs"; out_dir.mkdir(exist_ok=True)
summary = []
for rid, criterion, pkg, test in runs:
    out = out_dir / (rid + ".json")
    argv = [str(base / "bin/cks"), "semantic", "test", "--project-id", prep["project_id"], "--repo", prep["source"],
            "--graph", str(v / "graph"), "--vector", str(v / "vector"), "--store", str(base / "semantic-result.sqlite"),
            "--criterion-id", criterion, "--test-canonical-id", "github.com/0xmhha/knowledge-system/" + pkg + "." + test,
            "--dataset-id", identity["dataset_id"], "--version-dir", str(v), "--go-test-exact", "--out", str(out)]
    r = subprocess.run(argv, capture_output=True, text=True, env=env)
    row = {"review_id": rid, "criterion_id": criterion, "test": test, "exit_code": r.returncode, "stderr_tail": r.stderr[-300:]}
    if r.returncode == 0:
        rep = json.loads(out.read_text())
        row.update({k: rep.get(k) for k in ("command_passed", "snapshot_consistent", "test_passed", "test_observed", "output_sha256", "duration_ms")})
        row["snapshot_id"] = rep["snapshot"]["snapshot_id"]
        (E / ("linked-run-" + rid + ".json")).write_text(out.read_text())
    summary.append(row)
    print(json.dumps({k: row.get(k) for k in ("review_id", "exit_code", "test_passed", "snapshot_consistent")}))
(E / "linked-runs-summary.json").write_text(json.dumps({"env_scoped": {"GIT_CONFIG_COUNT": "1", "GIT_CONFIG_KEY_0": "commit.gpgsign", "GIT_CONFIG_VALUE_0": "false"},
    "result_snapshot": identity["source"]["snapshot_id"], "runs": summary}, indent=2) + "\n")
