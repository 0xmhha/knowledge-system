#!/Library/Frameworks/Python.framework/Versions/3.11/bin/python3.11
import os,sys,time
from pathlib import Path
root=Path('/private/tmp/ks-KM043A-source-check-final2-20261015')
if len(sys.argv)>1 and sys.argv[1]=="build" and (root/"delay-graph").exists():
 (root/"graph-running").write_text(str(os.getpid()))
 while not (root/"allow-graph").exists(): time.sleep(.01)
os.execv('/Users/kevin/work/github/0xmhha/auto-coding/knowledge/knowledge-system/bin/ckg',['/Users/kevin/work/github/0xmhha/auto-coding/knowledge/knowledge-system/bin/ckg']+sys.argv[1:])
