"""Actual WI-6 envelopes and retained identity, without independent gold claims."""
import hashlib
import json
import os

import wi6_dev_gold as gold


def restore_reader_coordination(version):
    """Restore required parent admission files for a newly owned data copy."""
    parent = version.parent
    leases = parent / '.readers'
    leases.mkdir(mode=0o700)
    for path in (parent / '.reindex.lock', leases / (version.name + '.lock')):
        descriptor = os.open(path, os.O_CREAT | os.O_EXCL | os.O_RDWR, 0o600)
        try:
            os.fsync(descriptor)
        finally:
            os.close(descriptor)


def sha(data):
    return hashlib.sha256(data).hexdigest()


def unpack(reply):
    if not isinstance(reply, dict):
        return None, 'malformed_protocol'
    if 'error' in reply:
        return None, 'protocol_error'
    result = reply.get('result')
    if not isinstance(result, dict):
        return None, 'missing_tool_result'
    if result.get('isError'):
        return None, 'tool_error'
    if isinstance(result.get('structuredContent'), dict):
        return result['structuredContent'], 'structured'
    content = result.get('content')
    if isinstance(content, list) and content and isinstance(content[0], dict):
        try:
            value = json.loads(content[0]['text'])
            if isinstance(value, dict):
                return value, 'text_json'
        except (ValueError, TypeError, KeyError):
            pass
    return None, 'malformed_tool_result'


def function_results(arm, value):
    if arm == 'baseline':
        return [value]
    if arm != 'expansion' or not isinstance(value.get('graph'), dict):
        raise ValueError('missing actual expansion graph envelope')
    functions = value['graph'].get('functions')
    if not isinstance(functions, list):
        raise ValueError('missing ordered expanded function population')
    return [r.get('result') if isinstance(r, dict) else None for r in functions]


def retained_node(connection, node_id, files):
    rows = connection.execute('SELECT n.id,n.type,n.name,n.canonical_id,n.file_path,n.start_line,n.end_line,n.start_byte,n.end_byte,b.source FROM nodes n JOIN blobs b ON b.node_id=n.id WHERE n.id=?', (node_id,)).fetchall()
    if len(rows) != 1:
        raise ValueError('retained node/blob identity absent or ambiguous')
    row = rows[0]
    blob = row[9].encode() if isinstance(row[9], str) else bytes(row[9])
    if row[1] not in ('Function', 'Method') or row[8] - row[7] != len(blob) or row[4] not in files:
        raise ValueError('retained full-function identity differs')
    return dict(node_id=row[0], kind=row[1], name=row[2], canonical_id=row[3], path=row[4],
                start_line=row[5], end_line=row[6], start_byte=row[7], end_byte_exclusive=row[8],
                function_sha256=sha(blob), file_sha256=files[row[4]], bytes=len(blob)), blob


def select_seed(connection, source, files, canonical_id=None):
    rows = connection.execute('SELECT id FROM nodes WHERE file_path=? AND start_byte=? AND end_byte=? AND type IN (?,?)',
                              (source['path'], source['start_byte'], source['end_byte_exclusive'], 'Function', 'Method')).fetchall()
    if canonical_id is not None:
        rows = [row for row in rows if connection.execute('SELECT canonical_id FROM nodes WHERE id=?', (row[0],)).fetchone()[0] == canonical_id]
    if len(rows) != 1:
        raise ValueError('exact retained seed coordinates absent or ambiguous')
    node, blob = retained_node(connection, rows[0][0], files)
    if any(node[k] != source[k] for k in ('path', 'start_line', 'end_line', 'start_byte', 'end_byte_exclusive', 'function_sha256', 'file_sha256')):
        raise ValueError('source-grounded seed differs from retained node/blob')
    return node, blob


def source_hit(result, retained, pins):
    if not isinstance(result, dict) or not isinstance(result.get('analysis'), dict):
        return {}, 'missing_analysis'
    source = result['analysis'].get('source')
    if not isinstance(source, dict):
        return {}, 'missing_source'
    node = retained.get(source.get('node_id'))
    if not node:
        return {}, 'missing_retained_node_evidence'
    expected = dict(file=node['path'], start_byte=node['start_byte'], start_line=node['start_line'],
                    bytes=node['bytes'], sha256=node['function_sha256'], commit=pins['commit'],
                    snapshot_sha256=pins['graph_snapshot_sha256'])
    if any(source.get(k) != v for k, v in expected.items()) or result.get('snapshot_sha256') != pins['graph_snapshot_sha256']:
        return {}, 'source_or_snapshot_mismatch'
    return dict(pins, path=node['path'], file_sha256=node['file_sha256'],
                function_sha256=node['function_sha256'], start_byte=node['start_byte'],
                end_byte_exclusive=node['end_byte_exclusive'], unit_kind='function'), 'verified_retained_identity'


def view(arm, reply, retained, pins, draft, locations):
    value, encoding = unpack(reply)
    empty = dict(arm=arm, encoding=encoding, raw_slots=[], complete=False,
                 actual_wire_response=True, quality_score=None, independent_gold=False)
    if value is None:
        return dict(empty, status=encoding)
    try:
        results = function_results(arm, value)
    except (ValueError, KeyError, TypeError):
        return dict(empty, status='malformed_analysis_envelope')
    surface = value if arm == 'baseline' else value['graph']
    hits, checks = [], []
    for result in results:
        hit, check = source_hit(result, retained, pins)
        hits.append(hit)
        checks.append(check)
    cap = 1 if arm == 'baseline' else 4
    # Preserve every actual slot, including a cap violation. Never refill after
    # dedup or silently truncate an invalid response to manufacture compliance.
    if len(hits) > cap:
        return dict(empty, status='returned_population_exceeds_declared_cap',
                    returned_slots=len(hits), identity_checks=checks)
    projected = gold.project_output(hits, draft, locations, pins, cap)['raw_slots']
    for slot, result, check in zip(projected, results, checks):
        slot.update(identity_check=check,
                    analysis_status=result.get('status') if isinstance(result, dict) else 'malformed',
                    analysis_partial=(not isinstance(result, dict) or result.get('status') != 'analyzed_bounded'))
    complete = bool(results) and surface.get('status') == ('analyzed_bounded' if arm == 'baseline' else 'expanded_bounded')
    complete = complete and not surface.get('stop_reasons') and all(
        isinstance(r, dict) and r.get('status') == 'analyzed_bounded'
        and isinstance(r.get('analysis'), dict) and r['analysis'].get('status') == 'analyzed_bounded'
        and r['analysis'].get('truncated') is False and c == 'verified_retained_identity'
        for r, c in zip(results, checks))
    return dict(empty, status=surface.get('status', 'missing_status'), raw_slots=projected,
                complete=bool(complete), stop_reasons=surface.get('stop_reasons', []),
                retained_scope_origin='single_owned_session_dataset_metadata_plus_SQL_node_blob_check',
                project_dataset_fields_returned_by_value_API=False,
                curated_flow='separate_not_joined' if arm == 'expansion' else None)
