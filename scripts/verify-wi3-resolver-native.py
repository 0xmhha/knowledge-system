#!/usr/bin/env python3
"""Fresh whole native checks using the existing source-bound owned runner."""
import argparse,importlib.util,json
from pathlib import Path

if __name__=='__main__':
    p=argparse.ArgumentParser();p.add_argument('--source',type=Path,required=True);p.add_argument('--out',type=Path,required=True);p.add_argument('--binding',type=Path,required=True);p.add_argument('--cache',type=Path,required=True);a=p.parse_args()
    spec=importlib.util.spec_from_file_location('native',Path(__file__).with_name('verify-wi3-reasons-native.py'));m=importlib.util.module_from_spec(spec);spec.loader.exec_module(m)
    binding=json.loads(a.binding.read_text())
    m.run(a.source.resolve(),a.out.resolve(),binding,a.cache.resolve())
    # The reused runner's historic mode described its old two-file patch.
    # Bind this run to its actual separately enumerated input/patch instead.
    path=a.out/'binary.json';binary=json.loads(path.read_text());binary['source_mode']=binding['source_mode'];path.write_text(json.dumps(binary,indent=2)+'\n')
