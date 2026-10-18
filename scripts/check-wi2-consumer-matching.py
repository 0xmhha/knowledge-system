#!/usr/bin/env python3
"""Check observed matching and source draft separation; never approve gold."""
import argparse
import json
import wi2_consumer_matching as g

if __name__ == '__main__':
    parser = argparse.ArgumentParser()
    parser.add_argument('--require-formal-gold', action='store_true')
    args = parser.parse_args()
    errors = g.validate()+g.current_indices()
    if args.require_formal_gold:
        errors.append('D04 unassigned; AI DEV draft and observed mapping are not independent approved gold')
    print(json.dumps(dict(errors=errors, D04='미정', formal_completed=0, formal_total=9,
                         freeze=None, execution_allowed=False), ensure_ascii=False))
    raise SystemExit(bool(errors))
