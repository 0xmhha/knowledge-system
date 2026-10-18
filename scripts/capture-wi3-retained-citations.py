#!/usr/bin/env python3
"""Copy source bytes for actual retained citations; optional original Git check."""
import argparse
import base64
import gzip
import hashlib
import json
from pathlib import Path
import subprocess


def main():
    parser=argparse.ArgumentParser();parser.add_argument('--runtime',type=Path,required=True);parser.add_argument('--prepared',type=Path,required=True);parser.add_argument('--original-repo',type=Path,required=True);a=parser.parse_args()
    out=a.runtime;dataset=a.prepared/'stablenet/retained-origins-v2';manifest=json.loads(gzip.decompress((out/'bindings/sources/manifest.json.gz').read_bytes()))
    files={(r['origin_id'],r['path']):r for r in manifest['files']};refs=[]
    def scan(x):
        if isinstance(x,dict):
            if {'project_id','dataset_id','origin_id','file','file_sha256','content_sha256','start_line','end_line'}<=set(x):refs.append(x)
            for value in x.values():scan(value)
        elif isinstance(x,list):
            for value in x:scan(value)
    scan(json.loads((out/'primary-results.json').read_text()));captured=[]
    for origin,path in sorted({(r['origin_id'],r['file']) for r in refs}):
        record=files[origin,path];raw=(dataset/'sources/blobs'/record['sha256']).read_bytes()
        assert hashlib.sha256(raw).hexdigest()==record['sha256'] and origin=='repo'
        original=subprocess.check_output(['git','show',manifest['identity']['source_commit']+':'+path],cwd=a.original_repo)
        assert original==raw
        captured.append(dict(origin_id=origin,path=path,sha256=record['sha256'],base64=base64.b64encode(raw).decode()))
    (out/'captured-source-files.json.gz').write_bytes(gzip.compress(json.dumps(captured,sort_keys=True,separators=(',',':')).encode(),mtime=0))
    (out/'source-capture-receipt.json').write_text(json.dumps(dict(captured_files=len(captured),retained_anchor_instances=len(refs),original_Git_byte_equal=True,source_commit=manifest['identity']['source_commit'],scope='all candidate and nested signal retained citations, not a new gold universe'),indent=2)+'\n')


if __name__=='__main__':main()
