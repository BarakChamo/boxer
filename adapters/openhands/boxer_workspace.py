"""OpenHands workspace that executes every command through boxer.

OpenHands (the software-agent-sdk) runs agent actions against a Workspace. LocalWorkspace uses
subprocess on the host; this subclass keeps the host filesystem as the workspace but routes
execute_command through `boxer run`, so the agent's commands land in the smolvm guest for the
worktree while file operations stay local. Pass it to Conversation like any workspace:

    from boxer_workspace import BoxerWorkspace
    conversation = Conversation(agent=agent, workspace=BoxerWorkspace(working_dir="/repo"))

Requirements: the `boxer` binary on PATH, the repository configured (boxer doctor), and
OpenHands' Process sandbox (RUNTIME=process) or the SDK used directly — the Docker and Remote
sandboxes already isolate execution and would only nest boxer inside them.
"""

from __future__ import annotations

import shlex
import subprocess
import time
from pathlib import Path

from openhands.sdk.workspace.local import LocalWorkspace
from openhands.sdk.workspace.models import CommandResult


class BoxerWorkspace(LocalWorkspace):
    """LocalWorkspace whose shell runs inside the boxer sandbox."""

    def execute_command(
        self,
        command: str,
        cwd: str | Path | None = None,
        timeout: float = 30.0,
    ) -> CommandResult:
        run_cwd = str(cwd) if cwd is not None else str(self.working_dir)
        argv = ["boxer", "run", "-c", command]
        started = time.monotonic()
        try:
            proc = subprocess.run(
                argv,
                cwd=run_cwd,
                capture_output=True,
                text=True,
                timeout=timeout,
                check=False,
            )
            return CommandResult(
                command=command,
                exit_code=proc.returncode,
                stdout=proc.stdout,
                stderr=proc.stderr,
                timeout_occurred=False,
            )
        except subprocess.TimeoutExpired as exc:
            return CommandResult(
                command=command,
                exit_code=-1,
                stdout=(exc.stdout or "") if isinstance(exc.stdout, str) else "",
                stderr=((exc.stderr or "") if isinstance(exc.stderr, str) else "")
                + f"\nboxer: timed out after {timeout:.0f}s ({time.monotonic() - started:.0f}s elapsed): {shlex.join(argv)}",
                timeout_occurred=True,
            )
