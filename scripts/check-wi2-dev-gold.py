#!/usr/bin/env python3
import argparse
import json
import wi2_dev_gold as g
p=argparse.ArgumentParser();p.add_argument('--require-formal-gold',action='store_true');a=p.parse_args();errors=g.validate()+g.current_indices()
if a.require_formal_gold:errors.append('AI DEV source drafts are not independently reviewed gold; D04 unassigned')
print(json.dumps(dict(errors=errors,formal_completed=0,scoring_allowed=False),ensure_ascii=False));raise SystemExit(bool(errors))
