"""Exact retained reads need no manager writer lease or mutable current fallback."""
import math
from pathlib import Path
import uuid

from manager import file_hash,load_registration,require,strict_read,sync_dir,write_new
from collection import domain_hash,owned_directory,writer
from update_inputs import load_input
from update_state import validate_job
from update_native import value
from broker import Native,READ


def read_observed(r,row,job,reader_directory,prompt=None,timeout_seconds=120,native_factory=Native):
    require(type(timeout_seconds) in (int,float) and math.isfinite(timeout_seconds) and 0<timeout_seconds<=120,'update_read_budget_invalid')
    require(load_registration(str(Path(r['state_root'])/'registration.json'))==r,'registration_changed')
    validate_job(row,job,r);profile=row['profile'];candidate=job.get('candidate')
    require(job.get('terminal') is True and job.get('build_complete') is True and job.get('error') is None
            and job.get('state') in ('held_for_review','succeeded') and isinstance(candidate,dict)
            and candidate.get('input_digest')==profile['input_digest'] and candidate.get('embedding_digest')==profile['embedding_digest']
            and isinstance(candidate.get('coordinates'),dict) and candidate['coordinates'].get('project_id')==r['project_id']
            and candidate['coordinates'].get('snapshot_id')==profile['source_pin']['snapshot_id']
            and all(job.get('gates',{}).get(g)=='passed'for g in profile['required_gates']),'update_candidate_invalid')
    version=Path(r['dataset_root'])/candidate['version']
    require(candidate['version']==row['request']['new_version'] and version.is_dir() and version.resolve()==version and not version.is_symlink(),'update_candidate_invalid')
    directory,_=load_input(r,row['input_id']);manifest=strict_read(version/'vector/manifest.json',16*1024*1024)
    setup=strict_read(Path(r['state_root'])/'setup.json')
    config={'version':1,'name':'cks','listen':{'transport':'stdio','mcp_stdio':True},'logging':{'level':'error','mode':'prod'},
            'sanitize':{'rules_path':str(Path(r['state_root'])/'sanitization_rules.yaml'),'default_action':'drop','fail_closed_on_unknown_rule':True},
            'backends':{'ckg':{'path':str(version/'graph/graph.db'),'source_root':str(directory/'project'),'timeout_ms':5000},
                        'ckv':{'path':str(version/'vector'),'provider':setup['embedder'],'embed_model':manifest['embedding_model'],
                               'embed_dim':setup.get('embed_dim',0),'query_prefix_policy':setup.get('query_prefix_policy','registry'),
                               'ollama_url':setup.get('ollama_url','http://localhost:11434')}}}
    parent=Path(reader_directory)
    require(parent.is_absolute() and parent.is_relative_to(Path(r['state_root'])/'updates') and parent.resolve()==parent,'update_reader_config_changed')
    parent=owned_directory(parent,r)
    key=domain_hash('cks.manager-update-reader.v1',config);path=parent/(key+'.json')
    with writer(parent):
        if path.exists():require(strict_read(path,1024*1024)==config,'update_reader_config_changed')
        else:
            require(len(list(parent.glob('*.json')))<129,'update_reader_limit')
            stage=parent/('.pending-'+uuid.uuid4().hex);write_new(stage,config);stage.rename(path);sync_dir(parent)
    current=Path(r['dataset_root'])/'current'
    def pointer():
        if not current.is_symlink():return None
        target=current.readlink()
        require(len(target.parts)==1 and target.name not in ('.','..','current'),'update_current_invalid')
        return target.name
    before=pointer()
    reader=native_factory(r,path,request_timeout=timeout_seconds)
    try:
        pack=value(reader.tool(READ,{'prompt':prompt or 'Observed source and knowledge evidence'}))
        require(pack.get('coordinates')==candidate['coordinates'] and isinstance(pack.get('bodies'),list)
                and isinstance(pack.get('citations'),list) and all(isinstance(c,dict) and c.get('project_id')==r['project_id']
                    and c.get('dataset_id')==candidate['coordinates']['dataset_id']for c in pack['citations']),'update_read_not_verified')
    finally:reader.close()
    after=pointer()
    return pack,pack['coordinates'] if before==after==candidate['version'] else None
