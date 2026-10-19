"""Separate historical AI-draft validity from current-source acceptance."""
import gzip,json,subprocess
from pathlib import Path
import viewer_readiness as v
ROOT=v.ROOT
PACKET=v.PACKET.parent/'draft-history-reconciliation-v1'
CHECKERS=['check-'+n+'-dev-gold.py'for n in ('wi1','wi2','wi3','wi4','ticket','wi6','wic')]

def derive(root=ROOT,p=None):
    p=p or root/PACKET;plan=v.read(p/'historical-plan.json.gz');history=v.read(p/'historical-receipts.json.gz');live=v.read(p/'receipts.json.gz')
    v.require([r['checker']for r in plan]==[r['checker']for r in history]==[r['checker']for r in live]==CHECKERS,'draft checker population differs')
    for planned,actual,current in zip(plan,history,live):
        v.require(all(actual[k]==planned[k]for k in planned)and actual['exit_code']==0 and actual['owned_clone_unchanged'],'historical replay failed or checkout changed')
        v.require(current['exit_code']==1,'live outdated-checker failure hidden')
        stdout=gzip.decompress((p/(actual['id']+'-historical.stdout.gz')).read_bytes());stderr=gzip.decompress((p/(actual['id']+'-historical.stderr.gz')).read_bytes())
        v.require(v.sha(stdout)==actual['stdout_sha256']and v.sha(stderr)==actual['stderr_sha256']and json.loads(stdout)['errors']==[],'historical raw outcome differs')
        b=subprocess.check_output(['git','show',actual['revision']+':'+actual['manifest']],cwd=root)
        v.require(v.sha(b)==actual['manifest_sha256']==v.sha((root/actual['manifest']).read_bytes()),'historical manifest differs from committed/live draft')
        v.require(json.loads(gzip.decompress((p/(actual['checker']+'.stdout.gz')).read_bytes()))['errors'],'live stale diagnostic omitted')
        for phase in ('before','after'):v.require(gzip.decompress((p/(actual['id']+'-'+phase+'.stdout.gz')).read_bytes())==b'','owned historical checkout changed')
    return dict(kind='AI_DEV_draft_historical_replay_not_current_source_or_gold_approval',historical_checkers_passed=7,work_item_scopes_covered=8,live_old_checkers_rejected=7,live_diagnostic_scope='initial phase80 working tree at base49662; not a latest-source pass',live_rejection_reason='source/tool versions and current reference differ; preserved, never weakened',WI_SP='mixed security checker not executed under user deferral; original evidence retained',new_DEV_samples=0,new_FINAL_samples=0,AI_draft_contents_newly_approved=False,independent_approved=0,current_source_acceptance=False,formal_completed=0,formal_total=9,freeze=None,execution_allowed=False,actual_LLM_calls=0)
def check(root=ROOT):
    p=root/PACKET
    try:
        m=v.read(p/'manifest.json');v.require(m['artifacts']==v.inventory(p),'draft reconciliation artifacts changed');v.require(m['tools']=={n:v.sha((root/n).read_bytes())for n in ('scripts/draft_history_reconciliation.py','scripts/test_draft_history_reconciliation.py')},'draft replay verifier changed');v.require(v.read(p/'SUMMARY.json')==derive(root,p),'draft replay result promoted/changed')
        ref=dict(path=str(PACKET/'manifest.json'),sha256=v.sha((p/'manifest.json').read_bytes()),formal_completed=0,independent_approved=0)
        v.require(all(v.read(root/n).get('latest_draft_history_reconciliation')==ref for n in v.PATHS),'draft history current references differ');return []
    except(ValueError,OSError,KeyError,TypeError)as e:return[str(e)]
if __name__=='__main__':
    errors=check()
    if errors:raise SystemExit('\n'.join(errors))
    print(json.dumps(derive(),ensure_ascii=False))
