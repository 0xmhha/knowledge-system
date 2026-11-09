"""List indexer-go fix commits that change non-test Go functions, with the changed function names (from diff hunk
headers and +/- func lines) and whether each name belongs to a same-name group of >=3 declarations at the parent."""
import collections, json, re, subprocess, sys
R = sys.argv[1]
def git(*a): return subprocess.run(['git', '-C', R, *a], capture_output=True, text=True).stdout
FUNC = re.compile(r'^func (?:\([^)]*\) )?([A-Za-z_]\w*)', re.M)
out = []
for line in git('log', '--format=%H\t%s', '-i', '-E', '--grep=^fix').splitlines():
    h, subj = line.split('\t', 1)
    files = [f for f in git('show', '--name-only', '--format=', h).split() if f.endswith('.go') and not f.endswith('_test.go')]
    if not files: continue
    diff = git('show', '-U0', '--format=', h, '--', *files)
    names = set()
    for l in diff.splitlines():
        if l.startswith('@@'):
            m = re.search(r'@@.*@@ func (?:\([^)]*\) )?([A-Za-z_]\w*)', l)
            if m: names.add(m.group(1))
        elif l[:1] in '+-' and not l.startswith(('+++', '---')):
            m = FUNC.match(l[1:])
            if m: names.add(m.group(1))
    parent = h + '^'
    src = git('grep', '-h', '-E', r'^func ', parent, '--', '*.go', ':!*_test.go')
    cnt = collections.Counter(FUNC.findall(src))
    out.append({'commit': h[:10], 'subject': subj, 'files': len(files), 'funcs': sorted(names),
                'same_name3': sorted(n for n in names if cnt[n] >= 3)})
json.dump(out, open(sys.argv[2], 'w'), indent=1, ensure_ascii=False)
n = len(out); f1_5 = [o for o in out if 1 <= len(o['funcs']) <= 5]
print('fix commits touching non-test go:', n)
print('with 1-5 changed functions:', len(f1_5))
print('of those, touching a same-name>=3 function:', sum(1 for o in f1_5 if o['same_name3']))
print('changed funcs: 0 ->', sum(1 for o in out if not o['funcs']), '; >5 ->', sum(1 for o in out if len(o['funcs']) > 5))
