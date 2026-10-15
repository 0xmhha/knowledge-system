#!/Library/Frameworks/Python.framework/Versions/3.11/bin/python3.11
import json,os,sys,time
from pathlib import Path
if len(sys.argv)>1 and sys.argv[1]=="mcp":
 with Path('/private/tmp/ks-KM043C1-worker-host-final3-20261016/native-processes.jsonl').open("a") as stream: stream.write(json.dumps({"pid":os.getpid(),"argv":sys.argv[1:],"at":time.time()})+"\n")
os.execv('/Users/kevin/work/github/0xmhha/auto-coding/knowledge/knowledge-system/bin/cks',['/Users/kevin/work/github/0xmhha/auto-coding/knowledge/knowledge-system/bin/cks']+sys.argv[1:])
