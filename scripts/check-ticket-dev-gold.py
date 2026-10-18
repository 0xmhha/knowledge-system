#!/usr/bin/env python3
"""Check shared ticket AI drafts without granting independent approval."""
import argparse,json
import ticket_dev_gold as g
if __name__=='__main__':
    p=argparse.ArgumentParser();p.add_argument('--require-formal-gold',action='store_true');a=p.parse_args()
    errors=g.validate()+g.current_indices()
    if a.require_formal_gold:errors.append('D04 unassigned; shared exposed ticket/local AI membership is not independent approved API gold')
    print(json.dumps(dict(errors=errors,D04='미정',formal_completed=0,formal_total=9,execution_allowed=False),ensure_ascii=False));raise SystemExit(bool(errors))
