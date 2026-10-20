#!/usr/bin/env python3
"""Check frozen source preparation bytes and limits; no independent gold verdict."""
import collections
import hashlib
import json
from pathlib import Path
import sys


def check(folder, root=None):
    errors=[]
    try:
        folder=folder.resolve();manifest=json.loads((folder/'manifest.json').read_text())
        if (manifest['kind']!='unapproved_source_preparation_not_gold_or_runtime'
                or manifest['approved_cases']!=0 or manifest['gold_records']!=0 or manifest['runtime_executed'] is not False):
            errors.append('source preparation promoted to gold or runtime')
        def artifact(name):
            path=(folder/name).resolve()
            if not path.is_relative_to(folder):raise ValueError('artifact path escapes packet')
            return path
        for row in manifest['artifacts']:
            if hashlib.sha256(artifact(row['path']).read_bytes()).hexdigest()!=row['sha256']:
                errors.append('artifact missing or hash differs')
        public=json.loads((folder/'public/inputs.json').read_text())
        private=json.loads((folder/'private/review-drafts.json').read_text())
        census=json.loads((folder/'private/rule-source-census.json').read_text())
        if public['kind']!='unapproved_public_source_queries_no_gold' or public['runtime_executed'] is not False:
            errors.append('public scope differs')
        if (private['approved'] is not False or private['gold_records']!=0 or private['cases_promoted']!=0
                or private['source_matching_is_graph_resolution'] is not False):
            errors.append('AI draft promoted to approval or graph result')
        if census['kind']!='source_only_call_syntax_not_rule_detection_or_gold':errors.append('call census promoted to detector result')
        files={row['path']:row for row in manifest['files']}
        if len(files)!=len(manifest['files']) or len(files)!=census['non_test_go_files']:
            errors.append('source file counts differ')
        ids=[r['id'] for r in public['cases']]
        if len(ids)!=12 or len(set(ids))!=12 or {r['id'] for r in private['cases']}!=set(ids):
            errors.append('public/private case identities differ')
        groups=collections.Counter(r['group_proposal'] for r in private['cases'])
        if groups!={'existing_policy':4,'security_proposal':4,'no_direct_overlay_reference':4}:
            errors.append('annotation proposal group coverage differs')
        allowed={'id','repository_commit','source','source_excerpt','prompt','split','scoring_allowed'}
        for row in public['cases']:
            if set(row)!=allowed or row['split'] is not None or row['scoring_allowed'] is not False:
                errors.append('public gold leakage or premature scoring/split')
            if 'private/' in json.dumps(row):errors.append('private path leaked to public input')
            source=row['source'];raw=artifact('public/'+row['source_excerpt']).read_bytes()
            if set(source)!={'path','start_line','end_line','start_byte','end_byte_exclusive','span_sha256','file_sha256','git_blob','source_name_hint','kind'}:
                errors.append('unexpected public source fields may leak expected answers')
            if (hashlib.sha256(raw).hexdigest()!=source['span_sha256']
                    or source['file_sha256']!=files[source['path']]['sha256']
                    or len(raw)!=source['end_byte_exclusive']-source['start_byte']
                    or raw.count(b'\n')!=source['end_line']-source['start_line']
                    or row['repository_commit']!=manifest['source_commit']):
                errors.append('selected source span/hash binding differs')
        for row in private['cases']:
            if (row['approved'] is not False or row['gold'] is not False or row['split'] is not None
                    or row['independent_review']!='pending' or row['semantic_family_overlap']!='not_reviewed'
                    or row['prior_literal_overlap'] is not False or row['prior_source_pool_overlap'] is not False
                    or row['safety_verdict']!='unknown'):
                errors.append('unreviewed case promoted to accepted/independent/safe')
            expected=row['expected_policy_ids_from_exact_overlay_spelling']
            if row['group_proposal']=='existing_policy' and not expected:errors.append('policy case has no source overlay target')
            if row['group_proposal']=='no_direct_overlay_reference' and expected:errors.append('relative untagged case has policy reference')
        patterns=private['security_overlay_proposal']['security_patterns']
        if len(patterns)!=4 or len({r['id'] for r in patterns})!=4:errors.append('security proposal IDs/count differ')
        indexed={r['id']:r for r in public['cases']}
        proposed=[r['security_entry_proposal'] for r in private['cases'] if r['group_proposal']=='security_proposal']
        if proposed!=patterns:errors.append('private case/security overlay proposals disagree')
        for row in private['cases']:
            if row['security_entry_proposal'] and row['security_entry_proposal']['matches']!=[indexed[row['id']]['source']['source_name_hint']]:
                errors.append('security proposal target differs from selected source')
        refs=private['policy_reference_inventory']
        if len(refs)!=manifest['policy_target_rows'] or dict(collections.Counter(r['classification'] for r in refs))!=manifest['policy_target_classifications']:
            errors.append('policy reference counts differ')
        for row in refs:
            if row['graph_resolution']!='not_run':errors.append('source-name hint promoted to graph resolution')
            for source in row['source_matches']:
                if source['file_sha256']!=files[source['path']]['sha256']:errors.append('policy source file binding differs')
        counts=collections.Counter();hints=collections.Counter()
        for row in census['calls']:
            raw=artifact('private/'+row['source_excerpt_private']).read_bytes()
            a=row['start_byte']-row['function_start_byte'];b=row['end_byte_exclusive']-row['function_start_byte']
            if (hashlib.sha256(raw).hexdigest()!=row['function_sha256'] or not 0<=a<b<=len(raw)
                    or hashlib.sha256(raw[a:b]).hexdigest()!=row['span_sha256']
                    or row['file_sha256']!=files[row['path']]['sha256']):errors.append('callsite source binding differs')
            if (row['semantic_binding']!='unresolved_including_import_shadowing_and_closures'
                    or row['gold_status']!='not_authored' or row['independent_fact_family'] is not None):
                errors.append('call spelling promoted to resolved flow or independent gold')
            counts[row['import_path_spelling']+'.'+row['callee']]+=1;hints.update(row['syntax_hints'])
        if dict(counts)!=census['call_counts'] or dict(hints)!=census['syntax_hint_counts']:errors.append('call inventory counts differ')
        raw=artifact('private/existing-policy.yaml.txt').read_bytes()
        if hashlib.sha256(raw).hexdigest()!=private['existing_overlay_reference']['sha256']:
            errors.append('original policy overlay bytes differ')
        if root is not None:
            for key in ('helper','generator'):
                row=manifest[key]
                if hashlib.sha256((root/row['path']).read_bytes()).hexdigest()!=row['sha256']:
                    errors.append('preparation source helper/generator changed')
    except (OSError,ValueError,KeyError,TypeError) as exc:
        errors.append('invalid preparation packet: '+str(exc))
    return errors


if __name__=='__main__':
    root=Path(__file__).resolve().parents[1]
    folder=root/'docs/spec-driven/operational-preparation/policy-security-drafts' if len(sys.argv)==1 else Path(sys.argv[1])
    errors=check(folder,root)
    for error in errors:print('FAIL:',error)
    print('source preparation:', 'FAIL' if errors else 'PASS; not gold, holdout, detector execution or independent acceptance')
    raise SystemExit(bool(errors))
