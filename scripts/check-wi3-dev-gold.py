#!/usr/bin/env python3
"""Check finite AI draft and approval boundaries, not semantic acceptance."""
import argparse
import json
import wi3_dev_gold as g

if __name__=='__main__':
    parser=argparse.ArgumentParser();parser.add_argument('--require-formal-gold',action='store_true');args=parser.parse_args()
    errors=g.validate()+g.current_indices()
    if args.require_formal_gold: errors.append('D04 unassigned; exposed bounded AI DEV is not independent approved corpus-wide gold')
    print(json.dumps(dict(errors=errors,D04='미정',formal_completed=0,formal_total=9,execution_allowed=False),ensure_ascii=False))
    raise SystemExit(bool(errors))
