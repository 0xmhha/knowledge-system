"""Failure oracles for the model and ground-truth identity gate."""

import importlib.util
import json
from pathlib import Path
import tempfile
import threading
import unittest
from http.server import BaseHTTPRequestHandler, HTTPServer


SCRIPT = Path(__file__).with_name("b0-preflight.py")
SPEC = importlib.util.spec_from_file_location("b0_preflight", SCRIPT)
PREFLIGHT = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(PREFLIGHT)
EXPORT_SPEC = importlib.util.spec_from_file_location(
    "b0_export_scenarios", Path(__file__).with_name("b0-export-scenarios.py"))
EXPORT = importlib.util.module_from_spec(EXPORT_SPEC)
EXPORT_SPEC.loader.exec_module(EXPORT)


class ModelHandler(BaseHTTPRequestHandler):
    changed = False
    drift = True

    def do_GET(self):
        if self.path == "/api/version":
            self.respond({"version": "fixture-1.0"})
            return
        if self.path != "/api/tags":
            self.send_error(404)
            return
        digest = ("b" if type(self).changed else "a") * 64
        self.respond({"models": [{"name": "fixture:latest", "digest": digest}]})

    def do_POST(self):
        if self.path != "/api/embed":
            self.send_error(404)
            return
        body = json.loads(self.rfile.read(int(self.headers["Content-Length"])))
        assert body["model"] == "fixture:latest" and body["truncate"] is False
        if type(self).drift:
            type(self).changed = True
        self.respond({"embeddings": [[0.1, 0.2, 0.3]]})

    def respond(self, value):
        body = json.dumps(value).encode()
        self.send_response(200)
        self.send_header("Content-Type", "application/json")
        self.send_header("Content-Length", str(len(body)))
        self.end_headers()
        self.wfile.write(body)

    def log_message(self, *_args):
        pass


class PreflightTest(unittest.TestCase):
    def test_question_anchor_change_is_rejected_before_measurement(self):
        raw = PREFLIGHT.DEFAULT_QUESTIONS.read_bytes()
        original = PREFLIGHT.validate_questions(raw)
        self.assertEqual(original["question_count"], 12)
        changed = json.loads(raw)
        changed["questions"][0]["evidence"][0]["anchor"] = "missing B0 anchor"
        with self.assertRaisesRegex(ValueError, "evidence anchor must appear once"):
            PREFLIGHT.validate_questions(json.dumps(changed).encode())

        changed = json.loads(raw)
        changed["questions"][0]["evidence"][0]["start_line"] = 1
        changed["questions"][0]["evidence"][0]["end_line"] = 1
        with self.assertRaisesRegex(ValueError, "citation span must contain"):
            PREFLIGHT.validate_questions(json.dumps(changed).encode())

    def test_draft_gold_cannot_be_exported(self):
        with tempfile.TemporaryDirectory() as temp:
            out = Path(temp) / "scenarios"
            with self.assertRaisesRegex(ValueError, "need human approval"):
                EXPORT.export(PREFLIGHT.DEFAULT_QUESTIONS, out)
            self.assertFalse(out.exists())

    def test_synthetic_approved_gold_exports_without_answers(self):
        with tempfile.TemporaryDirectory() as temp:
            source = Path(temp) / "questions.json"
            book = json.loads(PREFLIGHT.DEFAULT_QUESTIONS.read_bytes())
            for question in book["questions"]:
                question.update(review_state="approved", reviewer="fixture-reviewer",
                                reviewed_at="2026-10-01T00:00:00Z")
            source.write_text(json.dumps(book))
            out = Path(temp) / "scenarios"
            EXPORT.export(source, out)
            self.assertEqual(len(list(out.glob("*.yaml"))), 12)
            scenario = json.loads((out / "b0-abs-01.yaml").read_text())
            self.assertTrue(scenario["expect_no_citations"])
            self.assertNotIn("candidate_answer", (out / "b0-code-01.yaml").read_text())

    def test_model_digest_change_during_probe_is_rejected(self):
        ModelHandler.changed = False
        ModelHandler.drift = True
        server = HTTPServer(("127.0.0.1", 0), ModelHandler)
        thread = threading.Thread(target=server.serve_forever, daemon=True)
        thread.start()
        try:
            model, reason = PREFLIGHT.ollama_identity(
                f"http://127.0.0.1:{server.server_port}", "fixture")
            self.assertIsNone(model)
            self.assertEqual(reason, "model_digest_changed_during_probe")
        finally:
            server.shutdown()
            server.server_close()
            thread.join(timeout=5)

    def test_stable_model_identity_includes_dimension(self):
        ModelHandler.changed = False
        ModelHandler.drift = False
        server = HTTPServer(("127.0.0.1", 0), ModelHandler)
        thread = threading.Thread(target=server.serve_forever, daemon=True)
        thread.start()
        try:
            model, reason = PREFLIGHT.ollama_identity(
                f"http://127.0.0.1:{server.server_port}", "fixture")
            self.assertIsNone(reason)
            self.assertEqual((model["model"], model["digest"], model["dimension"]),
                             ("fixture:latest", "a" * 64, 3))
            self.assertEqual(model["server_version"], "fixture-1.0")
        finally:
            server.shutdown()
            server.server_close()
            thread.join(timeout=5)

    def test_remote_endpoint_is_rejected(self):
        with self.assertRaisesRegex(ValueError, "local HTTP loopback"):
            PREFLIGHT.ollama_identity("https://example.com", "fixture")


if __name__ == "__main__":
    unittest.main()
