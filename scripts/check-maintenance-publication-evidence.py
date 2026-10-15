#!/usr/bin/env python3
"""Verify C's managed history and private recovery without claiming D/E."""
import hashlib
import importlib.util
import io
import json
from pathlib import Path
import tarfile

ROOT = Path(__file__).resolve().parents[1]
PACKET = ROOT / 'system/eval/knowledge-roadmap-2026-10-15/km-02-publication-recovery'
spec = importlib.util.spec_from_file_location('publication_process_regression', ROOT / 'scripts/check-maintenance-runtime-evidence.py')
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
        runtime.require(manifest['checkpoint'] == 'KM02_3_C_managed_history_private_recovery_process_compatibility_only'
                        and manifest['managed_publication_verified'] is True and manifest['private_service_recovery_verified'] is True
                        and manifest['public_durable_connected'] is False and manifest['pre_gate_child_termination_verified'] is False
                        and manifest['synthetic_history_is_real_publication'] is False and manifest['KM02_3_complete'] is False,
                        'C history/private recovery promoted to public/full/child termination')
        rows = load('race-tests.json')
        selected = [r for r in rows if r['package'].endswith('/internal/system/maintenance')]
        runtime.require(len(selected) == 141 and all(r['state'] == 'pass' for r in selected), 'private recovery cases missing/skipped')
        names = {r['test'] for r in selected}
        runtime.require({'TestPrivateRecoveryUsesHistoricalPublicationWithoutInventingServingAdoption',
                         'TestCancelIntentSurvivesLegacyRecordAndMultipleRecoveryRestarts',
                         'TestPrivateRecoveryRefusesCommittedHistoryContradictingAcceptedCancel',
                         'TestAcceptedCancelOfBuiltHeldCandidateCannotRecoverAsHeld',
                         'TestPrivateRecoveryVerifiesActualRetainedHeldDatabase'} <= names, 'critical service recovery cases missing')
        setup = {r['test'] for r in rows if r['package'].endswith('/internal/setup') and r['state'] == 'pass'}
        required = {'TestManagedPublicationRealProcessKillRecovery/' + s for s in ('publication-intent', 'before-rename', 'after-rename', 'current-parent', 'publication-history')}
        required |= {'TestManagedPublicationReceiptSurvivesLaterTransitionAndRollback', 'TestOrdinaryManagedWriterSettlesPreviousCrashBeforeAdvancing'}
        runtime.require(required <= setup, 'real pointer/kill/later-writer history cases missing')
        history = load('historical-design.json')
        runtime.require(history['scope'] == 'historical_KM02_3_B_private_service_and_process_compatibility_only'
                        and history['revision'].startswith('29169b2b'), 'historical B checkpoint differs')
        fixture = (folder / 'fixture.tar.gz').read_bytes()
        import gzip
        with tarfile.open(fileobj=io.BytesIO(gzip.decompress(fixture))) as archive:
            entries = {m.name: m for m in archive}
            observations = json.loads(gzip.decompress((folder / 'runtime/observations.json.gz').read_bytes()))
            successes = {r['status']['job_id']: r for r in observations if r['status']['state'] == 'succeeded'}
            runtime.require(len(successes) == 2, 'actual public success cases differ')
            for job_id, row in successes.items():
                status, request = row['status'], row['request']
                name = 'project-a/dataset/.maintenance-publications/' + job_id + '.json'
                runtime.require(name in entries and entries[name].isfile(), 'public operation history missing')
                envelope = json.loads(archive.extractfile(entries[name]).read())
                proof = envelope['proof']
                body = json.dumps(proof, ensure_ascii=False, separators=(',', ':')).encode()
                checksum = hashlib.sha256(b'cks.managed-publication.v1\0' + len(body).to_bytes(8, 'big') + body).hexdigest()
                runtime.require(checksum == envelope['checksum'] and proof['outcome'] == 'committed'
                                and proof['operation_id'] == job_id and proof['project_id'] == request['project_id']
                                and proof['version'] == status['target_version'] and proof['identity']['dataset_id'] == status['candidate']['coordinates']['dataset_id']
                                and proof['identity']['input_digest'] == request['input_digest'] and proof['confirmed_at'] == status['publication']['published_at']
                                and proof['previous_version'] == request['base'].get('version', ''), 'public history/receipt join differs')
            runtime.require(not any(n.endswith('/pending.json') and '/.maintenance-publications/' in n for n in entries), 'public success left pending history')
        progress = json.loads((root / 'docs/spec-driven/KNOWLEDGE-ROADMAP-PROGRESS.json').read_text())
        item = next(i for i in progress['items'] if i['id'] == 'KM-02')
        runtime.require(item['subtasks'][2]['status'] == 'in_progress' and progress['completed_subtasks'] == 6,
                        'D/E unfinished but KM-02.3 marked complete')
    except (OSError, ValueError, KeyError, TypeError, tarfile.TarError) as error:
        return [str(error)]
    return []


if __name__ == '__main__':
    errors = check()
    for error in errors:
        print('FAIL:', error)
    print('publication checkpoint:', 'FAIL' if errors else 'PASS; C only; D/E and public durability pending')
    raise SystemExit(bool(errors))
