#!/usr/bin/env python3
"""Validate authoring evidence; --require-ready explicitly refuses this draft."""
import argparse
import json
from pathlib import Path
from independent_acceptance import PACKET, ROOT, current_index_errors, gold_errors, packet_errors, read


def check(root, packet):
    return packet_errors(root, packet)


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument('--packet', type=Path, default=ROOT / PACKET)
    parser.add_argument('--require-ready', action='store_true')
    args = parser.parse_args()
    errors = check(ROOT, args.packet)
    if args.packet.resolve() == (ROOT / PACKET).resolve():
        errors.extend(current_index_errors(ROOT))
    missing = {g['work_item_id']: gold_errors(g) for g in read(args.packet / 'gold-authoring-forms.json')}
    if args.require_ready:
        errors.append('D04 unassigned; no unseen FINAL, complete reviewed gold or F01–08 freeze; execution remains disabled')
    print(json.dumps({'preparation_errors': errors, 'gold_authoring_gaps': {k: len(v) for k, v in missing.items()},
                      'formal_ready': False, 'execution_allowed': False}, ensure_ascii=False))
    return bool(errors)


if __name__ == '__main__':
    raise SystemExit(main())
