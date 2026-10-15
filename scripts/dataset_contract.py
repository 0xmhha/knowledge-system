"""KM-02.1 design conformance oracle; does not build, publish or cancel jobs."""
import hashlib
import json
from pathlib import Path

from jsonschema import Draft202012Validator, FormatChecker

ROOT = Path(__file__).resolve().parents[1]
CONTRACT = ROOT / 'system/contracts/versioned-maintenance/v1'
SCHEMA = json.loads((CONTRACT / 'schema.json').read_text())
MACHINE = json.loads((CONTRACT / 'state-machine.json').read_text())


class Rejected(ValueError):
    def __init__(self, code):
        self.code = code
        super().__init__(code)


def require(condition, code):
    if not condition:
        raise Rejected(code)


def canonical(value):
    return json.dumps(value, sort_keys=True, ensure_ascii=False, separators=(',', ':'), allow_nan=False).encode()


def digest(value, namespace='cks.dataset-build.v1'):
    return hashlib.sha256(namespace.encode() + b'\0' + canonical(value)).hexdigest()


def decode(raw):
    require(len(raw) <= 64 << 10, 'invalid_request')
    def unique(pairs):
        result = {}
        for key, value in pairs:
            require(key not in result, 'invalid_request')
            result[key] = value
        return result
    def invalid_constant(_):
        raise Rejected('invalid_request')
    try:
        return json.loads(raw, object_pairs_hook=unique, parse_constant=invalid_constant, parse_float=invalid_constant)
    except (ValueError, TypeError):
        raise Rejected('invalid_request') from None


def shape(name, value, code='invalid_request'):
    schema = {'$schema': SCHEMA['$schema'], '$defs': SCHEMA['$defs'], '$ref': '#/$defs/' + name}
    require(not list(Draft202012Validator(schema, format_checker=FormatChecker()).iter_errors(value)), code)
    for kind, key in [('buildRequest', 'deadline_seconds'), ('jobStatus', 'revision'), ('profileContext', 'timeout_max_seconds')]:
        if name == kind:
            require(type(value[key]) is int, code)


def validate_request(request, profile):
    shape('buildRequest', request)
    shape('profileContext', profile)
    require(request['project_id'] == profile['project_id'], 'project_mismatch')
    require(request['profile_id'] == profile['profile_id'], 'profile_unknown')
    require(request['profile_digest'] == profile['profile_digest'], 'profile_stale')
    require({'source', 'identity', 'graph', 'vector'} <= set(profile['required_gates']), 'unsupported_capability')
    pin = request['source_pin']
    require(bool(pin['base_commit']) if pin['mode'] == 'committed' else True, 'invalid_request')
    require(pin['base_commit'] == '' if pin['mode'] == 'snapshot-only' else True, 'invalid_request')
    require(pin == profile['source_pin'], 'source_stale')
    require(request['input_digest'] == profile['input_digest'], 'input_stale')
    require(request['embedding_digest'] == profile['embedding_digest'], 'embedding_stale')
    if request['base']['state'] == 'pinned':
        require(request['base']['coordinates']['project_id'] == request['project_id'], 'project_mismatch')
    require(request['base'] == profile['observed_base'], 'base_stale')
    require(request['new_version'].casefold() not in {v.casefold() for v in profile['existing_versions']}, 'version_exists')
    require(request['activation'] in profile['allowed_activation'], 'unsupported_capability')
    require(request['deadline_seconds'] <= profile['timeout_max_seconds'], 'unsupported_capability')
    return digest(request)


def idempotency_decision(request, record):
    """Pure admission decision; durable lookup precedes mutable source/base checks."""
    shape('buildRequest', request)
    if record is None:
        return 'new'
    require((request['project_id'], request['profile_id'], request['idempotency_key']) ==
            (record['project_id'], record['profile_id'], record['idempotency_key']), 'idempotency_conflict')
    require(digest(request) == record['request_digest'], 'idempotency_conflict')
    return 'replay'


def validate_job_reference(reference, owner):
    shape('jobReference', reference)
    require(all(reference[k] == owner[k] for k in ('job_id', 'profile_id', 'project_id')), 'job_unknown')


def validate_status(status, request, profile):
    shape('jobStatus', status, 'invalid_status')
    shape('buildRequest', request)
    shape('profileContext', profile)
    require(status['project_id'] == request['project_id'] == profile['project_id'] and
            status['profile_id'] == request['profile_id'] == profile['profile_id'] and
            status['request_digest'] == digest(request) and status['target_version'] == request['new_version'], 'invalid_status')
    state = status['state']
    require(status['terminal'] == (state in MACHINE['terminal_states']), 'invalid_status')
    require(status['phase'] == 'finished' if status['terminal'] else status['phase'] != 'finished', 'invalid_status')
    if state == 'queued':
        require(status['phase'] == 'accepted' and not status['build_complete'] and status['candidate'] is None
                and all(v in ('not_run', 'not_configured') for v in status['gates'].values()), 'invalid_status')
    if state == 'recovering':
        require(status['phase'] == 'reconcile', 'invalid_status')
    candidate = status['candidate']
    if candidate is not None:
        pin = request['source_pin']
        coordinates = candidate['coordinates']
        require(candidate['version'] == request['new_version'] and coordinates['project_id'] == request['project_id']
                and coordinates['snapshot_id'] == pin['snapshot_id'] and coordinates['source_mode'] == pin['mode']
                and coordinates['base_commit'] == pin['base_commit'] and candidate['input_digest'] == request['input_digest']
                and candidate['embedding_digest'] == request['embedding_digest'], 'invalid_status')
    require({'source', 'identity', 'graph', 'vector'} <= set(profile['required_gates']), 'invalid_status')
    for name, result in status['gates'].items():
        require(result != 'not_configured' or name not in profile['required_gates'], 'invalid_status')
    if status['build_complete']:
        require(candidate is not None and all(status['gates'][name] == 'passed' for name in profile['required_gates'])
                and all(v in ('passed', 'not_configured') for v in status['gates'].values()), 'invalid_status')
    require(status['activated'] == (state == 'succeeded'), 'invalid_status')
    publication = status['publication']
    if state == 'succeeded':
        require(request['activation'] == 'activate_after_gate' and status['build_complete']
                and publication is not None and publication['base'] == request['base']
                and publication['version'] == candidate['version'] and publication['coordinates'] == candidate['coordinates'], 'invalid_status')
        body = {k: v for k, v in publication.items() if k != 'receipt_digest'}
        require(publication['receipt_digest'] == digest(body, 'cks.dataset-publication.v1'), 'invalid_status')
    else:
        require(publication is None, 'invalid_status')
    if state == 'held_for_review':
        require(request['activation'] == 'hold_for_review' and status['build_complete'], 'invalid_status')
    if state in ('failed', 'canceled', 'interrupted'):
        require(status['error'] is not None, 'invalid_status')
        if state in ('canceled', 'interrupted'):
            require(status['error']['code'] == state, 'invalid_status')
    else:
        require(status['error'] is None, 'invalid_status')


def validate_transition(before, after):
    require(before in MACHINE['states'] and after in MACHINE['states'], 'invalid_transition')
    require(before == after or after in MACHINE['transitions'][before], 'invalid_transition')


def validate_observation(before, after):
    """Poll order cannot replace a job's identity or rewind an observed revision."""
    shape('jobStatus', before, 'invalid_status')
    shape('jobStatus', after, 'invalid_status')
    for key in ('job_id', 'project_id', 'profile_id', 'request_digest', 'target_version'):
        require(before[key] == after[key], 'invalid_observation')
    require(after['revision'] >= before['revision'], 'invalid_observation')
    if after['revision'] == before['revision'] or before['terminal']:
        require(before == after, 'invalid_observation')
    validate_transition(before['state'], after['state'])


def cancel_decision(state, publication_committed=False):
    require(state in MACHINE['states'], 'invalid_status')
    if publication_committed or state == 'succeeded':
        return 'too_late'
    if state == 'recovering':
        return 'reconcile_required'
    if state in MACHINE['terminal_states']:
        return 'already_terminal'
    if state == 'cancel_requested':
        return 'already_requested'
    return 'accepted'
