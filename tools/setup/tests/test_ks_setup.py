"""Unit tests for ks-setup. Run: python3 -m unittest discover -s tools/setup/tests

The end-to-end test builds a real dataset with the mock embedder; it runs only
when KS_SETUP_E2E=1 and the binaries exist in bin/.
"""
import io
import json
import os
import shutil
import subprocess
import sys
import tempfile
import unittest
from pathlib import Path
from typing import Optional

sys.path.insert(0, str(Path(__file__).resolve().parents[1]))

from ks_setup import buildsrc, events, mcp_check, pack, pipeline, preflight, profile  # noqa: E402
from ks_setup.model import Run, Status, Step  # noqa: E402
from ks_setup.options import Options, choose_version, repo_root, slug  # noqa: E402
from ks_setup.ui import PlainView, fmt_elapsed, run_worker  # noqa: E402


def git_repo(root: Path, files: Optional[dict] = None) -> Path:
    src = root / "demo"
    src.mkdir()
    (src / "go.mod").write_text("module demo\n\ngo 1.22\n")
    (src / "a.go").write_text("package demo\n\nfunc A() int { return B() }\nfunc B() int { return 1 }\n")
    for rel, text in (files or {}).items():
        (src / rel).parent.mkdir(parents=True, exist_ok=True)
        (src / rel).write_text(text)
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

    def test_audit_gap_becomes_a_warning(self):
        run, parent = Run([]), Step("index", "Index")
        for n in ("0", "238"):
            events.apply(run, parent, events.parse(json.dumps(
                {"step": "reindex-gate", "type": "output", "message": f"  in build only (missing from DB): {n}"})))
        self.assertEqual(len(run.warnings), 1)
        self.assertIn("238 source files", run.warnings[0])


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

    def test_build_sources_flag(self):
        argv = pipeline.setup_argv(self.o, "v1", build_sources=Path("/d/proj-derived/vector-build-sources.json"))
        i = argv.index("--vector-build-sources")
        self.assertEqual(argv[i + 1], "/d/proj-derived/vector-build-sources.json")
        self.assertNotIn("--vector-build-sources", pipeline.setup_argv(self.o, "v1"))

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


class PackTests(unittest.TestCase):
    def make_pack(self, root: Path) -> Path:
        pk = root / "pack"
        (pk / "policies").mkdir(parents=True)
        (pk / "dk" / "flow").mkdir(parents=True)
        for f in ("policies/graph.yaml", "policies/vector.yaml", "dk/glossary.yaml", "dk/flow/corpus.jsonl", "filelist.yaml"):
            (pk / f).write_text("x: 1\n")
        (pk / "setup.yaml").write_text(
            "policy_file: policies/graph.yaml\nvector_policy: policies/vector.yaml\nembedder: ollama\n"
            "model_name: bge-m3\ndomain_knowledge: dk\nglossary_file: dk/glossary.yaml\n"
            "flow_corpus: dk/flow/corpus.jsonl\ncode_root: ${CKS_CODE_ROOT}\nfilelist: filelist.yaml\n")
        return pk

    def test_inputs_resolve_against_the_pack_and_the_source(self):
        with tempfile.TemporaryDirectory() as d:
            pk, src = self.make_pack(Path(d)), Path(d) / "src"
            loaded = pack.load(pk, src)
            self.assertEqual(loaded.inputs["policy_file"], (pk / "policies/graph.yaml").resolve())
            self.assertEqual(loaded.inputs["code_root"], src.resolve())
            o = Options(src=src, out=Path(d) / "kd" / "proj", name="proj", bin_dir=Path("/b"), pack_dir=pk)
            argv = pipeline.setup_argv(o, "v1", loaded)
            for flag in ("--policy-file", "--vector-policy", "--domain-knowledge", "--glossary-file",
                         "--flow-corpus", "--filelist", "--code-root", "--derived-dir"):
                self.assertIn(flag, argv)
            self.assertEqual(argv[argv.index("--derived-dir") + 1], str(Path(d) / "kd" / "proj-derived"))
            server = pipeline.gen_config_argv(o, pack=loaded)
            self.assertEqual(server[server.index("--domain-corpus-dir") + 1],
                             str(Path(d) / "kd" / "proj-derived" / "domain-corpus"))
            self.assertIn("--glossary", server)

    def test_graph_only_drops_vector_inputs_and_skip_removes_one(self):
        with tempfile.TemporaryDirectory() as d:
            pk = self.make_pack(Path(d))
            o = Options(src=Path(d), out=Path(d) / "kd", name="p", bin_dir=Path("/b"), pack_dir=pk,
                        graph_only=True, skip_pack_inputs=["filelist"])
            argv = pipeline.setup_argv(o, None, pipeline.load_pack(o))
            self.assertIn("--policy-file", argv)
            for flag in ("--vector-policy", "--flow-corpus", "--domain-knowledge", "--filelist", "--derived-dir"):
                self.assertNotIn(flag, argv)

    def test_preflight_reports_missing_inputs_and_model_mismatch(self):
        with tempfile.TemporaryDirectory() as d:
            pk = self.make_pack(Path(d))
            (pk / "filelist.yaml").unlink()
            o = Options(src=Path(d), out=Path(d) / "kd", name="p", pack_dir=pk, model="qwen3-embedding:4b")
            first = preflight.check_pack(o)[0]
            self.assertFalse(first.ok)
            self.assertIn("filelist", first.detail)
            o.skip_pack_inputs = ["filelist"]
            checks = preflight.check_pack(o)
            self.assertTrue(checks[0].ok)
            self.assertEqual([(c.name, c.fatal) for c in checks[1:]], [("pack model", False)])
            unset = Path(d) / "bad"
            unset.mkdir()
            (unset / "setup.yaml").write_text("code_root: ${NOT_SET_ANYWHERE_42}\n")
            self.assertFalse(preflight.check_pack(Options(src=Path(d), out=Path(d) / "kd", name="p", pack_dir=unset))[0].ok)


BUILD_TREE = {
    "internal/build/util.go": "package build\n\nfunc Util() {}\n",
    "internal/build/util_test.go": "package build\n\nimport \"testing\"\n\nfunc TestUtil(t *testing.T) {}\n",
    "internal/build/testdata/fixture.go": "package fixture\n",
    "build/ci.go": "//go:build none\n\npackage main\n\nfunc main() {}\n",
    "vendor/x/build/v.go": "package build\n",
    "tools/build.go": "package tools\n",
}


@unittest.skipUnless(shutil.which("go"), "go not installed")
class BuildSourceTests(unittest.TestCase):
    def test_lists_what_go_loads_under_build_dirs(self):
        with tempfile.TemporaryDirectory() as d:
            src = git_repo(Path(d), BUILD_TREE)
            # build constraints, testdata and vendor apply as they do for the graph
            self.assertEqual(buildsrc.discover(src, committed=True),
                             ["internal/build/util.go", "internal/build/util_test.go"])
            (src / "internal/build/new.go").write_text("package build\n")
            self.assertNotIn("internal/build/new.go", buildsrc.discover(src, committed=True))
            self.assertIn("internal/build/new.go", buildsrc.discover(src, committed=False))
            out = Path(d) / "list.json"
            buildsrc.write(out, ["internal/build/util.go"])
            self.assertEqual(json.loads(out.read_text()), {"schema_version": 1, "paths": ["internal/build/util.go"]})

    def test_no_build_dirs_means_no_list(self):
        with tempfile.TemporaryDirectory() as d:
            self.assertEqual(buildsrc.discover(git_repo(Path(d)), committed=True), [])

    def test_go_scope_gap_compares_go_only(self):
        with tempfile.TemporaryDirectory() as d:
            root = Path(d)
            (root / "graph").mkdir()
            (root / "vector").mkdir()
            (root / "graph" / "manifest.json").write_text(json.dumps({"files": [
                {"path": "a.go", "language": "go"}, {"path": "internal/build/b.go", "language": "go"},
                {"path": "x.proto", "language": "proto"}]}))
            (root / "vector" / "manifest.json").write_text(json.dumps({"input_files": [{"path": "a.go"}]}))
            self.assertEqual(buildsrc.go_scope_gap(root), ["internal/build/b.go"])
            (root / "vector" / "manifest.json").write_text(json.dumps({"chunk_count": 1}))
            self.assertEqual(buildsrc.go_scope_gap(root), [])


class ProfileTests(unittest.TestCase):
    def test_profile_and_compare(self):
        import sqlite3
        with tempfile.TemporaryDirectory() as d:
            root = Path(d) / "ds"
            (root / "graph").mkdir(parents=True)
            (root / "vector").mkdir()
            (root / "graph" / "manifest.json").write_text(json.dumps(
                {"schema_version": "1.24", "src_commit": "a" * 40, "languages": {"go": 2},
                 "files": [{}, {}], "stats": {"nodes": 10, "edges": 20}}))
            g = sqlite3.connect(root / "graph" / "graph.db")
            g.executescript("CREATE TABLE nodes(id, type, is_test); CREATE TABLE edges(src, dst, type);"
                            "CREATE TABLE node_prs(node_id); CREATE TABLE code_edges(src);"
                            "INSERT INTO nodes VALUES ('1','Function',0),('2','Policy',0);"
                            "INSERT INTO edges VALUES ('1','2','governed_by');")
            g.commit(); g.close()
            (root / "vector" / "manifest.json").write_text(json.dumps(
                {"chunk_count": 3, "embedding_model": "bge-m3", "embedding_dim": 1024, "languages": {"go": 3}}))
            v = sqlite3.connect(root / "vector" / "vector.db")
            v.executescript("CREATE TABLE chunks(chunk_kind, category, guidance, canonical_id, flow_meta, is_test);"
                            "INSERT INTO chunks VALUES ('symbol','consensus','g','c',NULL,0),"
                            "('doc','domain',NULL,NULL,NULL,0),('symbol',NULL,NULL,NULL,'{}',1);")
            v.commit(); v.close()
            p = profile.dataset_profile(root)
            self.assertEqual((p["graph"]["policy_nodes"], p["graph"]["governed_by_edges"], p["graph"]["files"]), (1, 1, 2))
            self.assertEqual((p["vector"]["categorized"], p["vector"]["domain_chunks"], p["vector"]["flow_chunks"]), (2, 1, 1))
            self.assertFalse(any(f.name.endswith(("-wal", "-shm")) for f in root.rglob("*")))
            rows = dict((m, (a, b)) for m, a, b in profile.compare(p, {"graph": {"nodes": 7}}))
            self.assertEqual(rows["graph nodes"], (10, 7))
            self.assertEqual(rows["graph language: go"], (2, None))


class RetentionTests(unittest.TestCase):
    def test_counts_version_directories_only(self):
        with tempfile.TemporaryDirectory() as d:
            out = Path(d)
            for v in ("v1", "v2"):
                (out / v / "graph").mkdir(parents=True)
                (out / v / "graph" / "graph.db").write_bytes(b"x" * 10)
            (out / "current").symlink_to("v2")
            (out / "rollback-targets.json").write_text("{}")
            kept = profile.retained_versions(out)
            self.assertEqual((kept["count"], kept["bytes"], kept["labels"]), (2, 20, ["v1", "v2"]))


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
            src = git_repo(Path(d), {"internal/build/util.go": "package build\n\nfunc Util() int { return 2 }\n"})
            out = Path(d) / "kd" / "demo"
            cmd = [sys.executable, "-m", "ks_setup", "--src", str(src), "--out", str(out), "--embedder", "mock",
                   "--no-build", "--plain", "--yes", "--port", "18931"]
            proc = subprocess.run(cmd, cwd=Path(__file__).resolve().parents[1], capture_output=True, text=True, timeout=600)
            self.assertEqual(proc.returncode, 0, proc.stdout + proc.stderr)
            self.assertIn("Dataset ready.", proc.stdout)
            self.assertTrue((out / "current" / "graph" / "graph.db").is_file())
            self.assertIn("transport: stdio", (out.parent / "demo-cks-stdio.yaml").read_text())
            report = json.loads((out.parent / "demo-setup-report.json").read_text())
            self.assertEqual(report["build_sources"], ["internal/build/util.go"])
            self.assertEqual(report["go_scope_gap"], [])
            # A second run reuses the project config and promotes a new version.
            proc = subprocess.run(cmd, cwd=Path(__file__).resolve().parents[1], capture_output=True, text=True, timeout=600)
            self.assertEqual(proc.returncode, 0, proc.stdout + proc.stderr)
            self.assertEqual(len(json.loads((out / "rollback-targets.json").read_text())["versions"]), 1)


if __name__ == "__main__":
    unittest.main()

class SemanticSetupTests(unittest.TestCase):
    def test_explicit_semantic_settings_and_held_candidate_are_preserved(self):
        o = Options(src=Path("/s/project"), out=Path("/d/project"), name="project",
                    project_id="project", semantic_store=Path("/d/meaning.sqlite"),
                    ontology_mode="combined", ontology_budget_ms=5000)
        argv = pipeline.gen_config_argv(o)
        self.assertEqual(argv[argv.index("--semantic-store") + 1], "/d/meaning.sqlite")
        self.assertEqual(argv[argv.index("--ontology-mode") + 1], "combined")
        self.assertEqual(argv[argv.index("--ontology-budget-ms") + 1], "5000")
        self.assertEqual(pipeline.init_argv(o)[-2:], ["--project-id", "project"])
        o.ontology = "ontology.yaml"
        o.semantic_candidate_dir = Path("/d/project/v-held")
        self.assertIn("--hold-for-review", pipeline.setup_argv(o, "v-held"))
        self.assertEqual(o.serving_dir, Path("/d/project/v-held"))
        self.assertNotEqual(o.server_config, Path("/d/project-cks.yaml"))
        self.assertIn("semantic", [s.key for s in pipeline.new_run(o).steps])

    def test_invalid_and_unpinned_inputs_are_rejected(self):
        cases = [dict(ontology_mode="typo"), dict(ontology_mode="combined"),
                 dict(ontology_budget_ms=0), dict(ontology_budget_ms=5001),
                 dict(graph_only=True, semantic_store=Path("/data/meaning.sqlite")),
                 dict(semantic_store=Path("/source/meaning.sqlite")),
                 dict(ontology="../outside.yaml"), dict(ontology="/outside.yaml")]
        for values in cases:
            with self.subTest(values=values):
                with self.assertRaises(ValueError):
                    Options(src=Path("/source"), out=Path("/data"), name="x", **values).validate_semantic()

    def test_semantic_preparation_does_not_activate_or_overwrite_operator_configs(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            source = root / "source"
            source.mkdir()
            (source / "ontology.yaml").write_text("project_id: demo\n")
            o = Options(src=source, out=root / "dataset", name="demo", ontology="ontology.yaml", project_id="demo")
            o.validate_semantic()
            candidate = o.out / "v-held"
            candidate.mkdir(parents=True)
            (candidate / "dataset-identity.json").write_text(json.dumps({"source": {"project_id": "demo"}, "dataset_id": "fixture"}))
            o.semantic_candidate_dir = candidate
            current = o.out / "current"
            current.symlink_to("old")
            server = root / "dataset-cks.yaml"
            server.write_text("operator chosen config\n")
            p = pipeline.Pipeline(o, pipeline.new_run(o), io.StringIO())
            calls = []
            p._exec = lambda argv, cb, **kwargs: calls.append(argv) or 0
            p.do_semantic(p.run.step("semantic"))
            self.assertEqual(os.readlink(current), "old")
            self.assertEqual(server.read_text(), "operator chosen config\n")
            self.assertNotIn("--activate", calls[0])
            self.assertEqual(calls[0][calls[0].index("--version-dir") + 1], str(candidate))
