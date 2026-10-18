#!/usr/bin/env python3
import argparse
import json
import wi1_consumer_matching as g
p=argparse.ArgumentParser();p.add_argument('--require-formal-gold',action='store_true');a=p.parse_args()
e=g.validate()+g.current_indices()
if a.require_formal_gold:e.append('AI DEV matching is not independent approved semantic gold; D04 unassigned')
print(json.dumps(dict(errors=e,formal_completed=0,scoring_allowed=False),ensure_ascii=False));raise SystemExit(bool(e))
