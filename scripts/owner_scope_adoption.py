"""Apply a direct user scope decision; preserve original AI/history receipts."""
import json
import subprocess
from pathlib import Path
import owner_schedule_contract as proposal
import viewer_readiness as v
ROOT=v.ROOT
PACKET=proposal.PACKET

def derive(root=ROOT):
    p=root/PACKET;decision=v.read(p/'SCOPE-DECISION.json');original=v.read(p/'PROPOSAL.json')
    v.require(not proposal.check(original),'original proposal/source no longer consistent')
    v.require(decision['decision']=='adopt_proposed_scope_split' and decision['approval_source']=='direct_user_reply_2026-10-19' and decision['proposal_sha256']==v.sha((p/'PROPOSAL.json').read_bytes()) and decision['adopted_scope']==original['proposal'],'scope decision changed')
    v.require(decision['independent_gold_approval'] is False and decision['formal_completed']==0 and decision['freeze']is None and decision['execution_allowed']is False,'scope approval promoted to independent acceptance')
    base=p.parent;owner=v.read(base/'full-host-owner-v1/SUMMARY.json');method=v.read(base/'ticket-method-schedules-v1/SUMMARY.json')
    expected=original['proposal']['actual_owner_schedules'];rows=owner['slots']
    v.require(len(rows)==10 and all([r['scenario']for r in x['observations']]==expected for x in rows),'actual owner schedules missing')
    v.require(owner['actual_factory_owner_schedules']==40 and method['valid_technical_observations']==10 and method['valid_bundle_controlled_late_sends']==24,'historical schedule denominator changed')
    v.require(owner['full_original5_complete']is False and owner['full12_requirement_acceptance']is False and method['full12_requirement_satisfaction']is False,'historical limited results were rewritten')
    refs={}
    for name in ('full-host-owner-v1','ticket-method-schedules-v1'):
        q=base/name;m=v.read(q/'manifest.json')
        revision={'full-host-owner-v1':'49197f0f3d5967913355278ba5d45d4a32ae614a','ticket-method-schedules-v1':'4b7b9decb76e234de8d2bae12575355f5f8415c1'}[name]
        for filename in ('manifest.json','SUMMARY.json'):
            rel=str((q/filename).relative_to(root));v.require(subprocess.check_output(['git','show',revision+':'+rel],cwd=root)==(q/filename).read_bytes(),'historical Git bytes changed')
        # Verify original manifest population, excluding later manifest-independent notes.
        for n,h in m['artifacts'].items():v.require(v.sha((q/n).read_bytes())==h,'historical evidence changed: '+name+'/'+n)
        refs[name]=dict(revision=revision,manifest_sha256=v.sha((q/'manifest.json').read_bytes()),summary_sha256=v.sha((q/'SUMMARY.json').read_bytes()))
    return dict(kind='user_adopted_owner_schedule_scope_DEV_mapping',decision_sha256=v.sha((p/'SCOPE-DECISION.json').read_bytes()),historical_inputs=refs,actual_owner_scenarios=expected,actual_owner_observations=40,separate_controlled_late_scope='late_delivery_before_confirmation',controlled_late_sends_across_original_bundles=24,post_ack_scope=original['proposal']['post_ack_scope'],user_scope_decision_applied=True,original_five_all_real_owner_claim=False,full12_acceptance=False,installed_consumer_positive_oracle=False,independent_families=0,independent_approved=0,formal_completed=0,freeze=None,execution_allowed=False)

def check(root=ROOT):
    try:
        p=root/PACKET;m=v.read(p/'ADOPTION-MANIFEST.json')
        v.require(m['tools']=={n:v.sha((root/n).read_bytes())for n in ('scripts/owner_scope_adoption.py','scripts/test_owner_scope_adoption.py','scripts/owner_schedule_contract.py')},'scope verifier tools changed')
        v.require(all(v.sha((p/n).read_bytes())==h for n,h in m['artifacts'].items()),'scope adoption evidence changed')
        v.require(v.read(p/'ADOPTED-SCOPE-MAPPING.json')==derive(root),'adopted scope mapping differs');return []
    except(ValueError,OSError,KeyError)as e:return[str(e)]
if __name__=='__main__':
    errors=check()
    if errors:raise SystemExit('\n'.join(errors))
    print(json.dumps(derive(),ensure_ascii=False))
