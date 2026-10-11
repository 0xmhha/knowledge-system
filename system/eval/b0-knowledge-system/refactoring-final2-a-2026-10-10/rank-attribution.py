#!/usr/bin/env python3
"""For each FINAL question, compare the retrieval-phase baseline_off citation
list of the candidate and reference captures: is the candidate list the
reference list with extra citations inserted (type-declaration insertion), and
do any inserted citations sit above the first gold-overlapping rank? Also
check that every row of a question/arm/binary returns the same citation list.
Run from the repository root; writes rank-attribution.json."""
import gzip, json
from pathlib import Path
A = Path("system/eval/b0-knowledge-system/refactoring-final2-a-2026-10-10")
def lists(cap):
    out = {}
    with gzip.open(A / "final-run" / cap / "rows.jsonl.gz", "rt") as f:
        for line in f:
            r = json.loads(line)
            if r["phase"] != "retrieval" or r["arm"] != "baseline_off": continue
            sc = r["call"]["response"]["structuredContent"]
            out.setdefault(r["request_id"], set()).add(tuple((c["file"], c["start_line"], c["end_line"]) for c in sc["citations"]))
    return out
cand, ref = lists("capture"), lists("reference-capture")
questions = {q["id"]: q for q in json.loads((A / "final-run/questions.json").read_text())["questions"]}
def gold_rank(cits, q):
    for i, (f, s, e) in enumerate(cits, 1):
        if any(f == ev["path"] and s <= ev["last"] and e >= ev["first"] for ev in q["evidence"]): return i
result = {}
for qid in sorted(cand):
    assert len(cand[qid]) == 1 and len(ref[qid]) == 1, qid
    c, r = list(next(iter(cand[qid]))), list(next(iter(ref[qid])))
    extra = [x for x in c if x not in r]
    subseq = [x for x in c if x in r] == r[:len([x for x in c if x in r])]
    gc, gr = gold_rank(c, questions[qid]), gold_rank(r, questions[qid])
    result[qid] = {"candidate_gold_rank": gc, "reference_gold_rank": gr, "reference_order_kept": subseq,
                   "dropped_from_reference": [list(x) for x in r if x not in c],
                   "inserted": [list(x) for x in extra], "inserted_above_gold": [list(x) for x in extra if gc and c.index(x) < gc - 1]}
(A / "rank-attribution.json").write_text(json.dumps(result, indent=1) + "\n")
for k, v in result.items(): print(k, v["reference_gold_rank"], "->", v["candidate_gold_rank"], "kept", v["reference_order_kept"], "ins", len(v["inserted"]), "above", [x[0].split('/')[-1]+":"+str(x[1]) for x in v["inserted_above_gold"]], "dropped", len(v["dropped_from_reference"]))
