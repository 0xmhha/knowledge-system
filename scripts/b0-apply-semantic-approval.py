#!/usr/bin/env python3
"""Apply recorded SF approvals to fresh DEVELOPMENT inputs, then build them.

The prepare phase builds mock review candidates solely to bind policy review
records to retained bytes. The build phase uses the approved BGE-M3 model.
Original sources and the former proposed datasets are never edited.
"""
import argparse
from contextlib import closing
import hashlib
import importlib.util
import json
import os
from pathlib import Path
import sqlite3
import subprocess

ROOT = Path(__file__).resolve().parent.parent
DATA = ROOT / 'system/eval/b0-knowledge-system'
SPEC = importlib.util.spec_from_file_location('proposed_sf', ROOT / 'scripts/b0-prepare-semantic-review.py')
SF = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(SF)


def write(path, value):
    path.write_text(SF.json_source(value))


def approval():
    raw = (DATA / 'human-review-m2max-2026-10-03.json').read_bytes()
    decisions = [d for d in json.loads(raw)['decisions'] if d.get('scope') == 'semantic_fixture_test_cases']
    if len(decisions) != 1:
        raise ValueError('one actual semantic fixture approval required')
    decision = decisions[0]
    if (decision['status'] != 'approved' or decision['reviewer'] != 'chat-user' or
            set(decision['ids']) != {'SF-03','SF-05-A','SF-05-B','SF-06'}):
        raise ValueError('SF approval scope differs from the reviewed four development cases')
    return decision, hashlib.sha256(raw).hexdigest()


def execute(out, label, args):
    log = out / 'checks' / (label + '.txt')
    log.parent.mkdir(exist_ok=True)
    if log.exists():
        raise ValueError('execution log exists: ' + str(log))
    env = dict(os.environ, CKV_REQUIRE_COMPLETE_EMBEDDINGS='1',
               GOMODCACHE='/Users/kevin/.gvm/pkgsets/go1.26.8/global/pkg/mod')
    with log.open('xb') as stream:
        value = subprocess.run([str(a) for a in args], cwd=ROOT, stdout=stream,
                               stderr=subprocess.STDOUT, env=env, timeout=600)
    check = {'id':label, 'command':[str(a) for a in args], 'exit_code':value.returncode,
             'log':str(log.relative_to(out)), 'sha256':SF.sha(log)}
    ledger = out / 'approval-checks.json'
    previous = json.loads(ledger.read_text()) if ledger.exists() else []
    write(ledger, previous + [check])
    if value.returncode:
        raise RuntimeError(label + ' failed: ' + str(log))
    return log.read_bytes()


def final_derivation(book):
    """Bind approved FINAL source definitions to the identical reviewed facts.

    Only the already declared partition marker and namespace substitutions
    are admitted. This is inherited test-case approval, not a fresh review.
    Changed cap, implementation, tests or vocabulary require a new decision.
    """
    fixtures = {f['id']: f for f in book['fixtures']}
    proofs = []
    for family in ['F-03', 'F-05', 'F-06']:
        before, after = fixtures[family+'-DEV'], fixtures[family+'-FINAL']
        if after['review_state'] != 'approved' or after['evaluation_partition'] != 'final' or set(before['sources']) != set(after['sources']):
            raise ValueError('FINAL source definitions lack exact input approval')
        for path, text in before['sources'].items():
            translated = text.replace('DEV-', 'FINAL-').replace('b0dev', 'b0final').replace('b0-f03-dev', 'b0-f03-final')
            if translated != after['sources'][path]:
                raise ValueError('FINAL introduces an unreviewed test fact: '+family+'/'+path)
            proofs.append({'family':family,'path':path,
                'development_sha256':hashlib.sha256(text.encode()).hexdigest(),
                'final_sha256':hashlib.sha256(translated.encode()).hexdigest(),
                'fact_equivalence':'exact bytes after declared partition marker/module/project namespace substitutions'})
    return proofs


def prepare(out, binary, partition='development'):
    decision, review_sha = approval()
    reviewed = json.loads((DATA / 'semantic-fixture-preparation-m2max-2026-10-03/real/review-manifest.json').read_bytes())
    old = {c['id']: c for c in reviewed['cases']}
    derivation = final_derivation(json.loads(SF.MAT.DEFAULT.read_bytes())) if partition == 'final' else None
    proposed = SF.prepare(out, SF.MAT.DEFAULT, binary, 'mock', 'bge-m3:latest', 'http://127.0.0.1:11434', partition)
    records = json.loads((out / 'fixtures/materialization.json').read_text())
    states = {f['id']+'-'+s['name']:s for f in records['fixtures'] for s in f['states']}
    for case in proposed['cases']:
        label = case['id']
        if partition == 'development' and (case['source_sha256'] != old[label]['source_sha256'] or case['supplement_commit'] != old[label]['supplement_commit']):
            raise ValueError('fresh candidate differs from the exact reviewed SF source: ' + label)
        state = states[label]
        repo = out / 'fixtures' / state['repository']
        case['proposed_candidate_dataset'] = case.pop('dataset')
        sf_id = {'F-03':'SF-03','F-05':'SF-05-'+state['name'].upper(),'F-06':'SF-06'}[case['family']]
        if case['family'] != 'F-03':
            execute(out, label+'-record-review', [binary,'knowledge','review','record','--project-root',repo,
                '--version-dir',out/label/'dataset/review','--kind','policy','--id','refund-cap-policy',
                '--decision','verified','--reviewer',decision['reviewer'],'--reason',
                sf_id+' approved by chat user for synthetic test cases only; not operating policy, enforcement or criterion acceptance.'])
            file = repo/'ontology.yaml'
            ontology = json.loads(file.read_text())
            for concept in ontology['concepts']:
                concept.update(status='verified',reviewed_by=decision['reviewer'])
            write(file, ontology)
            if case['family'] == 'F-06':
                file = repo/'spec.yaml'
                spec = json.loads(file.read_text())
                for requirement in spec['requirements']:
                    requirement.update(status='verified',reviewed_by=decision['reviewer'])
                write(file, spec)
            execute(out, label+'-approved-lock', [binary,'knowledge','lock','--project-root',repo])
            execute(out, label+'-approved-validate', [binary,'knowledge','validate','--project-root',repo])
        source_review = {'schema_version':1,'sf_id':sf_id,'decision':decision,
             'human_review_sha256':review_sha,'reviewed_source_commit':case['supplement_commit'],
             'reviewed_source_sha256':case['source_sha256'],'scope':'synthetic fixture only; no criterion acceptance'}
        if derivation:
            source_review.update(approval_binding='inherited unchanged test facts plus approved FINAL definitions', final_derivation=derivation)
        write(repo/'fixture-review.json', source_review)
        SF.MAT.git(repo,'add','--all')
        SF.MAT.git(repo,'commit','--quiet','-m',label+' apply actual human test-fixture approval')
        case.update(approved_commit=SF.MAT.git(repo,'rev-parse','HEAD'),
                    approved_tree=SF.MAT.git(repo,'rev-parse','HEAD^{tree}'),
                    approved_source_sha256={p:SF.sha(repo/p) for p in SF.MAT.git(repo,'ls-files').splitlines()},
                    sf_id=sf_id,review_state='approved',reviewer=decision['reviewer'],reviewed_at=decision['reviewed_at'])
        for p, expected in state['source_sha256'].items():
            if SF.sha(repo/p) != expected:
                raise ValueError('approval changed frozen base source: '+label+'/'+p)
        SF.MAT.git(repo,'bundle','create',str(out/label/'approved-source.bundle'),'--all')
    result = dict(schema_version=1,status='source_bound_approvals_prepared',partition=partition,
                  human_review_sha256=review_sha,fixture_manifest_sha256=SF.sha(SF.MAT.DEFAULT),
                  binary_sha256=SF.sha(binary),cases=proposed['cases'],quality_metrics=None,final_derivation=derivation,
                  final_queries_executed=False)
    write(out/'approved-manifest.json',result)
    return result


def build(out, binary):
    decision, review_sha = approval()
    result = json.loads((out/'approved-manifest.json').read_text())
    if result['status'] != 'source_bound_approvals_prepared' or result['human_review_sha256'] != review_sha or result['fixture_manifest_sha256'] != SF.sha(SF.MAT.DEFAULT) or result['binary_sha256'] != SF.sha(binary):
        raise ValueError('approved input/binary changed or build already published')
    records = json.loads((out/'fixtures/materialization.json').read_text())
    states = {f['id']+'-'+s['name']:s for f in records['fixtures'] for s in f['states']}
    for case in result['cases']:
        label = case['id']; repo = out/'fixtures'/states[label]['repository']; directory = out/label
        if SF.MAT.git(repo,'status','--porcelain') or SF.MAT.git(repo,'rev-parse','HEAD') != case['approved_commit']:
            raise ValueError('approved source checkout changed')
        for p, expected in case['approved_source_sha256'].items():
            if SF.sha(repo/p) != expected: raise ValueError('approved source bytes changed')
        execute(out,label+'-approved-setup',[binary,'setup','--src',repo,'--out',directory/'approved-dataset',
                '--project-id',case['project_id'],'--version','approved','--embedder','ollama','--model-name','bge-m3:latest',
                '--ollama-url','http://127.0.0.1:11434','--query-prefix-policy','registry'])
        version = directory/'approved-dataset/approved'
        identity = json.loads((version/'dataset-identity.json').read_text())
        if identity['source']['source_commit'] != case['approved_commit']: raise ValueError('source tuple mismatch')
        if identity['dataset_id'] == case['proposed_candidate_dataset']['dataset_id']: raise ValueError('approved dataset reused proposed identity')
        projection_path = directory/'projection-approved.json'
        args=[binary,'semantic','build','--repo',repo,'--project-id',case['project_id'],'--dataset-id',identity['dataset_id'],
              '--graph',version/'graph','--vector',version/'vector','--version-dir',version,
              '--store',directory/'semantic-approved.db','--out',projection_path,'--ontology','ontology.yaml','--extract-only']
        if case['family']=='F-06': args.extend(['--spec','spec.yaml','--docs','README.md'])
        if case['family']!='F-03': args.append('--include-packs')
        execute(out,label+'-approved-extract',args)
        projection=json.loads(projection_path.read_text())
        with closing(sqlite3.connect('file:'+str(version/'graph/graph.db')+'?mode=ro',uri=True)) as graph:
            symbol={'F-03':'ProjectIdentity','F-05':'Alpha','F-06':'RefundLimit'}[case['family']]
            anchors=graph.execute("SELECT canonical_id,file_path,start_line,end_line FROM nodes WHERE type='Function' AND name=?",(symbol,)).fetchall()
        if len(anchors)!=1: raise ValueError('unique native anchor required')
        canonical,file,start,end=anchors[0]
        raw=SF.MAT.git(repo,'show',case['approved_commit']+':'+file,raw=True)
        span={'id':'fixture:code:'+symbol,'snapshot':projection['snapshot'],'kind':'code','path':file,
              'start_line':start,'end_line':end,'content_sha256':hashlib.sha256(b''.join(raw.splitlines(keepends=True)[start-1:end])).hexdigest(),
              'canonical_id':canonical,'extractor':'b0-approved-sf-source-v1'}
        projection['evidence'].append(span)
        if case['family']!='F-03':
            concept=projection['concepts'][0]
            if concept['status']!='verified' or concept['reviewed_by']!=decision['reviewer']: raise ValueError('review metadata not extracted from approved source')
            projection['assertions']=[{'id':'fixture:implementation:'+symbol,'predicate':'IMPLEMENTED_BY',
                'subject_id':concept['id'],'object_id':canonical,'evidence_ids':[concept['evidence_id'],span['id']],
                'status':'verified','reviewed_by':decision['reviewer']}]
        elif any(c['status']!='proposed' or c.get('reviewed_by') for c in projection['concepts']):
            raise ValueError('SF-03 must preserve original proposed core concepts')
        for evidence in projection['evidence']:
            raw=SF.MAT.git(repo,'show',case['approved_commit']+':'+evidence['path'],raw=True)
            if hashlib.sha256(b''.join(raw.splitlines(keepends=True)[evidence['start_line']-1:evidence['end_line']])).hexdigest()!=evidence['content_sha256']:
                raise ValueError('semantic evidence bytes mismatch')
        if any(a['predicate'] in ['CHECKED_BY','TESTED_BY','ACCEPTED_BY'] for a in projection.get('assertions',[]) or []):
            raise ValueError('unrelated test or criterion acceptance edge created')
        write(projection_path,projection)
        execute(out,label+'-approved-promote',[binary,'semantic','promote','--input',projection_path,'--repo',repo,
                '--graph',version/'graph','--vector',version/'vector','--version-dir',version,
                '--store',directory/'semantic-approved.db','--activate'])
        case.update(dataset=identity,version_dir=str(version),projection_sha256=SF.sha(projection_path),
                    projection=str(projection_path.relative_to(out)),semantic_store=str(directory/'semantic-approved.db'))
        write(out/'build-progress.json',result)
    result.update(status='approved_'+result['partition']+'_datasets_built_pending_evaluation')
    write(out/'approved-built-manifest.json',result)
    return result


def main():
    parser=argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--out',type=Path,required=True)
    parser.add_argument('--binary',type=Path,required=True)
    parser.add_argument('--phase',choices=['prepare','build'],required=True)
    parser.add_argument('--partition',choices=['development','final'],default='development')
    args=parser.parse_args()
    result=prepare(args.out.resolve(),args.binary.resolve(),args.partition) if args.phase=='prepare' else build(args.out.resolve(),args.binary.resolve())
    print(json.dumps({'status':result['status'],'cases':len(result['cases']),'quality_metrics':None}))


if __name__=='__main__':
    main()
