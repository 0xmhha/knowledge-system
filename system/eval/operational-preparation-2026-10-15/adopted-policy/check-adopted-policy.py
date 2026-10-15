#!/usr/bin/env python3
"""Check source-pinned local policy evidence, without claiming ontology quality."""
import collections
import hashlib
import json
from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]
PACKET = ROOT / 'system/eval/operational-preparation-2026-10-15/adopted-policy'

def digest(path):
    return hashlib.sha256(path.read_bytes()).hexdigest()

def check(folder=PACKET, root=ROOT):
    errors = []
    try:
        folder = folder.resolve()
        def load(name):
            return json.loads((folder / name).read_text())
        manifest = load('manifest.json')
        if (manifest['kind'] != 'adopted_local_policy_overlay_not_ontology_or_independent_acceptance'
                or manifest['production_activated'] is not False
                or manifest['independent_acceptance'] != 'not_recorded'):
            errors.append('local verification promoted to production/independent acceptance')
        names = [r['path'] for r in manifest['artifacts']]
        actual = {p.relative_to(folder).as_posix() for p in folder.rglob('*') if p.is_file() and p.name != 'manifest.json'}
        if len(names) != len(set(names)) or set(names) != actual:
            errors.append('artifact inventory differs')
        for row in manifest['artifacts']:
            p = (folder / row['path']).resolve()
            if not p.is_relative_to(folder) or digest(p) != row['sha256'] or p.stat().st_size != row['bytes']:
                errors.append('artifact bytes/path/hash differs')
        profile = load('profile.json')
        if (digest(root / manifest['profile']) != manifest['profile_sha256']
                or digest(folder / 'profile.json') != manifest['profile_sha256']
                or digest(root / manifest['runner']) != manifest['runner_sha256']):
            errors.append('profile or runner differs')
        if (profile['source_commit'] != manifest['source_commit']
                or profile['domain_sync_managed'] is not False
                or profile['independent_content_review'] != 'not_recorded'
                or digest(root / profile['policy_path']) != profile['policy_sha256']
                or digest(folder / 'policy.yaml.txt') != profile['policy_sha256']):
            errors.append('policy/source scope differs')
        summary = load('summary.json')
        expected = {'policy_nodes': 42, 'policy_edges': 146, 'corrections_verified': 7,
                    'canonical_and_source_pin_checks': 7, 'public_negative_controls': 3,
                    'selected_go_files': 843, 'parse_errors_count': 0, 'cks_exit': 0,
                    'ontology_effect_evaluated': False, 'production_activated': False,
                    'independent_acceptance': 'not_recorded'}
        if any(type(summary.get(k)) is not type(v) or summary[k] != v for k, v in expected.items()):
            errors.append('verification scope/counts differ')
        if summary['graph_sha256'] != manifest['owned_graph_sha256'] or summary['policy_sha256'] != profile['policy_sha256']:
            errors.append('graph/policy identity differs')
        generator = load('generator-status.json')
        if (generator['original_policy_reproduced'] is not True or generator['corrected_policy_reproduced'] is not False
                or generator['regeneration_fix_complete'] is not False):
            errors.append('generator recurrence hidden or promoted to fixed')
        if digest(folder / 'regenerated-policy.yaml.txt') != profile['parent_policy_sha256']:
            errors.append('regenerated original differs')
        diagnosis = root / 'docs/spec-driven/operational-preparation/policy-reference-diagnosis'
        changes = json.loads((diagnosis / 'correction-proposals.json').read_text())['changes']
        results = load('cks-results.json')
        edges = load('persisted-policy-edges.json')
        if len(results) != 7 or len(edges) != 146 or len(load('persisted-policies.json')) != 42:
            errors.append('coverage rows missing')
        ast_rows = json.loads((diagnosis / 'probe.stdout.json').read_text())['proposed']['references']
        expected_edges = collections.Counter(('policy:' + r['policy_id'], r['target'], m['path'], m['line'])
            for r in ast_rows for m in r['matches'])
        actual_edges = collections.Counter((r['dst'], r['qualified_name'], r['file_path'], r['start_line']) for r in edges)
        if actual_edges != expected_edges:
            errors.append('persisted references differ from all source-bound AST locations')
        for row, result in zip(changes, results, strict=True):
            match = row['proposed_matches'][0]
            if (result['policy_id'], result['target']) != (row['policy_id'], row['new']):
                errors.append('query identity differs')
            linked = [e for e in edges if e['dst'] == 'policy:' + row['policy_id'] and e['qualified_name'] == row['new']]
            if len(linked) != 1 or (linked[0]['file_path'], linked[0]['start_line']) != (match['path'], match['line']):
                errors.append('persisted edge resolves to wrong file/line')
            raw = (diagnosis / 'sources' / (match['path'].replace('/', '__') + '.txt')).read_bytes()
            expected_source = hashlib.sha256(raw[match['start_byte']:match['end_byte']]).hexdigest()
            response = result['response']
            if (response['status'] != 'annotations_found' or response['symbol']['node_id'] != linked[0]['id']
                    or response['symbol']['source_sha256']['value'] != expected_source
                    or not any(a['id'] == 'policy:' + row['policy_id'] for a in response['annotations'])):
                errors.append('public annotation identity/source differs')
            for annotation in response['annotations']:
                if any(annotation['provenance'][k]['status'] != 'unknown' for k in ('source_sha256','source_line','version','reviewer','reviewed_at','approval')):
                    errors.append('missing original provenance invented')
        controls = load('controls.json')
        if len(controls) != 3:
            errors.append('negative controls missing')
        for c in controls:
            status = 'no_annotations' if c['kind'] == 'assembler_must_not_have_contract_policy' else 'source_mismatch'
            if c['response']['status'] != status or c['response']['annotations']:
                errors.append('negative control leaked annotations')
        stdin = [json.loads(line) for line in (folder / 'cks/mcp.stdin.jsonl').read_text().splitlines()]
        stdout = [json.loads(line) for line in (folder / 'cks/mcp.stdout.jsonl').read_text().splitlines()]
        by_id = {r['id']: r for r in stdout if 'id' in r}
        if len(stdin) != 26 or len(by_id) != 26 or any(r['id'] not in by_id for r in stdin):
            errors.append('public wire requests/responses incomplete')
        wire_tools = [r for r in stdin if r['method'] == 'tools/call']
        if len(wire_tools) != 24 or any(r['params']['name'] != 'cks.context.policy_security' for r in wire_tools):
            errors.append('actual tool call inventory differs')
        by_node = {r['response']['symbol']['node_id']: r['response'] for r in results}
        by_canonical = {r['symbol']['canonical_id']: r for r in by_node.values()}
        canonical_count = pin_count = 0
        for request in wire_tools:
            reply = by_id[request['id']]
            if 'error' in reply or reply['result'].get('isError'):
                errors.append('wire tool call failed')
            args = request['params']['arguments']
            value = reply['result'].get('structuredContent')
            if value is None:
                value = json.loads(reply['result']['content'][0]['text'])
            if 'canonical_id' in args and (value['status'] != 'annotations_found' or value['symbol']['canonical_id'] != args['canonical_id']):
                errors.append('canonical query did not resolve exact symbol')
            expected_response = None
            if 'canonical_id' in args:
                canonical_count += 1
                expected_response = by_canonical.get(args['canonical_id'])
            elif args.get('node_id') in by_node and not any(args.get(k) == '0' * 64 for k in ('expected_source_sha256','expected_snapshot_sha256')):
                expected_response = by_node[args['node_id']]
                if 'expected_source_sha256' in args:
                    pin_count += 1
                    if (args['expected_source_sha256'] != expected_response['symbol']['source_sha256']['value']
                            or args['expected_snapshot_sha256'] != expected_response['snapshot_sha256']):
                        errors.append('wire source/snapshot pin differs')
            if expected_response is not None and (value['annotations'] != expected_response['annotations'] or value['symbol'] != expected_response['symbol']):
                errors.append('wire selected-symbol/canonical/pin parity differs')
            if any(args.get(k) == '0' * 64 for k in ('expected_source_sha256','expected_snapshot_sha256')):
                if value['status'] != 'source_mismatch' or value['annotations']:
                    errors.append('wire stale control leaked data')
        if canonical_count != 7 or pin_count != 7:
            errors.append('wire canonical/source pin checks missing')
        if len(load('tools.json')) != 33 or any(r['exit_code'] != 0 for r in load('commands.json')):
            errors.append('runtime schema/build command differs')
    except (OSError, ValueError, KeyError, TypeError, IndexError) as exc:
        errors.append('invalid adopted policy evidence: ' + str(exc))
    return errors

if __name__ == '__main__':
    errors = check()
    for error in errors:
        print('FAIL:', error)
    print('adopted policy:', 'FAIL' if errors else 'PASS; local overlay only; regeneration and ontology effect remain separate')
    raise SystemExit(bool(errors))
