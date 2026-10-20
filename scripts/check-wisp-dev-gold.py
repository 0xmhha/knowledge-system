#!/usr/bin/env python3
"""Validate finite WI-SP AI drafts without claiming semantic acceptance."""
import argparse,json
import wisp_dev_gold as g
if __name__=='__main__':
    parser=argparse.ArgumentParser();parser.add_argument('--require-formal-gold',action='store_true');a=parser.parse_args()
    errors=g.validate()+g.current_indices()
    if a.require_formal_gold:errors.append('D04 unassigned; literal operator/proposed security metadata is not independent approved semantic gold')
    print(json.dumps(dict(errors=errors,D04='미정',formal_completed=0,formal_total=9,execution_allowed=False),ensure_ascii=False));raise SystemExit(bool(errors))
