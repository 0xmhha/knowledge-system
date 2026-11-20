"""Write the core-label review sheet of the oracle-seed-offline experiment.

For each question the sheet shows the symptom, the fix commit, every oracle
location (core first) and the fix diff hunks that touch each location at the
base commit. A location that no hunk touches is flagged: the fix did not change
it, which is a reason to doubt a core label.

Usage:
    python3 review_sheet.py --data <experiment data dir> --repo <go-stablenet> --out <sheet.md>
"""

from __future__ import annotations

import argparse
import json
import re
import subprocess
from dataclasses import dataclass
from pathlib import Path

GIT_TIMEOUT_SECONDS = 60
CONTEXT_LINES = 3
# A hunk counts for a location when its base range comes this close to it.
NEAR_LINES = 3
HUNK_RE = re.compile(r"^@@ -(\d+)(?:,(\d+))? \+\d+(?:,\d+)? @@")


@dataclass(frozen=True, slots=True)
class Hunk:
    start: int
    end: int
    text: str


def run_git(repo: Path, *args: str) -> str:
    """Run git in repo and return stdout."""
    out = subprocess.run(
        ["git", "-C", str(repo), *args],
        check=True,
        capture_output=True,
        text=True,
        timeout=GIT_TIMEOUT_SECONDS,
    )
    return out.stdout


def file_hunks(repo: Path, base: str, fix: str, path: str) -> list[Hunk]:
    """Return the hunks of base..fix for one file with their base line ranges."""
    diff = run_git(repo, "diff", f"-U{CONTEXT_LINES}", base, fix, "--", path)
    hunks: list[Hunk] = []
    cur: list[str] = []
    start = end = 0
    for line in diff.splitlines():
        m = HUNK_RE.match(line)
        if m:
            if cur:
                hunks.append(Hunk(start, end, "\n".join(cur)))
            start = int(m.group(1))
            count = int(m.group(2)) if m.group(2) is not None else 1
            end = start + max(count, 1) - 1
            cur = [line]
        elif cur:
            cur.append(line)
    if cur:
        hunks.append(Hunk(start, end, "\n".join(cur)))
    return hunks


def touching(hunks: list[Hunk], start: int, end: int) -> list[Hunk]:
    """Return the hunks whose base range overlaps [start, end] within NEAR_LINES."""
    return [h for h in hunks if h.start <= end + NEAR_LINES and h.end >= start - NEAR_LINES]


def load(path: Path) -> dict:
    return json.loads(path.read_text(encoding="utf-8"))


def question_section(repo: Path, q: dict, prompt: str, base: dict, seeds: list[str]) -> list[str]:
    """Render one question as Markdown lines."""
    fix_subject = run_git(repo, "log", "-1", "--format=%s", base["fix"]).strip()
    stat = run_git(repo, "diff", "--stat=120", base["base"], base["fix"]).rstrip()
    out = [
        f"## {q['id']}",
        "",
        f"- 증상: {prompt}",
        f"- 수정 커밋: `{base['fix'][:12]}` {fix_subject}",
        f"- 기준 커밋: `{base['base'][:12]}`",
        f"- blind 출발 심볼: {', '.join(f'`{s}`' for s in seeds) or '없음'}",
        f"- 모델이 매긴 확신도: {q.get('confidence', '?')}",
        "",
        "<details><summary>바뀐 파일 전체</summary>",
        "",
        "```text",
        stat,
        "```",
        "",
        "</details>",
        "",
        "| # | 지금 라벨 | 위치(기준 커밋) | 심볼 | 이 위치를 바꾼 hunk | 결정 |",
        "|---|---|---|---|---|---|",
    ]
    cites = sorted(q["citations"], key=lambda c: (not c["core"], c["file"], c["start_line"]))
    cache: dict[str, list[Hunk]] = {}
    details: list[str] = []
    for i, c in enumerate(cites, 1):
        hunks = cache.setdefault(c["file"], file_hunks(repo, base["base"], base["fix"], c["file"]))
        hit = touching(hunks, c["start_line"], c["end_line"])
        label = "**core**" if c["core"] else "related"
        mark = f"{len(hit)}개" if hit else "**없음**"
        where = f"`{c['file']}:{c['start_line']}-{c['end_line']}`"
        out.append(f"| {i} | {label} | {where} | `{c['symbol']}` | {mark} | |")
        if c["core"]:
            details += [f"### {q['id']} #{i} `{c['symbol']}` (core)", ""]
            if not hit:
                details += ["수정 커밋은 이 위치를 바꾸지 않았다.", ""]
            for h in hit:
                details += ["```diff", h.text, "```", ""]
    out += ["", "결정 칸에는 `유지`, `related로`, `core로` 중 하나를 쓴다.", ""]
    return out + details


def main() -> None:
    ap = argparse.ArgumentParser(description=__doc__.splitlines()[0])
    ap.add_argument("--data", type=Path, required=True)
    ap.add_argument("--repo", type=Path, required=True)
    ap.add_argument("--out", type=Path, required=True)
    args = ap.parse_args()

    oracle = load(args.data / "oracle-relayed.json")["questions"]
    prompts = {q["id"]: q["prompt"] for q in load(args.data / "questions-symptom.json")["questions"]}
    bases = {b["id"]: b for b in load(args.data / "base-commits.json")["bases"]}
    seeds = {s["id"]: [x["symbol"] for x in s["seeds"]] for s in load(args.data / "seeds-blind.json")["seeds"]}

    lines = [
        "# core 라벨 검토표",
        "",
        "문항마다 core 위치가 버그의 원인인지 정한다. 기준은 \"시니어가 이 증상을 받으면 결국 찾아내야 하는 곳\"이다.",
        "core 위치의 diff는 표 아래에 있다. related 위치는 표에만 있다. 바뀐 hunk가 **없음**인 core는 특히 의심한다.",
        "",
    ]
    for q in oracle:
        lines += question_section(args.repo, q, prompts[q["id"]], bases[q["id"]], seeds[q["id"]])
    args.out.parent.mkdir(parents=True, exist_ok=True)
    args.out.write_text("\n".join(lines) + "\n", encoding="utf-8")


if __name__ == "__main__":
    main()
