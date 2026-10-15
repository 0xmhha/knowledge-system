"""Verify later maintenance checkpoints in their immutable Git snapshots.

The snapshot uses the actual read-only repository for git show/archive. It
never changes HEAD or the user's work tree and never reuses old Go results for
new sources.
"""
import argparse
import hashlib
import io
import json
import os
from pathlib import Path, PurePosixPath
import subprocess
import tarfile
import tempfile

ROOT = Path(__file__).resolve().parents[1]
BASE = 'system/eval/knowledge-roadmap-2026-10-15/'
CHECKPOINTS = {
    'comparison-contract': {
        'revision': 'c7de05e1', 'base': 'system/eval/knowledge-roadmap-2026-10-16/', 'folder': 'km-06-contract',
        'scope': 'historical_KM06_1_routing_contract_no_actual_source_or_MCP_execution',
        'scripts': ['check-comparison-contract-evidence.py', 'test_comparison_contract_evidence.py'],
    },
    'project-applicability': {
        'revision': 'ba807f5e', 'base': 'system/eval/knowledge-roadmap-2026-10-16/', 'folder': 'km-05-applicability',
        'scope': 'historical_KM05_complete_library_scope_not_public_comparison_MCP',
        'scripts': ['check-project-applicability-evidence.py', 'test_project_applicability_evidence.py'],
    },
    'mapping-review': {
        'revision': 'f2eef29d', 'base': 'system/eval/knowledge-roadmap-2026-10-16/', 'folder': 'km-05-mapping-review',
        'scope': 'historical_KM05_3_direct_mapping_recorded_review_not_applicability_or_MCP',
        'scripts': ['check-mapping-review-evidence.py', 'test_mapping_review_evidence.py'],
    },
    'project-bindings': {
        'revision': '3e3c1022', 'base': 'system/eval/knowledge-roadmap-2026-10-16/', 'folder': 'km-05-bindings',
        'scope': 'historical_KM05_2_source_bound_proposed_project_mappings_not_semantic_approval',
        'scripts': ['check-project-bindings-evidence.py', 'test_project_bindings_evidence.py'],
    },
    'industry-scheme': {
        'revision': 'f597c04a', 'base': 'system/eval/knowledge-roadmap-2026-10-16/', 'folder': 'km-05-scheme',
        'scope': 'historical_KM05_1_optional_vocabulary_not_project_mapping',
        'scripts': ['check-industry-scheme-evidence.py', 'test_industry_scheme_evidence.py'],
    },
    'manager-periodic': {
        'revision': '2f9ddb52', 'base': 'system/eval/knowledge-roadmap-2026-10-16/', 'folder': 'km-04-periodic',
        'scope': 'historical_KM04_complete_actual_two_project_periodic_lifecycle_not_quality',
        'scripts': ['check-manager-periodic-evidence.py', 'test_manager_periodic_evidence.py'],
    },
    'manager-updates-MCP': {
        'revision': '4a2ca4cd', 'base': 'system/eval/knowledge-roadmap-2026-10-16/', 'folder': 'km-04-updates-MCP',
        'scope': 'historical_KM04_3_complete_integrated_updates_MCP_Claude_not_final_periodic',
        'scripts': ['check-manager-updates-MCP-evidence.py', 'test_manager_updates_MCP_evidence.py'],
    },
    'manager-worker-host': {
        'revision': 'ebec0858', 'base': 'system/eval/knowledge-roadmap-2026-10-16/', 'folder': 'km-04-worker-host',
        'scope': 'historical_KM04_3_C1_actual_CLI_worker_lifecycle_not_MCP_operation',
        'scripts': ['check-manager-worker-host-evidence.py', 'test_manager_worker_host_evidence.py'],
    },
    'manager-update-worker': {
        'revision': 'a801f551', 'folder': 'km-04-worker',
        'scope': 'historical_KM04_3_B2_foreground_native_worker_not_sustained_or_host_kill',
        'scripts': ['check-manager-update-worker-evidence.py', 'test_manager_update_worker_evidence.py'],
    },
    'manager-update-state': {
        'revision': 'd00224a5', 'folder': 'km-04-state',
        'scope': 'historical_KM04_3_B1_durable_ledger_not_native_worker',
        'scripts': ['check-manager-update-state-evidence.py', 'test_manager_update_state_evidence.py'],
    },
    'manager-source-check': {
        'revision': '8bfe40e8', 'folder': 'km-04-source-check',
        'scope': 'historical_KM04_3_A_native_original_source_boundary_not_worker',
        'scripts': ['check-manager-source-check-evidence.py', 'test_manager_source_check_evidence.py'],
    },
    'manager-update-inputs': {
        'revision': '2c03322f', 'folder': 'km-04-inputs',
        'scope': 'historical_KM04_2_observed_inputs_not_scheduler_runtime',
        'scripts': ['check-manager-update-inputs-evidence.py', 'test_manager_update_inputs_evidence.py'],
    },
    'manager-update-contract': {
        'revision': '7268ac7f', 'folder': 'km-04-contract',
        'scope': 'historical_KM04_1_policy_design_not_scheduler_runtime',
        'scripts': ['check-knowledge-manager-update-contract-evidence.py', 'test_knowledge_manager_update_contract_evidence.py'],
    },
    'manager-host': {
        'revision': 'cea1c943', 'folder': 'km-03-host',
        'scope': 'historical_KM03_complete_actual_local_Claude_transport_not_model_quality',
        'scripts': ['check-knowledge-manager-host-evidence.py', 'test_knowledge_manager_host_evidence.py'],
    },
    'manager-broker': {
        'revision': 'c51f7423', 'folder': 'km-03-broker',
        'scope': 'historical_KM03_4_B_scoped_broker_not_actual_Claude_host',
        'scripts': ['check-knowledge-manager-broker-evidence.py', 'test_knowledge_manager_broker_evidence.py'],
    },
    'manager-document-index': {
        'revision': 'bb71aae3', 'folder': 'km-03-document-index',
        'scope': 'historical_KM03_4_A_native_document_index_not_manager_broker_or_host',
        'scripts': ['check-manager-document-index-evidence.py', 'test_manager_document_index_evidence.py'],
    },
    'manager-review': {
        'revision': '3d39801c', 'folder': 'km-03-review',
        'scope': 'historical_KM03_3_supplied_review_and_filtered_input_only',
        'scripts': ['check-knowledge-manager-review-evidence.py', 'test_knowledge_manager_review_evidence.py'],
    },
    'manager-collection': {
        'revision': '322daf39', 'folder': 'km-03-collection',
        'scope': 'historical_KM03_2_proposed_collection_not_review_or_reader',
        'scripts': ['check-knowledge-manager-collection-evidence.py', 'test_knowledge_manager_collection_evidence.py'],
    },
    'manager-bootstrap': {
        'revision': '2c2b7ed1', 'folder': 'km-03-bootstrap',
        'scope': 'historical_KM03_1_owned_registration_metadata_only',
        'scripts': ['check-knowledge-manager-bootstrap-evidence.py', 'test_knowledge_manager_bootstrap_evidence.py'],
    },
    'public-failures': {
        'revision': '6705ed57', 'folder': 'km-02-public-failures',
        'scope': 'historical_KM02_complete_actual_public_failure_matrix',
        'scripts': ['check-maintenance-failure-evidence.py', 'test_maintenance_failure_evidence.py'],
    },
    'durable-retention': {
        'revision': 'ae3f987d', 'folder': 'km-02-durable-retention',
        'scope': 'historical_KM02_3_E2_actual_reader_rollback_GC_complete',
        'scripts': ['check-maintenance-retention-evidence.py', 'test_maintenance_retention_evidence.py'],
    },
    'durable-host': {
        'revision': '0fa42cef', 'folder': 'km-02-durable-host',
        'scope': 'historical_KM02_3_E1_actual_durable_host_only',
        'scripts': ['check-maintenance-durable-host-evidence.py', 'test_maintenance_durable_host_evidence.py'],
    },
    'owned-staging': {
        'revision': 'ed74cd7f', 'folder': 'km-02-owned-staging',
        'scope': 'historical_KM02_3_D2_owned_staging_process_compatibility_only',
        'scripts': ['check-maintenance-staging-evidence.py', 'test_maintenance_staging_evidence.py'],
    },
    'command-supervision': {
        'revision': '1283a22c', 'folder': 'km-02-command-supervision',
        'scope': 'historical_KM02_3_D1_parent_deadline_supervision_process_compatibility_only',
        'scripts': ['check-maintenance-supervision-evidence.py', 'test_maintenance_supervision_evidence.py'],
    },
    'journal-foundation': {
        'revision': 'fff83134', 'folder': 'km-02-journal-foundation',
        'scope': 'historical_KM02_3_A_private_foundation_and_process_compatibility_only',
        'scripts': ['check-maintenance-journal-foundation.py', 'test_maintenance_journal_foundation.py'],
    },
    'publication-recovery': {
        'revision': '17633b22', 'folder': 'km-02-publication-recovery',
        'scope': 'historical_KM02_3_C_managed_history_private_recovery_process_compatibility_only',
        'scripts': ['check-maintenance-publication-evidence.py', 'test_maintenance_publication_evidence.py'],
    },
    'private-service': {
        'revision': '29169b2b', 'folder': 'km-02-durable-service',
        'scope': 'historical_KM02_3_B_private_service_and_process_compatibility_only',
        'scripts': ['check-maintenance-durable-service-evidence.py', 'test_maintenance_durable_service_evidence.py'],
    },
}


def verify(checkpoint='journal-foundation', root=ROOT):
    selected = CHECKPOINTS[checkpoint]
    revision = subprocess.check_output(['git', 'rev-parse', selected['revision'] + '^{commit}'], cwd=root, text=True).strip()
    def read(name):
        return subprocess.check_output(['git', 'show', revision + ':' + name], cwd=root)
    folder = selected.get('base',BASE) + selected['folder']
    manifest, binding = json.loads(read(folder + '/manifest.json')), json.loads(read(folder + '/source-binding.json'))
    paths = set(manifest['source_sha256']) | set(binding['approved_input_sha256']) | {'scripts', BASE.rstrip('/'), folder, 'system/contracts/versioned-maintenance'}
    paths.update('docs/spec-driven/' + name for name in ('KNOWLEDGE-ROADMAP-PROGRESS.json', 'KNOWLEDGE-MANAGER-AND-COMPARISON-SPEC.md',
        'KNOWLEDGE-ROADMAP-TEST-MATRIX.md', 'PROJECT-COLLECTION-ISOLATION.md', 'VERSIONED-MAINTENANCE-CONTRACT.md', 'VERSIONED-MAINTENANCE-RUNTIME.md'))
    for name in paths:
        p = PurePosixPath(name)
        if p.is_absolute() or '..' in p.parts or str(p) != name:
            raise ValueError('unsafe committed path')
    data = subprocess.check_output(['git', 'archive', '--format=tar', revision, '--', *sorted(paths)], cwd=root)
    env = os.environ.copy()
    env.update(GIT_DIR=subprocess.check_output(['git', 'rev-parse', '--absolute-git-dir'], cwd=root, text=True).strip(), PYTHONDONTWRITEBYTECODE='1')
    receipts = []
    with tempfile.TemporaryDirectory(prefix='ks-maintenance-checkpoint-', dir='/private/tmp') as temp:
        snapshot = Path(temp).resolve()
        env['GIT_WORK_TREE'] = str(snapshot)
        with tarfile.open(fileobj=io.BytesIO(data)) as archive:
            for item in archive:
                target = (snapshot / item.name).resolve()
                if not target.is_relative_to(snapshot) or not (item.isdir() or item.isfile()):
                    raise ValueError('unsafe committed archive')
                if item.isdir():
                    target.mkdir(parents=True, exist_ok=True)
                else:
                    target.parent.mkdir(parents=True, exist_ok=True)
                    target.write_bytes(archive.extractfile(item).read())
        for script in selected['scripts']:
            result = subprocess.run(['python3', 'scripts/' + script], cwd=snapshot, env=env, capture_output=True, timeout=90)
            receipts.append({'script': script, 'exit_code': result.returncode,
                             'stdout_sha256': hashlib.sha256(result.stdout).hexdigest(), 'stderr': result.stderr.decode().strip()})
            if result.returncode:
                raise ValueError('historical checkpoint proof failed: ' + result.stdout.decode() + result.stderr.decode())
    return {'scope': selected['scope'], 'revision': revision,
            'manifest_sha256': hashlib.sha256(read(folder + '/manifest.json')).hexdigest(),
            'verification_exit_code': 0, 'checks': receipts, 'current_Go_validation_reused': False}


if __name__ == '__main__':
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--checkpoint', choices=sorted(CHECKPOINTS), default='journal-foundation')
    print(json.dumps(verify(parser.parse_args().checkpoint), indent=2))
