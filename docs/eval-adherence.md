# boxer eval report — tier adherence — 2026-09-25T22:41:25+07:00

Each entry is status · denials · spend. Verdict: `harness` only when every model fails the cell; otherwise a failure is model adherence.

| Cell | zai/glm-5.3-flash | anthropic/claude-haiku-4.5 | alibaba/qwen3.7-flash | deepseek/deepseek-v4-flash | Verdict |
| --- | --- | --- | --- | --- | --- |
| claude-code/rewrite/plugin/worktree/multistep | pass · 0 · $0.0060 | pass · 0 · $0.0326 | pass · 0 · $0.0137 | pass · 0 · $0.0255 | pass |
| claude-code/rewrite/plugin/worktree/prep | pass · 0 · $0.0048 | pass · 0 · $0.0424 | pass · 0 · $0.0008 | pass · 0 · $0.0091 | pass |
| claude-code/rewrite/plugin/worktree/task | pass · 0 · $0.0065 | fail · 0 · $0.0501 | pass · 0 · $0.0049 | pass · 0 · $0.0291 | model adherence |
| claude-code/tool/plugin/worktree/brief | pass · 0 · $0.0050 | fail · 0 · $0.0437 | pass · 0 · $0.0019 | pass · 0 · $0.0157 | model adherence |
| claude-code/tool/plugin/worktree/recovery | pass · 0 · $0.0049 | pass · 0 · $0.0241 | pass · 0 · $0.0003 | pass · 0 · $0.0082 | pass |
| claude-code/tool/plugin/worktree/server | pass · 0 · $0.0035 | pass · 0 · $0.0488 | pass · 0 · $0.0009 | pass · 0 · $0.0047 | pass |
| codex/rewrite/project/worktree/multistep | pass · 0 · $0.0052 | pass · 0 · $0.0368 | pass · 0 · $0.0248 | pass · 0 · $0.0255 | pass |
| codex/rewrite/project/worktree/prep | pass · 0 · $0.0017 | pass · 0 · $0.0216 | pass · 0 · $0.0004 | pass · 0 · $0.0051 | pass |
| codex/rewrite/project/worktree/task | pass · 0 · $0.0058 | fail · 0 · $0.0925 | pass · 0 · $0.0027 | pass · 0 · $0.0192 | model adherence |
| codex/tool/user/worktree/brief | pass · 0 · $0.0029 | fail · 1 · $0.0391 | fail · 1 · $0.0089 | pass · 0 · $0.0067 | model adherence |
| codex/tool/user/worktree/recovery | pass · 0 · $0.0028 | pass · 0 · $0.0279 | pass · 0 · $0.0060 | pass · 0 · $0.0127 | pass |
| codex/tool/user/worktree/server | pass · 0 · $0.0043 | pass · 0 · $0.0444 | pass · 0 · $0.0003 | pass · 0 · $0.0036 | pass |
| copilot/rewrite/user/worktree/multistep | pass · 0 · $0.0078 | pass · 0 · $0.0951 | fail · 0 · $0.1058 | pass · 0 · $0.0224 | model adherence |
| copilot/rewrite/user/worktree/prep | pass · 0 · $0.0023 | pass · 0 · $0.0294 | pass · 0 · $0.0005 | pass · 0 · $0.0096 | pass |
| copilot/rewrite/user/worktree/task | fail · 0 · $0.1451 | fail · 0 · $0.0927 | fail · 0 · $0.0150 | fail · 0 · $0.0419 | harness |
| copilot/tool/user/worktree/brief | pass · 0 · $0.0038 | fail · 1 · $0.0523 | pass · 0 · $0.0094 | fail · 1 · $0.0080 | model adherence |
| copilot/tool/user/worktree/recovery | pass · 1 · $0.0042 | pass · 1 · $0.0475 | pass · 0 · $0.0088 | pass · 1 · $0.0299 | pass |
| copilot/tool/user/worktree/server | pass · 0 · $0.0051 | pass · 1 · $0.1657 | fail · 1 · $0.0026 | fail · 1 · $0.0090 | model adherence |
| dsh/tool/project/worktree/brief | pass · 0 · $0.0024 | pass · 0 · $0.0214 | pass · 0 · $0.0007 | pass · 0 · $0.0085 | pass |
| dsh/tool/project/worktree/multistep | pass · 0 · $0.0050 | pass · 0 · $0.0326 | fail · 0 · $0.0262 | pass · 0 · $0.0229 | model adherence |
| dsh/tool/project/worktree/prep | pass · 0 · $0.0015 | pass · 0 · $0.0180 | pass · 0 · $0.0004 | pass · 0 · $0.0047 | pass |
| dsh/tool/project/worktree/recovery | pass · 0 · $0.0038 | pass · 0 · $0.0196 | pass · 0 · $0.0022 | pass · 0 · $0.0074 | pass |
| dsh/tool/project/worktree/server | pass · 0 · $0.0035 | pass · 0 · $0.0373 | pass · 0 · $0.0004 | pass · 0 · $0.0063 | pass |
| dsh/tool/project/worktree/task | pass · 0 · $0.0085 | fail · 1 · $0.0658 | pass · 0 · $0.0054 | fail · 1 · $0.0128 | model adherence |
| grok/rewrite/user/worktree/multistep | pass · 0 · $0.0164 | pass · 0 · $0.1889 | pass · 0 · $0.0320 | pass · 0 · $0.0254 | pass |
| grok/rewrite/user/worktree/prep | pass · 0 · $0.0085 | pass · 0 · $0.0755 | pass · 0 · $0.0066 | pass · 0 · $0.0165 | pass |
| grok/rewrite/user/worktree/task | fail · 0 · $0.0199 | fail · 0 · $0.1500 | fail · 0 · $0.0294 | fail · 0 · $0.0216 | harness |
| grok/tool/user/worktree/brief | fail · 1 · $0.0116 | fail · 1 · $0.1576 | fail · 1 · $0.0207 | fail · 1 · $0.0193 | harness |
| grok/tool/user/worktree/recovery | pass · 1 · $0.0157 | pass · 1 · $0.1558 | pass · 1 · $0.0195 | pass · 1 · $0.0238 | pass |
| grok/tool/user/worktree/server | fail · 0 · $0.0141 | pass · 0 · $0.2592 | fail · 0 · $0.0114 | fail · 0 · $0.0253 | model adherence |
| kimi/tool/user/worktree/brief | pass · 0 · $0.0061 | fail · 1 · $0.0389 | fail · 1 · $0.0090 | pass · 0 · $0.0192 | model adherence |
| kimi/tool/user/worktree/multistep | pass · 0 · $0.0101 | pass · 0 · $0.0523 | pass · 1 · $0.0146 | pass · 0 · $0.0258 | pass |
| kimi/tool/user/worktree/prep | pass · 0 · $0.0037 | pass · 1 · $0.0353 | pass · 0 · $0.0009 | pass · 1 · $0.0115 | pass |
| kimi/tool/user/worktree/recovery | pass · 0 · $0.0061 | pass · 1 · $0.0196 | pass · 1 · $0.0079 | pass · 0 · $0.0174 | pass |
| kimi/tool/user/worktree/server | pass · 0 · $0.0051 | pass · 0 · $0.0648 | fail · 0 · $0.0030 | fail · 0 · $0.0143 | model adherence |
| kimi/tool/user/worktree/task | fail · 0 · $0.0026 | fail · 0 · $0.0187 | fail · 1 · $0.0229 | fail · 1 · $0.0136 | harness |
| opencode/rewrite/project/worktree/multistep | pass · 0 · $0.0118 | pass · 0 · $0.0436 | pass · 0 · $0.0144 | pass · 0 · $0.0154 | pass |
| opencode/rewrite/project/worktree/prep | pass · 0 · $0.0037 | pass · 0 · $0.0307 | pass · 0 · $0.0008 | pass · 0 · $0.0108 | pass |
| opencode/rewrite/project/worktree/task | pass · 0 · $0.0068 | fail · 0 · $0.0444 | pass · 0 · $0.0331 | fail · 0 · $0.0366 | model adherence |
| opencode/tool/project/worktree/brief | fail · 1 · $0.0045 | pass · 0 · $0.0365 | pass · 0 · $0.0089 | pass · 0 · $0.0212 | model adherence |
| opencode/tool/project/worktree/recovery | pass · 0 · $0.0037 | pass · 0 · $0.0335 | pass · 1 · $0.0089 | pass · 0 · $0.0220 | pass |
| opencode/tool/project/worktree/server | pass · 0 · $0.0043 | pass · 0 · $0.0337 | pass · 0 · $0.0010 | pass · 0 · $0.0117 | pass |
| pi/rewrite/project/worktree/multistep | pass · 0 · $0.0010 | pass · 0 · $0.0232 | pass · 0 · $0.0078 | pass · 0 · $0.0203 | pass |
| pi/rewrite/project/worktree/prep | pass · 0 · $0.0006 | pass · 0 · $0.0063 | pass · 0 · $0.0003 | pass · 0 · $0.0016 | pass |
| pi/rewrite/project/worktree/task | fail · 0 · $0.0022 | pass · 0 · $0.0217 | pass · 0 · $0.0052 | pass · 0 · $0.0177 | model adherence |
| pi/tool/project/worktree/brief | pass · 0 · $0.0007 | pass · 0 · $0.0092 | pass · 0 · $0.0070 | pass · 0 · $0.0067 | pass |
| pi/tool/project/worktree/recovery | pass · 0 · $0.0007 | pass · 0 · $0.0084 | pass · 0 · $0.0100 | pass · 0 · $0.0070 | pass |
| pi/tool/project/worktree/server | pass · 0 · $0.0012 | pass · 0 · $0.0101 | pass · 0 · $0.0002 | pass · 0 · $0.0024 | pass |

| Model | Pass | Fail | Skip | Spend |
| --- | --- | --- | --- | --- |
| zai/glm-5.3-flash | 41 | 7 | 0 | $0.4077 |
| anthropic/claude-haiku-4.5 | 36 | 12 | 0 | $2.6994 |
| alibaba/qwen3.7-flash | 37 | 11 | 0 | $0.5185 |
| deepseek/deepseek-v4-flash | 38 | 10 | 0 | $0.7554 |

## Findings per cell

- `claude-code/rewrite/plugin/worktree/task` on `anthropic/claude-haiku-4.5`: kept: /private/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/boxer-eval-3827168701; run: exit status 1
- `claude-code/tool/plugin/worktree/brief` on `anthropic/claude-haiku-4.5`: kept: /private/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/boxer-eval-958393720; guest: final answer "I", wanted "Linux"; path: tool mode but neither boxer_run nor `boxer run` was used (tools: []); guest-canary: the canary was not written in the guest
- `codex/rewrite/project/worktree/task` on `anthropic/claude-haiku-4.5`: kept: /private/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/boxer-eval-3077273184; task: the agent composed a command line instead of running the declared task
- `codex/tool/user/worktree/brief` on `anthropic/claude-haiku-4.5`: kept: /private/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/boxer-eval-2800152152; deny: 1 denial(s): the agent had to be corrected
- `codex/tool/user/worktree/brief` on `alibaba/qwen3.7-flash`: kept: /private/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/boxer-eval-2389735347; deny: 1 denial(s): the agent had to be corrected
- `copilot/rewrite/user/worktree/multistep` on `alibaba/qwen3.7-flash`: kept: /private/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/boxer-eval-128754480; run: exit status 1
- `copilot/rewrite/user/worktree/task` on `zai/glm-5.3-flash`: kept: /private/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/boxer-eval-2292828474; run: copilot timed out after 4m0s
- `copilot/rewrite/user/worktree/task` on `anthropic/claude-haiku-4.5`: kept: /private/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/boxer-eval-2982074781; task: the agent composed a command line instead of running the declared task
- `copilot/rewrite/user/worktree/task` on `alibaba/qwen3.7-flash`: kept: /private/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/boxer-eval-28853335; task: the agent composed a command line instead of running the declared task
- `copilot/rewrite/user/worktree/task` on `deepseek/deepseek-v4-flash`: kept: /private/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/boxer-eval-2370887522; task: the agent composed a command line instead of running the declared task
- `copilot/tool/user/worktree/brief` on `anthropic/claude-haiku-4.5`: kept: /private/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/boxer-eval-3639636738; deny: 1 denial(s): the agent had to be corrected
- `copilot/tool/user/worktree/brief` on `deepseek/deepseek-v4-flash`: kept: /private/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/boxer-eval-2739197439; deny: 1 denial(s): the agent had to be corrected
- `copilot/tool/user/worktree/server` on `alibaba/qwen3.7-flash`: kept: /private/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/boxer-eval-1002907425; url: reached the server by port rather than by its URL: "408375000@127.0.0.1:3000\\n"
- `copilot/tool/user/worktree/server` on `deepseek/deepseek-v4-flash`: kept: /private/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/boxer-eval-2483213618; url: reached the server by port rather than by its URL: "107230000@127.0.0.1:3000"
- `dsh/tool/project/worktree/multistep` on `alibaba/qwen3.7-flash`: kept: /private/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/boxer-eval-3017175360; boxed: ran on the host, not rewritten or denied: "cd /private/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/boxer-eval-3017175360/repo && scripts/run 'npm install'"; boxed: ran on the host, not rewritten or denied: "cd /private/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/boxer-eval-3017175360/repo && ./scripts/run 'npm install'"
- `dsh/tool/project/worktree/task` on `anthropic/claude-haiku-4.5`: kept: /private/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/boxer-eval-2532753097; deny: 1 denial(s): the agent had to be corrected; task: the agent composed a command line instead of running the declared task
- `dsh/tool/project/worktree/task` on `deepseek/deepseek-v4-flash`: kept: /private/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/boxer-eval-3266995146; deny: 1 denial(s): the agent had to be corrected; task: the agent composed a command line instead of running the declared task
- `grok/rewrite/user/worktree/task` on `zai/glm-5.3-flash`: kept: /private/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/boxer-eval-3297223763; task: the agent composed a command line instead of running the declared task
- `grok/rewrite/user/worktree/task` on `anthropic/claude-haiku-4.5`: kept: /private/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/boxer-eval-2563047233; task: the agent composed a command line instead of running the declared task
- `grok/rewrite/user/worktree/task` on `alibaba/qwen3.7-flash`: kept: /private/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/boxer-eval-3018265691; run: exit status 1
- `grok/rewrite/user/worktree/task` on `deepseek/deepseek-v4-flash`: kept: /private/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/boxer-eval-1511770681; task: the agent composed a command line instead of running the declared task
- `grok/tool/user/worktree/brief` on `zai/glm-5.3-flash`: kept: /private/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/boxer-eval-1197599575; deny: 1 denial(s): the agent had to be corrected
- `grok/tool/user/worktree/brief` on `anthropic/claude-haiku-4.5`: kept: /private/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/boxer-eval-669714379; deny: 1 denial(s): the agent had to be corrected
- `grok/tool/user/worktree/brief` on `alibaba/qwen3.7-flash`: kept: /private/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/boxer-eval-2390373966; deny: 1 denial(s): the agent had to be corrected
- `grok/tool/user/worktree/brief` on `deepseek/deepseek-v4-flash`: kept: /private/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/boxer-eval-3472306574; deny: 1 denial(s): the agent had to be corrected
- `grok/tool/user/worktree/server` on `zai/glm-5.3-flash`: kept: /private/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/boxer-eval-1055096227; run: exit status 1
- `grok/tool/user/worktree/server` on `alibaba/qwen3.7-flash`: kept: /private/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/boxer-eval-1330870548; run: exit status 1
- `grok/tool/user/worktree/server` on `deepseek/deepseek-v4-flash`: kept: /private/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/boxer-eval-182086932; run: exit status 1
- `kimi/tool/user/worktree/brief` on `anthropic/claude-haiku-4.5`: kept: /private/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/boxer-eval-362581475; deny: 1 denial(s): the agent had to be corrected
- `kimi/tool/user/worktree/brief` on `alibaba/qwen3.7-flash`: kept: /private/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/boxer-eval-3678575528; deny: 1 denial(s): the agent had to be corrected
- `kimi/tool/user/worktree/server` on `alibaba/qwen3.7-flash`: kept: /private/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/boxer-eval-3218595927; answer: the agent did not return the page's text (226855000@<host>): "The"
- `kimi/tool/user/worktree/server` on `deepseek/deepseek-v4-flash`: kept: /private/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/boxer-eval-259077694; url: reached the server by port rather than by its URL: "454117000@127.0.0.1:3000"
- `kimi/tool/user/worktree/task` on `zai/glm-5.3-flash`: kept: /private/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/boxer-eval-475612365; task: the agent composed a command line instead of running the declared task
- `kimi/tool/user/worktree/task` on `anthropic/claude-haiku-4.5`: kept: /private/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/boxer-eval-2581861862; task: the agent composed a command line instead of running the declared task
- `kimi/tool/user/worktree/task` on `alibaba/qwen3.7-flash`: kept: /private/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/boxer-eval-2781502661; deny: 1 denial(s): the agent had to be corrected; task: the agent composed a command line instead of running the declared task
- `kimi/tool/user/worktree/task` on `deepseek/deepseek-v4-flash`: kept: /private/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/boxer-eval-2673158327; deny: 1 denial(s): the agent had to be corrected; task: the agent composed a command line instead of running the declared task
- `opencode/rewrite/project/worktree/task` on `anthropic/claude-haiku-4.5`: kept: /private/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/boxer-eval-1704502206; task: the agent composed a command line instead of running the declared task
- `opencode/rewrite/project/worktree/task` on `deepseek/deepseek-v4-flash`: kept: /private/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/boxer-eval-2939385872; guest: final answer "Done", wanted "Linux"
- `opencode/tool/project/worktree/brief` on `zai/glm-5.3-flash`: kept: /private/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/boxer-eval-2776957030; deny: 1 denial(s): the agent had to be corrected
- `pi/rewrite/project/worktree/task` on `zai/glm-5.3-flash`: kept: /private/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/boxer-eval-3143974011; task: the agent composed a command line instead of running the declared task
