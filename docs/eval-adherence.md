# boxer eval report — tier adherence — 2026-09-18T18:57:52+08:00

Each entry is status · denials · spend. Verdict: `harness` only when every model fails the cell; otherwise a failure is model adherence.

| Cell | zai/glm-5.3-flash | Verdict |
| --- | --- | --- |
| claude-code/rewrite/plugin/worktree/multistep | pass · 0 · $0.0041 | pass |
| claude-code/tool/plugin/worktree/brief | pass · 0 · $0.0031 | pass |
| claude-code/tool/plugin/worktree/recovery | pass · 0 · $0.0038 | pass |
| codex/rewrite/project/worktree/multistep | pass · 0 · $0.0045 | pass |
| codex/tool/user/worktree/brief | pass · 0 · $0.0017 | pass |
| codex/tool/user/worktree/recovery | pass · 0 · $0.0008 | pass |
| opencode/rewrite/project/worktree/multistep | pass · 0 · $0.0079 | pass |
| opencode/tool/project/worktree/brief | pass · 0 · $0.0030 | pass |
| opencode/tool/project/worktree/recovery | pass · 0 · $0.0043 | pass |
| pi/rewrite/project/worktree/multistep | pass · 0 · $0.0014 | pass |
| pi/tool/project/worktree/brief | pass · 0 · $0.0007 | pass |
| pi/tool/project/worktree/recovery | pass · 0 · $0.0005 | pass |

| Model | Pass | Fail | Skip | Spend |
| --- | --- | --- | --- | --- |
| zai/glm-5.3-flash | 12 | 0 | 0 | $0.0358 |

## Findings per cell


_interrupted; the cell in flight is not listed. Run `boxer down --all` to reclaim its VM._
