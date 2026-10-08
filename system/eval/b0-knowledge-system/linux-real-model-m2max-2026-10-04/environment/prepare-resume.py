from pathlib import Path
import shutil
root=Path('/private/tmp/ks-linux-real-model-20261004');base=root/'diagnostic-after/typescript'
shutil.copyfile(base/'mcp.mcp.log',base/'mcp-initialization-20s.log')
probe=Path('scripts/wbs-mcp-pin-probe.py').read_text();assert 'timeout=20' in probe
(root/'mcp-pin-probe-90s.py').write_text(probe.replace('timeout=20','timeout=90'))
s=(root/'run-real-install.py').read_text()
s=s.replace('diag.mkdir(mode=0o700)','diag.mkdir(mode=0o700,exist_ok=True)').replace("diag/'commands.json'","diag/'resume-commands.json'").replace('timeout=600','timeout=1200')
s=s.replace("['empty-go','typescript','unsupported-python']","['typescript','unsupported-python']")
start=s.index(" base=diag/kind;");end=s.index(" mcp=base/'mcp.yaml';")
segment=s[start:end]
segment=segment.replace('base.mkdir();','base.mkdir(exist_ok=True);')
s=s[:start]+" base=diag/kind;dataset=base/'dataset';cfg=base/'setup.yaml'\n if kind=='unsupported-python':\n"+''.join(' '+line+'\n' for line in segment.splitlines())+s[end:]
start=s.index(" mcp=base/'mcp.yaml';");end=s.index("prompt=f'Where",start)
segment=s[start:end]
s=s[:start]+" mcp=base/'mcp.yaml'\n if kind=='unsupported-python':\n"+''.join(' '+line+'\n' for line in segment.splitlines())+' '+s[end:]
s=s.replace("'/scripts/wbs-mcp-pin-probe.py'","'/out/mcp-pin-probe-90s.py'")
s=s.replace("run(base,'mcp',", "run(base,'mcp-retry-90s',").replace("base/'mcp.stdout.txt'","base/'mcp-retry-90s.stdout.txt'")
s=s.replace("print('three actual-model installations and 72 SDK rows captured; independent source/integrity audit still required')","print('remaining two installations captured with 90s diagnostic probe; first failure preserved; independent audit required')")
(root/'resume-real-install.py').write_text(s)
