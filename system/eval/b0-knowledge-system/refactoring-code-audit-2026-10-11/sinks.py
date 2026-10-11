"""Classify every production CallSite by the call expression text sliced from
the snapshot source with the node's AST byte range, and map it to its
enclosing function via contains edges. Writes sinks.json."""
import json, re, sqlite3, collections, sys
A = sys.argv[1]
db = sqlite3.connect(f"file:{A}/graph/graph.db?mode=ro&immutable=1", uri=True)
SINKS = {
 "exec":       r"^exec\.(Command|CommandContext)\b",
 "remove":     r"^os\.(RemoveAll|Remove)\b",
 "write":      r"^os\.(WriteFile|Create|OpenFile|Rename|Symlink|Link|Chmod|MkdirAll|Mkdir)\b",
 "sqlopen":    r"^sql\.Open\b",
 "sqlexec":    r"\.(Exec|ExecContext|Query|QueryContext|QueryRow|QueryRowContext)\(\s*(fmt\.Sprintf|\"[^\"]*\"\s*\+|[a-zA-Z_]+\s*\+)",
 "httpget":    r"^http\.(Get|Post|Head)\b|\.Do\(",
 "readall":    r"^(io|ioutil)\.ReadAll\(",
 "jsondecode": r"^json\.NewDecoder\(",
 "unmarshal":  r"^(yaml|json)\.Unmarshal\(",
 "pathjoin":   r"^filepath\.(Join|Abs|Clean|Rel|EvalSymlinks)\b",
 "tempdir":    r"^os\.(MkdirTemp|CreateTemp)\b",
 "lock":       r"\.(Lock|RLock|Unlock|RUnlock)\(\)|^unix\.Flock|^syscall\.Flock",
 "goroutine":  r"^go ",
 "signal":     r"^signal\.(Notify|Ignore)\b",
 "kill":       r"\.Kill\(|^unix\.Kill|^syscall\.Kill|\.Signal\(",
 "tls":        r"InsecureSkipVerify",
 "listen":     r"^(net\.Listen|http\.ListenAndServe|\w+\.ListenAndServe)\b",
 "panic":      r"^panic\(",
 "unsafe":     r"^unsafe\.",
}
src_cache = {}
def text(path, a, b):
    if path not in src_cache: src_cache[path] = open(f"{A}/src/{path}", "rb").read()
    return src_cache[path][a:b].decode("utf-8", "replace")
parent = {}
for cs, fn, ft, fq, fp, fs in db.execute("""select e.dst, s.id, s.type, s.qualified_name, s.file_path, s.start_line from edges e join nodes s on s.id=e.src
                                             where e.type='contains' and s.type in ('Function','Method')"""):
    parent[cs] = (fn, ft, fq, fp, fs)
out = collections.defaultdict(list); total = 0
for nid, fp, line, a, b in db.execute("select id,file_path,start_line,start_byte,end_byte from nodes where type='CallSite' and file_path not like '%_test.go'"):
    total += 1
    t = text(fp, a, b).strip()
    for k, rx in SINKS.items():
        if re.search(rx, t):
            p = parent.get(nid)
            out[k].append({"file": fp, "line": line, "call": t[:240], "fn_id": p[0] if p else None, "fn": p[2] if p else None})
json.dump(out, open(f"{A}/sinks.json", "w"), indent=1)
print("production callsites", total)
for k, v in out.items(): print(f"{k:11s} {len(v)}")
