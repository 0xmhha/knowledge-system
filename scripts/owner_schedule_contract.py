"""Check a pinned source-scope proposal; never manufacture a scope approval."""
import gzip,hashlib,json,re
from pathlib import Path
ROOT=Path(__file__).resolve().parents[1]
PACKET=Path('docs/spec-driven/operational-preparation/source-review-supplement/followups/owner-schedule-contract-v1')

def facts(source):
 s=source.decode();match=re.search(r'case u := <-api\.uninstallSyncSubscription:(.*?)case event := <-sub\.Chan\(\):',s,re.S)
 if not match or re.sub(r'\s','',match.group(1))!='delete(syncSubscriptions,u.c)close(u.uninstalled)':raise ValueError('single-owner delete/ack branch differs')
 start=s.index('func (api *DownloaderAPI) SubscribeSyncStatus(status chan interface{}) *SyncStatusSubscription {')
 body=re.sub(r'\s','',s[start:])
 expected='func(api*DownloaderAPI)SubscribeSyncStatus(statuschaninterface{})*SyncStatusSubscription{api.installSyncSubscription<-statusreturn&SyncStatusSubscription{api:api,c:status}}'
 if body!=expected:raise ValueError('new registration/reuse source contract differs')
 return dict(no_same_owner_status_send_between_accept_and_ack=True,old_subscription_boundary_must_exclude_new_registration=True)

def check(data=None,source=None):
 p=ROOT/PACKET;data=data if data is not None else json.loads((p/'PROPOSAL.json').read_text());source=source if source is not None else gzip.decompress((p/'original-api.go.gz').read_bytes())
 errors=[]
 try:facts(source)
 except(ValueError,UnicodeDecodeError)as error:errors.append(str(error))
 if hashlib.sha256(source).hexdigest()!=data['source_sha256']:errors.append('pinned source changed')
 if data['human_scope_decision']!='pending'or data['independent_approved']!=0 or data['full_original5_complete']is not False or data['formal_completed']!=0 or data['freeze']is not None or data['execution_allowed']is not False:errors.append('AI source proposal was promoted to approval or acceptance')
 if data['proposal']['actual_owner_schedules']!=['normal_delivery_then_unsubscribe','blocked_delivery_then_consumer_exit','confirmation_delayed','concurrent_unsubscribe_once']or data['proposal']['separate_controlled_owner_schedule']!='late_delivery_before_confirmation':errors.append('unreviewed schedule scope changed')
 return errors

if __name__=='__main__':
 errors=check()
 if errors:raise SystemExit('\n'.join(errors))
 print('source-scope proposal: consistent; human decision pending, independent/formal0')
