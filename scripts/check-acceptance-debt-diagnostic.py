#!/usr/bin/env python3
"""Direct provider diagnostics never approve semantic gold or formal execution."""
import argparse,json
import acceptance_debt_diagnostic as g
if __name__=='__main__':
    p=argparse.ArgumentParser();p.add_argument('--require-formal-gold',action='store_true');a=p.parse_args();errors=g.validate()+g.current_indices()
    if a.require_formal_gold:errors.append('Actual direct graph diagnostic is not complete semantic gold or independent acceptance')
    print(json.dumps(dict(errors=errors,formal_completed=0,formal_total=9,execution_allowed=False),ensure_ascii=False));raise SystemExit(bool(errors))
