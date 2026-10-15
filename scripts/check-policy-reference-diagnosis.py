#!/usr/bin/env python3
"""Check preparation evidence without promoting AST links to independent acceptance."""
import collections
import hashlib
import json
from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]
PACKET = ROOT / 'docs/spec-driven/operational-preparation/policy-reference-diagnosis'
EXPECTED = {
    'compiler.Compile': ('compile.Compile', 'systemcontracts/compile/compiler/compiler.go', 40),
    'compiler.compiledTy.ExportContractCode': ('compile.compiledTy.ExportContractCode', 'systemcontracts/compile/compiler/compiler.go', 123),
    'compiler.solcVersion': ('compile.solcVersion', 'systemcontracts/compile/compiler/compiler.go', 37),
    'types.ErrInvalidFeePayer': ('txpool.ErrInvalidFeePayer', 'core/txpool/errors.go', 69),
    'legacypool.queueTxEvent': ('legacypool.LegacyPool.queueTxEvent', 'core/txpool/legacypool/legacypool.go', 1284),
    'engine.Engine.GetValidators': ('wbftengine.Engine.GetValidators', 'consensus/wbft/engine/engine.go', 1097),
}

def digest(path):
    return hashlib.sha256(path.read_bytes()).hexdigest()

def check(folder=PACKET, root=ROOT):
    errors = []
    try:
        folder = folder.resolve()
        manifest = json.loads((folder / 'manifest.json').read_text())
        if (manifest['kind'] != 'local_ast_resolver_diagnosis_not_operational_acceptance'
                or manifest['probe_executed'] is not True
                or any(manifest[k] is not False for k in ('operational_runtime_executed', 'original_overlay_modified', 'proposed_overlay_applied'))
                or manifest['approved_corrections'] != 0 or manifest['gold_records'] != 0
                or manifest['independent_acceptance'] != 'not_recorded'):
            errors.append('diagnostic preparation promoted to adoption, gold or acceptance')
        names = [r['path'] for r in manifest['artifacts']]
        if len(names) != len(set(names)):
            errors.append('duplicate artifact paths')
        if set(names) != {p.relative_to(folder).as_posix() for p in folder.rglob('*') if p.is_file() and p.name != 'manifest.json'}:
            errors.append('artifact inventory omits or invents packet files')
        for row in manifest['artifacts']:
            path = (folder / row['path']).resolve()
            if not path.is_relative_to(folder) or digest(path) != row['sha256'] or path.stat().st_size != row['bytes']:
                errors.append('artifact path, bytes or hash differs')
        previous_path = root / manifest['previous_source_manifest']
        previous = json.loads(previous_path.read_text())
        if digest(previous_path) != manifest['previous_source_manifest_sha256'] or previous['source_commit'] != manifest['source_commit']:
            errors.append('pinned source predecessor binding differs')
        if json.loads((folder / 'source-files.json').read_text()) != previous['files']:
            errors.append('source file inventory differs from pinned predecessor')
        for key in ('helper', 'generator', 'source_reader'):
            if digest(root / manifest[key]['path']) != manifest[key]['sha256']:
                errors.append('live preparation tool differs')
        if len(manifest['frontend_commit']) != 40 or not manifest['frontend_commit'].startswith('179d1c8d'):
            errors.append('frontend source revision differs')
        frontend = json.loads((folder / 'frontend-source-inventory.json').read_text())
        if len(frontend) != 504 or any(digest(root / row['path']) != row['sha256'] for row in frontend):
            errors.append('pinned frontend/module source inventory differs')
        raw = (folder / 'original-policy.yaml.txt').read_bytes()
        if raw != (previous_path.parent / 'private/existing-policy.yaml.txt').read_bytes():
            errors.append('original overlay bytes differ')
        if raw != (root / 'projects/stablenet/policies/graph.yaml').read_bytes():
            errors.append('bound project overlay was modified')
        proposed = raw.decode()
        for old, (new, _, _) in EXPECTED.items():
            proposed = proposed.replace('        - ' + old + '\n', '        - ' + new + '\n')
        if (folder / 'proposed-policy.yaml.txt').read_text() != proposed:
            errors.append('proposal changes more than the seven declared target rows')
        report = json.loads((folder / 'probe.stdout.json').read_text())
        if (report['kind'] != 'ckg_ast_frontend_and_policy_resolver_preparation_only'
                or report['source_file_count'] != 882 or len(report['generated_skipped']) != 35
                or report['code_node_count'] != 126397
                or any(report[k] is not False for k in ('type_aware', 'index_persisted', 'model_executed', 'approved'))
                or report['gold_records'] != 0 or report['independent_acceptance'] != 'not_recorded'):
            errors.append('AST probe scope/result differs or was promoted')
        prior_refs = json.loads((previous_path.parent / 'private/review-drafts.json').read_text())['policy_reference_inventory']
        original = report['original']['references']
        after = report['proposed']['references']
        if ([(r['policy_id'], r['target']) for r in original]
                != [(r['policy_id'], r['target_spelling']) for r in prior_refs] or len(after) != 146):
            errors.append('policy reference identities/count differ')
        changes = []
        for before, next_row in zip(original, after, strict=True):
            new = EXPECTED.get(before['target'], (before['target'],))[0]
            if next_row['target'] != new or next_row['policy_id'] != before['policy_id']:
                errors.append('wrong target substitution or policy identity')
            if new != before['target']:
                changes.append({'policy_id': before['policy_id'], 'old': before['target'], 'new': new,
                                'original_matches': before['matches'], 'proposed_matches': next_row['matches'],
                                'approved': False, 'applied': False})
                _, path, line = EXPECTED[before['target']]
                if len(next_row['matches']) != 1 or (next_row['matches'][0]['path'], next_row['matches'][0]['line']) != (path, line):
                    errors.append('proposed target resolved to wrong source')
                expected_before = 'cmd/evm/internal/compiler/compiler.go' if before['target'] == 'compiler.Compile' else None
                if ([r['path'] for r in before['matches']] != ([expected_before] if expected_before else [])):
                    errors.append('original wrong match/missing target evidence differs')
            elif next_row['matches'] != before['matches']:
                errors.append('unmodified target match changed')
        proposal = json.loads((folder / 'correction-proposals.json').read_text())
        if proposal != {'kind': 'unapproved_unapplied_overlay_corrections', 'changes': changes} or len(changes) != 7:
            errors.append('correction proposal scope or approval differs')
        sources = {r['path']: r for r in previous['files']}
        for name in ('original', 'proposed'):
            data = report[name]
            nodes = data['result']['Nodes']
            if len(nodes) != 42 or len({n['id'] for n in nodes}) != 42:
                errors.append('policy node count/identity differs')
            node_ids = {n['qualified_name']: n['id'] for n in nodes}
            expected_edges = collections.Counter()
            missing = []
            for row in data['references']:
                matches = row['matches']
                if len(matches) > 1:
                    errors.append('ambiguous target requires separate review')
                for match in matches:
                    if match['qualified_name'] != row['target'] or match['path'] not in sources:
                        errors.append('match identity or source file differs')
                if matches:
                    expected_edges[(matches[-1]['id'], node_ids[row['policy_id']])] += 1
                else:
                    missing.append((row['policy_id'], row['target']))
            edges = data['result']['Edges']
            if (collections.Counter((e['src'], e['dst']) for e in edges) != expected_edges
                    or any(e['type'] != 'governed_by' for e in edges)):
                errors.append('resolver edge direction/identity/count differs')
            warnings = data['result']['Warnings'] or []
            if sorted((w['PolicyID'], w['TargetRef']) for w in warnings) != sorted(missing):
                errors.append('warnings do not match missing targets')
            if (len(edges), len(warnings)) != ((140, 6) if name == 'original' else (146, 0)):
                errors.append('diagnostic edge/warning totals differ')
        for path in {m['path'] for r in changes for k in ('original_matches', 'proposed_matches') for m in r[k]}:
            snapshot = folder / 'sources' / (path.replace('/', '__') + '.txt')
            if digest(snapshot) != sources[path]['sha256']:
                errors.append('diagnostic source snapshot differs')
        commands = json.loads((folder / 'commands.json').read_text())
        if commands['build_exit'] != 0 or commands['probe_exit'] != 0 or commands['target_package_build_or_execution'] is not False:
            errors.append('probe failed or subject code executed')
    except (OSError, ValueError, KeyError, TypeError, IndexError) as exc:
        errors.append('invalid diagnosis packet: ' + str(exc))
    return errors

if __name__ == '__main__':
    errors = check()
    for error in errors:
        print('FAIL:', error)
    print('policy reference diagnosis:', 'FAIL' if errors else 'PASS; seven unapplied proposals, no operational acceptance')
    raise SystemExit(bool(errors))
