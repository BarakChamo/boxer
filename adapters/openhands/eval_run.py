"""Eval runner for BoxerWorkspace against the real OpenHands SDK.

Without --live: calls BoxerWorkspace.execute_command directly (no model), prints the command's
stdout as the answer. With --live: runs a Conversation through the Vercel AI Gateway (AI_GATEWAY_API_KEY)
and the SDK's terminal tool, which executes through the same workspace. The last stdout line is
one JSON object: {"answer": str, "tools": [str], "skip": str}.
"""

from __future__ import annotations

import argparse
import json
import os
import sys

sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))
from boxer_workspace import BoxerWorkspace  # noqa: E402


def report(answer: str = "", tools: list[str] | None = None, skip: str = "") -> None:
    print(json.dumps({"answer": answer, "tools": tools or [], "skip": skip}))


def main() -> int:
    ap = argparse.ArgumentParser()
    ap.add_argument("--repo", required=True)
    ap.add_argument("--command", required=True)
    ap.add_argument("--live", action="store_true")
    ap.add_argument("--prompt", default="")
    ap.add_argument("--model", default=os.environ.get("OPENHANDS_EVAL_MODEL", "openai/openai/gpt-5-mini"))
    ap.add_argument("--base-url", default=os.environ.get("OPENHANDS_EVAL_BASE_URL", "https://ai-gateway.vercel.sh/coding-agent/v1"))
    args = ap.parse_args()

    workspace = BoxerWorkspace(working_dir=args.repo)
    if not args.live:
        result = workspace.execute_command(args.command, timeout=300)
        if result.exit_code != 0:
            print(result.stderr, file=sys.stderr)
        report(answer=result.stdout, tools=["execute_command"])
        return 0

    key = os.environ.get("AI_GATEWAY_API_KEY")
    if not key:
        report(skip="AI_GATEWAY_API_KEY is not set")
        return 0
    try:
        from openhands.sdk import LLM, Agent, Conversation, Tool
        from openhands.tools.terminal import TerminalTool  # noqa: F401  (registers the tool)
    except ImportError as exc:
        report(skip=f"OpenHands tools package missing in the virtualenv ({exc}); pip install openhands-tools")
        return 0

    tools_used: list[str] = []
    final: list[str] = []

    def on_event(event) -> None:
        name = type(event).__name__
        if name == "ActionEvent":
            tools_used.append(getattr(event, "tool_name", name))
        if name == "MessageEvent" and getattr(getattr(event, "llm_message", None), "role", "") == "assistant":
            for block in getattr(event.llm_message, "content", []):
                text = getattr(block, "text", None)
                if text:
                    final.append(text)

    # LiteLLM: the "openai/" prefix picks the OpenAI protocol; the rest is the gateway model id.
    llm = LLM(model=args.model, api_key=key, base_url=args.base_url, usage_id="boxer-eval")
    agent = Agent(llm=llm, tools=[Tool(name=TerminalTool.name)])
    conversation = Conversation(agent=agent, workspace=workspace, callbacks=[on_event], visualizer=None)
    conversation.send_message(args.prompt)
    try:
        conversation.run()
    except Exception as exc:  # provider errors are reported, never crash the cell
        text = str(exc)
        if any(k in text.lower() for k in ("quota", "rate limit", "credit", "billing")):
            report(skip=f"provider stopped the turn: {text[:200]}")
            return 0
        raise
    report(answer=(final[-1] if final else ""), tools=tools_used)
    return 0


if __name__ == "__main__":
    sys.exit(main())
