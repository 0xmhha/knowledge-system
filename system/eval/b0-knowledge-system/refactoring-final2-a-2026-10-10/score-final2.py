#!/usr/bin/env python3
"""Score FINAL2 part A (candidate vs reference) and part B, then write the
frozen-rule verdict summary. Scores are written with O_EXCL by the summarizer;
existing outputs are refused. Run from the repository root:

  python3 .../score-final2.py <output-dir>
"""
import gzip, json, statistics, subprocess, sys
from pathlib import Path

ROOT = Path.cwd()
A = ROOT / "system/eval/b0-knowledge-system/refactoring-final2-a-2026-10-10"
B = ROOT / "system/eval/b0-knowledge-system/refactoring-final2-b-2026-10-10"
RA = Path("/private/tmp/ks-n11-eval-m13oocru")
RB = Path(json.loads((B / "preparation.json").read_text())["base"])
OUT = Path(sys.argv[1]); OUT.mkdir(parents=True, exist_ok=True)
SCORER = ROOT / "scripts/refactoring-eval-summarize.py"

def score(name, run, source, split, reference):
    out = OUT / f"{name}-score.json"
    argv = ["python3", str(SCORER), "--capture-dir", str(run / "capture"), "--questions", str(run / "questions.json"),
            "--protocol", str(run / "protocol.json"), "--source-root", str(source), "--split", split, "--output", str(out)]
    if reference:
        argv += ["--reference-capture-dir", str(run / "reference-capture")]
    subprocess.run(argv, check=True)
    return json.loads(out.read_text())

def arm_quality(summary, arm):
    pq = summary["per_question"][arm]
    return {m: statistics.mean(v[m] for v in pq.values()) for m in ("citation_recall_at_10", "mean_expected_reciprocal_rank_at_10")}

result = {"schema_version": 1}
for split in ("DEV", "FINAL"):
    s = score(f"a-{split.lower()}", RA / f"run-F2A-{split}", RA / "source", split, True)
    ratios = {a: v["warm_p95_ratio_to_baseline_off"] for a, v in s["arm_summaries"].items()}
    result[f"A_{split}"] = {
        "status": s["status"], "reference_status": s["reference"]["status"],
        "rows": s["observed_rows"], "failed_rows": sum(v["failed_rows"] for v in s["arm_summaries"].values()),
        "candidate_warm_p95_ratio": ratios,
        "reference_warm_p95_ratio": {a: v["warm_p95_ratio_to_baseline_off"] for a, v in s["reference"]["arm_summaries"].items()},
        "candidate_quality_baseline_off": arm_quality(s, "baseline_off"),
        "reference_quality_baseline_off": arm_quality(s["reference"], "baseline_off"),
        "binary_comparison_baseline_off": s["binary_comparison"]["baseline_off"],
        "candidate_to_reference_warm_p95": {a: s["binary_comparison"][a]["warm_p95_ratio_candidate_to_reference"] for a in ratios},
        "groups": s["frozen_threshold_application"]["groups"],
    }
fa = result["A_FINAL"]
over = sorted(a for a, r in fa["candidate_warm_p95_ratio"].items() if a != "baseline_off" and (r is None or r > 1.25))
result["N10_verdict"] = {"rule": "candidate run, every non-baseline arm warm p95 / baseline_off <= 1.25",
                         "arms_over": over, "met": fa["status"] == "descriptive" and not over}
allq = fa["binary_comparison_baseline_off"]["all"]
result["N08_verdict"] = {"rule": "candidate-minus-reference on the same FINAL questions; every group below ten units",
                         "all_recall_delta": allq["citation_recall_at_10"], "all_mrr_delta": allq["mean_expected_reciprocal_rank_at_10"],
                         "verdict": "inconclusive (small groups); descriptive only"}
for split in ("DEV", "FINAL"):
    s = score(f"b-{split.lower()}", RB / f"run-F2B-{split}", RB / "source", split, False)
    result[f"B_{split}"] = {"status": s["status"], "rows": s["observed_rows"],
                            "abstention_mechanical": s["abstention_mechanical"],
                            "per_question_baseline_on": s["per_question"]["baseline_on"],
                            "per_question_baseline_off": s["per_question"]["baseline_off"]}
bf = result["B_FINAL"]["abstention_mechanical"]
result["N09_verdict"] = {"rule": "ABS off-basis citation rate, knowledge on vs off; contrast recall on vs off; mechanical only",
                         "abs_off_basis_rate": {a: bf[a]["off_basis_citation_rate"] for a in bf},
                         "contrast_recall_on": {q: v["citation_recall_at_10"] for q, v in result["B_FINAL"]["per_question_baseline_on"].items() if "ANS" in q},
                         "contrast_recall_off": {q: v["citation_recall_at_10"] for q, v in result["B_FINAL"]["per_question_baseline_off"].items() if "ANS" in q},
                         "answer_level_abstention": None, "verdict": "inconclusive (two ABS units); descriptive only"}
(OUT / "final2-verdicts.json").write_text(json.dumps(result, ensure_ascii=False, indent=1) + "\n")
print(json.dumps({k: result[k] for k in ("N10_verdict", "N08_verdict", "N09_verdict")}, ensure_ascii=False, indent=1))
