#!/usr/bin/env python3
"""Export approved B0 gold to the existing cks eval scenario format."""

import argparse
import hashlib
import importlib.util
import json
import os
from pathlib import Path
import shutil
import sys
import tempfile


ROOT = Path(__file__).resolve().parent.parent
SOURCE = Path(__file__).with_name("b0-preflight.py")
SPEC = importlib.util.spec_from_file_location("b0_preflight", SOURCE)
PREFLIGHT = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(PREFLIGHT)


def export(questions_path, output_dir):
    raw = questions_path.read_bytes()
    checked = PREFLIGHT.validate_questions(raw)
    book = json.loads(raw)
    if checked["approved_count"] != checked["question_count"]:
        raise ValueError("B0 gold answers need human approval before scenario export")
    if output_dir.exists():
        raise ValueError("scenario output directory already exists")
    output_dir.parent.mkdir(parents=True, exist_ok=True)
    temp = Path(tempfile.mkdtemp(prefix=".b0-scenarios-", dir=output_dir.parent))
    try:
        for question in book["questions"]:
            scenario = {"version": 1, "name": question["id"].lower(),
                        "prompt": question["prompt"], "runs": 5,
                        "match_mode": "overlap", "expected_commit": checked["corpus_commit"]}
            if question["expected_behavior"] == "abstain":
                scenario["expect_no_citations"] = True
            else:
                scenario["expected_citations"] = [
                    {"file": ref["path"], "start_line": ref["start_line"],
                     "end_line": ref["end_line"], "anchor": ref["anchor"]}
                    for ref in question["evidence"]]
            # JSON is accepted by the YAML v1 scenario decoder. Do not include
            # candidate answers in files handed to the MCP evaluation runner.
            (temp / (question["id"].lower() + ".yaml")).write_text(
                json.dumps(scenario, ensure_ascii=False, indent=2) + "\n")
        (temp / "manifest.json").write_text(json.dumps({
            "schema_version": 1, "corpus_commit": checked["corpus_commit"],
            "corpus_tree": checked["corpus_tree"],
            "question_set_sha256": hashlib.sha256(raw).hexdigest(),
            "scenario_count": checked["question_count"]}, indent=2) + "\n")
        os.rename(temp, output_dir)
    except BaseException:
        shutil.rmtree(temp, ignore_errors=True)
        raise


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--questions", type=Path, default=PREFLIGHT.DEFAULT_QUESTIONS)
    parser.add_argument("--out-dir", type=Path, required=True)
    args = parser.parse_args()
    export(args.questions, args.out_dir)


if __name__ == "__main__":
    try:
        main()
    except (OSError, ValueError) as exc:
        print(f"b0-export-scenarios: {exc}", file=sys.stderr)
        sys.exit(1)
