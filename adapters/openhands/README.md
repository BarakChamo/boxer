# boxer workspace for OpenHands

`BoxerWorkspace` is an OpenHands SDK `LocalWorkspace` whose commands run in the boxer sandbox.

```python
from boxer_workspace import BoxerWorkspace
from openhands.sdk import Conversation

conversation = Conversation(agent=agent, workspace=BoxerWorkspace(working_dir="/path/to/repo"))
```

- Files: host filesystem, unchanged (`file_upload`, `file_download`, and the agent's file tools).
- Commands: `boxer run -c <command>` from the requested `cwd`, so the guest working directory
  follows the host one; exit codes, stdout, stderr, and timeouts propagate.
- Requirements: `boxer` on `PATH`, smolvm installed, the repository configured (`boxer doctor`).
  Use with the SDK or `RUNTIME=process`; the Docker and Remote sandboxes already isolate.

Test without OpenHands installed: `python3 test_boxer_workspace.py` (stubs the SDK, uses a fake
`boxer`).
