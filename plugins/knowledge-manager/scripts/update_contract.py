"""Explicit bounded update policy and mechanical outcome/trigger contracts.

This module does not start a worker or turn observations into semantic review.
"""
import copy
import fnmatch
import math
from pathlib import Path
import re

from manager import digest, require
from collection import SECRET_NAMES, SKIP_DIRS


def defaults(registration):
    return {'schema_version':1,'project_id':registration['project_id'],'profile_id':registration['profile_id'],
            'registration_digest':digest(registration),'enabled':False,'triggers':['manual'],
            'interval_seconds':3600,'activation':'hold_for_review','deadline_seconds':3600,
            'collection':{'exclude':[],'context_file':None,'max_files':5000,'max_file_bytes':2*1024*1024,
                          'max_total_bytes':128*1024*1024,'materials':[]},
            'retry':{'max_attempts':3,'initial_seconds':30,'max_seconds':3600}}


def bounded(value, low, high):
    return type(value) is int and low <= value <= high


def validate(value, registration):
    template=defaults(registration)
    require(isinstance(value,dict) and set(value)==set(template),'update_policy_invalid')
    require(type(value['schema_version']) is int and value['schema_version']==1,'update_policy_invalid')
    require(value['project_id']==registration['project_id'] and value['profile_id']==registration['profile_id'], 'update_project_mismatch')
    require(value['registration_digest']==digest(registration),'update_registration_changed')
    require(type(value['enabled']) is bool and isinstance(value['triggers'],list) and 1<=len(value['triggers'])<=3
            and all(isinstance(t,str) and t in ('manual','periodic','change') for t in value['triggers'])
            and len(set(value['triggers']))==len(value['triggers']),'update_policy_invalid')
    require(bounded(value['interval_seconds'],1,604800) and bounded(value['deadline_seconds'],1,3600)
            and value['activation'] in ('hold_for_review','activate_after_gate'),'update_policy_invalid')
    collection=value['collection']
    require(isinstance(collection,dict) and set(collection)==set(template['collection']),'update_policy_invalid')
    patterns=collection['exclude']
    require(isinstance(patterns,list) and len(patterns)<=64 and all(isinstance(p,str) and 0<len(p)<=256
            and not p.startswith('/') and '..' not in p.split('/') and '\x00' not in p for p in patterns),'exclusion_invalid')
    require(bounded(collection['max_files'],1,10000) and bounded(collection['max_file_bytes'],1,8*1024*1024)
            and bounded(collection['max_total_bytes'],1,256*1024*1024),'collection_limit')
    context=collection['context_file']
    if context is not None:
        require(isinstance(context,str),'provided_material_invalid')
        path=Path(context)
        require(path.is_absolute() and str(path)==str(path.resolve()) and path.is_file() and not path.is_symlink()
                and path.stat().st_nlink==1 and path.stat().st_size<=collection['max_file_bytes']
                and not any(path.is_relative_to(Path(registration[key])) for key in ('dataset_root','state_root')),'provided_material_invalid')
    materials=collection['materials']
    require(isinstance(materials,list) and len(materials)<=16,'update_material_invalid')
    roots,ids=[],set()
    source=Path(registration['source_root'])
    for material in materials:
        require(isinstance(material,dict) and set(material)=={'id','role','root'}
                and isinstance(material['id'],str) and re.fullmatch('[a-z][a-z0-9_-]{0,31}',material['id'])
                and material['role'] in ('document','knowledge','ontology') and isinstance(material['root'],str),'update_material_invalid')
        path=Path(material['root'])
        require(path.is_absolute() and str(path)==str(path.resolve()) and path.is_dir() and path!=source
                and path.is_relative_to(source) and not path.is_symlink() and material['id'] not in ids
                and not any(path.is_relative_to(old) or old.is_relative_to(path) for old in roots),'update_material_invalid')
        require(not any(part in SKIP_DIRS- {'.cks'} or any(fnmatch.fnmatchcase(part.lower(),pattern) for pattern in SECRET_NAMES)
                        for part in path.relative_to(source).parts),'update_material_invalid')
        roots.append(path)
        ids.add(material['id'])
    retry=value['retry']
    require(isinstance(retry,dict) and set(retry)==set(template['retry'])
            and bounded(retry['max_attempts'],1,10) and bounded(retry['initial_seconds'],1,3600)
            and bounded(retry['max_seconds'],retry['initial_seconds'],86400),'update_retry_invalid')
    result=copy.deepcopy(value)
    result['triggers']=sorted(result['triggers'])
    result['collection']['exclude']=sorted(set(patterns))
    result['collection']['materials']=sorted(result['collection']['materials'],key=lambda row:row['id'])
    return result


def clock(value):
    require(type(value) in (int,float) and math.isfinite(value) and value>=0,'update_clock_invalid')
    return value


def due(policy, trigger, at, last_checked=None):
    """A due check is not a completed tick, no-op or successful build."""
    clock(at)
    if last_checked is not None:
        clock(last_checked)
    require(trigger in ('manual','periodic','change'),'update_trigger_invalid')
    allowed=policy['enabled'] and trigger in policy['triggers']
    next_at=None if last_checked is None else last_checked+policy['interval_seconds']
    skew=last_checked is not None and at<last_checked
    return {'due':bool(allowed and (trigger=='manual' or next_at is None or at>=next_at)),
            'clock_skew':skew,'next_at':next_at,'trigger':trigger,'enabled':policy['enabled']}


def retry_delay(policy, failed_attempt):
    require(bounded(failed_attempt,1,policy['retry']['max_attempts']),'update_attempt_invalid')
    if failed_attempt==policy['retry']['max_attempts']:
        return None
    return min(policy['retry']['max_seconds'],policy['retry']['initial_seconds']*2**(failed_attempt-1))


def completed(job, policy, required_gates, current_coordinates=None):
    """Mechanical terminal result; caller must observe current/read separately."""
    require(job.get('project_id')==policy['project_id'] and job.get('profile_id')==policy['profile_id'],'update_project_mismatch')
    require(job.get('terminal') is True and job.get('build_complete') is True and job.get('error') is None,'update_not_successful')
    candidate=job.get('candidate')
    require(isinstance(candidate,dict) and isinstance(candidate.get('coordinates'),dict)
            and candidate['coordinates'].get('project_id')==policy['project_id']
            and candidate.get('version')==job.get('target_version') and isinstance(candidate.get('version'),str)
            and re.fullmatch('[A-Za-z0-9][A-Za-z0-9._-]{0,127}',candidate['version']) and candidate['version'].lower()!='current'
            and all(isinstance(candidate['coordinates'].get(k),str) and re.fullmatch('[0-9a-f]{64}',candidate['coordinates'][k]) for k in ('dataset_id','snapshot_id')),'update_candidate_invalid')
    require(isinstance(required_gates,list) and required_gates and all(isinstance(g,str) for g in required_gates)
            and isinstance(job.get('gates'),dict) and all(job['gates'].get(g)=='passed' for g in required_gates),'update_gate_not_passed')
    if policy['activation']=='hold_for_review':
        require(job.get('state')=='held_for_review' and job.get('activated') is False,'update_not_successful')
        return 'held'
    require(job.get('state')=='succeeded' and job.get('activated') is True
            and current_coordinates==candidate['coordinates'],'update_publication_not_observed')
    return 'succeeded'
