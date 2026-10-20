#!/usr/bin/env python3
"""Verify KM-02.3-B private service checkpoint, without public recovery claims."""
import importlib.util
import json
from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]
PACKET = ROOT / 'system/eval/knowledge-roadmap-2026-10-15/km-02-durable-service'
spec = importlib.util.spec_from_file_location('private_service_public_regression', ROOT / 'scripts/check-maintenance-runtime-evidence.py')
runtime = importlib.util.module_from_spec(spec)
spec.loader.exec_module(runtime)


def check(folder=PACKET, root=ROOT):
    errors = runtime.check(folder, root)
    if errors:
        return errors
    try:
        manifest = json.loads((folder / 'manifest.json').read_text())
        runtime.require(manifest['checkpoint'] == 'KM02_3_B_private_service_only_not_public_durable_recovery'
                        and manifest['private_service_verified'] is True and manifest['public_durable_connected'] is False
                        and manifest['uncertain_write_returns_status'] is False and manifest['KM02_3_complete'] is False,
                        'private service promoted to public/durable completion or uncertain snapshot')
        rows = json.loads((folder / 'race-tests.json').read_text())
        package = 'github.com/0xmhha/knowledge-system/internal/system/maintenance'
        selected = [r for r in rows if r['package'] == package]
        runtime.require(len(selected) == 123 and all(r['state'] == 'pass' for r in selected), 'private service tests missing/skipped')
        names = {r['test'] for r in selected}
        required = {'TestPrivateDurableServiceReloadsTerminalAndReplaysBeforeChangedPins',
                    'TestPrivateDurableServiceKeepsUnsettledJobsRecovering', 'TestPrivateDurableServiceEnforcesOneRegistryOwner',
                    'TestDurableCancelIsPersistedBeforeAcknowledgment', 'TestFailedCancelPersistenceNeverAcknowledgesOrInventsTerminal',
                    'TestPrivateDurableFailurePersistsThroughActualAdmissionAndRestart'}
        required |= {'TestPrivateDurableAdmissionFailureCannotAcknowledgeOrRun/' + s for s in ('before-rename', 'after-rename')}
        runtime.require(required <= names, 'critical private admission/restart/cancel tests missing')
        setup_names = {r['test'] for r in rows if r['package'].endswith('/internal/setup') and r['state'] == 'pass'}
        runtime.require({'TestTrustedJournalAccessSharesExistingDatasetExclusion', 'TestCanceledJournalAdmissionDoesNotCreateDataset'} <= setup_names,
                        'shared writer/canceled admission proof missing')
        history = json.loads((folder / 'historical-design.json').read_text())
        runtime.require(history['scope'] == 'historical_KM02_3_A_private_foundation_and_process_compatibility_only'
                        and history['revision'].startswith('fff83134') and history['verification_exit_code'] == 0,
                        'historical private checkpoint differs')
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
    print('durable service checkpoint:', 'FAIL' if errors else 'PASS; private B only; public C–E pending')
    raise SystemExit(bool(errors))
