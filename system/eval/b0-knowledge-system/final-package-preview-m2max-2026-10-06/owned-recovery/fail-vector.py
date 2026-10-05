#!/usr/bin/env python3
import subprocess,sys
if "build" in sys.argv[1:]:sys.exit(23)
sys.exit(subprocess.run(['/private/tmp/ks-final-package-darwin-20261006/unpacked/knowledge-system-darwin-arm64-10cbffaa104e-dirty-preview/ckv',*sys.argv[1:]]).returncode)
