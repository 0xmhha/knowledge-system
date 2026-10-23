"""Turn merged PRs into answer-known `cks eval` scenarios (docs/spec-driven/RETRIEVAL-EVAL-SPEC.md).

`collect` reads merged PRs, applies the fixed exclusion rules, takes each PR's
answer ranges from the pre-merge source with the Go standard parser
(scripts/retrieval_declspans.go), and assigns every PR to a shared index snapshot whose answer
bytes are identical. `assemble` joins that sample with human-language prompts,
rejects prompts that name the answer, and writes one scenario YAML per PR.
"""
from __future__ import annotations

import argparse
import json
import re
import subprocess
import sys
from collections.abc import Iterable, Mapping, Sequence
from dataclasses import asdict, dataclass, field
from pathlib import Path

GIT_TIMEOUT_SECONDS = 120
GH_TIMEOUT_SECONDS = 300
MAX_CODE_FILES = 10
MAX_SAMPLE = 100
MAX_PER_INTENT = 30
SNAPSHOT_GROUPS = 5
PR_FETCH_LIMIT = 2000
# PRs merged anywhere else are cherry-picks carrying another PR's change.
DEFAULT_BRANCH = 'master'

# Fixed label → intent map from the spec; a PR whose labels map to more than
# one intent has no single answer and is excluded.
LABEL_INTENT = {
    'Type: Bug': 'bug_fix',
    'Type: Feature': 'feature_add',
    'Type: API Change': 'feature_add',
    'Type: Behavior Change': 'feature_add',
    'Type: Internal Cleanup': 'refactor',
    'Type: Testing': 'test_add',
    'Type: Security': 'security',
    'Type: Documentation': 'docs_update',
}
# Round-robin order for sampling; fixed so the sample does not depend on
# dictionary iteration.
INTENT_ORDER = ('bug_fix', 'feature_add', 'refactor', 'test_add', 'security', 'docs_update')
# Intents whose answer may be test or documentation files.
NON_CODE_INTENTS = {'test_add', 'docs_update'}
EXCLUDED_TITLE = re.compile(r'^\s*(cherry[- ]?pick|ci:|deps:|build\(deps\)|release)', re.I)
GENERATED_MARK = 'Code generated'
HUNK = re.compile(r'^@@ -(\d+)(?:,(\d+))? \+\d+(?:,\d+)? @@')
IDENT = re.compile(r'[A-Za-z_][A-Za-z0-9_]*')
MIN_LEAK_LEN = 3


class ConversionError(Exception):
    """A PR or prompt set that cannot be turned into a valid scenario."""


@dataclass(frozen=True, slots=True)
class Decl:
    kind: str
    name: str
    recv: str
    start_line: int
    end_line: int
    anchor: str
    text: str = ''

    def key(self) -> tuple[str, str, str]:
        return (self.kind, self.recv, self.name)


@dataclass(frozen=True, slots=True)
class FileChange:
    path: str
    old_lines: tuple[int, ...]
    insert_after: tuple[int, ...]
    added: bool
    deleted: bool


@dataclass(slots=True)
class Truth:
    file: str
    kind: str
    name: str
    recv: str
    start_line: int
    end_line: int
    anchor: str


@dataclass(slots=True)
class Candidate:
    number: int
    title: str
    body: str
    intent: str
    merge_commit: str
    base_commit: str
    base_time: int
    truth: list[Truth] = field(default_factory=list)
    extra_files: dict[str, list[str]] = field(default_factory=dict)
    snapshot: str = ''


# --- pure rules (unit-tested) ---

def intent_for(labels: Iterable[str]) -> str | None:
    """Return the single intent the labels map to, or None."""
    found = {LABEL_INTENT[name] for name in labels if name in LABEL_INTENT}
    return found.pop() if len(found) == 1 else None


def file_role(path: str, generated: bool) -> str:
    """Classify a changed path as code, test, generated, doc or other."""
    if path.endswith('_test.go'):
        return 'test'
    if path.endswith('.go'):
        return 'generated' if generated or path.endswith('.pb.go') else 'code'
    if path.endswith('.md'):
        return 'doc'
    return 'other'


def parse_diff(diff: str) -> list[FileChange]:
    """Parse `git diff -U0 --no-renames` output into pre-image line sets."""
    changes: list[FileChange] = []
    path, old, after, added, deleted = None, [], [], False, False

    def flush() -> None:
        if path is not None:
            changes.append(FileChange(path, tuple(old), tuple(after), added, deleted))

    for line in diff.splitlines():
        if line.startswith('diff --git '):
            flush()
            path = line.split(' b/', 1)[1]
            old, after, added, deleted = [], [], False, False
        elif line.startswith('new file mode'):
            added = True
        elif line.startswith('deleted file mode'):
            deleted = True
        elif match := HUNK.match(line):
            start, count = int(match.group(1)), int(match.group(2) or '1')
            if count == 0:
                after.append(start)
            else:
                old.extend(range(start, start + count))
    flush()
    return changes


def touched_decls(decls: Sequence[Decl], change: FileChange) -> list[Decl]:
    """Return declarations whose pre-image range a change edits.

    A pure insertion counts only when it lands inside a declaration, i.e.
    both neighbouring old lines belong to it.
    """
    out = []
    for d in decls:
        inside = any(d.start_line <= n <= d.end_line for n in change.old_lines)
        inserted = any(d.start_line <= n and n + 1 <= d.end_line for n in change.insert_after)
        if inside or inserted:
            out.append(d)
    return out


def exclusion_reason(title: str, intent: str | None, roles: Mapping[str, int], base_branch: str = DEFAULT_BRANCH) -> str | None:
    """Apply the spec's fixed exclusion rules to one PR."""
    if base_branch != DEFAULT_BRANCH:
        return 'cherry-pick to a release branch'
    if intent is None:
        return 'no single mapped Type label'
    if EXCLUDED_TITLE.search(title):
        return 'cherry-pick, ci, dependency or release PR'
    if roles.get('code', 0) > MAX_CODE_FILES:
        return f'more than {MAX_CODE_FILES} non-test Go files'
    if intent not in NON_CODE_INTENTS and roles.get('code', 0) == 0:
        return 'generated-only' if roles.get('generated', 0) else 'no non-test Go file changed'
    return None


def sample(candidates: Sequence[Candidate]) -> list[Candidate]:
    """Pick up to MAX_SAMPLE PRs round-robin by intent, ascending PR number."""
    queues = {i: sorted((c for c in candidates if c.intent == i), key=lambda c: c.number) for i in INTENT_ORDER}
    picked: list[Candidate] = []
    taken = dict.fromkeys(INTENT_ORDER, 0)
    while len(picked) < MAX_SAMPLE:
        progressed = False
        for intent in INTENT_ORDER:
            if len(picked) == MAX_SAMPLE:
                break
            if queues[intent] and taken[intent] < MAX_PER_INTENT:
                picked.append(queues[intent].pop(0))
                taken[intent] += 1
                progressed = True
        if not progressed:
            break
    return sorted(picked, key=lambda c: c.number)


def snapshot_groups(candidates: Sequence[Candidate], groups: int = SNAPSHOT_GROUPS) -> list[list[Candidate]]:
    """Split candidates by base time into contiguous groups."""
    ordered = sorted(candidates, key=lambda c: (c.base_time, c.number))
    if not ordered:
        return []
    size = -(-len(ordered) // groups)
    return [ordered[i:i + size] for i in range(0, len(ordered), size)]


def leaked_terms(prompt: str, truth: Sequence[Truth]) -> list[str]:
    """Return answer names that appear in the prompt (case-insensitive).

    Names are declaration, receiver and changed-file stems. Package path
    words are allowed by the spec, so a file stem equal to its directory
    name (gzip/gzip.go) is treated as the package word and not checked.
    """
    names: set[str] = set()
    for t in truth:
        path = Path(t.file)
        stem = '' if path.stem == path.parent.name else path.stem
        names.update(n for n in (t.name, t.recv, stem) if len(n) >= MIN_LEAK_LEN)
    words = {w.lower() for w in IDENT.findall(prompt)}
    lowered = prompt.lower()
    hits = []
    for n in sorted(names):
        if n.lower() in words or (len(n) > MIN_LEAK_LEN and n.lower() in lowered):
            hits.append(n)
    return hits


# --- git / gh boundary ---

def run(args: Sequence[str], cwd: Path | None = None, stdin: str | None = None, timeout: int = GIT_TIMEOUT_SECONDS) -> str:
    result = subprocess.run(list(args), cwd=cwd, input=stdin, capture_output=True, text=True, timeout=timeout, check=False)
    if result.returncode != 0:
        raise ConversionError(f'{args[0]} {args[1]} failed: {result.stderr.strip()[:300]}')
    return result.stdout


def git(repo: Path, *args: str, stdin: str | None = None) -> str:
    return run(['git', '-C', str(repo), *args], stdin=stdin)


def show(repo: Path, rev: str, path: str) -> str | None:
    result = subprocess.run(['git', '-C', str(repo), 'show', f'{rev}:{path}'], capture_output=True, text=True,
                            timeout=GIT_TIMEOUT_SECONDS, check=False)
    return result.stdout if result.returncode == 0 else None


def decls_of(source: str, declspans: Path) -> list[Decl]:
    out = run([str(declspans)], stdin=source)
    lines = source.split('\n')
    decls = []
    for row in out.splitlines():
        d = json.loads(row)
        text = '\n'.join(lines[d['start_line'] - 1:d['end_line']])
        decls.append(Decl(d['kind'], d['name'], d.get('recv', ''), d['start_line'], d['end_line'], d['anchor'], text))
    return decls


def fetch_prs(repo_slug: str, since: str, until: str) -> list[dict]:
    fields = 'number,title,body,labels,mergeCommit,mergedAt,baseRefName'
    out = run(['gh', 'pr', 'list', '-R', repo_slug, '--state', 'merged', '--search', f'merged:{since}..{until}',
               '--limit', str(PR_FETCH_LIMIT), '--json', fields], timeout=GH_TIMEOUT_SECONDS)
    return json.loads(out)


def build_candidate(pr: dict, repo: Path, declspans: Path) -> tuple[Candidate | None, str | None]:
    labels = [label['name'] for label in pr.get('labels', [])]
    intent = intent_for(labels)
    merge = (pr.get('mergeCommit') or {}).get('oid', '')
    if not merge:
        return None, 'no merge commit'
    base = git(repo, 'rev-parse', f'{merge}^1').strip()
    changes = parse_diff(git(repo, 'diff', '-U0', '--no-renames', base, merge))
    roles: dict[str, int] = {}
    files: dict[str, list[str]] = {}
    sources: dict[str, str] = {}
    for ch in changes:
        source = '' if ch.added else (show(repo, base, ch.path) or '')
        role = file_role(ch.path, GENERATED_MARK in source[:2000])
        if role == 'code' and ch.added:
            role = 'added'
        roles[role] = roles.get(role, 0) + 1
        files.setdefault(role, []).append(ch.path)
        sources[ch.path] = source
    reason = exclusion_reason(pr['title'], intent, roles, pr.get('baseRefName', DEFAULT_BRANCH))
    if reason:
        return None, reason
    answer_roles = {'code'} | ({'test'} if intent == 'test_add' else set())
    truth = []
    for ch in changes:
        if ch.added or ch.deleted or file_role(ch.path, False) not in answer_roles or not ch.path.endswith('.go'):
            continue
        for d in touched_decls(decls_of(sources[ch.path], declspans), ch):
            truth.append(Truth(ch.path, d.kind, d.name, d.recv, d.start_line, d.end_line, d.anchor))
    if not truth:
        return None, 'no pre-existing declaration changed'
    base_time = int(git(repo, 'show', '-s', '--format=%ct', base).strip())
    extra = {k: v for k, v in files.items() if k != 'code'}
    return Candidate(pr['number'], pr['title'], pr.get('body') or '', intent, merge, base, base_time, truth, extra), None


def moved_truth(c: Candidate, snapshot: str, repo: Path, declspans: Path) -> list[Truth] | None:
    """Return c's answer at snapshot lines, or None unless every answer is byte-identical there."""
    moved = []
    for t in c.truth:
        source = show(repo, snapshot, t.file)
        if source is None:
            return None
        base = show(repo, c.base_commit, t.file) or ''
        want = next(d for d in decls_of(base, declspans) if d.start_line == t.start_line and d.key() == (t.kind, t.recv, t.name))
        same = [d for d in decls_of(source, declspans) if d.key() == want.key() and d.text == want.text]
        if len(same) != 1:
            return None
        moved.append(Truth(t.file, t.kind, t.name, t.recv, same[0].start_line, same[0].end_line, same[0].anchor))
    return moved


def pin_snapshots(picked: Sequence[Candidate], repo: Path, declspans: Path) -> None:
    """Attach each PR to the newest shared snapshot not after its base where its answer bytes match.

    Shared snapshots are the earliest base of each time group. A PR that
    matches none is indexed at its own base, so no index ever holds code
    from after the PR's base.
    """
    shared = sorted(({'commit': min(g, key=lambda c: (c.base_time, c.number)).base_commit,
                      'time': min(c.base_time for c in g)} for g in snapshot_groups(picked)),
                    key=lambda s: s['time'], reverse=True)
    for c in picked:
        c.snapshot = c.base_commit
        for snap in shared:
            if snap['time'] > c.base_time:
                continue
            moved = moved_truth(c, snap['commit'], repo, declspans)
            if moved is not None:
                c.truth, c.snapshot = moved, snap['commit']
                break


def collect(args: argparse.Namespace) -> None:
    repo, declspans = Path(args.repo), Path(args.declspans)
    prs = fetch_prs(args.repo_slug, args.since, args.until)
    candidates, excluded = [], []
    for pr in sorted(prs, key=lambda p: p['number']):
        cand, reason = build_candidate(pr, repo, declspans)
        if cand is None:
            excluded.append({'number': pr['number'], 'title': pr['title'], 'reason': reason})
        else:
            candidates.append(cand)
    picked = sample(candidates)
    pin_snapshots(picked, repo, declspans)
    out = {'repo': args.repo_slug, 'since': args.since, 'until': args.until, 'fetched': len(prs),
           'eligible': len(candidates), 'excluded': excluded, 'sample': [asdict(c) for c in picked],
           'snapshots': sorted({c.snapshot for c in picked})}
    Path(args.out).write_text(json.dumps(out, indent=2, ensure_ascii=False) + '\n')
    print(json.dumps({k: out[k] if not isinstance(out[k], list) else len(out[k]) for k in out}, ensure_ascii=False))


def scenario_yaml(c: Mapping, prompt: str) -> str:
    lines = ['version: 1', f'name: pr-{c["number"]}', f'description: {json.dumps(c["title"], ensure_ascii=False)}',
             f'prompt: {json.dumps(prompt, ensure_ascii=False)}', f'expected_intent: {c["intent"]}',
             f'expected_commit: {c["snapshot"]}', 'expected_citations:']
    for t in c['truth']:
        lines += [f'  - file: {t["file"]}', f'    start_line: {t["start_line"]}', f'    end_line: {t["end_line"]}',
                  f'    anchor: {json.dumps(t["anchor"], ensure_ascii=False)}']
    return '\n'.join(lines) + '\n'


def assemble(args: argparse.Namespace) -> None:
    data = json.loads(Path(args.sample).read_text())
    prompts: dict[str, str] = json.loads(Path(args.prompts).read_text())
    out = Path(args.out)
    out.mkdir(parents=True, exist_ok=False)
    problems = []
    for c in data['sample']:
        prompt = prompts.get(str(c['number']), '').strip()
        if not prompt:
            problems.append(f'pr-{c["number"]}: missing prompt')
            continue
        leaks = leaked_terms(prompt, [Truth(**t) for t in c['truth']])
        if leaks:
            problems.append(f'pr-{c["number"]}: prompt names {", ".join(leaks)}')
            continue
        by_snapshot = out / c['snapshot']
        by_snapshot.mkdir(exist_ok=True)
        (by_snapshot / f'pr-{c["number"]}.yaml').write_text(scenario_yaml(c, prompt))
    if problems:
        raise ConversionError('\n'.join(problems))


def main(argv: Sequence[str] | None = None) -> int:
    parser = argparse.ArgumentParser(description=__doc__.splitlines()[0])
    sub = parser.add_subparsers(dest='command', required=True)
    c = sub.add_parser('collect')
    c.add_argument('--repo', required=True)
    c.add_argument('--repo-slug', required=True)
    c.add_argument('--since', required=True)
    c.add_argument('--until', required=True)
    c.add_argument('--declspans', required=True)
    c.add_argument('--out', required=True)
    a = sub.add_parser('assemble')
    a.add_argument('--sample', required=True)
    a.add_argument('--prompts', required=True)
    a.add_argument('--out', required=True)
    args = parser.parse_args(argv)
    try:
        {'collect': collect, 'assemble': assemble}[args.command](args)
    except ConversionError as error:
        print(error, file=sys.stderr)
        return 1
    return 0


if __name__ == '__main__':
    sys.exit(main())
