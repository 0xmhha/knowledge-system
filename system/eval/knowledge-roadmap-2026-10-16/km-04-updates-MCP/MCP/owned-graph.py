#!/Library/Frameworks/Python.framework/Versions/3.11/bin/python3.11
import json,os,sys,time
from pathlib import Path
root=Path('/private/tmp/ks-KM043C2-updates-MCP-final-20261016')
if len(sys.argv)>1 and sys.argv[1]=="build" and(root/"controlled-delay").exists():
 label=(root/"controlled-delay").read_text().strip()
 (root/("graph-"+label+".json")).write_text(json.dumps({"pid":os.getpid(),"started_monotonic":time.monotonic()}))
 time.sleep(8)
 (root/("late-"+label)).write_text("late graph escaped owner")
os.execv('/Users/kevin/work/github/0xmhha/auto-coding/knowledge/knowledge-system/bin/ckg',['/Users/kevin/work/github/0xmhha/auto-coding/knowledge/knowledge-system/bin/ckg']+sys.argv[1:])
