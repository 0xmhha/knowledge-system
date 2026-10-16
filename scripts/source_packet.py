"""Capture and verify source spans against a commit's offline Git object chain."""
import hashlib
import json
from pathlib import Path
import subprocess


def sha(raw):
    return hashlib.sha256(raw).hexdigest()


def oid(kind, raw):
    return hashlib.sha1(kind.encode() + b' ' + str(len(raw)).encode() + b'\0' + raw).hexdigest()


def safe(root, name):
    p = Path(name)
    if p.is_absolute() or '..' in p.parts or not (root / p).resolve().is_relative_to(root.resolve()) or (root / p).is_symlink():
        raise ValueError('unsafe source artifact')
    return root / p


class SourceCapture:
    def __init__(self, repo, output, revision):
        self.repo, self.output, self.revision = repo, output, revision
        self.objects, self.files = {}, {}
        self.save_object('commit', revision)

    def git(self, *args):
        return subprocess.check_output(['git', '-C', str(self.repo), *args])

    def save_object(self, kind, identity):
        key = kind + ':' + identity
        if key not in self.objects:
            raw = self.git('cat-file', kind, identity)
            if oid(kind, raw) != identity:
                raise ValueError('Git object identity mismatch')
            artifact = 'objects/' + identity + '.' + kind
            p = safe(self.output, artifact)
            p.parent.mkdir(parents=True, exist_ok=True)
            p.write_bytes(raw)
            self.objects[key] = artifact

    def source(self, path):
        if path not in self.files:
            safe(self.output, path)
            raw = self.git('show', self.revision + ':' + path)
            parts = Path(path).parts
            for i in range(len(parts)):
                treeish = self.revision + '^{tree}' if i == 0 else self.revision + ':' + '/'.join(parts[:i])
                self.save_object('tree', self.git('rev-parse', treeish).decode().strip())
            artifact = 'sources/' + path + '.txt'
            p = safe(self.output, artifact)
            p.parent.mkdir(parents=True, exist_ok=True)
            p.write_bytes(raw)
            self.files[path] = {'artifact': artifact, 'git_blob': oid('blob', raw), 'sha256': sha(raw)}
        return safe(self.output, self.files[path]['artifact']).read_bytes()

    def span(self, path, start, end):
        raw = self.source(path)
        lines = raw.splitlines(keepends=True)
        if not 1 <= start <= end <= len(lines):
            raise ValueError('source span outside file')
        begin, finish = sum(map(len, lines[:start-1])), sum(map(len, lines[:end]))
        return {'path': path, 'start_line': start, 'end_line': end, 'start_byte': begin,
                'end_byte_exclusive': finish, 'span_sha256': sha(raw[begin:finish])}

    def proof(self):
        return {'commit': self.revision, 'objects': self.objects, 'files': self.files}


def tree_entries(raw):
    entries, pos = {}, 0
    while pos < len(raw):
        space, zero = raw.index(b' ', pos), raw.index(b'\0', pos)
        name = raw[space+1:zero].decode()
        if name in entries or '/' in name or len(raw[zero+1:zero+21]) != 20:
            raise ValueError('invalid Git tree')
        entries[name] = (raw[pos:space].decode(), raw[zero+1:zero+21].hex())
        pos = zero + 21
    return entries


def verify(root, revision):
    proof = json.loads((root / 'git-proof.json').read_text())
    if proof['commit'] != revision:
        raise ValueError('unexpected source revision')

    def obj(kind, identity):
        raw = safe(root, proof['objects'][kind + ':' + identity]).read_bytes()
        if oid(kind, raw) != identity:
            raise ValueError('Git object hash mismatch')
        return raw

    tree = obj('commit', revision).splitlines()[0].decode().removeprefix('tree ')
    for path, record in proof['files'].items():
        safe(root, path)
        current, parts = tree, Path(path).parts
        for i, name in enumerate(parts):
            mode, current = tree_entries(obj('tree', current))[name]
            if not (mode in ('100644', '100755') if i == len(parts)-1 else mode == '40000'):
                raise ValueError('unsupported source mode')
        raw = safe(root, record['artifact']).read_bytes()
        if current != record['git_blob'] or current != oid('blob', raw) or sha(raw) != record['sha256']:
            raise ValueError('commit-to-source binding mismatch')
    return proof
