#!/usr/bin/env python3
"""Current KM-02.3-A checkpoint; public durability remains unimplemented."""
import importlib.util
import json
from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]
PACKET = ROOT / 'system/eval/knowledge-roadmap-2026-10-15/km-02-journal-foundation'
spec = importlib.util.spec_from_file_location('journal_runtime_regression', ROOT / 'scripts/check-maintenance-runtime-evidence.py')
runtime = importlib.util.module_from_spec(spec)
spec.loader.exec_module(runtime)


def check(folder=PACKET, root=ROOT):
    errors = runtime.check(folder, root)
    if errors:
        return errors
    try:
        manifest = json.loads((folder / 'manifest.json').read_text())
        runtime.require(manifest['checkpoint'] == 'KM02_3_A_private_journal_and_public_compatibility_only'
                        and manifest['private_journal_foundation_verified'] is True
                        and manifest['KM02_3_complete'] is False, 'private foundation promoted to full/public recovery')
        rows = json.loads((folder / 'race-tests.json').read_text())
        package = 'github.com/0xmhha/knowledge-system/internal/system/maintenance'
        selected = [r for r in rows if r['package'] == package]
        runtime.require(len(selected) == 114 and all(r['state'] == 'pass' for r in selected), 'journal/race cases missing or skipped')
        names = {r['test'] for r in selected}
        required = {'TestJournalProcessLossLeavesOneWholeRecordAndReleasesLock/' + s for s in ('before-rename', 'after-rename')}
        required |= {'TestJournalPersistenceFailureRequiresReopen/' + s for s in ('before-rename', 'after-rename')}
        required |= {'TestJournalFlockPersistsUntilOwnerCloses', 'TestJournalBoundsRecordsWithoutEvictingReplay',
                     'TestJournalRejectsCorruptAndRehashedFalseState/semantic', 'TestDigestPreservesExactLargeIntegerIdentity'}
        runtime.require(required <= names, 'critical journal checks missing')
        history = json.loads((folder / 'historical-design.json').read_text())
        runtime.require(history['scope'] == 'historical_KM02_2_process_lifetime_runtime_not_current_Go_validation'
                        and history['revision'].startswith('31c1f30d') and history['verification_exit_code'] == 0,
                        'historical runtime scope differs')
        progress = json.loads((root / 'docs/spec-driven/KNOWLEDGE-ROADMAP-PROGRESS.json').read_text())
        item = next(i for i in progress['items'] if i['id'] == 'KM-02')
        runtime.require(item['subtasks'][2]['status'] == 'in_progress' and progress['completed_subtasks'] == 6,
                        'unfinished public recovery marked complete')
    except (OSError, ValueError, KeyError, TypeError) as error:
        return [str(error)]
    return []


if __name__ == '__main__':
    errors = check()
    for error in errors:
        print('FAIL:', error)
    print('journal foundation:', 'FAIL' if errors else 'PASS; private storage only; KM-02.3 still in progress')
    raise SystemExit(bool(errors))
