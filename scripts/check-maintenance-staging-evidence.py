#!/usr/bin/env python3
"""D2 owned staging checkpoint; public durable E remains pending."""
import importlib.util
import json
from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]
PACKET = ROOT / 'system/eval/knowledge-roadmap-2026-10-15/km-02-owned-staging'
spec = importlib.util.spec_from_file_location('supervision_process_regression', ROOT / 'scripts/check-maintenance-runtime-evidence.py')
runtime = importlib.util.module_from_spec(spec)
spec.loader.exec_module(runtime)


def check(folder=PACKET, root=ROOT):
    errors = runtime.check(folder, root)
    if errors:
        return errors
    try:
        def load(name):
            return json.loads((folder / name).read_text())
        manifest = load('manifest.json')
        runtime.require(manifest['checkpoint'] == 'KM02_3_D2_owned_staging_process_compatibility_only'
                        and manifest['parent_deadline_supervision_verified'] is True and manifest['staging_cleanup_verified'] is True
                        and manifest['public_durable_connected'] is False and manifest['KM02_3_complete'] is False,
                        'staging checkpoint promoted to full/public durability')
        rows = load('race-tests.json')
        setup = {r['test'] for r in rows if r['package'].endswith('/internal/setup') and r['state'] == 'pass'}
        required = {'TestGuardianPreservesExactArgvOutputAndWorkingDirectory', 'TestGuardianCancellationStopsSameGroupLateWriter',
                    'TestInheritedWriterLeaseSurvivesParentDescriptorClose', 'TestGuardianOwnerProcessHelper',
                    'TestGuardianActualOwnerKillStopsWriterAndReleasesLease', 'TestGuardianAbsoluteDeadlineWorksWhileOwnerSocketStaysOpen',
                    'TestDerivedFreshVerificationHonorsContextDeadline', 'TestOwnedStagingRemovesOnlyExactCandidateScratch',
                    'TestOwnedStagingRefusesForeignOrLinkedPaths', 'TestOwnedStagingRecoversRealProcessKillBoundaries',
                    'TestOwnedStagingCanceledCleanupRetainsRecoveryRecord'}
        runtime.require(required <= setup, 'parent/lease/deadline/context supervision cases missing')
        selected = [r for r in rows if r['package'].endswith('/internal/system/maintenance')]
        runtime.require(len(selected) == 144 and all(r['state'] == 'pass' for r in selected), 'existing maintenance recovery cases differ'
                        )
        runtime.require('TestStagingProtocolRequiresReleasedWriterBeforeInterrupting' in {r['test'] for r in selected}, 'lease/staging protocol recovery missing')
        history = load('historical-design.json')
        runtime.require(history['scope'] == 'historical_KM02_3_D1_parent_deadline_supervision_process_compatibility_only'
                        and history['revision'].startswith('1283a22c'), 'historical D1 proof differs')
        progress = json.loads((root / 'docs/spec-driven/KNOWLEDGE-ROADMAP-PROGRESS.json').read_text())
        item = next(i for i in progress['items'] if i['id'] == 'KM-02')
        runtime.require(item['subtasks'][2]['status'] == 'in_progress' and progress['completed_subtasks'] == 6,
                        'unfinished public E counted complete')
    except (OSError, ValueError, KeyError, TypeError) as error:
        return [str(error)]
    return []


if __name__ == '__main__':
    errors = check()
    for error in errors:
        print('FAIL:', error)
    print('owned staging checkpoint:', 'FAIL' if errors else 'PASS; D2 only; public E pending')
    raise SystemExit(bool(errors))
