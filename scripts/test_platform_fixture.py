import importlib.util
import os
from pathlib import Path
import subprocess
import tempfile
import unittest

SPEC = importlib.util.spec_from_file_location("platform_fixture", Path(__file__).with_name("prepare-platform-fixture.py"))
MODULE = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(MODULE)


class PlatformFixtureTest(unittest.TestCase):
    def test_primary_and_worktree_copy_only_tracked_current_bytes(self):
        with tempfile.TemporaryDirectory() as temp:
            base = Path(temp)
            repo = base / "repo"
            repo.mkdir()
            subprocess.run(["git", "init", "-q", str(repo)], check=True)
            (repo / "main.go").write_text("old\n")
            (repo / "deleted.go").write_text("remove\n")
            (repo / "run.sh").write_text("#!/bin/sh\n")
            (repo / "run.sh").chmod(0o755)
            outside = base / "outside.txt"
            outside.write_text("must not copy target\n")
            (repo / "link").symlink_to(outside)
            subprocess.run(["git", "-C", str(repo), "add", "."], check=True)
            subprocess.run(["git", "-C", str(repo), "-c", "commit.gpgsign=false", "-c", "user.name=fixture",
                            "-c", "user.email=fixture@example.invalid", "commit", "-qm", "fixture"], check=True)
            worktree = base / "worktree"
            subprocess.run(["git", "-C", str(repo), "worktree", "add", "--quiet", "--detach", str(worktree)], check=True)
            for index, source in enumerate([repo, worktree]):
                (source / "main.go").write_text("changed\n")
                (source / "deleted.go").unlink()
                (source / "untracked-secret.txt").write_text("private\n")
                output = base / f"out{index}"
                self.assertEqual(MODULE.prepare(source, output), 3)
                self.assertEqual((output / "main.go").read_text(), "changed\n")
                self.assertFalse((output / ".git").exists())
                self.assertFalse((output / "untracked-secret.txt").exists())
                self.assertFalse((output / "deleted.go").exists())
                self.assertTrue((output / "run.sh").stat().st_mode & 0o111)
                self.assertTrue((output / "link").is_symlink())
                self.assertEqual(os.readlink(output / "link"), str(outside))
                with self.assertRaises(FileExistsError):
                    MODULE.prepare(source, output)


if __name__ == "__main__":
    unittest.main()
