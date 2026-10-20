"""Observe only descendants of owned Popen children; never signal by process name."""
import os
import signal
import subprocess
import threading
import time


def snapshot(pid):
    p=subprocess.run(['ps','-p',str(pid),'-o','pid=,ppid=,pgid=,stat=,lstart=,command='],capture_output=True,text=True)
    if p.returncode:return None
    fields=p.stdout.strip().split(None,9)
    if len(fields)<10:return None
    return {'pid':int(fields[0]),'ppid':int(fields[1]),'pgid':int(fields[2]),'stat':fields[3],'birth':' '.join(fields[4:9]),'command':fields[9]}


def alive(record):
    now=snapshot(record['pid'])
    return now is not None and now['birth']==record['birth'] and now['command']==record['command']


def wait_gone(records,seconds=8):
    deadline=time.monotonic()+seconds
    while True:
        remaining=[r for r in records if alive(r)]
        if not remaining or time.monotonic()>=deadline:break
        time.sleep(.1)
    return {'all_observed_identities_gone':not remaining,'observed_count':len(records),'remaining_pids':[r['pid']for r in remaining]}


def rescue(records):
    """Exceptional cleanup only, explicitly recorded and never a normal pass."""
    sent=[]
    for number in [signal.SIGTERM,signal.SIGKILL]:
        for r in reversed(records):
            if alive(r):
                try:os.kill(r['pid'],number);sent.append({'pid':r['pid'],'signal':int(number),'identity':r})
                except ProcessLookupError:pass
        if wait_gone(records,2)['all_observed_identities_gone']:break
    return sent


class Descendants:
    def __init__(self,pid):self.pid=pid;self.records={};self.stop=threading.Event();self.thread=threading.Thread(target=self.loop,daemon=True)
    def sample(self,pid,seen):
        if pid in seen:return
        seen.add(pid)
        p=subprocess.run(['pgrep','-P',str(pid)],capture_output=True,text=True)
        for row in p.stdout.splitlines():
            child=int(row);record=snapshot(child)
            if record is not None:self.records[(child,record['birth'])]=record;self.sample(child,seen)
    def loop(self):
        while not self.stop.is_set():self.sample(self.pid,set());self.stop.wait(.1)
    def __enter__(self):self.thread.start();return self
    def __exit__(self,*args):self.stop.set();self.thread.join(3)
