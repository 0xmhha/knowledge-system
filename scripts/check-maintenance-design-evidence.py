#!/usr/bin/env python3
"""Verify KM-02.1 evidence without promoting schema examples to runtime success."""
import gzip
import hashlib
import importlib.util
import json
from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]
PACKET = ROOT / 'system/eval/knowledge-roadmap-2026-10-15/km-02-contract'


def module(name, file):
    spec = importlib.util.spec_from_file_location(name, ROOT / 'scripts' / file)
    result = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(result)
    return result


suite = module('maintenance_design_suite', 'check-versioned-maintenance-contract.py')
history = module('maintenance_history_verifier', 'verify_committed_collection.py')
roadmap = module('maintenance_roadmap_guard', 'check-knowledge-roadmap.py')


def require(value, message):
    if not value:
        raise ValueError(message)


def sha(value):
    return hashlib.sha256(value).hexdigest()


def check(folder=PACKET, root=ROOT):
    try:
        folder, root = folder.resolve(), root.resolve()
        def raw(name):
            path = folder / name
            return path.read_bytes() if path.exists() else gzip.decompress((folder / (name + '.gz')).read_bytes())
        def load(name):
            return json.loads(raw(name))
        manifest = load('manifest.json')
        require(manifest['scope'] == 'KM02_1_design_contract_conformance_not_runtime'
                and manifest['KM02_1_design_complete'] is True and manifest['KM02_runtime_complete'] is False
                and manifest['public_versioned_tools_exposed'] is False and manifest['maintenance_executed'] is False
                and manifest['independent_acceptance'] is False, 'design scope promoted to runtime/acceptance')
        names = [r['path'] for r in manifest['artifacts']]
        actual = {p.relative_to(folder).as_posix() for p in folder.rglob('*') if p.is_file() and p.name != 'manifest.json'}
        require(len(names) == len(set(names)) and set(names) == actual, 'artifact inventory differs')
        for r in manifest['artifacts']:
            path = (folder / r['path']).resolve()
            require(path.is_relative_to(folder), 'unsafe artifact path')
            data = path.read_bytes()
            require(sha(data) == r['sha256'] and len(data) == r['bytes'], 'artifact bytes differ')
            if 'uncompressed_sha256' in r:
                require(sha(gzip.decompress(data)) == r['uncompressed_sha256'], 'raw decompression differs')
        for name, digest in manifest['source_sha256'].items():
            path = (root / name).resolve()
            require(path.is_relative_to(root) and sha(path.read_bytes()) == digest, 'contract source differs')
        result = load('contract-results.json')
        rows = suite.evaluate()
        require(len(rows) == len({r['id'] for r in rows}) == result['total'] == result['passed'] == 101
                and result['cases'] == rows and result['failed'] == 0 and all(r['passed'] for r in rows)
                and result['new_runtime_MCP_tools_exposed'] is False and result['dataset_maintenance_executed'] is False,
                'contract cases missing/failed/promoted')
        require({r['kind'] for r in rows} == {'request', 'status', 'transition', 'observation', 'cancel', 'idempotency', 'reference', 'decode', 'fingerprint'},
                'contract coverage kind missing')
        for case in json.loads((suite.contract.CONTRACT / 'fixtures.json').read_text())['cases']:
            if case['id'] in ('ST-reject-no-publication', 'ST-reject-vector-not-run', 'ST-reject-foreign-candidate',
                              'TR-cancel_requested-succeeded', 'JSON-duplicate'):
                require(case['expected'] != 'accept', 'critical negative case promoted')
        from jsonschema import Draft202012Validator
        Draft202012Validator.check_schema(suite.contract.SCHEMA)
        machine = suite.contract.MACHINE
        require(machine['transitions']['cancel_requested'] == ['canceled', 'failed', 'interrupted', 'recovering']
                and all(not machine['transitions'][s] for s in machine['terminal_states']), 'cancel/terminal rules weakened')
        audit = load('source-audit.json')
        require(len(audit) == 6, 'source capability audit missing')
        for r in audit:
            lines = (root / r['path']).read_bytes().splitlines(keepends=True)
            require(sha(b''.join(lines[r['start_line']-1:r['end_line']])) == r['span_sha256'], 'capability source audit differs')
        receipt = load('probe/receipt.json')
        require(receipt['MCP_tool_count'] == 35 and receipt['MCP_exit_code'] == 0 and receipt['MCP_calls'] == 0
                and receipt['requests'] == 2 and receipt['versioned_build_tools_exposed'] is False
                and receipt['public_cancel_exposed'] is False and receipt['paid_model_calls'] == 0
                and all(r['exit_code'] == 0 for r in receipt['CLI_help']), 'read-only capability probe differs')
        requests = [json.loads(line) for line in raw('probe/mcp.stdin.jsonl').splitlines()]
        replies = [json.loads(line) for line in raw('probe/mcp.stdout.jsonl').splitlines()]
        require([r['method'] for r in requests] == ['initialize', 'tools/list'] and len(replies) == 2
                and [r['id'] for r in requests] == [r['id'] for r in replies] and all('error' not in r for r in replies), 'probe wire differs')
        tools = replies[1]['result']['tools']
        require(tools == load('probe/tools.json') and len(tools) == 35, 'probe schema differs')
        names = {t['name'] for t in tools}
        require({'cks.ops.index', 'cks.ops.setup', 'cks.ops.setup_status', 'cks.ops.reindex'} <= names
                and not any(n.endswith('.cancel') or n.startswith('cks.dataset.') for n in names), 'new runtime tool claimed')
        help_text = (raw('probe/setup-help.stdout.txt') + raw('probe/setup-help.stderr.txt')).decode()
        require(all(flag in help_text for flag in ('--project-id', '--source-mode', '--version', '--hold-for-review', '--gate-test-bin')),
                'CLI capability no longer supported')
        observed = json.loads((suite.contract.CONTRACT / 'capabilities.json').read_text())
        require(observed['runtime_exposed'] is False and observed['observed']['versioned_MCP_build'] is False
                and observed['observed']['public_job_cancel'] is False and observed['observed']['durable_MCP_job_registry'] is False
                and observed['observed']['internal_single_job_cancel'] is True, 'capability distinction promoted')
        historical = history.verify(root)
        require(historical == load('committed-collection-verification.json') and historical['verification_exit_code'] == 0
                and historical['current_Go_validation_reusable'] is True and historical['matching_current_Go_sources'] == 1652,
                'historical runtime/source reuse differs')
        validation = load('validation.json')
        require(set(validation['gates']) == {'contract_cases', 'historical_collection', 'current_roadmap', 'documentation', 'source_binding'}
                and all(v == 0 for v in validation['gates'].values()) and validation['Go_runtime_tests'] == 'reused_after_source_and_dependency_identity_check',
                'final gate missing/failed')
        progress = json.loads((root / 'docs/spec-driven/KNOWLEDGE-ROADMAP-PROGRESS.json').read_text())
        item = next(i for i in progress['items'] if i['id'] == 'KM-02')
        require(item['subtasks'][0]['status'] == 'complete' and item['status'] in ('in_progress', 'complete'), 'design completion differs')
        require(not roadmap.check(root=root), 'current roadmap rows/evidence disagree')
    except (OSError, ValueError, KeyError, TypeError, IndexError) as error:
        return [str(error)]
    return []


if __name__ == '__main__':
    errors = check()
    for error in errors:
        print('FAIL:', error)
    print('maintenance design evidence:', 'FAIL' if errors else 'PASS; design complete; public execution still pending')
    raise SystemExit(bool(errors))
