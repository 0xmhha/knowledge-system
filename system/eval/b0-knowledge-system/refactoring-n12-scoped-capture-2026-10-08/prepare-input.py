#!/usr/bin/env python3
"""Prepare a private, derived Git source input; no evaluation or publication."""
import hashlib, json, os, subprocess, tempfile
from pathlib import Path

ROOT = Path.cwd()
OUT = ROOT / "system/eval/b0-knowledge-system/refactoring-n12-scoped-capture-2026-10-08"
REVISION = "667a9af5f8608e6e984f58f3c3b61350dde0ea48"
DOCS = {"docs/spec-driven/ontology-pilot.yaml", "docs/spec-driven/spec-pilot.yaml",
        "docs/spec-driven/PUBLIC-CONTRACT-V2.md", "docs/spec-driven/WORKSHEET-CATALOG-CONTRACT-V1.md",
        "docs/spec-driven/SOURCE-SNAPSHOT-ADR.md"}

def run(*args, cwd=ROOT, **kw):
    return subprocess.check_output(args, cwd=cwd, **kw)

def sha(b): return hashlib.sha256(b).hexdigest()
def save(name, obj): (OUT/name).write_text(json.dumps(obj, indent=2, ensure_ascii=False)+"\n")
def sensitive(path):
    parts=path.lower().split("/"); base=parts[-1]
    return (any(x in {".aws","secrets"} for x in parts[:-1]) or base == ".env" or
            base.startswith((".env.","id_rsa","id_ed25519")) or
            base in {"credentials.json",".npmrc",".netrc"} or
            Path(base).suffix in {".pem",".key",".p12",".pfx",".keystore"})

assert run("git", "rev-parse", "HEAD", text=True).strip() == REVISION
rows=[]; exclusions={}; excluded_paths=[]
for record in run("git", "ls-tree", "-rlz", REVISION).split(b"\0"):
    if not record: continue
    meta,path=record.split(b"\t",1); path=path.decode("utf-8")
    mode,kind,oid,size=meta.decode().split()
    selected = path.startswith(("cmd/","internal/","pkg/","third_party/")) or path in DOCS|{"go.mod","go.sum"}
    if not selected:
        prefix=path.split("/")[0]; exclusions[prefix]=exclusions.get(prefix,0)+1
        excluded_paths.append({"path":path,"git_object_id":oid,"reason":"outside explicit proposed source input scope"})
        continue
    assert mode in {"100644","100755"} and kind == "blob", path
    assert ".." not in Path(path).parts and not path.startswith("/") and "\\" not in path, path
    assert not sensitive(path), path
    rows.append({"path":path,"git_mode":mode,"git_object_id":oid,"size":int(size)})
rows.sort(key=lambda x:x["path"])
assert len(rows)==1410
base=Path(tempfile.mkdtemp(prefix="ks-n12-scoped-capture-",dir="/private/tmp"))
source=base/"source-input"; source.mkdir(mode=0o700)
# Fetch only admitted Git blobs. No original eval artifacts or live untracked files.
proc=subprocess.Popen(["git","cat-file","--batch"],cwd=ROOT,stdin=subprocess.PIPE,stdout=subprocess.PIPE)
for row in rows:
    proc.stdin.write((row["git_object_id"]+"\n").encode()); proc.stdin.flush()
    oid,kind,size=proc.stdout.readline().decode().split()
    assert oid==row["git_object_id"] and kind=="blob" and int(size)==row["size"]
    data=proc.stdout.read(int(size)); assert proc.stdout.read(1)==b"\n"
    assert len(data)==row["size"]
    path=source/row["path"]; path.parent.mkdir(parents=True,exist_ok=True)
    path.write_bytes(data); path.chmod(0o700 if row["git_mode"]=="100755" else 0o600)
    row["sha256"]=sha(data)
proc.stdin.close(); assert proc.wait()==0
run("git","init","-q","--initial-branch=n12-proposed-source-input",cwd=source)
run("git","add","--all",cwd=source)
env=dict(os.environ, GIT_AUTHOR_NAME="N12 preparation",GIT_COMMITTER_NAME="N12 preparation",
         GIT_AUTHOR_EMAIL="n12-preparation@localhost",GIT_COMMITTER_EMAIL="n12-preparation@localhost",
         GIT_AUTHOR_DATE="2026-10-08T00:00:00+00:00",GIT_COMMITTER_DATE="2026-10-08T00:00:00+00:00")
run("git","-c","commit.gpgsign=false","-c","core.hooksPath=/dev/null","commit","-q","-m",
    "Prepare proposed N12 source input from "+REVISION,cwd=source,env=env)
commit=run("git","rev-parse","HEAD",cwd=source,text=True).strip()
assert not run("git","status","--porcelain",cwd=source)
assert len(run("git","rev-list","--all",cwd=source,text=True).splitlines())==1
save("source-input-inventory.json",rows)
save("excluded-original-paths.json",excluded_paths)
bindings=[]
audit=json.loads((ROOT/"system/eval/b0-knowledge-system/refactoring-n12-canonical-preparation-2026-10-08/source-audit.json").read_text())
for entry in audit:
    ref=entry["ref"];data=(source/ref["path"]).read_bytes()
    assert sha(data)==ref["file_sha256"]
    span=b"".join(data.splitlines(keepends=True)[ref["first"]-1:ref["last"]])
    assert sha(span)==ref["content_sha256"]
    bindings.append({"review_id":entry["review_id"],"role":entry["role"],"ref":ref,
                     "original_revision":REVISION,"derived_source_commit":commit,"file_and_span_unchanged":True})
assert len(bindings)==69
save("review-source-bindings.json",bindings)
approved=json.loads((ROOT/"system/eval/b0-knowledge-system/approved-inputs-m2max-2026-10-06/build-sources.json").read_text())["paths"]
assert len(approved)==29 and all((source/path).is_file()for path in approved)
save("preparation.json",{"original_revision":REVISION,"derived_source_commit":commit,"owned_working_directory":str(base),
      "source_root":str(source),"files":len(rows),"bytes":sum(x["size"]for x in rows),"go_files":sum(x["path"].endswith(".go")for x in rows),
      "included_roots":["cmd/","internal/","pkg/","third_party/"],"included_documents":sorted(DOCS),
      "module_inputs":["go.mod","go.sum"],"excluded_original_paths":len(excluded_paths),"excluded_by_root":exclusions,
      "input_scope":"proposed technical source scope, not human-reviewed operating corpus",
      "original_git_history_included":False,"derived_git_history_commits":1,"original_review_refs_unchanged":len(bindings),
      "approved_build_go_sources":approved,"human_scope_approved":False,"new_model_calls":0,"published":False})
print(json.dumps({"base":str(base),"files":len(rows),"derived_commit":commit,"review_refs":len(bindings)}))
