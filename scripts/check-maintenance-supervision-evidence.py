#!/usr/bin/env python3
"""D1 supervisor checkpoint; staging cleanup and public durable E remain pending."""
import importlib.util
import json
from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]
PACKET = ROOT / 'system/eval/knowledge-roadmap-2026-10-15/km-02-command-supervision'
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
        runtime.require(manifest['checkpoint'] == 'KM02_3_D1_parent_deadline_supervision_process_compatibility_only'
                        and manifest['parent_deadline_supervision_verified'] is True and manifest['staging_cleanup_verified'] is False
                        and manifest['public_durable_connected'] is False and manifest['KM02_3_complete'] is False,
                        'supervision promoted to cleanup/full/public durability')
        rows = load('race-tests.json')
        setup = {r['test'] for r in rows if r['package'].endswith('/internal/setup') and r['state'] == 'pass'}
        required = {'TestGuardianPreservesExactArgvOutputAndWorkingDirectory', 'TestGuardianCancellationStopsSameGroupLateWriter',
                    'TestInheritedWriterLeaseSurvivesParentDescriptorClose', 'TestGuardianOwnerProcessHelper',
                    'TestGuardianActualOwnerKillStopsWriterAndReleasesLease', 'TestGuardianAbsoluteDeadlineWorksWhileOwnerSocketStaysOpen',
                    'TestDerivedFreshVerificationHonorsContextDeadline'}
        runtime.require(required <= setup, 'parent/lease/deadline/context supervision cases missing')
        selected = [r for r in rows if r['package'].endswith('/internal/system/maintenance')]
        runtime.require(len(selected) == 141 and all(r['state'] == 'pass' for r in selected), 'existing maintenance recovery cases differ')
        history = load('historical-design.json')
        runtime.require(history['scope'] == 'historical_KM02_3_C_managed_history_private_recovery_process_compatibility_only'
                        and history['revision'].startswith('17633b22'), 'historical C proof differs')
        progress = json.loads((root / 'docs/spec-driven/KNOWLEDGE-ROADMAP-PROGRESS.json').read_text())
        item = next(i for i in progress['items'] if i['id'] == 'KM-02')
        runtime.require(item['subtasks'][2]['status'] == 'in_progress' and progress['completed_subtasks'] == 6,
                        'unfinished staging/public E counted complete')
    except (OSError, ValueError, KeyError, TypeError) as error:
        return [str(error)]
    return []


if __name__ == '__main__':
    errors = check()
    for error in errors:
        print('FAIL:', error)
    print('supervision checkpoint:', 'FAIL' if errors else 'PASS; D1 only; cleanup/public E pending')
    raise SystemExit(bool(errors))
