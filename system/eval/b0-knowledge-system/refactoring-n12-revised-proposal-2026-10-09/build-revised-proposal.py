"""Apply the adopted N12 pre-review modifications as a revised PROPOSED set.

Inputs: the held candidate's proposed projection (immutable, read only), the
pre-review recommendations, and current source declarations resolved by the
go/ast locator (locate.go.txt). Nothing here becomes verified: the product
requires a named reviewer (ReviewedBy) and none has been given.
Run from the repository root: python3 <this> <locate-binary-or-'go run' path>
"""
import hashlib, json, subprocess, sys
from pathlib import Path

D = Path("system/eval/b0-knowledge-system/refactoring-n12-revised-proposal-2026-10-09")
HELD = Path("system/eval/b0-knowledge-system/refactoring-n12-anchor-binding-2026-10-09")
PRE = Path("system/eval/b0-knowledge-system/refactoring-n12-criterion-gap-tests-2026-10-09/prereview.json")
MOD = "github.com/0xmhha/knowledge-system/"
SEM, VEC, ST2 = "internal/system/semantic", "internal/vector/store/sqlitevec", "internal/system/composer/stage2"

def canon(d, recv, name, pointer):
    # Mirrors CKG goCanonicalID: <importpath>.(<*?Recv>).<Method> or <importpath>.<Func>.
    pkg = MOD + d
    return f"{pkg}.({'*' if pointer else ''}{recv}).{name}" if recv else f"{pkg}.{name}"

targets = {  # key -> (dir, receiver, name)
    "ValidateCodeAnchors": (SEM, "", "ValidateCodeAnchors"),
    "PutAligned": (SEM, "Store", "PutAligned"),
    "PutAlignedRetained": (SEM, "Store", "PutAlignedRetained"),
    "ActivateAligned": (SEM, "Store", "ActivateAligned"),
    "ActivateAlignedRetained": (SEM, "Store", "ActivateAlignedRetained"),
    "VerifiedImplementations": (SEM, "ActiveProjection", "VerifiedImplementations"),
    "TestForeignCodeAnchorPromotionPreservesActiveDataset": (SEM, "", "TestForeignCodeAnchorPromotionPreservesActiveDataset"),
    "TestActiveProjectionExposesOnlyVerifiedImplementations": (SEM, "", "TestActiveProjectionExposesOnlyVerifiedImplementations"),
    "SearchDetailed": (VEC, "Store", "SearchDetailed"),
    "TestStoreFilterReturnsKWhenGlobalTopCandidatesAreExcluded": (VEC, "", "TestStoreFilterReturnsKWhenGlobalTopCandidatesAreExcluded"),
    "TestStoreFilterLargeCandidateSetFallsBackToExact": (VEC, "", "TestStoreFilterLargeCandidateSetFallsBackToExact"),
    "TestSearchDetailedFilteredCancellationAfterStartIsNotComplete": (VEC, "", "TestSearchDetailedFilteredCancellationAfterStartIsNotComplete"),
    "applyProvidedOntology": (ST2, "Searcher", "applyProvidedOntology"),
    "TestProvidedOntologyUnverifiedRelationPreservesCKGAndCKVBaseline": (ST2, "", "TestProvidedOntologyUnverifiedRelationPreservesCKGAndCKVBaseline"),
}
q = "".join(f"{d}|{r}|{n}\n" for d, r, n in targets.values())
locs = json.loads(subprocess.run(sys.argv[1:], input=q, capture_output=True, text=True, check=True).stdout)
loc = {k: dict(l, canonical_id=canon(*targets[k], l["pointer_receiver"])) for k, l in zip(targets, locs)}

held = json.loads((HELD / "projection-proposed.json").read_text())
assertions = {a["id"]: a for a in held["assertions"]}
pre = json.loads(PRE.read_text())
ids = {r["review_id"]: r for r in pre["functional_relations"]}
# F3-T id -> held assertion id (worksheet order matches review ids).
ws = json.loads((HELD / "human-review-worksheet.json").read_text())
t2a = {r["review_id"]: r["assertion"]["id"] for r in ws["functional_relation_review"]}

def new_rel(rid, pred, subj, obj, criterion=None, note=None):
    return {"review_id": rid, "predicate": pred, "subject_id": subj, "object_id": obj,
            "object_location": loc.get(obj.rsplit(".", 1)[-1]) if obj.startswith(MOD) else None,
            "subject_location": loc.get(subj.rsplit(".", 1)[-1]) if subj.startswith(MOD) else None,
            "criterion_id": criterion, "note": note, "status": "proposed"}

C = lambda k: loc[k]["canonical_id"]
ops = []
def keep(t, note=None): ops.append({"review_id": t, "op": "keep", "held_assertion": t2a[t], "note": note})
for t in ["F3-T01", "F3-T02", "F3-T04", "F3-T06", "F3-T09", "F3-T12", "F3-T14", "F3-T16"]: keep(t)
keep("F3-T03", "test now also asserts exact distance order (new test bytes, not in held candidate)")
keep("F3-T05", "covers exact-sized candidate_limit only; vec0 path added as F3-R01/R02")
keep("F3-T13", "baseline restriction lives here; unverified relation filtering added as F3-R12")
ops.append({"review_id": "F3-T07", "op": "narrow", "held_assertion": t2a["F3-T07"], "scope": "unfiltered empty-store pre-start cancellation and deadline only", "counts_as_full_criterion_proof": False})
ops.append({"review_id": "F3-T10", "op": "narrow", "held_assertion": t2a["F3-T10"], "scope": "coordinate-drift partial control; fixture has no code anchor", "counts_as_full_criterion_proof": False})
ops.append({"review_id": "F3-T15", "op": "narrow", "held_assertion": t2a["F3-T15"], "scope": "proposed concepts with CKV-only baseline; no verified concept + unverified relation, no CKG citation", "counts_as_full_criterion_proof": False})
ops.append({"review_id": "F3-T17", "op": "narrow", "held_assertion": t2a["F3-T17"], "scope": "ambiguous proposed alias with CKV document hits only", "counts_as_full_criterion_proof": False})
ops.append({"review_id": "F3-T08", "op": "retarget_object", "held_assertion": t2a["F3-T08"], "from": assertions[t2a["F3-T08"]]["object_id"], "to": C("ValidateCodeAnchors"), "to_location": loc["ValidateCodeAnchors"]})
ops.append({"review_id": "F3-T11", "op": "retarget_subject", "held_assertion": t2a["F3-T11"], "from": assertions[t2a["F3-T11"]]["subject_id"], "to": C("ValidateCodeAnchors"), "to_location": loc["ValidateCodeAnchors"],
            "note": "ValidateDatasetAlignment is the test's positive control; rejection is attributed by ErrCodeAnchorSnapshotMismatch/ErrEvidenceSnapshotMismatch"})
added = [
    new_rel("F3-R01", "TESTED_BY", C("SearchDetailed"), C("TestStoreFilterLargeCandidateSetFallsBackToExact"), "ac-filtered-completeness"),
    new_rel("F3-R02", "CHECKED_BY", "ac-filtered-completeness", C("TestStoreFilterLargeCandidateSetFallsBackToExact"), "ac-filtered-completeness", "vec0-path candidate_limit"),
    new_rel("F3-R03", "TESTED_BY", C("SearchDetailed"), C("TestSearchDetailedFilteredCancellationAfterStartIsNotComplete"), "ac-filtered-completeness"),
    new_rel("F3-R04", "CHECKED_BY", "ac-filtered-completeness", C("TestSearchDetailedFilteredCancellationAfterStartIsNotComplete"), "ac-filtered-completeness", "filtered in-flight cancellation, exact and large sets"),
]
n = 5
for gate in ["PutAligned", "PutAlignedRetained", "ActivateAligned", "ActivateAlignedRetained"]:
    added.append(new_rel(f"F3-R{n:02d}", "IMPLEMENTED_BY", "snapshot-bound-semantic-relations", C(gate), "ac-source-evidence", "gate chain that refuses persistence/activation")); n += 1
for gate in ["PutAligned", "ActivateAligned"]:
    added.append(new_rel(f"F3-R{n:02d}", "TESTED_BY", C(gate), C("TestForeignCodeAnchorPromotionPreservesActiveDataset"), "ac-source-evidence")); n += 1
added.append(new_rel("F3-R11", "TESTED_BY", C("VerifiedImplementations"), C("TestActiveProjectionExposesOnlyVerifiedImplementations"), "ac-ontology-fallback"))
added.append(new_rel("F3-R12", "IMPLEMENTED_BY", "optional-ontology-baseline-preservation", C("VerifiedImplementations"), "ac-ontology-fallback", "excludes unverified concept relations"))
added.append(new_rel("F3-R13", "TESTED_BY", C("applyProvidedOntology"), C("TestProvidedOntologyUnverifiedRelationPreservesCKGAndCKVBaseline"), "ac-ontology-fallback"))
added.append(new_rel("F3-R14", "CHECKED_BY", "ac-ontology-fallback", C("TestProvidedOntologyUnverifiedRelationPreservesCKGAndCKVBaseline"), "ac-ontology-fallback", "verified concept + unverified relation; CKG-only citation preserved"))

claims = [{"claim_id": r["review_id"], "op": "reword" if r["recommendation"] == "modify" else "keep",
           "statement": r["proposed_change"] if r["recommendation"] == "modify" else None,
           "authority_kind": "repository_contract_or_ADR_not_organization_policy",
           "seed_fix": r["proposed_change"] if r["review_id"].endswith("P02") else None} for r in pre["claims"]]
legacy = [{"assertion_id": r["review_id"], "op": "retarget_object" if r["recommendation"] == "modify" else "keep",
           "to": "dataset" if r["recommendation"] == "modify" else None, "note": r["reason"]} for r in pre["legacy_support_about_relations"]]

criteria = {
  "ac-filtered-recall": {"full_proof": ["F3-T03"], "remaining_gap": "eligible count equals K (boundary of 'at least K')"},
  "ac-filtered-completeness": {"full_proof": ["F3-T05", "F3-R02", "F3-R04"], "partial": ["F3-T07"], "remaining_gap": None},
  "ac-source-evidence": {"full_proof": ["F3-T12"], "partial": ["F3-T10"], "remaining_gap": "CKV is a coordinate-only mock in F3-T12"},
  "ac-ontology-fallback": {"full_proof": ["F3-R14"], "partial": ["F3-T15", "F3-T17"], "remaining_gap": "F3-R14 composes VerifiedImplementations (F3-R11) through the resolver interface; no single real-store end-to-end case"},
}
out = {"schema_version": 1, "kind": "revised_proposal_after_user_adoption",
       "base_candidate": held["snapshot"], "source_revision_for_locations": subprocess.check_output(["git", "rev-parse", "HEAD"], text=True).strip(),
       "decision": "system/eval/b0-knowledge-system/refactoring-n12-revised-proposal-2026-10-09/decision.json",
       "functional_relation_ops": ops, "added_relations": added, "claims": claims, "legacy_relations": legacy,
       "criterion_coverage_after_revision": criteria,
       "all_status": "proposed", "reviewer": None, "verified_count": 0,
       "requires_before_verified": ["named reviewer (product ReviewedBy)", "new coordinated capture/CKG/CKV including the changed tests and gate change", "projection re-extraction and alignment checks"]}
(D / "revised-proposal.json").write_text(json.dumps(out, ensure_ascii=False, indent=1) + "\n")
print(len(ops), "ops;", len(added), "added;", sum(c["op"] == "reword" for c in claims), "claims reworded;", sum(l["op"] != "keep" for l in legacy), "legacy retargeted")
