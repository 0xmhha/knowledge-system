#!/usr/bin/env python3
"""Guard fresh full-book evaluations before dispatching the existing matrix CLI.

Recorded human judgments are checked for consistency, not authenticated. The
registry is a trusted local operator record, not a signed global contamination
history. FINAL reservation precedes dispatch and conservatively consumes a set
without claiming any response was observed. Direct capture is still diagnostic.
"""
import argparse
import contextlib
import datetime as dt
import fcntl
import importlib.util
import json
import os
from pathlib import Path
import stat
import subprocess
import sys
import tempfile
import uuid

SPEC = importlib.util.spec_from_file_location('fresh_inputs', Path(__file__).with_name('refactoring-eval-input-check.py'))
GATE = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(GATE)


def require(ok, message):
    if not ok:
        raise ValueError(message)


def bounded(path, limit=10 << 20):
    with Path(path).open('rb') as stream:
        raw = stream.read(limit + 1)
    require(len(raw) <= limit, 'input exceeds limit')
    return raw


@contextlib.contextmanager
def registry_lock(path):
    # A persistent companion inode must never be unlinked/age-reclaimed.
    fd = os.open(str(path) + '.lock', os.O_RDWR | os.O_CREAT | os.O_NOFOLLOW, 0o600)
    try:
        require(stat.S_ISREG(os.fstat(fd).st_mode), 'registry lock not regular')
        try:
            fcntl.flock(fd, fcntl.LOCK_EX | fcntl.LOCK_NB)
        except BlockingIOError:
            raise ValueError('another evaluation owns the registry lock')
        yield fd
    finally:
        os.close(fd)


def load_registry(path):
    fd = os.open(path, os.O_RDONLY | os.O_NOFOLLOW)
    with os.fdopen(fd, 'rb') as stream:
        require(stat.S_ISREG(os.fstat(stream.fileno()).st_mode), 'registry not regular')
        raw = stream.read((10 << 20) + 1)
    obj = GATE.decode(raw)
    require(isinstance(obj, dict) and obj.get('schema_version') == 1 and isinstance(obj.get('records'), list), 'invalid registry')
    for row in obj['records']:
        require(isinstance(row, dict) and row.get('state') == 'FINAL_reserved' and isinstance(row.get('final_clusters'), list) and bool(row['final_clusters']), 'invalid reservation')
        GATE.timestamp(row['reserved_at'])
        require(all(isinstance(c, str) and c for c in row['final_clusters']) and all(isinstance(row.get(k), str) and GATE.SHA64.fullmatch(row[k]) for k in ('questions_sha256','protocol_sha256')), 'invalid reservation binding')
    return obj


def reserve(path, registry, record):
    registry['records'].append(record)
    raw = json.dumps(registry, indent=2).encode() + b'\n'
    require(len(raw) <= 10 << 20, 'registry exceeds limit')
    fd, temp = tempfile.mkstemp(prefix='.eval-reservation-', dir=path.parent)
    try:
        with os.fdopen(fd, 'wb') as stream:
            stream.write(raw); stream.flush(); os.fsync(stream.fileno())
        os.replace(temp, path)
        parent = os.open(path.parent, os.O_RDONLY)
        try:
            os.fsync(parent)
        finally:
            os.close(parent)
    finally:
        if os.path.exists(temp):
            os.unlink(temp)


def binding_for(protocol, config_raw, binary_path):
    pins = protocol.get('execution_binding', {})
    require(isinstance(pins, dict) and isinstance(pins.get('dataset_id'), str) and bool(pins['dataset_id']), 'frozen execution dataset binding missing')
    require(pins.get('config_sha256') == GATE.sha(config_raw), 'config differs from reviewed protocol')
    # Stream potentially large binaries; avoid placing their bytes in memory.
    import hashlib
    with binary_path.open('rb') as stream:
        digest = hashlib.file_digest(stream, 'sha256').hexdigest()
    require(pins.get('binary_sha256') == digest, 'binary differs from reviewed protocol')
    m = protocol['model']
    return dict(schema_version=1, dataset_id=pins['dataset_id'], source_commit=protocol['source_commit'],
                config_sha256=pins['config_sha256'], binary_sha256=digest, provider=m['provider'], model=m['model'],
                model_digest=m['digest'], dimension=m['dimension'], runtime_context_tokens=m['runtime_options']['num_ctx'],
                runtime_batch_tokens=m['runtime_options']['num_batch'], retrieval_k=protocol['retrieval_k'])


def requests_for(protocol, book, split):
    scope = protocol.get('request_scope')
    require(isinstance(scope, dict) and set(scope) == {'knowledge_as_of', 'knowledge_subsystem'}, 'frozen request scope missing')
    require(all(isinstance(x, str) and x.strip() for x in scope.values()), 'invalid request scope')
    require(dt.date.fromisoformat(scope['knowledge_as_of']).isoformat() == scope['knowledge_as_of'], 'invalid scope date')
    selected = [q for q in book['questions'] if q['split'] == split]
    require(bool(selected), 'selected split is empty')
    return {'schema_version':1, 'requests':[dict(id=q['id'], tool='cks.context.get_for_task_v2', arguments=dict(prompt=q['prompt'], **scope)) for q in selected]}


def run(args, launch=subprocess.run):
    p_raw, b_raw, base_raw = map(bounded, (args.protocol, args.questions, args.baseline))
    protocol, book = map(GATE.decode, (p_raw, b_raw))
    human_raw = bounded(args.review) if args.review else None
    human = GATE.decode(human_raw) if human_raw else None
    known_raw = bounded(args.known_observations)
    known = GATE.decode(known_raw)
    require(isinstance(known, list), 'known observations must be array')
    def source(commit, path):
        spec = commit + ':' + path
        size = int(subprocess.check_output(['git','-C',str(args.repo),'cat-file','-s',spec]))
        require(size <= 32 << 20, 'source exceeds capture limit')
        return subprocess.check_output(['git','-C',str(args.repo),'show',spec])
    result = GATE.audit(p_raw, b_raw, base_raw, human, known, source)
    if not result['input_ready']:
        print(json.dumps(result, indent=2)); return 2
    require(protocol.get('execution_allowed') is True, 'protocol execution not approved')
    config_raw = bounded(args.config)
    binary = args.binary.resolve(strict=True)
    binding = binding_for(protocol, config_raw, binary)
    requests = requests_for(protocol, book, args.split)
    out = args.output.resolve()
    require(not out.is_relative_to(args.repo.resolve()), 'run output must be outside source repo')
    require(not out.exists(), 'output already exists')
    # Registry must already exist. Only init creates it, exclusively.
    with registry_lock(args.registry) as lock_fd:
        registry = load_registry(args.registry)
        result = GATE.audit(p_raw, b_raw, base_raw, human, known + registry['records'], source)
        require(result['input_ready'], 'FINAL already reserved/observed or inputs no longer ready')
        # Re-read the frozen bytes immediately before reservation and dispatch.
        for path, raw in ((args.protocol,p_raw),(args.questions,b_raw),(args.baseline,base_raw),(args.config,config_raw)):
            require(bounded(path) == raw, 'input changed during runner preflight')
        require(GATE.decode(bounded(args.review)) == human, 'human record changed during preflight')
        require(GATE.decode(bounded(args.known_observations)) == known, 'observation record changed during preflight')
        out.mkdir(mode=0o700)
        for name, obj in (('requests.json',requests),('execution-binding.json',binding),('input-audit.json',result)):
            path=out/name; path.write_text(json.dumps(obj,indent=2)+'\n'); path.chmod(0o600)
        frozen_paths=[]
        for name, raw in (('protocol.json',p_raw),('questions.json',b_raw),('baseline.json',base_raw),('review.json',human_raw),('known-observations.json',known_raw)):
            path=out/name; path.write_bytes(raw); path.chmod(0o600); frozen_paths.append(path)
        if args.split == 'FINAL':
            reserve(args.registry, registry, dict(state='FINAL_reserved', reserved_at=dt.datetime.now(dt.timezone.utc).isoformat(),
                    run_id=str(uuid.uuid4()), final_clusters=sorted({q['cluster_id'] for q in book['questions'] if q['split']=='FINAL'}),
                    questions_sha256=GATE.sha(b_raw), protocol_sha256=GATE.sha(p_raw)))
        # Hold the OS lock through every arm/phase, including subprocess failure.
        command=[str(binary),'eval','matrix','--requests',str(out/'requests.json'),'--config',str(args.config.resolve()),
                 '--cks-mcp',str(binary),'--output',str(out/'capture'),'--environment-ledger','--evaluation-binding',str(out/'execution-binding.json'),
                 '--warmup',str(protocol['warm_latency']['warmup_runs']),'--retrieval-runs',str(protocol['retrieval_runs']),
                 '--warm-runs',str(protocol['warm_latency']['measured_runs']),'--cold-runs',str(protocol['cold_start']['runs'])]
        for path in [*frozen_paths,args.registry]:
            command += ['--lock-file',str(path.resolve())]
        # The matrix process inherits the same open-file-description lock. If
        # this wrapper is SIGKILLed, a still-running matrix retains exclusion.
        child = launch(command, cwd=args.repo, pass_fds=(lock_fd,))
        return child.returncode


def main():
    parser=argparse.ArgumentParser(description=__doc__)
    sub=parser.add_subparsers(dest='action',required=True)
    init=sub.add_parser('init',help='create a new registry; refuses an existing file')
    init.add_argument('--registry',type=Path,required=True)
    cli=sub.add_parser('run',help='require full fresh-book review, frozen runtime pins and local FINAL reservation')
    for name in ('protocol','questions','known-observations','registry','config','binary','output'):
        cli.add_argument('--'+name,type=Path,required=True)
    cli.add_argument('--review',type=Path)
    cli.add_argument('--repo',type=Path,default=GATE.ROOT)
    cli.add_argument('--baseline',type=Path,default=GATE.ROOT/'system/eval/b0-knowledge-system/protocol-m2max-draft.json')
    cli.add_argument('--split',choices=('DEV','FINAL'),required=True)
    args=parser.parse_args()
    if args.action=='init':
        fd=os.open(args.registry,os.O_WRONLY|os.O_CREAT|os.O_EXCL|os.O_NOFOLLOW,0o600)
        with os.fdopen(fd,'wb') as stream:
            stream.write(b'{"schema_version":1,"records":[]}\n');stream.flush();os.fsync(stream.fileno())
        return 0
    return run(args)


if __name__=='__main__':
    try:
        sys.exit(main())
    except (ValueError,OSError,KeyError,TypeError,subprocess.CalledProcessError) as error:
        print('refactoring-eval-run: '+str(error),file=sys.stderr);sys.exit(2)
