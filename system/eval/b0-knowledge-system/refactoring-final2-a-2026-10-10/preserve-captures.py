#!/usr/bin/env python3
"""Copy the FINAL2 run directories into the repository in the N11 layout:
run-level JSON files as is, each capture's configs/report/locked inputs as is,
rows and per-arm footprints gzip-compressed (mtime 0 for stable bytes).
Refuses to overwrite. Run from the repository root."""
import gzip, json, shutil
from pathlib import Path
A = Path("system/eval/b0-knowledge-system/refactoring-final2-a-2026-10-10")
B = Path("system/eval/b0-knowledge-system/refactoring-final2-b-2026-10-10")
RB = Path(json.loads((B / "preparation.json").read_text())["base"])
RUNS = [(Path("/private/tmp/ks-n11-eval-m13oocru/run-F2A-DEV"), A / "dev-run"), (Path("/private/tmp/ks-n11-eval-m13oocru/run-F2A-FINAL"), A / "final-run"),
        (RB / "run-F2B-DEV", B / "dev-run"), (RB / "run-F2B-FINAL", B / "final-run")]
def gz(src, dst):
    with src.open("rb") as i, gzip.GzipFile(dst, "xb", mtime=0) as o: shutil.copyfileobj(i, o)
for src, dst in RUNS:
    dst.mkdir()
    for p in sorted(src.iterdir()):
        if p.is_file(): shutil.copy2(p, dst / p.name)
    for cap in ("capture", "reference-capture"):
        c = src / cap
        if not c.exists(): continue
        d = dst / cap; d.mkdir()
        for p in sorted(c.iterdir()):
            if p.name == "rows.jsonl": gz(p, d / "rows.jsonl.gz")
            elif p.name == "footprints":
                for arm in sorted(p.iterdir()):
                    files = list(arm.iterdir()); assert [f.name for f in files] == ["cks-mcp.jsonl"], files
                    gz(files[0], d / f"footprint-{arm.name}.jsonl.gz")
            elif p.is_dir(): shutil.copytree(p, d / p.name)
            else: shutil.copy2(p, d / p.name)
    print(dst, sum(f.stat().st_size for f in dst.rglob("*") if f.is_file()))
