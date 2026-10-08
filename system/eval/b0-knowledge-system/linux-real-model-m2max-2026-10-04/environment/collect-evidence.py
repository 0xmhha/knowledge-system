from pathlib import Path
import json,shutil,hashlib,tarfile
root=Path('/private/tmp/ks-linux-real-model-20261004');dst=Path('system/eval/b0-knowledge-system/linux-real-model-m2max-2026-10-04');dst.mkdir()
def copy(source,relative):
 target=dst/relative;target.parent.mkdir(parents=True,exist_ok=True);shutil.copyfile(source,target)
# Only named diagnostic artifacts. Never copy model store, private keys or databases.
for source in root.iterdir():
 if source.is_file() and source.suffix in ['.json','.txt','.py']:copy(source,Path('environment')/source.name)
for kind in ['empty-go','typescript','unsupported-python']:
 base=root/'diagnostic-after'/kind
 for source in base.iterdir():
  if source.is_file():copy(source,Path('projects')/kind/source.name)
 for source in (base/'src').iterdir():
  if source.is_file():copy(source,Path('projects')/kind/'source-current'/source.name)
 for source in (base/'matrix').rglob('*'):
  if source.is_file():copy(source,Path('projects')/kind/'matrix'/source.relative_to(base/'matrix'))
 for version in (base/'dataset').iterdir():
  if not version.is_dir() or version.is_symlink():continue
  for source in version.rglob('*'):
   if not source.is_file():continue
   rel=source.relative_to(version)
   if source.suffix=='.json' or rel.parts[:2]==('sources','blobs'):
    copy(source,Path('projects')/kind/'versions'/version.name/rel)
copy(root/'diagnostic-after/commands.json',Path('commands.json'))
copy(root/'diagnostic-after/resume-commands.json',Path('resume-commands.json'))
pair=Path('/private/tmp/ks-fix14-20261004/arm64')
copy(pair/'signature/public.pem',Path('package/test-public.pem'))
for source in (pair/'signature/sidecar').iterdir():
 if source.is_file():copy(source,Path('package')/source.name)
archive=next((pair/'first').glob('*.tar.gz'))
with tarfile.open(archive) as t:
 for source in t.getmembers():
  if source.name.endswith('/manifest.json') and len(Path(source.name).parts)==2:
   (dst/'package/manifest.json').write_bytes(t.extractfile(source).read())
(root/'evidence-location.txt').write_text(str(dst.resolve())+'\n')
print(dst)
