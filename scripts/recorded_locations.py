"""Map locations that a previous machine recorded into retained evidence.

Retained drivers and packets name the machine they ran on: the Go 1.23.12 SDK
under that account's gvm root, and the checkout root of the run. A tool is
identified by its version and digest and a checkout file by its repository
path, so these helpers compare recorded paths by role instead of by prefix and
find this machine's copy. The SDK location comes from KS_GO1_23_ROOT or the
per-user gvm default.
"""
import os
from pathlib import Path
import re

VERSION = 'go1.23.12'
ENV = 'KS_GO1_23_ROOT'
# Any account's gvm install of this version; the previous machine used one.
RECORDED_ROOT = re.compile(r'^/Users/[^/]+/\.gvm/gos/' + re.escape(VERSION) + r'(?=/|$)')


def root() -> Path:
    """Return the configured SDK root; it is not required to exist."""
    return Path(os.environ.get(ENV) or Path.home() / '.gvm/gos' / VERSION)


def gofmt() -> str:
    return str(root() / 'bin/gofmt')


def tool_name(recorded: str) -> str:
    """Map a recorded SDK path to its location inside the SDK, e.g. bin/go."""
    match = RECORDED_ROOT.match(recorded)
    if not match:
        raise ValueError('not a recorded ' + VERSION + ' SDK path')
    return recorded[match.end():].lstrip('/')


def local(recorded) -> Path:
    """Return this machine's copy of a tool recorded under another SDK root."""
    return root() / tool_name(str(recorded))


def local_env(env: dict) -> dict:
    """Return env with the recorded SDK root in GOROOT and PATH moved to this machine."""
    result = dict(env)
    if 'GOROOT' in result and RECORDED_ROOT.match(result['GOROOT']):
        result['GOROOT'] = str(root())
    if 'PATH' in result:
        result['PATH'] = os.pathsep.join(str(local(d)) if RECORDED_ROOT.match(d) else d for d in result['PATH'].split(os.pathsep))
    return result


def in_run_checkout(path: str, relative) -> bool:
    """Return whether path names relative under an absolute checkout root.

    A run records its own checkout root, which need not be this machine's.
    """
    tail = Path(relative).parts
    return Path(path).is_absolute() and Path(path).parts[-len(tail):] == tail
