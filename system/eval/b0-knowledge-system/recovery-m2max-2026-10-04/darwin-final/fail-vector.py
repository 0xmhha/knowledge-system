#!/usr/bin/env python3
import subprocess,sys
if "build" in sys.argv[1:]:sys.exit(23)
sys.exit(subprocess.run(['/private/tmp/ks-translated-source-20261004/darwin/unpacked/knowledge-system-darwin-arm64-7b5d7bb89618-dirty-preview/ckv',*sys.argv[1:]]).returncode)
