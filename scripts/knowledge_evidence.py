"""Shared bounded packet/source and unchanged-Go evidence checks."""
import gzip
import hashlib
import importlib.util
import json
import subprocess
from pathlib import Path


def require(value,message):
    if not value:
        raise ValueError(message)


def sha(data):
    return hashlib.sha256(data).hexdigest()


def raw(folder,name):
    path = folder/name
    return path.read_bytes() if path.exists() else gzip.decompress((folder/(name+'.gz')).read_bytes())


def load(folder,name):
    return json.loads(raw(folder,name))


def verify_packet(folder,root):
    folder,root = folder.resolve(),root.resolve()
    manifest = load(folder,'manifest.json')
    actual = {p.relative_to(folder).as_posix()for p in folder.rglob('*')if p.is_file()and p.name!='manifest.json'}
    require(actual == {r['path']for r in manifest['artifacts']} and len(actual)==len(manifest['artifacts']),'artifact inventory differs')
    for row in manifest['artifacts']:
        path = (folder/row['path']).resolve()
        require(path.is_relative_to(folder)and path.is_file(),'unsafe artifact')
        data = path.read_bytes()
        require(sha(data)==row['sha256']and len(data)==row['bytes'],'artifact bytes differ')
        if 'uncompressed_sha256'in row:
            require(sha(gzip.decompress(data))==row['uncompressed_sha256'],'raw artifact differs')
    binding = load(folder,'source-binding.json')
    for name,digest in {**binding['source_sha256'],**binding['approved_input_sha256'],**manifest['source_sha256']}.items():
        path = (root/name).resolve()
        require(path.is_relative_to(root)and sha(path.read_bytes())==digest,'source/input bytes differ')
    require(binding['approval_or_completion_states_changed']is False,'prior acceptance changed')
    return manifest,binding


def unchanged_go(root,validation):
    reference = validation['Go_reference']
    relative = Path(reference['manifest'])
    require(not relative.is_absolute()and '..'not in relative.parts,'unsafe Go reference')
    previous = root/relative.parent
    data = (root/relative).read_bytes()
    require(sha(data)==reference['manifest_sha256'] and reference['revision'].startswith('ae3f987d'),'Go reference differs')
    manifest = json.loads(data)
    binding_bytes = (previous/'source-binding.json').read_bytes()
    require(sha(binding_bytes)==next(r['sha256']for r in manifest['artifacts']if r['path']=='source-binding.json'),'prior Go binding differs')
    binding = json.loads(binding_bytes)
    for name,digest in {**binding['source_sha256'],**binding['approved_input_sha256']}.items():
        require(sha((root/name).read_bytes())==digest,'Go/dependency/fixture inputs changed')
    expected = {n for n in binding['source_sha256']if n.endswith('.go')and n.startswith(('cmd/','internal/','pkg/'))}
    actual = {p.relative_to(root).as_posix()for directory in ('cmd','internal','pkg')for p in (root/directory).rglob('*.go')if p.is_file()}
    require(expected==actual and len(actual)==reference['maintained_Go_files_equal']==1253,'Go files added/removed or scope changed')
    spec = importlib.util.spec_from_file_location('unchanged_go_roadmap',root/'scripts/check-knowledge-roadmap.py')
    roadmap = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(roadmap)
    for label in ('module','race'):
        name = label+'.jsonl.gz'
        data = (previous/name).read_bytes()
        require(sha(data)==next(r['sha256']for r in manifest['artifacts']if r['path']==name),'prior raw Go proof differs')
        counts,_ = roadmap.test_inventory(gzip.decompress(data))
        require(counts==validation[label] and counts['failed_tests']==counts['failed_packages']==0,'Go count/fail/skip differs')
    require(validation['current_Go_validation_reused']is True and validation['Go_inputs_unchanged']is True,'Go reference presented as fresh execution')
    return roadmap


def current_go(root,folder,validation,binding,race_packages=6):
    """Validate a new count=1 execution when maintained Go has changed."""
    require(validation['current_Go_validation_reused']is False and validation['Go_count_flag']==1,'changed Go reused historical results')
    expected = {n for n in binding['source_sha256']if n.endswith('.go')and n.startswith(('cmd/','internal/','pkg/'))}
    actual = {p.relative_to(root).as_posix()for directory in ('cmd','internal','pkg')for p in(root/directory).rglob('*.go')if p.is_file()}
    require(actual==expected and len(actual)==validation['maintained_Go_files'],'current Go inventory missing additions/deletions')
    spec = importlib.util.spec_from_file_location('current_go_roadmap',root/'scripts/check-knowledge-roadmap.py')
    roadmap = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(roadmap)
    for label in ('module','race'):
        counts,_ = roadmap.test_inventory(raw(folder,label+'.jsonl'))
        require(counts==validation[label]and counts['failed_tests']==counts['failed_packages']==0,'current Go raw/count/fail/skip differs')
        require(validation['commands'][label]['exit_code']==0 and '-count=1'in validation['commands'][label]['argv'],'current Go command failed/cached')
    require(validation['commands']['module']['argv'][-1]=='./...'and '-race'in validation['commands']['race']['argv']
            and validation['race']['packages']==race_packages,'Go execution scope reduced')
    return roadmap


def unchanged_document_index_go(root,validation):
    """Reuse A's new Go run only when all Go/dependency/fixture inputs match."""
    ref = validation['Go_reference']
    require(ref['revision'].startswith('bb71aae3'),'unexpected Go reference')
    path = Path(ref['manifest'])
    require(not path.is_absolute()and '..'not in path.parts,'unsafe Go reference')
    body = (root/path).read_bytes()
    require(sha(body)==ref['manifest_sha256']and body==subprocess.check_output(['git','show',ref['revision']+':'+path.as_posix()],cwd=root),'Go reference not immutable committed proof')
    manifest = json.loads(body)
    folder = root/path.parent
    binding_data = (folder/'source-binding.json').read_bytes()
    require(sha(binding_data)==next(r['sha256']for r in manifest['artifacts']if r['path']=='source-binding.json'),'Go source manifest changed')
    binding = json.loads(binding_data)
    # Plugin Python/skills are independently verified by each new checkpoint.
    # They are not inputs to Go compilation/tests; all other prior bound
    # dependencies, approved fixtures and the entire maintained Go set remain.
    for name,digest in {**binding['source_sha256'],**binding['approved_input_sha256']}.items():
        if not name.startswith('plugins/knowledge-manager/'):
            require(sha((root/name).read_bytes())==digest,'Go/dependency/fixture input changed')
    expected = {n for n in binding['source_sha256']if n.endswith('.go')and n.startswith(('cmd/','internal/','pkg/'))}
    actual = {p.relative_to(root).as_posix()for d in ('cmd','internal','pkg')for p in(root/d).rglob('*.go')if p.is_file()}
    require(expected==actual and len(actual)==ref['maintained_Go_files_equal']==1256,'Go files added or removed')
    spec = importlib.util.spec_from_file_location('reused_document_go_roadmap',root/'scripts/check-knowledge-roadmap.py')
    roadmap = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(roadmap)
    for label in ('module','race'):
        data = (folder/(label+'.jsonl.gz')).read_bytes()
        require(sha(data)==next(r['sha256']for r in manifest['artifacts']if r['path']==label+'.jsonl.gz'),'Go raw proof changed')
        counts,_ = roadmap.test_inventory(gzip.decompress(data))
        require(counts==validation[label]and counts['failed_tests']==counts['failed_packages']==0,'Go proof counts/skip changed')
    require(validation['current_Go_validation_reused']is True and validation['Go_inputs_unchanged']is True,'referenced Go presented as fresh run')
    return roadmap


def unchanged_checkpoint_go(root,validation,revision_prefix):
    """Use a committed fresh Go run only while all its Go inputs still match.

    Plugin code is Python and is bound/verified independently by the new packet.
    The build/CI/config/fixture inputs remain included; this is not a new Go run.
    """
    ref=validation['Go_reference']
    require(ref['revision'].startswith(revision_prefix),'unexpected Go reference')
    path=Path(ref['manifest'])
    require(not path.is_absolute()and '..'not in path.parts,'unsafe Go reference')
    body=(root/path).read_bytes()
    require(sha(body)==ref['manifest_sha256']and body==subprocess.check_output(['git','show',ref['revision']+':'+path.as_posix()],cwd=root),'Go reference not immutable committed proof')
    manifest=json.loads(body);folder=root/path.parent
    binding_data=(folder/'source-binding.json').read_bytes()
    require(sha(binding_data)==next(r['sha256']for r in manifest['artifacts']if r['path']=='source-binding.json'),'Go source manifest changed')
    binding=json.loads(binding_data)
    for name,digest in {**binding['source_sha256'],**binding['approved_input_sha256']}.items():
        if not name.startswith('plugins/knowledge-manager/'):
            require(sha((root/name).read_bytes())==digest,'Go/dependency/fixture input changed')
    expected={n for n in binding['source_sha256']if n.endswith('.go')and n.startswith(('cmd/','internal/','pkg/'))}
    actual={p.relative_to(root).as_posix()for d in ('cmd','internal','pkg')for p in(root/d).rglob('*.go')if p.is_file()}
    require(expected==actual and len(actual)==ref['maintained_Go_files_equal'],'Go files added or removed')
    spec=importlib.util.spec_from_file_location('checkpoint_go_roadmap',root/'scripts/check-knowledge-roadmap.py')
    roadmap=importlib.util.module_from_spec(spec);spec.loader.exec_module(roadmap)
    prior=load(folder,'validation.json')
    require(prior['current_Go_validation_reused']is False and prior['Go_count_flag']==1,'reference was not fresh count1 execution')
    for label in ('module','race'):
        data=(folder/(label+'.jsonl.gz')).read_bytes()
        require(sha(data)==next(r['sha256']for r in manifest['artifacts']if r['path']==label+'.jsonl.gz'),'Go raw proof changed')
        counts,_=roadmap.test_inventory(gzip.decompress(data))
        require(counts==validation[label]==prior[label]and counts['failed_tests']==counts['failed_packages']==0,'Go proof counts/skip changed')
        require(prior['commands'][label]['exit_code']==0 and '-count=1'in prior['commands'][label]['argv'],'prior Go command differs')
    require(validation['current_Go_validation_reused']is True and validation['Go_inputs_unchanged']is True,'referenced Go presented as fresh run')
    return roadmap
