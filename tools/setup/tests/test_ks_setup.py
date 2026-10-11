"""Unit tests for ks-setup. Run: python3 -m unittest discover -s tools/setup/tests

The end-to-end test builds a real dataset with the mock embedder; it runs only
when KS_SETUP_E2E=1 and the binaries exist in bin/.
"""
import io
import json
import os
import subprocess
import sys
import tempfile
import unittest
from pathlib import Path

sys.path.insert(0, str(Path(__file__).resolve().parents[1]))

from ks_setup import events, mcp_check, pipeline, preflight  # noqa: E402
from ks_setup.model import Run, Status, Step  # noqa: E402
from ks_setup.options import Options, choose_version, repo_root, slug  # noqa: E402
from ks_setup.ui import PlainView, fmt_elapsed, run_worker  # noqa: E402


def git_repo(root: Path) -> Path:
    src = root / "demo"
    src.mkdir()
    (src / "go.mod").write_text("module demo\n\ngo 1.22\n")
    (src / "a.go").write_text("package demo\n\nfunc A() int { return B() }\nfunc B() int { return 1 }\n")
    env = dict(os.environ, GIT_AUTHOR_NAME="t", GIT_AUTHOR_EMAIL="t@t", GIT_COMMITTER_NAME="t", GIT_COMMITTER_EMAIL="t@t")
    for argv in (["git", "init", "-q"], ["git", "add", "."], ["git", "commit", "-qm", "init"]):
        subprocess.run(argv, cwd=src, check=True, env=env)
    return src


class EventTests(unittest.TestCase):
    def test_parse_json_and_text(self):
        e = events.parse('{"step":"graph-build","type":"start","message":"Build the graph index"}')
        self.assertEqual((e.step, e.type, e.message), ("graph-build", "start", "Build the graph index"))
        e = events.parse("setup: promoted x")
        self.assertEqual((e.step, e.type), ("", "output"))
        self.assertIsNone(events.parse("   "))

    def test_substeps_follow_the_engine(self):
        run, parent = Run([]), Step("index", "Index")
        for line in [
            '{"step":"reindex-resources","type":"status","resources":{"source_files":2,"source_bytes":2048}}',
            '{"step":"graph-build","type":"start","message":"Build the graph index"}',
            '{"step":"graph-build","type":"output","message":"built 10 nodes"}',
            '{"step":"graph-build","type":"done"}',
            '{"step":"reindex-gate","type":"warning","message":"canonical_id coverage gate disabled"}',
            '{"step":"vector-build","type":"start","message":"Build the vector index"}',
            '{"step":"vector-build","type":"error","message":"ollama unreachable"}',
        ]:
            events.apply(run, parent, events.parse(line))
        status = {s.key: s.status for s in parent.substeps}
        self.assertEqual(status, {"graph-build": Status.DONE, "reindex-gate": Status.WARNING,
                                  "vector-build": Status.FAILED})
        self.assertEqual(parent.substep("graph-build").title, "Graph index (ckg)")
        self.assertIn("2 source files", parent.note)
        self.assertTrue(any("canonical_id" in w for w in run.warnings))
        self.assertIn("[graph-build] built 10 nodes", run.log)


class OptionTests(unittest.TestCase):
    def test_version_labels_never_collide(self):
        with tempfile.TemporaryDirectory() as d:
            out = Path(d)
            first = choose_version(out, "23f0236ecbda", now=0)
            self.assertTrue(first.endswith("-23f0236e"))
            (out / first).mkdir()
            self.assertEqual(choose_version(out, "23f0236ecbda", now=0), first + "-2")

    def test_paths_and_layout(self):
        o = Options(src=Path("/s/proj"), out=Path("/d/proj"), name="proj")
        self.assertEqual(o.setup_config, Path("/d/proj-setup.yaml"))
        self.assertEqual(o.serving_dir, Path("/d/proj/current"))
        o.graph_only = True
        self.assertEqual(o.serving_dir, Path("/d/proj"))
        self.assertEqual(slug("My Project!"), "my-project")
        self.assertTrue((repo_root() / "go.mod").is_file())


class CommandTests(unittest.TestCase):
    def setUp(self):
        self.o = Options(src=Path("/s/proj"), out=Path("/d/proj"), name="proj", bin_dir=Path("/b"))

    def test_versioned_setup(self):
        argv = pipeline.setup_argv(self.o, "v1")
        self.assertEqual(argv[:6], ["/b/cks", "setup", "--config", "/d/proj-setup.yaml", "--version", "v1"])
        for flag in ("--progress", "--embed-dim", "--ollama-url", "--query-prefix-policy", "--source-mode"):
            self.assertIn(flag, argv)
        self.assertEqual(pipeline.setup_env(self.o)["CKV_REQUIRE_COMPLETE_EMBEDDINGS"], "1")

    def test_graph_only_is_unversioned(self):
        self.o.graph_only = True
        argv = pipeline.setup_argv(self.o, None)
        self.assertIn("--skip-vector", argv)
        self.assertNotIn("--version", argv)
        self.assertNotIn("CKV_REQUIRE_COMPLETE_EMBEDDINGS", pipeline.setup_env(self.o)
                         if "CKV_REQUIRE_COMPLETE_EMBEDDINGS" not in os.environ else {})

    def test_server_config_reads_current(self):
        argv = pipeline.gen_config_argv(self.o)
        self.assertEqual(argv[argv.index("--dataset-dir") + 1], "/d/proj/current")
        self.assertEqual(argv[argv.index("--out") + 1], "/d/proj-cks.yaml")
        rules = Path(argv[argv.index("--sanitize-rules") + 1])
        self.assertTrue(rules.is_absolute() and rules.is_file(), rules)

    def test_stdio_variant(self):
        cfg = "listen:\n    http_addr: 127.0.0.1:1\n    mcp_stdio: false\n    transport: http\n"
        out = mcp_check.stdio_variant(cfg)
        self.assertIn("mcp_stdio: true", out)
        self.assertIn("transport: stdio", out)
        with self.assertRaises(ValueError):
            mcp_check.stdio_variant("listen: {}\n")


class PreflightTests(unittest.TestCase):
    def test_source_and_output_checks(self):
        with tempfile.TemporaryDirectory() as d:
            src = git_repo(Path(d))
            o = Options(src=src, out=Path(d) / "ds", name="demo")
            self.assertTrue(all(c.ok for c in preflight.check_source(o)))
            (src / "a.go").write_text("package demo\n")
            warn = [c for c in preflight.check_source(o) if not c.ok]
            self.assertEqual([(c.name, c.fatal) for c in warn], [("uncommitted changes", False)])
            inside = Options(src=src, out=src / "ds", name="demo")
            self.assertFalse(preflight.check_output(inside)[0].ok)
            self.assertFalse(preflight.check_source(Options(src=Path(d), out=Path(d) / "x", name="x"))[0].ok)

    def test_ollama(self):
        o = Options(src=Path("/s"), out=Path("/d"), name="x")
        tags = {"models": [{"name": "bge-m3:latest"}]}
        fake = lambda url: {"version": "0.9"} if url.endswith("/api/version") else tags  # noqa: E731
        self.assertTrue(all(c.ok for c in preflight.check_ollama(o, fake)))
        tags["models"] = []
        model = preflight.check_ollama(o, fake)[1]
        self.assertFalse(model.ok)
        self.assertIn("ollama pull", model.detail)

        def down(url):
            raise OSError("connection refused")
        self.assertFalse(preflight.check_ollama(o, down)[0].ok)
        o.graph_only = True
        self.assertEqual(preflight.check_ollama(o, down), [])


class FailureTests(unittest.TestCase):
    def test_failed_preflight_skips_the_rest(self):
        with tempfile.TemporaryDirectory() as d:
            o = Options(src=Path(d) / "missing", out=Path(d) / "ds", name="x", build=False, bin_dir=Path(d))
            run = pipeline.new_run(o)
            lines = []
            ok = PlainView(o, run, "log", write=lines.append).live(pipeline.Pipeline(o, run, io.StringIO()).execute, lambda: None)
            self.assertFalse(ok)
            self.assertEqual(run.step("preflight").status, Status.FAILED)
            self.assertTrue(all(run.step(k).status == Status.SKIPPED for k in ("build", "init", "index", "serve", "verify")))
            self.assertTrue(any(l.startswith("Setup failed:") for l in lines))

    def test_interrupt_cancels_and_waits_for_the_worker(self):
        import threading
        cancelled, ticks = threading.Event(), []

        def work():
            cancelled.wait(5)
            return False

        def tick():
            ticks.append(1)
            if len(ticks) in (2, 3):  # a second Ctrl-C must not cancel twice or abandon the worker
                raise KeyboardInterrupt
        calls = []
        ok = run_worker(work, lambda: (calls.append(1), cancelled.set()), tick, 0.01)
        self.assertFalse(ok)
        self.assertEqual(calls, [1])
        self.assertTrue(cancelled.is_set())

    def test_elapsed_format(self):
        self.assertEqual([fmt_elapsed(x) for x in (None, 5, 65, 3725)], ["", "5s", "1m05s", "1h02m"])


@unittest.skipUnless(os.environ.get("KS_SETUP_E2E") == "1" and (repo_root() / "bin" / "cks").is_file(),
                     "set KS_SETUP_E2E=1 with built binaries")
class EndToEndTests(unittest.TestCase):
    def test_mock_dataset_round_trip(self):
        with tempfile.TemporaryDirectory() as d:
            src = git_repo(Path(d))
            out = Path(d) / "kd" / "demo"
            cmd = [sys.executable, "-m", "ks_setup", "--src", str(src), "--out", str(out), "--embedder", "mock",
                   "--no-build", "--plain", "--yes", "--port", "18931"]
            proc = subprocess.run(cmd, cwd=Path(__file__).resolve().parents[1], capture_output=True, text=True, timeout=600)
            self.assertEqual(proc.returncode, 0, proc.stdout + proc.stderr)
            self.assertIn("Dataset ready.", proc.stdout)
            self.assertTrue((out / "current" / "graph" / "graph.db").is_file())
            self.assertIn("transport: stdio", (out.parent / "demo-cks-stdio.yaml").read_text())
            # A second run reuses the project config and promotes a new version.
            proc = subprocess.run(cmd, cwd=Path(__file__).resolve().parents[1], capture_output=True, text=True, timeout=600)
            self.assertEqual(proc.returncode, 0, proc.stdout + proc.stderr)
            self.assertEqual(len(json.loads((out / "rollback-targets.json").read_text())["versions"]), 1)


if __name__ == "__main__":
    unittest.main()
