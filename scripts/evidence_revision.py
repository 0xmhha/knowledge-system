"""Run a historical stage test at the revision its packet was verified at.

A stage packet binds the live tree as it stood when the packet was produced:
source and tool digests, the formal acceptance state and ledger references.
Later stages change those inputs on purpose, so checking an old packet against
the current tree reports drift, not a defect in the evidence. A historical test
module therefore ends with

    load_tests = evidence_revision.tests(__file__)

and its tests are loaded from a detached worktree at the evidence-history
commit recorded in evidence-revisions.json, where every bound input is the one
the packet saw. The current checkout must still carry the same evidence: no
file under a recorded packet path may be modified or removed since that
commit, and a difference fails the module instead of being skipped. The commits live in the evidence tags, which the
machine handoff restores first.
"""
import contextlib
import importlib.abc
import importlib.machinery
import importlib.util
import json
from pathlib import Path
import shutil
import subprocess
import sys
import tempfile
from types import ModuleType
from typing import Iterator
import unittest

ROOT = Path(__file__).resolve().parents[1]
SCRIPTS = ROOT / 'scripts'
REVISIONS = SCRIPTS / 'evidence-revisions.json'


class RevisionUnavailable(RuntimeError):
    """The recorded commit or its packet cannot be reproduced from this checkout."""


def _git(*args: str) -> subprocess.CompletedProcess:
    return subprocess.run(['git', *args], cwd=ROOT, capture_output=True, text=True, timeout=300)


def revisions(test_module: str) -> list[dict]:
    """Return the recorded runs of a test module.

    A module normally has one run. It has several when its packet was verified
    at one commit and later tests of unrelated behaviour were added at another;
    each run then names the tests it owns with "only" or "exclude".
    """
    rows = json.loads(REVISIONS.read_text())['modules']
    if test_module not in rows:
        raise RevisionUnavailable('no evidence revision recorded for ' + test_module)
    return rows[test_module].get('runs', [rows[test_module]])


@contextlib.contextmanager
def checkout(commit: str) -> Iterator[Path]:
    """Yield a detached worktree at commit and remove it afterwards."""
    if _git('cat-file', '-e', commit + '^{commit}').returncode:
        raise RevisionUnavailable(commit + ' is not present; fetch the evidence tags first')
    base = Path(tempfile.mkdtemp(prefix='ks-evidence-revision-'))
    tree = base / 'tree'
    try:
        result = _git('worktree', 'add', '--detach', '--quiet', str(tree), commit)
        if result.returncode:
            raise RevisionUnavailable('cannot check out ' + commit + ': ' + result.stderr.strip())
        # Built binaries are untracked; tests that run them use the current build.
        if (ROOT / 'bin').is_dir():
            (tree / 'bin').symlink_to(ROOT / 'bin')
        yield tree.resolve()
    finally:
        _git('worktree', 'remove', '--force', str(tree))
        shutil.rmtree(base, ignore_errors=True)
        _git('worktree', 'prune')


def unchanged(commit: str, paths: list[str]) -> None:
    """Raise unless every file under paths at commit is unmodified in the checkout.

    Later stages may add files beside a packet, such as an adoption record, but
    no file the historical test verified may be modified or removed.
    """
    for path in paths:
        if _git('cat-file', '-e', commit + ':' + path).returncode:
            raise RevisionUnavailable(path + ' is absent at ' + commit)
        if _git('diff', '--quiet', '--diff-filter=MDT', commit, '--', path).returncode:
            raise RevisionUnavailable(path + ' differs from the evidence verified at ' + commit)


def _restore(tree: Path, commit: str, paths: list[str]) -> None:
    result = subprocess.run(['git', 'checkout', commit, '--', *paths], cwd=tree, capture_output=True, text=True, timeout=300)
    if result.returncode:
        raise RevisionUnavailable('cannot restore ' + ', '.join(paths) + ' from ' + commit + ': ' + result.stderr.strip())


def _modules_in(folder: Path) -> list[str]:
    names = []
    for name, module in list(sys.modules.items()):
        origin = getattr(module, '__file__', None)
        if origin and Path(origin).resolve().parent == folder:
            names.append(name)
    return names


def _exec(name: str, path: Path) -> ModuleType:
    spec = importlib.util.spec_from_file_location(name, path)
    module = importlib.util.module_from_spec(spec)
    sys.modules[name] = module
    spec.loader.exec_module(module)
    return module


class _PortableLoader(importlib.machinery.SourceFileLoader):
    """Run the current checkout's source under the recorded file's location."""

    def __init__(self, fullname: str, recorded: Path, current: Path):
        super().__init__(fullname, str(recorded))
        self.current = current

    def get_code(self, fullname: str):
        return compile(self.current.read_bytes(), str(self.current), 'exec', dont_inherit=True)


class _PortableFinder(importlib.abc.MetaPathFinder):
    def __init__(self, scripts: Path, portable: list[str]):
        self.scripts = scripts
        self.portable = portable

    def find_spec(self, fullname, path=None, target=None):
        name = fullname + '.py'
        if name not in self.portable or not (self.scripts / name).is_file():
            return None
        return _ORIGINAL_SPEC(fullname, self.scripts / name, loader=_PortableLoader(fullname, self.scripts / name, SCRIPTS / name))


_ORIGINAL_SPEC = importlib.util.spec_from_file_location


@contextlib.contextmanager
def _portable(scripts: Path, portable: list[str]) -> Iterator[None]:
    """Route named imports and file-location loads of portable scripts."""
    def spec_from_file_location(name, location=None, *, loader=None, **options):
        recorded = Path(location) if location is not None else None
        if loader is None and recorded is not None and recorded.name in portable and recorded.resolve().parent == scripts:
            loader = _PortableLoader(name, recorded, SCRIPTS / recorded.name)
        return _ORIGINAL_SPEC(name, location, loader=loader, **options)

    finder = _PortableFinder(scripts, portable)
    sys.meta_path.insert(0, finder)
    importlib.util.spec_from_file_location = spec_from_file_location
    try:
        yield
    finally:
        importlib.util.spec_from_file_location = _ORIGINAL_SPEC
        sys.meta_path.remove(finder)


@contextlib.contextmanager
def recorded(test_module: str, row: dict) -> Iterator[ModuleType]:
    """Import test_module from its recorded commit and keep that import context.

    While the context is open, sibling script imports resolve inside the
    worktree and the current checkout's script modules are set aside; they are
    restored on exit. Scripts listed under "portable" keep their recorded
    location, so ROOT and the bytes that packets bind stay those of the
    worktree, but execute the current checkout's source. The two differ only
    where a previous machine's location was replaced by a lookup on this one.

    An "overlay" restores paths from a later commit into the worktree. It
    rebuilds the tree a run was verified in when no single commit holds it:
    sources were committed before the packet, or packet files that were
    ignored then were committed afterwards.
    """
    overlay = row.get('overlay', {'commit': row['commit'], 'paths': []})
    for path in row['packets']:
        unchanged(overlay['commit'] if path in overlay['paths'] else row['commit'], [path])
    portable = row.get('portable', [])
    with checkout(row['commit']) as tree:
        if overlay['paths']:
            _restore(tree, overlay['commit'], overlay['paths'])
        scripts = tree / 'scripts'
        saved = {k: sys.modules.pop(k) for k in _modules_in(SCRIPTS)}
        sys.path.insert(0, str(scripts))
        try:
            with _portable(scripts, portable):
                yield _exec(test_module, scripts / (test_module + '.py'))
        finally:
            sys.path.remove(str(scripts))
            for k in _modules_in(scripts):
                sys.modules.pop(k, None)
            sys.modules.update(saved)


class _RecordedSuite(unittest.TestSuite):
    """Defer the worktree until the suite runs, so only one exists at a time."""

    def __init__(self, test_module: str, loader: unittest.TestLoader):
        super().__init__()
        self.test_module = test_module
        self.loader = loader

    def run(self, result, debug=False):
        try:
            for row in revisions(self.test_module):
                with recorded(self.test_module, row) as module:
                    # A portable test module carries this hook too; load its tests directly.
                    vars(module).pop('load_tests', None)
                    _select(self.loader.loadTestsFromModule(module), row).run(result)
        except RevisionUnavailable as exc:
            result.addError(_Failure(self.test_module), (type(exc), exc, exc.__traceback__))
        return result


def _flatten(suite: unittest.TestSuite) -> Iterator[unittest.TestCase]:
    for test in suite:
        if isinstance(test, unittest.TestSuite):
            yield from _flatten(test)
        else:
            yield test


def _names(test: unittest.TestCase, names: list[str]) -> bool:
    local = test.id().split('.', 1)[-1]
    return any(local == n or local.startswith(n + '.') for n in names)


def _select(suite: unittest.TestSuite, row: dict) -> unittest.TestSuite:
    """Keep the tests a run owns, in their original order."""
    tests = list(_flatten(suite))
    if 'only' in row:
        tests = [t for t in tests if _names(t, row['only'])]
    if 'exclude' in row:
        tests = [t for t in tests if not _names(t, row['exclude'])]
    return unittest.TestSuite(tests)


class _Failure(unittest.TestCase):
    def __init__(self, test_module: str):
        super().__init__('runTest')
        self.test_module = test_module

    def runTest(self):
        pass

    def id(self):
        return self.test_module + '.evidence_revision'


def tests(test_file: str):
    """Return a load_tests hook that runs test_file's tests at its revision."""
    test_module = Path(test_file).stem

    def load_tests(loader: unittest.TestLoader, standard_tests: unittest.TestSuite, pattern: str | None) -> unittest.TestSuite:
        return _RecordedSuite(test_module, loader)
    return load_tests
