#!/Library/Frameworks/Python.framework/Versions/3.11/bin/python3.11
import os,sys
from pathlib import Path
if len(sys.argv)>1 and sys.argv[1]=="build" and Path('/private/tmp/ks-KM043B2-worker-runtime-20261015/fail-graph').exists():sys.exit(23)
os.execv('/Users/kevin/work/github/0xmhha/auto-coding/knowledge/knowledge-system/bin/ckg',['/Users/kevin/work/github/0xmhha/auto-coding/knowledge/knowledge-system/bin/ckg']+sys.argv[1:])
