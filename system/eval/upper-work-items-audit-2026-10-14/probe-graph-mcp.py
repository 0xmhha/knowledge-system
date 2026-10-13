#!/usr/bin/env python3
"""Audit probe: owned synthetic graph only; no chain, model, or personal data."""
import argparse
import json
from pathlib import Path
import select
import sqlite3
import subprocess
import tempfile


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument('--binary', required=True)
    parser.add_argument('--out', required=True)
    args = parser.parse_args()
    binary = str(Path(args.binary).resolve())
    with tempfile.TemporaryDirectory(prefix='ks-upper-wi-probe-') as tmp:
        root = Path(tmp)
        src, graph = root / 'src', root / 'graph'
        src.mkdir()
        (src / 'go.mod').write_text('module example.test/audit\n\ngo 1.26\n')
        (src / 'safe.go').write_text('package audit\nfunc Safe() int { return 1 }\nfunc Caller() int { return Safe() }\n')
        builds = []

        def build(*extra):
            run = subprocess.run([binary, 'build', '--src', str(src), '--out',
                                  str(graph), '--no-cache', '--force', *extra],
                                 capture_output=True, text=True, timeout=90)
            builds.append({'exit_code': run.returncode, 'stderr': run.stderr})
            run.check_returncode()

        build()
        with sqlite3.connect(graph / 'graph.db') as db:
            qname = db.execute("SELECT qualified_name FROM nodes WHERE name='Safe' AND type='Function'").fetchone()[0]
        policy, security = root / 'policy.yaml', root / 'security.yaml'
        policy.write_text('policies:\n  - id: audit.policy\n    name: Audit policy\n    category: audit\n    description: Synthetic reviewed annotation\n    governs:\n      - ' + qname + '\n')
        security.write_text('security_patterns:\n  - id: audit.security\n    name: Audit security\n    category: access-control\n    severity: high\n    description: Synthetic manually assigned risk\n    remediation: Synthetic hint\n    matches:\n      - ' + qname + '\n')
        build('--policy-file', str(policy), '--security-pattern-file', str(security))
        with (root / 'mcp.stderr').open('w+') as stderr:
            proc = subprocess.Popen([binary, 'mcp', '--graph', str(graph)],
                                    stdin=subprocess.PIPE, stdout=subprocess.PIPE,
                                    stderr=stderr, text=True, bufsize=1)
            responses = []

            def request(method, params=None):
                msg = {'jsonrpc': '2.0', 'id': len(responses) + 1, 'method': method}
                if params is not None:
                    msg['params'] = params
                proc.stdin.write(json.dumps(msg) + '\n')
                proc.stdin.flush()
                if not select.select([proc.stdout], [], [], 30)[0]:
                    raise TimeoutError(method)
                response = json.loads(proc.stdout.readline())
                responses.append({'request': msg, 'response': response})
                return response

            try:
                request('initialize', {'protocolVersion': '2024-11-05', 'capabilities': {},
                                       'clientInfo': {'name': 'upper-wi-audit', 'version': '1'}})
                proc.stdin.write(json.dumps({'jsonrpc': '2.0', 'method': 'notifications/initialized'}) + '\n')
                proc.stdin.flush()
                request('tools/list')
                request('tools/call', {'name': 'get_subgraph', 'arguments': {'seed_qname': qname, 'depth': 1}})
                request('tools/call', {'name': 'search_text', 'arguments': {'query': 'Audit', 'node_kinds': ['Policy', 'SecurityPattern']}})
                request('tools/call', {'name': 'find_symbol', 'arguments': {'name': 'audit.policy', 'exact': True}})
                request('tools/call', {'name': 'find_symbol', 'arguments': {'name': 'audit.security', 'exact': True}})
                request('tools/call', {'name': 'find_symbol', 'arguments': {'name': 'audit.missing', 'exact': True}})
                proc.stdin.close()
                code = proc.wait(timeout=15)
            finally:
                if proc.poll() is None:
                    proc.kill()
                    proc.wait()
            stderr.seek(0)
            result = {'scope': 'synthetic manual annotation visibility, not automatic vulnerability detection',
                      'fixture_qname': qname, 'builds': builds, 'responses': responses,
                      'mcp_exit_code': code, 'mcp_stderr': stderr.read()}
        Path(args.out).write_text(json.dumps(result, indent=2) + '\n')
        assert code == 0, code
        assert all('error' not in r['response'] for r in responses)
        assert all(not r['response']['result'].get('isError', False) for r in responses)
        subgraph = responses[2]['response']['result']['structuredContent']
        assert {'Policy', 'SecurityPattern'} <= {n['type'] for n in subgraph['nodes']}
        assert {'governed_by', 'has_security_pattern'} <= {e['type'] for e in subgraph['edges']}
        for index, expected in [(4, 'Policy'), (5, 'SecurityPattern')]:
            nodes = responses[index]['response']['result']['structuredContent']['nodes']
            assert len(nodes) == 1 and nodes[0]['type'] == expected
        assert responses[6]['response']['result']['structuredContent']['nodes'] == []
        print('graph MCP synthetic annotation probe: PASS')


if __name__ == '__main__':
    main()
