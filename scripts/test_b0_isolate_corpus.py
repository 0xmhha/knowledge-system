"""The B0 checkout must not inherit a source worktree's recovery history."""

import json
from pathlib import Path
import subprocess
import sys
import tempfile
import unittest


SCRIPT = Path(__file__).with_name("b0-isolate-corpus.py")


def git(repo, *args, check=True):
    result = subprocess.run(["git", "-C", str(repo), *args], text=True,
                            capture_output=True, check=False)
    if check and result.returncode:
        raise AssertionError(f"git {args}: {result.stderr}")
    return result


class IsolateCorpusTest(unittest.TestCase):
    def make_source(self, root):
        repo = root / "source"
        repo.mkdir()
        git(repo, "init", "-q")
        git(repo, "config", "user.name", "fixture")
        git(repo, "config", "user.email", "fixture@example.test")
        git(repo, "config", "commit.gpgsign", "false")
        for number in range(1, 4):
            (repo / "main.go").write_text(f"package fixture\nconst Version = {number}\n")
            git(repo, "add", "main.go")
            git(repo, "commit", "-qm", f"version {number}")
            if number == 2:
                pinned = git(repo, "rev-parse", "HEAD").stdout.strip()
                tree = git(repo, "rev-parse", "HEAD^{tree}").stdout.strip()
        newer = git(repo, "rev-parse", "HEAD").stdout.strip()
        shared = root / "shared-worktree"
        git(repo, "worktree", "add", "--quiet", "--detach", str(shared), pinned)
        return shared, pinned, tree, newer

    def invoke(self, source, out, commit, tree):
        return subprocess.run([sys.executable, str(SCRIPT), "--source", str(source),
                               "--out", str(out), "--commit", commit, "--tree", tree],
                              text=True, capture_output=True, check=False)

    def test_shared_worktree_history_does_not_enter_isolated_checkout(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            source, pinned, tree, newer = self.make_source(root)
            # The later commit is visible through the shared worktree's Git
            # object store despite HEAD being pinned to the older commit.
            self.assertEqual(git(source, "cat-file", "-t", newer).stdout.strip(), "commit")
            out = root / "isolated"
            result = self.invoke(source, out, pinned, tree)
            self.assertEqual(result.returncode, 0, result.stderr)
            self.assertEqual(json.loads(result.stdout)["unreachable_commits"], 0)
            self.assertEqual(git(out, "rev-parse", "HEAD").stdout.strip(), pinned)
            self.assertEqual(git(out, "rev-parse", "HEAD^{tree}").stdout.strip(), tree)
            self.assertNotEqual(git(out, "cat-file", "-t", newer, check=False).returncode, 0)
            self.assertEqual(git(out, "fsck", "--no-reflogs", "--unreachable").stdout.strip(), "")
            self.assertTrue((out / ".git").is_dir())

    def test_wrong_tree_and_existing_output_fail_closed(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            source, pinned, tree, _ = self.make_source(root)
            out = root / "isolated"
            result = self.invoke(source, out, pinned, "0" * 40)
            self.assertEqual(result.returncode, 2)
            self.assertIn("differs from the pinned", result.stderr)
            self.assertFalse(out.exists())
            self.assertFalse(list(root.glob("isolated-stage-*")))
            out.mkdir()
            marker = out / "keep.txt"
            marker.write_text("untouched")
            result = self.invoke(source, out, pinned, tree)
            self.assertEqual(result.returncode, 2)
            self.assertEqual(marker.read_text(), "untouched")


if __name__ == "__main__":
    unittest.main()
