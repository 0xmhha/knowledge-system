from pathlib import Path
import json,shutil
root=Path('/private/tmp/ks-translated-source-20261004');dst=Path('system/eval/b0-knowledge-system/translated-source-notice-m2max-2026-10-04');dst.mkdir()
def copy(source,relative):
 target=dst/relative;target.parent.mkdir(parents=True,exist_ok=True);shutil.copyfile(source,target)
for source in root.iterdir():
 if source.is_file() and source.suffix in ['.json','.txt','.py']:copy(source,Path('environment')/source.name)
for target in ['darwin','linux-arm64','linux-amd64']:
 base=root/target
 for source in base.iterdir():
  if source.is_file() and source.suffix in ['.json','.txt']:copy(source,Path(target)/source.name)
 for dirname in ['install','runtime','collected-notices']:
  path=base/dirname
  if path.exists():
   for source in path.rglob('*'):
    if source.is_file() and '.git' not in source.parts and 'dataset' not in source.parts and source.suffix not in ['.db','.db-wal','.db-shm']:
     copy(source,Path(target)/dirname/source.relative_to(path))
 copy(base/'signature/public.pem',Path(target)/'signature/public.pem')
 for dirname in ['signature/sidecar','tampered/sidecar']:
  for source in (base/dirname).iterdir():
   if source.is_file():copy(source,Path(target)/dirname/source.name)
for name in ['scripts/license_inventory.py','scripts/test_license_inventory.py']:copy(Path(name),Path('source')/name)
print(dst)
