# boxer for OpenHands

Two pieces, both thin. The one that matters is the shell.

## The terminal's shell is boxer-bash (recommended)

OpenHands' terminal tool spawns its own interactive shell in a PTY; a workspace's
`execute_command` is not on that path. So give the terminal a shell that lives in the sandbox:

```sh
boxer shim install --shell            # writes ~/.local/share/boxer/shims/boxer-bash
```

```python
from openhands.sdk import Agent, Conversation, Tool
from openhands.tools.terminal import TerminalTool

agent = Agent(llm=llm, tools=[Tool(name=TerminalTool.name,
    params={"shell_path": "/Users/you/.local/share/boxer/shims/boxer-bash", "terminal_type": "subprocess"})])
conversation = Conversation(agent=agent, workspace="/path/to/repo")
```

`boxer-bash` is `exec boxer run -- bash "$@"`: the PTY, the prompt markers OpenHands relies on,
and every command, compound lines included, run in the guest for the worktree. Files stay on the
host. Verified live with the SDK 1.49 (`boxer-eval --tier t2 --cell openhands`): the agent ran
`uname -a` in the guest, nothing touched the host.

The same wrapper works for any harness that lets you name its shell binary; boxer keeps no
per-harness code for it.

## `BoxerWorkspace`

A `LocalWorkspace` whose `execute_command` runs `boxer run -c <command>`. Useful when your own code
calls `workspace.execute_command`; it does not affect the agent's terminal tool. Test without
OpenHands installed: `python3 test_boxer_workspace.py`.

## Requirements

`boxer` on `PATH`, smolvm installed, the repository configured (`boxer doctor`), an image with
bash (the default `node` image has it; alpine does not). Use with the SDK or `RUNTIME=process`; the
Docker and Remote sandboxes already isolate execution.
