"""Transparent owned stdio observer with signal handlers that never take locks."""
import hashlib
import json
import os
import signal
import subprocess
import sys
import threading
import time


def run(args):
    lock=threading.Lock();input_lock=threading.Lock();pending=[];done=threading.Event();started=time.monotonic()
    def record(kind,**data):
        with lock,args.log.open('a')as f:f.write(json.dumps({'kind':kind,'time':time.time(),**data})+'\n')
    command=[str(args.binary),'mcp','--config',str(args.config)];child=subprocess.Popen(command,stdin=subprocess.PIPE,stdout=subprocess.PIPE,stderr=subprocess.PIPE)
    record('start',wrapper_pid=os.getpid(),child_pid=child.pid,command=command,binary_sha256=hashlib.sha256(args.binary.read_bytes()).hexdigest())
    def eof():
        with input_lock:
            if not child.stdin.closed:
                try:child.stdin.close()
                except OSError:pass
                record('stdin_eof')
    def on_signal(number,_frame):
        # Repeated SIGINT/SIGTERM can interrupt any main-thread operation.
        # Appending only, with no logging, buffered I/O or lock in the handler.
        pending.append(number)
    for number in [signal.SIGINT,signal.SIGTERM]:signal.signal(number,on_signal)
    def signal_worker():
        index=0
        while not done.is_set()or index<len(pending):
            while index<len(pending):
                number=pending[index];index+=1;record('signal',signal=number);eof()
            done.wait(.005)
    def read_input():
        count=0
        try:
            for raw in sys.stdin.buffer:
                row=json.loads(raw)
                if row.get('method')=='tools/call':
                    count+=1
                    if count>64:record('budget_refused',tool_requests=count);break
                record('request',raw=raw.decode())
                with input_lock:
                    if child.stdin.closed:break
                    child.stdin.write(raw);child.stdin.flush()
        except (OSError,ValueError)as e:record('input_end',reason=type(e).__name__)
        finally:eof()
    def read_output():
        try:
            for raw in child.stdout:record('response',raw=raw.decode());sys.stdout.buffer.write(raw);sys.stdout.buffer.flush()
        except (OSError,BrokenPipeError):record('output_closed');eof()
    def read_error():
        for raw in child.stderr:record('stderr',raw=raw.decode())
    workers=[threading.Thread(target=f,daemon=True)for f in [read_input,read_output,read_error]];signals=threading.Thread(target=signal_worker,daemon=True)
    signals.start()
    for worker in workers:worker.start()
    terminated=False;forced=False
    try:child.wait(timeout=90)
    except subprocess.TimeoutExpired:
        eof()
        try:child.wait(timeout=3)
        except subprocess.TimeoutExpired:
            terminated=True;child.terminate()
            try:child.wait(timeout=3)
            except subprocess.TimeoutExpired:forced=True;child.kill();child.wait(timeout=3)
    for worker in workers[1:]:worker.join(2)
    done.set();signals.join(2)
    record('exit',child_pid=child.pid,exit_code=child.returncode,terminate_sent=terminated,forced_kill=forced,child_reaped=child.poll()is not None,elapsed_seconds=time.monotonic()-started,signal_worker_joined=not signals.is_alive(),output_workers_joined=all(not w.is_alive()for w in workers[1:]),signal_handler_contract='append_only_no_lock_or_IO')
    return child.returncode
