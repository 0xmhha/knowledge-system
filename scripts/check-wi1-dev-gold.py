#!/usr/bin/env python3
import argparse
import json
from pathlib import Path
from wi1_dev_gold import PACKET, ROOT, current_indices, validate_packet


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument('--packet', type=Path, default=ROOT / PACKET)
    parser.add_argument('--require-formal-gold', action='store_true')
    args = parser.parse_args(); errors = validate_packet(ROOT, args.packet)
    if args.packet.resolve() == (ROOT / PACKET).resolve():
        errors.extend(current_indices(ROOT))
    if args.require_formal_gold:
        errors.append('AI DEV draft is not independently reviewed formal gold; D04 remains unassigned')
    print(json.dumps({'errors': errors, 'independent_approved': 0, 'formal_scoring_allowed': False}, ensure_ascii=False))
    return bool(errors)


if __name__ == '__main__':
    raise SystemExit(main())
