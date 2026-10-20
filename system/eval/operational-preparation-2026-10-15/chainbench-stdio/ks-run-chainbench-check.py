import os,sys,subprocess,tempfile
from pathlib import Path
mode=sys.argv[1]
env=os.environ.copy()
for key in list(env):
 if key.startswith("CHAINBENCH_"):env.pop(key)
env.update(CHAINBENCH_DATA_ROOT=tempfile.mkdtemp(prefix="ks-op-chainbench-state-",dir="/private/tmp"),GOTOOLCHAIN="go1.26.9",GOCACHE="/private/tmp/ks-corpus-go-cache")
Path(env["CHAINBENCH_DATA_ROOT"]).mkdir(parents=True,exist_ok=True)
commands={"tests":["go","test","./..."],"build":["make","build"],"fmt-vet":["make","fmt-check","vet"],"lint":["go","run","github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.12.2","run","--timeout","15m","./..."]}
raise SystemExit(subprocess.call(commands[mode],cwd="/private/tmp/ks-op-chainbench-shutdown",env=env))
