#!/usr/bin/env python3
"""Fresh technical failure metadata verification, never independent acceptance."""
import argparse,json
import wi3_reason_fix as g
if __name__=='__main__':
    p=argparse.ArgumentParser();p.add_argument('--require-formal-gold',action='store_true');a=p.parse_args();errors=g.validate()+g.current_indices()
    if a.require_formal_gold:errors.append('Fresh native and controlled MCP regressions do not approve independent semantic gold')
    print(json.dumps(dict(errors=errors,formal_completed=0,formal_total=9,execution_allowed=False),ensure_ascii=False));raise SystemExit(bool(errors))
