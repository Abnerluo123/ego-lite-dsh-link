from __future__ import annotations

import io
import json
import unittest
from unittest.mock import patch

import server
from server import (
    MARKER,
    apply_tabs,
    handle,
    invoke,
    output_config,
    state_body,
    task_source,
    validate_key,
    validate_page,
    validate_selector,
    validate_url,
)


class ValidationTests(unittest.TestCase):
    def test_urls(self):
        for good in ("https://example.com", "http://localhost:3000/x?a=1", " https://a.b/c "):
            self.assertTrue(validate_url(good))
        for bad in ("", "file:///etc/passwd", "javascript:alert(1)", "//example.com", "https://user:pass@example.com"):
            with self.subTest(bad=bad), self.assertRaises(ValueError):
                validate_url(bad)

    def test_selector_key_page(self):
        self.assertEqual(validate_selector("ref=1"), "ref=1")
        with self.assertRaises(ValueError):
            validate_selector("  ")
        with self.assertRaises(ValueError):
            validate_key("Enter;process.exit(1)")
        self.assertEqual(validate_page("p12"), "p12")
        with self.assertRaises(ValueError):
            validate_page("page2")

    def test_task_name_is_escaped(self):
        script = task_source("return 1;", 'name";process.exit(1);//')
        self.assertIn('taskSpace("name\\\";process.exit(1);//")', script)


class OutputTests(unittest.TestCase):
    def tearDown(self):
        server.BRIEF = False
        server.MAXCHARS = 0
        server.AUTOSWITCH = True
        server.LAST_TABS = []

    def test_brief_body_skips_snapshot(self):
        server.BRIEF = True
        body = state_body("p1")
        self.assertNotIn("statePage.snapshot()", body)
        self.assertIn("brief:true", body)

    def test_apply_tabs_follows_single_new_tab(self):
        server.LAST_TABS = ["p1"]
        result = {"__tabs": ["p1", "p2"]}
        apply_tabs(result)
        self.assertEqual(server.PAGE, "p2")
        self.assertEqual(result.get("autoSwitchedTo"), "p2")
        self.assertNotIn("__tabs", result)

    def test_apply_tabs_ignores_two_new_tabs(self):
        server.PAGE = "p1"
        server.LAST_TABS = ["p1"]
        apply_tabs({"__tabs": ["p1", "p2", "p3"]})
        self.assertEqual(server.PAGE, "p1")

    def test_output_config(self):
        self.assertEqual(output_config("true", "900", "false"), {"brief": True, "maxChars": 900, "autoSwitch": False})
        with self.assertRaises(ValueError):
            output_config("", "10", "")


class InvokeTests(unittest.TestCase):
    def test_marker_on_stderr(self):
        result = type("R", (), {"returncode": 1, "stdout": "", "stderr": MARKER + '{"ok":true}\n'})()
        with patch("server.cli_path", return_value="ego-browser"), patch("subprocess.run", return_value=result):
            self.assertEqual(invoke("const v=1;"), {"ok": True})

    def test_missing_marker_raises(self):
        result = type("R", (), {"returncode": 1, "stdout": "", "stderr": "bootstrap blocked"})()
        with patch("server.cli_path", return_value="ego-browser"), patch("subprocess.run", return_value=result):
            with self.assertRaisesRegex(RuntimeError, "bootstrap blocked"):
                invoke("const v=1;")


class ProtocolTests(unittest.TestCase):
    def test_initialize_and_tools(self):
        response = handle({"jsonrpc": "2.0", "id": 1, "method": "initialize", "params": {"protocolVersion": "2026-07-28"}})
        self.assertEqual(response["result"]["protocolVersion"], "2026-07-28")
        listed = handle({"jsonrpc": "2.0", "id": 2, "method": "tools/list"})
        names = {tool["name"] for tool in listed["result"]["tools"]}
        self.assertEqual(len(names), 47)
        for want in ("ego_lite_run", "ego_lite_network_start", "ego_lite_state", "ego_lite_output"):
            self.assertIn(want, names)

    def test_notification_has_no_response(self):
        self.assertIsNone(handle({"jsonrpc": "2.0", "method": "notifications/initialized"}))

    def test_argument_validation(self):
        with self.assertRaises(ValueError):
            server.call("ego_lite_navigate", {"url": "https://example.com", "command": "whoami"})
        with self.assertRaises(ValueError):
            server.call("ego_lite_fill", {"selector": "#q"})
        with self.assertRaises(ValueError):
            server.call("nope", {})

    def test_stdio_round_trip(self):
        messages = [
            {"jsonrpc": "2.0", "id": 1, "method": "initialize", "params": {"protocolVersion": "2025-06-18"}},
            {"jsonrpc": "2.0", "method": "notifications/initialized"},
            {"jsonrpc": "2.0", "id": 2, "method": "tools/list"},
        ]
        source = io.BytesIO("".join(json.dumps(item) + "\n" for item in messages).encode())
        output = io.StringIO()
        server.serve(source, output)
        responses = [json.loads(line) for line in output.getvalue().splitlines()]
        self.assertEqual([item["id"] for item in responses], [1, 2])


if __name__ == "__main__":
    unittest.main()
