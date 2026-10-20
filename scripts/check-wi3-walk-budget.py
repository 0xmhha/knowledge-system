#!/usr/bin/env python3
"""Technical walk/time/measurement checks do not approve independent gold."""
import argparse,json
import wi3_walk_budget as g
if __name__=='__main__':
    p=argparse.ArgumentParser();p.add_argument('--require-formal-gold',action='store_true');p.add_argument('--require-stored',action='store_true');a=p.parse_args()
    errors=g.validate()+g.current_indices()
    if a.require_stored:errors.extend(g.storage_errors())
    if a.require_formal_gold:errors.append('Exposed technical walk/budget/measurement regression is not independent semantic approval')
    print(json.dumps(dict(errors=errors,formal_completed=0,formal_total=9,execution_allowed=False)));raise SystemExit(bool(errors))
