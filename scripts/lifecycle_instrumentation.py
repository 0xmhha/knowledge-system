"""Declared observation-only API edits in owned checkouts; never a production patch."""
import subprocess
API='eth/downloader/api.go'
BRIDGE='eth/downloader/ks_owned_rpc_lifecycle.go'
MAIN='cmd/ks-owned-rpc-lifecycle/main.go'
GOFMT='/Users/kevin/.gvm/gos/go1.23.12/bin/gofmt'


def instrument(data):
    text=data.decode()
    replacements=[
      ('syncSubscriptions[i] = struct{}{}','syncSubscriptions[i] = struct{}{}\nksOwnedLifecycle("install", i, syncSubscriptions)'),
      ('delete(syncSubscriptions, u.c)','ksOwnedLifecycle("before_delete", u.c, syncSubscriptions)\ndelete(syncSubscriptions, u.c)\nksOwnedLifecycle("after_delete", u.c, syncSubscriptions)'),
      ('close(u.uninstalled)','ksOwnedLifecycle("ack_close_begin", u.c, syncSubscriptions)\nclose(u.uninstalled)\nksOwnedLifecycle("ack_close_end", u.c, syncSubscriptions)'),
      ('i <- false','ksOwnedLifecycle("send_begin_install_false", i, syncSubscriptions)\ni <- false\nksOwnedLifecycle("send_end_install_false", i, syncSubscriptions)'),
      ('c <- notification','ksOwnedLifecycle("send_begin_progress", c, syncSubscriptions)\nc <- notification\nksOwnedLifecycle("send_end_progress", c, syncSubscriptions)'),
      ('c <- false','ksOwnedLifecycle("send_begin_done_false", c, syncSubscriptions)\nc <- false\nksOwnedLifecycle("send_end_done_false", c, syncSubscriptions)'),
      ('func (s *SyncStatusSubscription) Unsubscribe() {','func (s *SyncStatusSubscription) Unsubscribe() {\nksOwnedLifecycle("method_entry", s.c, nil)\ndefer ksOwnedLifecycle("method_return_defer", s.c, nil)'),
      ('case <-req.uninstalled:','case <-req.uninstalled:\nksOwnedLifecycle("method_ack_observed", s.c, nil)'),
    ]
    for old,new in replacements:
        if text.count(old)!=1:raise ValueError('nonunique lifecycle source anchor '+old)
        text=text.replace(old,new)
    count=text.count('case <-s.c:')
    if count not in (1,2):raise ValueError('unexpected method receive sites')
    text=text.replace('case <-s.c:','case <-s.c:\nksOwnedLifecycle("method_status_received_after_io", s.c, nil)')
    out=subprocess.check_output([GOFMT],input=text.encode())
    lines=out.decode().splitlines()
    anchors={k:next(i+1 for i,l in enumerate(lines)if l.strip()==v)for k,v in [('broadcast','c <- notification'),('original_uninstall','s.api.uninstallSyncSubscription <- &req')]if any(l.strip()==v for l in lines)}
    return out,anchors
