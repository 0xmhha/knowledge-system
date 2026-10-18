#!/usr/bin/env python3
"""Owned stdio probe: whole-function responses, never independent rule gold."""
import argparse
import gzip
import importlib.util
import json
from pathlib import Path
import shutil
import subprocess
import sys

R = Path(__file__).resolve().parents[1]
sys.path.insert(0, str(R/'scripts'))
import wi2_dev_gold as g
f = g.flow


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument('--workspace', type=Path, required=True)
    parser.add_argument('--prepared', type=Path, required=True)
    parser.add_argument('--reuse-project-runtime', type=Path,
                        help='retain an earlier completed primary project run including incomplete responses')
    a = parser.parse_args()
    work, prepared = a.workspace.resolve(), a.prepared.resolve()
    out = work/'runtime'
    out.mkdir(parents=True)
    spec = importlib.util.spec_from_file_location('wire', R/'scripts/eval-value-dependencies.py')
    wire = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(wire)

    def write(name, value):
        path = out/name
        path.parent.mkdir(parents=True, exist_ok=True)
        path.write_text(json.dumps(value, ensure_ascii=False, indent=2)+'\n')

    def inventory():
        base = prepared/'stablenet/retained-origins-v2'
        return {str(p.relative_to(base)): f.sha(p.read_bytes())
                for sub in ('graph', 'sources', 'vector')
                for p in (base/sub).rglob('*')
                if p.is_file() and not p.name.endswith(('lock', 'lease'))}

    sessions = []

    def connect(label, command, namespace):
        path = out/label
        path.mkdir()
        client = wire.Client(None, None, path, command=command, namespace=namespace)
        client.request('initialize', {'protocolVersion': '2024-11-05', 'capabilities': {},
                                     'clientInfo': {'name': 'wi2-dev-matching', 'version': '1'}})
        write(label+'/tools.json', client.request('tools/list')['result']['tools'])
        return client

    def close(client, label):
        client.close()
        client.proc.stdout.close()
        result = dict(label=label, exit=client.proc.returncode, requests=client.id)
        sessions.append(result)
        write(label+'/session.json', result)
        assert result['exit'] == 0, result

    before = inventory()
    write('retained-before.json', before)
    dataset = prepared/'stablenet/retained-origins-v2'
    (out/'retained-source-manifest.json.gz').write_bytes(gzip.compress((dataset/'sources/manifest.json').read_bytes(), mtime=0))
    shutil.copyfile(dataset/'dataset-identity.json', out/'dataset-identity.json')
    shutil.copyfile(prepared/'go-stablenet.yaml', out/'project-config.txt')
    inputs, functions = g.source_inputs()
    observations = f.packed(R/g.PACKET/'flow-stdout.json.gz')
    by_key = {(o['path'], o['span']['start']): o for o in observations}
    by_file = {i['Path']: i['Source'] for i in inputs}
    ckg, cks = str(prepared/'bin/ckg'), str(prepared/'read-cks')
    pins = f.read(R/g.BASE/'followups/stablenet-comparison-v1/binary-pins.json')
    write('binaries.json', dict(ckg_sha256=f.sha(Path(ckg).read_bytes()),
                              cks_sha256=f.sha(Path(cks).read_bytes()), historical_pins=pins))
    graph = work/'project-graph'
    shutil.copytree(prepared/'stablenet/retained-origins-v2/graph', graph)
    selected = []
    discovery = connect('discovery', [ckg, 'mcp', '--graph', str(graph), '--namespace', 'wi2'], 'wi2.context.')
    try:
        for fn in functions:
            if fn['stratum'] != 'project':
                continue
            obs = by_key[fn['path'], fn['start']]
            result = discovery.tool('find_symbol', {'name': obs['name'], 'exact': False})
            line = by_file[fn['path']].encode()[:fn['start']].count(b'\n')+1
            nodes = [n for n in result['nodes'] if n['file'] == fn['path']
                     and n['line'] == line and n.get('canonical_id')]
            selected.append(dict(function_id=fn['id'], node=nodes[0] if len(nodes) == 1 else None,
                                 status='unique_canonical' if len(nodes) == 1 else 'missing_or_ambiguous',
                                 discovery=result))
    finally:
        close(discovery, 'discovery')
    write('project-discovery.json', selected)

    def query(label, config, chosen, scope, commit):
        if label == 'project' and a.reuse_project_runtime:
            previous = a.reuse_project_runtime.resolve()
            shutil.copytree(previous/label, out/label)
            for suffix in ('results', 'boundaries'):
                shutil.copyfile(previous/(label+'-'+suffix+'.json'), out/(label+'-'+suffix+'.json'))
            sessions.append(f.read(out/label/'session.json'))
            write('project-reuse.json', dict(previous=str(previous),
                  policy='reuse every primary response, including budget exhaustion; no retry-until-pass'))
            return f.read(out/(label+'-results.json'))
        client = connect(label, [cks, 'mcp', '--config', str(config)], 'cks.context.')
        responses, boundaries = [], []
        try:
            for sel in chosen:
                if sel['node'] is None:
                    responses.append(dict(function_id=sel['function_id'], project=scope,
                                          arguments=None, default_off_response=None, response=None,
                                          status='consumer_symbol_unavailable_unknown'))
                    write(label+'-results.json', responses)
                    continue
                fn = next(fn for fn in functions if fn['id'] == sel['function_id'])
                expected_commit = commit if commit else fn['source_commit']
                arguments = dict(node_id=sel['node']['id'], expected_commit=expected_commit,
                                 expected_source_sha256=fn['function_sha256'])
                # One unruled discovery request supplies the graph-manifest identity.
                base = client.tool('value_dependencies', arguments)
                arguments['expected_snapshot_sha256'] = base['snapshot_sha256']
                assert base['analysis'] is None or base['analysis'].get('rules') is None
                rule_args = dict(rule_pack='go-local-review@1.0.0', function=arguments,
                                 max_candidates=64, max_flow_steps=4096)
                result = client.tool('rule_candidates', rule_args)
                responses.append(dict(function_id=fn['id'], project=scope, arguments=rule_args,
                                      default_off_response=base, response=result))
                write(label+'-results.json', responses)
                print(label, fn['id'], result['status'], flush=True)
            first = responses[0]['arguments']
            for key in ('expected_source_sha256', 'expected_snapshot_sha256', 'expected_commit'):
                args = json.loads(json.dumps(first))
                args['function'][key] = '0'*len(args['function'][key])
                result = client.tool('rule_candidates', args)
                assert result['status'] == 'source_mismatch' and result['analysis'] is None
                boundaries.append(dict(kind='wrong_'+key, arguments=args, response=result))
            for field, value in [('rule_pack', 'unsupported@0'), ('max_candidates', 65), ('max_flow_steps', 4097)]:
                args = dict(first, **{field: value})
                raw = client.request('tools/call', {'name': 'cks.context.rule_candidates', 'arguments': args})
                assert 'error' in raw or raw.get('result', {}).get('isError'), raw
                boundaries.append(dict(kind='invalid_'+field, arguments=args, raw_response=raw))
            args = dict(first, max_flow_steps=1)
            limited = client.tool('rule_candidates', args)
            assert limited['analysis']['rules']['status'] == 'incomplete'
            boundaries.append(dict(kind='bounded_incomplete', arguments=args, response=limited))
        finally:
            close(client, label)
        write(label+'-boundaries.json', boundaries)
        return responses

    project_results = query('project', prepared/'go-stablenet.yaml', selected,
                            'go-stablenet', None)
    # Preserve all forty original fragment bytes and the published prefix exactly.
    source = work/'controlled-source'
    source.mkdir()
    adapters = []
    for inp in inputs:
        if not inp['Path'].startswith('controlled/'):
            continue
        path = source/inp['Path']
        path.parent.mkdir(parents=True, exist_ok=True)
        # All original examples declare F at the same byte in package wi2dev.
        # The graph's AST fallback ID omits the file, so give the synthetic
        # package a unique same-width spelling; fragment bytes/offsets stay exact.
        package = 'w'+f.sha(inp['Path'].encode())[:5]
        raw = inp['Source'].replace('package wi2dev\n', 'package '+package+'\n', 1).encode()
        path.write_bytes(raw)
        adapters.append(dict(path=inp['Path'], original_file_sha256=f.sha(inp['Source'].encode()),
                             actual_file_sha256=f.sha(raw), package=package,
                             prefix_bytes=len(g.PREFIX.encode()), byte_offset_delta=0))
    (source/'go.mod').write_text('module example.invalid/wi2-dev\n\ngo 1.26.8\n')
    for cmd in [['git', 'init', '-q'], ['git', 'add', '.'],
                ['git', '-c', 'user.name=WI2 owned fixture', '-c', 'user.email=fixture@example.invalid',
                 '-c', 'commit.gpgsign=false', 'commit', '-qm', 'test: pin exposed WI2 DEV fragments']]:
        subprocess.run(cmd, cwd=source, check=True, capture_output=True, timeout=30)
    commit = subprocess.check_output(['git', 'rev-parse', 'HEAD'], cwd=source, text=True).strip()
    write('controlled-source.json', dict(commit=commit, original_fragments_unchanged=True,
          published_prefix_sha256=f.sha(g.PREFIX.encode()), adapters=adapters,
          package_adapter='unique same-width synthetic package spelling only; fragment bytes unchanged',
          files={r['path']:r['actual_file_sha256'] for r in adapters}))
    cg = work/'controlled-graph'
    command = [ckg, 'build', '--src', str(source), '--out', str(cg), '--lang', 'go']
    run = subprocess.run(command, capture_output=True, timeout=180)
    write('controlled-build.json', dict(command=command, exit=run.returncode,
          stdout=run.stdout.decode(), stderr=run.stderr.decode()))
    run.check_returncode()
    controls = connect('controlled-discovery', [ckg, 'mcp', '--graph', str(cg), '--namespace', 'wi2'], 'wi2.context.')
    selected_controls = []
    try:
        for fn in functions:
            if fn['stratum'] != 'controlled':
                continue
            obs = by_key[fn['path'], fn['start']]
            result = controls.tool('find_symbol', {'name': obs['name'], 'exact': False})
            nodes = [n for n in result['nodes'] if n['file'] == fn['path']
                     and n['line'] == g.PREFIX.count('\n')+1]
            assert len(nodes) == 1, (fn['id'], nodes)
            selected_controls.append(dict(function_id=fn['id'], node=nodes[0], discovery=result))
    finally:
        close(controls, 'controlled-discovery')
    write('controlled-discovery.json', selected_controls)
    config = work/'controlled.yaml'
    config.write_text('version: 1\nname: wi2-owned-dev\nbackends:\n  ckg:\n    path: '+str(cg/'graph.db')+
                      '\n    source_root: '+str(source)+'\n  ckv:\n    path: '+str(work/'mock-vector')+
                      '\n    provider: mock\nlisten:\n  transport: stdio\nlogging:\n  level: error\nsanitize:\n  rules_path: '+
                      str(R/'system/policies/sanitization_rules.yaml')+
                      '\n  default_action: drop\n  fail_closed_on_unknown_rule: true\n')
    controlled_results = query('controlled', config, selected_controls, 'controlled-dev-fixture', commit)
    shutil.copyfile(config, out/'controlled-config.txt')
    after = inventory()
    write('retained-after.json', after)
    assert before == after
    queried = sum(r['response'] is not None for r in project_results+controlled_results)
    write('receipt.json', dict(project_functions=len(project_results), controlled_functions=len(controlled_results),
          actual_queried_functions=queried,
          selected_project_calls=68, controlled_inputs=40, sessions=sessions, original_retained_files_preserved=True,
          retries=0, primary_rule_calls=queried, bounded_extra_rule_calls=2,
          external_model_calls=0, current_scores=None, independent_approved=0, formal_completed=0))


if __name__ == '__main__':
    main()
