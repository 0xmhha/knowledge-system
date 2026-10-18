#!/usr/bin/env python3
"""Validate finite source review drafts, refusing formal semantic promotion."""
import argparse,json
import wic_dev_gold as g
if __name__=='__main__':
    parser=argparse.ArgumentParser();parser.add_argument('--require-formal-gold',action='store_true');a=parser.parse_args();errors=g.validate()+g.current_indices()
    if a.require_formal_gold:errors.append('Review/approval pending; finite AI source/consumer contracts are not independent complete semantic gold')
    print(json.dumps(dict(errors=errors,formal_completed=0,formal_total=9,execution_allowed=False),ensure_ascii=False));raise SystemExit(bool(errors))
