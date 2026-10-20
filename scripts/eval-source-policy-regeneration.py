#!/usr/bin/env python3
"""Verify explicit source-aware regeneration against the approved pinned overlay."""
import argparse
import hashlib
import importlib.util
import json
import os
import shutil
import yaml
from pathlib import Path
import sqlite3
import subprocess
import time

ROOT = Path(__file__).resolve().parents[1]

def sha(raw):
    return hashlib.sha256(raw).hexdigest()

def write(path, data):
    path.write_text(json.dumps(data, ensure_ascii=False, indent=2) + '\n')

def main():
    ap = argparse.ArgumentParser(description=__doc__)
    for name in ('source-repo', 'ckg-binary', 'cks-binary', 'out'):
        ap.add_argument('--' + name, type=Path, required=True)
    args = ap.parse_args()
    out = args.out.resolve()
    if out.exists():
        raise ValueError('fresh owned directory required')
    out.mkdir(parents=True)
    profile_path = ROOT / 'projects/stablenet/evaluation/policy-0bf2f4d1.profile.json'
    profile = json.loads(profile_path.read_text())
    policy = ROOT / profile['policy_path']
    if sha(policy.read_bytes()) != profile['policy_sha256']:
        raise ValueError('evaluation policy differs')
    spec = importlib.util.spec_from_file_location('source_reader', ROOT / 'scripts/prepare-policy-security-inputs.py')
    reader = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(reader)
    files = reader.source_files(args.source_repo, profile['source_commit'])
    inventory = [{'path': r['Path'], 'git_blob': r['Blob'], 'sha256': sha(r['Source'].encode())} for r in files]
    previous = json.loads((ROOT / 'docs/spec-driven/operational-preparation/policy-security-drafts/manifest.json').read_text())
    if inventory != previous['files']:
        raise ValueError('source inventory differs')
    source = out / 'source'
    source.mkdir()
    for row in files:
        path = source / row['Path']
        path.parent.mkdir(parents=True, exist_ok=True)
        path.write_bytes(row['Source'].encode())
    for name in ('go.mod', 'go.sum'):
        raw = subprocess.check_output(['git', '-C', str(args.source_repo), 'show', profile['source_commit'] + ':' + name])
        (source / name).write_bytes(raw)
        inventory.append({'path': name, 'sha256': sha(raw)})
    write(out / 'source-inventory.json', inventory)
    (out / 'policy.yaml.txt').write_bytes(policy.read_bytes())
    (out / 'profile.json').write_bytes(profile_path.read_bytes())
    bootstrap_env = dict(os.environ, GOTOOLCHAIN='go1.26.9', GOPROXY='off')
    go_root = subprocess.check_output(['go', 'env', 'GOROOT'], cwd=ROOT, env=bootstrap_env, text=True).strip()
    env = dict(os.environ, GOTOOLCHAIN='local', GOCACHE='/private/tmp/ks-corpus-go-cache', GOPROXY='off', GOSUMDB='off')
    env['PATH'] = str(Path(go_root) / 'bin') + os.pathsep + env['PATH']
    write(out / 'environment.json', {k: env[k] for k in ('GOTOOLCHAIN', 'GOCACHE', 'GOPROXY', 'GOSUMDB')})
    receipts = []
    def run(label, command, timeout=120):
        start = time.monotonic()
        result = subprocess.run(command, cwd=out, env=env, capture_output=True, timeout=timeout)
        (out / (label + '.stdout.txt')).write_bytes(result.stdout)
        (out / (label + '.stderr.txt')).write_bytes(result.stderr)
        receipt = {'label': label, 'command': command, 'cwd': str(out), 'exit_code': result.returncode,
                   'elapsed_seconds': time.monotonic() - start}
        receipts.append(receipt)
        write(out / 'commands.json', receipts)
        result.check_returncode()
        return result
    ckg, cks = args.ckg_binary.resolve(), args.cks_binary.resolve()
    for label, binary in (('ckg-buildinfo', ckg), ('cks-buildinfo', cks)):
        run(label, ['go', 'version', '-m', str(binary)])
    # Prove legacy compatibility first; source-aware mode is explicitly selected.
    entries = ROOT / 'projects/stablenet/domain-knowledge'
    run('legacy-sync', [str(cks), 'domain', 'sync', '--project', str(entries),
        '--ckv-out', str(out/'legacy-vector.yaml.txt'), '--ckg-out', str(out/'legacy-policy.yaml.txt')])
    if sha((out/'legacy-policy.yaml.txt').read_bytes()) != profile['parent_policy_sha256']:
        raise ValueError('legacy policy changed')
    for name in ('refused-vector.yaml.txt','refused-policy.yaml.txt'):
        (out/name).write_text('preserved-before-refusal\n')
    rejected_command=[str(cks),'domain','sync','--project',str(entries),'--code-root',str(source),
        '--ckv-out',str(out/'refused-vector.yaml.txt'),'--ckg-out',str(out/'refused-policy.yaml.txt')]
    rejected=subprocess.run(rejected_command,cwd=out,env=env,capture_output=True)
    (out/'unmodified-source-sync.stderr.txt').write_bytes(rejected.stderr)
    write(out/'unmodified-source-sync.json',{'exit_code':rejected.returncode,'command':rejected_command,'public_error_is_sanitized':True})
    if rejected.returncode==0:
        raise ValueError('wrong-file declaration was not refused: '+rejected.stderr.decode())
    if any((out/name).read_text()!='preserved-before-refusal\n'for name in ('refused-vector.yaml.txt','refused-policy.yaml.txt')):
        raise ValueError('refused generation changed output')
    owned=out/'owned-domain-knowledge';shutil.copytree(entries,owned)
    entry=owned/'entries/A11.txpool.type_taxonomy_admission.yaml'
    data=yaml.safe_load(entry.read_text());changed=0
    for anchor in data['code_anchors']:
        if anchor.get('symbol')=='ErrInvalidFeePayer':
            if anchor['file']!='core/types/transaction_signing.go':raise ValueError('unexpected original anchor')
            anchor['file']='core/txpool/errors.go';changed+=1
    if changed!=1:raise ValueError('unexpected anchor correction count')
    entry.write_text(yaml.safe_dump(data,allow_unicode=True,sort_keys=False))
    run('source-aware-sync',[str(cks),'domain','sync','--project',str(owned),'--code-root',str(source),
        '--ckv-out',str(out/'regenerated-vector.yaml.txt'),'--ckg-out',str(out/'regenerated-policy.yaml.txt')])
    regenerated=(out/'regenerated-policy.yaml.txt').read_bytes()
    expected=yaml.safe_load(policy.read_bytes());observed=yaml.safe_load(regenerated)
    for view in (expected,observed):
        for item in view['policies']:item['governs']=sorted(item.get('governs',[]))
    if observed!=expected:raise ValueError('source-aware policy differs semantically from adopted overlay')
    write(out/'generator-status.json',{'legacy_original_policy_reproduced':True,
        'uncorrected_wrong_file_refused':True,'both_outputs_preserved_on_refusal':True,
        'owned_entry_file_corrections':1,'corrected_policy_reproduced':True,'equality_scope':'all policy fields and sorted governs; approved replacement retained old list order',
        'generated_policy_sha256':sha(regenerated),'approved_policy_sha256':profile['policy_sha256'],
        'regeneration_fix_complete':True,'source_aware_mode':'explicit --code-root',
        'original_entries_or_approved_overlay_changed':False,'independent_acceptance':False})
    policy=out/'regenerated-policy.yaml.txt'
    graph = out / 'graph'
    run('index', [str(ckg), 'build', '--src', str(source), '--out', str(graph), '--lang', 'go',
                  '--policy-file', str(policy), '--no-cache', '--fail-on-parse-errors'], timeout=1200)
    db = graph / 'graph.db'
    with sqlite3.connect('file:' + str(db) + '?mode=ro', uri=True) as conn:
        conn.row_factory = sqlite3.Row
        nodes = [dict(r) for r in conn.execute("SELECT id,type,name,qualified_name,file_path,start_line FROM nodes WHERE type='Policy' ORDER BY id")]
        edges = [dict(r) for r in conn.execute("SELECT n.id,n.qualified_name,n.file_path,n.start_line,e.dst FROM edges e JOIN nodes n ON n.id=e.src WHERE e.type='governed_by' ORDER BY e.dst,n.qualified_name")]
        write(out / 'persisted-policies.json', nodes)
        write(out / 'persisted-policy-edges.json', edges)
    if len(nodes) != 42 or len(edges) != 146:
        raise ValueError('persisted policy coverage differs: ' + str((len(nodes), len(edges))))
    diagnosis = ROOT / 'docs/spec-driven/operational-preparation/policy-reference-diagnosis/correction-proposals.json'
    changes = json.loads(diagnosis.read_text())['changes']
    for row in changes:
        matches = [e for e in edges if e['qualified_name'] == row['new'] and e['dst'] == 'policy:' + row['policy_id']]
        expected = row['proposed_matches'][0]
        if len(matches) != 1 or (matches[0]['file_path'], matches[0]['start_line']) != (expected['path'], expected['line']):
            raise ValueError('wrong persisted source match: ' + row['new'])
    if any(e['qualified_name'] == 'compiler.Compile' and e['dst'] == 'policy:A10.codegen.contract_regen_procedure' for e in edges):
        raise ValueError('assembler still has contract-generation policy')
    lib_spec = importlib.util.spec_from_file_location('mcp_probe', ROOT / 'scripts/eval-value-dependencies.py')
    lib = importlib.util.module_from_spec(lib_spec)
    lib_spec.loader.exec_module(lib)
    cksroot = out / 'cks'
    cksroot.mkdir()
    sanitize = cksroot / 'sanitization_rules.yaml'
    sanitize.write_bytes((ROOT / 'system/policies/sanitization_rules.yaml').read_bytes())
    config = cksroot / 'cks.yaml'
    config.write_text('version: 1\nname: source-pinned-policy-evaluation\nbackends:\n  ckg:\n    path: ' + str(db)
        + '\n  ckv:\n    provider: mock\nlisten:\n  transport: stdio\nlogging:\n  level: error\nsanitize:\n  rules_path: '
        + str(sanitize) + '\n  default_action: drop\n  fail_closed_on_unknown_rule: true\n')
    client = lib.Client(None, None, cksroot, command=[str(cks), 'mcp', '--config', str(config)], namespace='cks.context.')
    results = []
    controls = []
    try:
        client.request('initialize', {'protocolVersion': '2024-11-05', 'capabilities': {}, 'clientInfo': {'name': 'source-pinned-policy-check', 'version': '1'}})
        tools = client.request('tools/list')['result']['tools']
        write(out / 'tools.json', tools)
        for row in changes:
            match = next(e for e in edges if e['qualified_name'] == row['new'] and e['dst'] == 'policy:' + row['policy_id'])
            span = row['proposed_matches'][0]
            source_bytes = (source / span['path']).read_bytes()
            symbol_sha = sha(source_bytes[span['start_byte']:span['end_byte']])
            response = client.tool('policy_security', {'node_id': match['id']})
            results.append({'policy_id': row['policy_id'], 'target': row['new'], 'response': response})
            write(out / 'cks-results.json', results)
            if response['status'] != 'annotations_found' or not any(a['id'] == 'policy:' + row['policy_id'] for a in response['annotations']):
                raise ValueError('MCP policy not found: ' + row['new'])
            if response['symbol']['source_sha256']['value'] != symbol_sha:
                raise ValueError('returned source bytes differ from pinned Git blob')
            pinned = client.tool('policy_security', {'node_id': match['id'],
                'expected_source_sha256': symbol_sha, 'expected_snapshot_sha256': response['snapshot_sha256']})
            canonical = client.tool('policy_security', {'canonical_id': response['symbol']['canonical_id']})
            if pinned['annotations'] != response['annotations'] or canonical['annotations'] != response['annotations']:
                raise ValueError('source/snapshot or canonical identity changed annotations')
            for annotation in response['annotations']:
                prov = annotation['provenance']
                if any(prov[k]['status'] != 'unknown' for k in ('source_sha256', 'source_line', 'version', 'reviewer', 'reviewed_at', 'approval')):
                    raise ValueError('invented original YAML provenance')
        for key in ('expected_source_sha256', 'expected_snapshot_sha256'):
            rejected = client.tool('policy_security', {'node_id': edges[0]['id'], key: '0' * 64})
            controls.append({'kind': key, 'response': rejected})
            if rejected['status'] != 'source_mismatch' or rejected['annotations']:
                raise ValueError('stale control returned policy data')
        with sqlite3.connect('file:' + str(db) + '?mode=ro', uri=True) as conn:
            wrong = conn.execute("SELECT id FROM nodes WHERE qualified_name='compiler.Compile' AND file_path='cmd/evm/internal/compiler/compiler.go'").fetchall()
        if len(wrong) != 1:
            raise ValueError('assembler control ambiguous/missing')
        unrelated = client.tool('policy_security', {'node_id': wrong[0][0]})
        controls.append({'kind': 'assembler_must_not_have_contract_policy', 'response': unrelated})
        if unrelated['annotations'] or unrelated['status'] != 'no_annotations':
            raise ValueError('assembler inherited contract policy')
        write(out / 'controls.json', controls)
    finally:
        client.close()
    if client.proc.returncode != 0:
        raise ValueError('CKS did not exit cleanly')
    manifest = json.loads((graph / 'manifest.json').read_text())
    summary = {'kind': 'local_source_aware_policy_regeneration_verification', 'source_commit': profile['source_commit'],
               'source_scope': '882 tracked non-test Go blobs plus original go.mod/go.sum; no Git metadata, non-Go assets or tests copied',
               'source_execution': False, 'target_package_build': False,
               'policy_sha256': sha(policy.read_bytes()), 'approved_policy_sha256':profile['policy_sha256'], 'graph_sha256': sha(db.read_bytes()),
               'policy_nodes': len(nodes), 'policy_edges': len(edges), 'corrections_verified': len(results),
               'public_negative_controls': len(controls), 'canonical_and_source_pin_checks': len(results),
               'selected_go_files': len(manifest['files']), 'parse_errors_count': manifest['parse_errors_count'],
               'unresolved_refs_count': manifest['unresolved_refs_count'],
               'cks_exit': client.proc.returncode, 'binary_sha256': {'ckg': sha(ckg.read_bytes()), 'cks': sha(cks.read_bytes())},
               'ckv_backend': 'mock; annotation path only, no vector/ontology quality claim',
               'independent_acceptance': 'not_recorded', 'ontology_effect_evaluated': False,
               'production_activated': False}
    write(out / 'summary.json', summary)
    print(json.dumps(summary, ensure_ascii=False, indent=2))

if __name__ == '__main__':
    main()
