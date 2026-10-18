#!/usr/bin/env python3
"""Validate context and observed source mapping; never approve formal gold."""
import argparse
import json
import wi3_consumer_matching as g

if __name__=='__main__':
    parser=argparse.ArgumentParser();parser.add_argument('--require-formal-gold',action='store_true');a=parser.parse_args()
    errors=g.validate()+g.current_indices()
    if a.require_formal_gold:errors.append('D04 unassigned; partial observed matching and AI overlay are not independent approved gold')
    print(json.dumps(dict(errors=errors,D04='미정',formal_completed=0,formal_total=9,execution_allowed=False),ensure_ascii=False));raise SystemExit(bool(errors))
