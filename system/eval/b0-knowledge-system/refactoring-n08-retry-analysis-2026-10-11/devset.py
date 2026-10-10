"""Build the N08 retry development set: every observed non-ABS question from
N11 (DEV3 run question book) and FINAL2 A. Writes devset.json."""
import json
R="/private/tmp/ks-n11-eval-m13oocru"
out={}
for f in ("run-DEV3","run-F2A-FINAL"):
    for q in json.load(open(f"{R}/{f}/questions.json"))["questions"]:
        if q["group"]=="ABS" or q["id"] in out: continue
        out[q["id"]]={"id":q["id"],"group":q["group"],"origin":f,"prompt":q["prompt"],"evidence":[{"path":e["path"],"first":e.get("first",e.get("start_line")),"last":e.get("last",e.get("end_line"))} for e in q["evidence"]]}
json.dump(list(out.values()),open("devset.json","w"),ensure_ascii=False,indent=1)
print(len(out))
