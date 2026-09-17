"""Runs without OpenHands installed: stubs the two SDK modules, then checks that commands are
routed through `boxer run -c` with the right cwd and that exit codes and timeouts propagate."""

import os
import stat
import subprocess
import sys
import tempfile
import types
import unittest
from pathlib import Path

# --- stub the SDK surface this adapter touches ------------------------------------------------
sdk = types.ModuleType("openhands.sdk")
ws = types.ModuleType("openhands.sdk.workspace")
local = types.ModuleType("openhands.sdk.workspace.local")
models = types.ModuleType("openhands.sdk.workspace.models")


class CommandResult:
    def __init__(self, command, exit_code, stdout, stderr, timeout_occurred):
        self.command, self.exit_code, self.stdout, self.stderr, self.timeout_occurred = (
            command, exit_code, stdout, stderr, timeout_occurred,
        )


class LocalWorkspace:
    def __init__(self, *, working_dir):
        self.working_dir = str(working_dir)


models.CommandResult = CommandResult
local.LocalWorkspace = LocalWorkspace
for name, mod in {"openhands": types.ModuleType("openhands"), "openhands.sdk": sdk,
                  "openhands.sdk.workspace": ws, "openhands.sdk.workspace.local": local,
                  "openhands.sdk.workspace.models": models}.items():
    sys.modules[name] = mod

sys.path.insert(0, str(Path(__file__).parent))
from boxer_workspace import BoxerWorkspace  # noqa: E402


class BoxerWorkspaceTest(unittest.TestCase):
    def setUp(self):
        self.bindir = tempfile.mkdtemp()
        fake = Path(self.bindir) / "boxer"
        # Records argv and cwd, then runs the shell line locally so exit codes are real.
        fake.write_text('#!/bin/sh\necho "argv: $*" >&2\necho "cwd: $PWD" >&2\n[ "$1" = run ] && [ "$2" = -c ] && exec sh -c "$3"\nexit 99\n')
        fake.chmod(fake.stat().st_mode | stat.S_IEXEC)
        os.environ["PATH"] = self.bindir + os.pathsep + os.environ["PATH"]

    def test_routes_through_boxer_run(self):
        repo = tempfile.mkdtemp()
        w = BoxerWorkspace(working_dir=repo)
        r = w.execute_command("echo hi; exit 3")
        self.assertEqual(r.exit_code, 3)
        self.assertEqual(r.stdout.strip(), "hi")
        self.assertIn("argv: run -c echo hi; exit 3", r.stderr)
        self.assertIn(f"cwd: {os.path.realpath(repo)}", r.stderr)
        self.assertFalse(r.timeout_occurred)

    def test_cwd_override(self):
        repo = tempfile.mkdtemp()
        sub = Path(repo) / "sub"
        sub.mkdir()
        r = BoxerWorkspace(working_dir=repo).execute_command("true", cwd=sub)
        self.assertIn(f"cwd: {os.path.realpath(sub)}", r.stderr)

    def test_timeout(self):
        r = BoxerWorkspace(working_dir=tempfile.mkdtemp()).execute_command("sleep 5", timeout=0.5)
        self.assertTrue(r.timeout_occurred)
        self.assertEqual(r.exit_code, -1)
        self.assertIn("timed out", r.stderr)


if __name__ == "__main__":
    unittest.main()
