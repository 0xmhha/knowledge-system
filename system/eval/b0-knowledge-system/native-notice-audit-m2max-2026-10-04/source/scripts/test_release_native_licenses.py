"""Signed archive regression: both verifiers must check vendored notices.

Set KS_VERIFIER_BIN to exercise the installed Go verifier as well as Python.
The fake archive binaries are never executed. All signing keys are temporary.
"""
import hashlib
import importlib.util
import io
import json
import os
from pathlib import Path
import subprocess
import tarfile
import tempfile
import unittest

SCRIPTS = Path(__file__).resolve().parent
spec = importlib.util.spec_from_file_location("release_verify", SCRIPTS / "verify-release.py")
verify_module = importlib.util.module_from_spec(spec)
spec.loader.exec_module(verify_module)


def encoded(value):
    return (json.dumps(value, sort_keys=True) + "\n").encode()


def digest(data):
    return hashlib.sha256(data).hexdigest()


class NativeNoticeVerificationTests(unittest.TestCase):
    def setUp(self):
        self.temp = tempfile.TemporaryDirectory()
        self.addCleanup(self.temp.cleanup)
        self.root = Path(self.temp.name)
        self.private = self.root / "private.pem"
        self.public = self.root / "public.pem"
        subprocess.run(["openssl", "genpkey", "-algorithm", "ED25519", "-out", str(self.private)], check=True, capture_output=True)
        subprocess.run(["openssl", "pkey", "-in", str(self.private), "-pubout", "-out", str(self.public)], check=True, capture_output=True)

    def archive(self, native, tamper=False, member_count=None):
        module_notice = b"module notice\n"
        native_notice = b"native notice\n"
        files = {"LICENSE": b"project notice", "INSTALLATION.md": b"instructions",
                 "policies/sanitization_rules.yaml": b"policy", "modules.txt": b"build info",
                 "third-party-licenses/module/LICENSE": module_notice}
        inventory = {"schema_version": 1, "review_status": "pending", "modules": [{
            "license_files": [{"path": "third-party-licenses/module/LICENSE", "sha256": digest(module_notice)}]}]}
        if native:
            path = "third-party-licenses/vendored-grammar/LICENSE"
            files[path] = b"changed notice\n" if tamper else native_notice
            inventory["vendored_assets"] = [{"asset": "grammar", "license_files": [
                {"path": path, "sha256": digest(native_notice)}]}]
        files["third-party-licenses.json"] = encoded(inventory)
        manifest = {"scope": "host-preview", "commit": "a" * 40, "host_os": "darwin",
                    "host_arch": "arm64", "binaries": {},
                    "third_party_license_inventory": {"sha256": digest(files["third-party-licenses.json"])}}
        for name in ("cks", "ckg", "ckv"):
            files[name] = b"fixture binary " + name.encode()
            manifest["binaries"][name] = {"sha256": digest(files[name])}
        files["manifest.json"] = encoded(manifest)
        archive = self.root / "fixture.tar.gz"
        with tarfile.open(archive, "w:gz") as tar:
            for name, data in files.items():
                info = tarfile.TarInfo("fixture/" + name)
                info.size = len(data)
                info.mode = 0o755 if name in manifest["binaries"] else 0o644
                tar.addfile(info, io.BytesIO(data))
            if member_count is not None:
                for i in range(member_count - len(files)):
                    info = tarfile.TarInfo(f"fixture/notice-directory-{i}")
                    info.type = tarfile.DIRTYPE
                    tar.addfile(info)
        signed = self.root / "signed"
        subprocess.run(["python3", str(SCRIPTS / "release-sidecar.py"), "--archive", str(archive),
                        "--private-key", str(self.private), "--public-key", str(self.public),
                        "--out-dir", str(signed)], check=True, capture_output=True)
        return archive, signed / "release.json", signed / "release.json.sig"

    def check_both(self, native, tamper=False, member_count=None, exceeds_bound=False):
        archive, release, signature = self.archive(native, tamper, member_count)
        if exceeds_bound:
            with self.assertRaisesRegex(ValueError, "member count"):
                verify_module.verify(self.public, release, signature, archive, "darwin", "arm64")
        elif tamper:
            with self.assertRaisesRegex(ValueError, "third-party license file differs"):
                verify_module.verify(self.public, release, signature, archive, "darwin", "arm64")
        else:
            self.assertEqual(verify_module.verify(self.public, release, signature, archive, "darwin", "arm64")["status"], "verified")
        binary = os.environ.get("KS_VERIFIER_BIN")
        if binary:
            result = subprocess.run([binary, "package", "verify", "--public-key", str(self.public),
                                     "--release", str(release), "--signature", str(signature),
                                     "--archive", str(archive), "--target-os", "darwin", "--target-arch", "arm64"],
                                    capture_output=True, text=True)
            if tamper or exceeds_bound:
                self.assertNotEqual(result.returncode, 0, "Go verifier accepted a correctly signed archive with an inconsistent vendored notice")
                # The public CLI intentionally redacts internal error details.
                self.assertEqual(json.loads(result.stderr)["code"], "operation_failed")
            else:
                self.assertEqual(result.returncode, 0, result.stderr)
                self.assertEqual(json.loads(result.stdout)["status"], "verified")

    def test_notice_expanded_archive_is_accepted(self):
        self.check_both(True, member_count=110)

    def test_member_bound_is_inclusive(self):
        self.check_both(True, member_count=256)

    def test_member_bound_plus_one_is_rejected(self):
        self.check_both(True, member_count=257, exceeds_bound=True)

    def test_legacy_inventory_without_native_array_is_accepted(self):
        self.check_both(False)

    def test_native_notices_match_inventory(self):
        self.check_both(True)

    def test_resigned_native_notice_tampering_is_rejected(self):
        self.check_both(True, tamper=True)


if __name__ == "__main__":
    unittest.main()
