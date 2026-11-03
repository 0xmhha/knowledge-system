"""Tally T1 (knowledge tool calls), T2 (cost, tokens, turns) and L1 (leakage) for each arm from runs/<arm>/stream.jsonl.

usage: python3 tally.py > tally.json
C1-C4 are judged by reading runs/<arm>/diagnosis.md, not here.
"""
import collections, json, os, re

M = os.path.dirname(os.path.abspath(__file__))
LEAK = re.compile(r'test-data/pr-77(?!-procedure)|expert-fix|oracle/|wemade/go-stablenet|knowledge-system/projects|pr-77-procedure/(work|runs|plugins)/(A0|A1)')


def tally(arm):
    other = 'A1' if arm == 'A0' else 'A0'
    tools, leaks, result = collections.Counter(), [], None
    for line in open(os.path.join(M, 'runs', arm, 'stream.jsonl')):
        try:
            d = json.loads(line)
        except ValueError:
            continue
        if d.get('type') == 'result':
            result = d
        msg = d.get('message') if isinstance(d.get('message'), dict) else {}
        for block in msg.get('content') or []:
            if not isinstance(block, dict) or block.get('type') != 'tool_use':
                continue
            name = block.get('name', '')
            tools[name.split('__')[-1] if 'stablenet-knowledge' in name else name] += 1
            text = json.dumps(block.get('input'))
            for m in LEAK.finditer(text):
                hit = m.group(0)
                if hit.endswith('/' + arm):
                    continue  # the arm's own directory
                leaks.append({'tool': name, 'match': hit, 'input': text[:300]})
    usage = (result or {}).get('usage') or {}
    return {
        'tool_calls': dict(tools.most_common()),
        'review_function': tools.get('cks_context_review_function', 0),
        'value_dependencies': tools.get('cks_context_value_dependencies', 0),
        'cost_usd': (result or {}).get('total_cost_usd'),
        'num_turns': (result or {}).get('num_turns'),
        'duration_ms': (result or {}).get('duration_ms'),
        'terminal_reason': (result or {}).get('terminal_reason'),
        'usage': {k: usage.get(k) for k in ('input_tokens', 'cache_creation_input_tokens', 'cache_read_input_tokens', 'output_tokens')},
        'model_usage': {k: round(v.get('costUSD', 0), 4) for k, v in ((result or {}).get('modelUsage') or {}).items()},
        'permission_denials': len((result or {}).get('permission_denials') or []),
        'leaks': leaks,
    }


print(json.dumps({arm: tally(arm) for arm in ('A0', 'A1')}, indent=2, ensure_ascii=False))
