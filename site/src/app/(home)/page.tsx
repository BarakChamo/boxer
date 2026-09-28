import Link from 'next/link';

// Static export under a Pages base path: next/link adds it to routes, but not to plain assets.
const base = process.env.BOXER_BASE_PATH ?? '';

const install = 'curl -fsSL https://raw.githubusercontent.com/BarakChamo/boxer/main/install.sh | sh';

const stats = [
  { value: '33 ms', label: 'per command in a running sandbox, within 4 ms of docker exec' },
  { value: '10 agents', label: 'and 6 orchestrators, each with its own setup page' },
  { value: '99.4%', label: 'of checks passed by live agents across 55 setups' },
];

const steps = [
  {
    title: 'Install it into your agent',
    body: 'boxer install claude-code writes hooks, an MCP server and a skill into your repo. Commit them once.',
  },
  {
    title: 'Keep working as you do',
    body: 'When a session starts, boxer boots that worktree’s sandbox, runs your setup and starts your dev server. npm, node, make and the other build tools run inside it.',
  },
  {
    title: 'Run as many as you want',
    body: 'Every new worktree gets its own sandbox, whoever created it: you, the agent, or an orchestrator. Deleted worktrees are cleaned up.',
  },
];

const highlights = [
  ['A sandbox per worktree', 'Created when an agent starts working there. Nothing to configure per task.'],
  ['No change to your agent', 'Hooks rewrite npm test into the sandbox before the shell runs it. Output and exit codes come back as usual.'],
  ['A URL per dev server', 'Every worktree serves port 3000 inside its sandbox, and gets its own https://branch.app.localhost outside.'],
  ['A real boundary', 'Each sandbox is a microVM with its own kernel. It sees the worktree, and reaches only the hosts you allow.'],
  ['Fast enough to forget', '33 ms per command. A Next.js session is ready in 7.6 s, against 6.8 s with no sandbox.'],
  ['Local and open source', 'Runs on your Mac or Linux machine. No account, no upload, Apache-2.0.'],
];

const compare = [
  ['', 'boxer', 'Agent’s built-in sandbox', 'A container per worktree', 'Cloud sandbox'],
  ['Separate ports and dev server per task', 'yes, automatic', 'no', 'yes, if you map ports', 'yes'],
  ['Own kernel per task', 'yes', 'no', 'no', 'yes'],
  ['Follows worktrees the agent creates', 'yes', 'n/a', 'no, start each one', 'no'],
  ['Code stays on your machine', 'yes', 'yes', 'yes', 'no'],
  ['Cost of one command', '33 ms', '~8 ms', '~29 ms', 'a network round trip'],
];

const tools = [
  ['Claude Code', 'setup/claude-code'],
  ['Codex', 'setup/codex'],
  ['Gemini CLI', 'setup/gemini-cli'],
  ['Copilot CLI', 'setup/copilot'],
  ['OpenCode', 'setup/opencode'],
  ['pi', 'setup/pi'],
  ['Grok', 'setup/grok'],
  ['Kimi Code', 'setup/kimi-and-dsh'],
  ['DSH', 'setup/kimi-and-dsh'],
  ['fx', 'setup/other-agents'],
  ['Conductor', 'orchestrators/conductor'],
  ['T3 Code', 'orchestrators/t3-code'],
  ['Paperclip', 'orchestrators/paperclip'],
  ['herdr', 'orchestrators/herdr'],
  ['Multica', 'orchestrators/multica'],
  ['OpenHands', 'orchestrators/openhands'],
];

export default function HomePage() {
  return (
    <main className="mx-auto flex w-full max-w-5xl flex-1 flex-col gap-24 px-6 py-20">
      <section className="flex flex-col items-center gap-6 text-center">
        <h1 className="text-4xl font-bold tracking-tight sm:text-6xl">
          Parallel coding agents,
          <br />
          each in its own sandbox
        </h1>
        <p className="text-fd-muted-foreground max-w-2xl text-lg sm:text-xl">
          boxer gives every git worktree its own microVM, dev server and URL. Install it into Claude
          Code, Codex or your orchestrator once, and every task your agents start is isolated from
          your machine and from each other.
        </p>
        <pre className="bg-fd-card border-fd-border max-w-full overflow-x-auto rounded-lg border px-4 py-3 text-left text-sm">
          <code>{`curl -sSL https://smolmachines.com/install.sh | bash
${install}`}</code>
        </pre>
        <div className="flex flex-wrap justify-center gap-3">
          <Link
            href="/docs/quickstart"
            className="bg-fd-primary text-fd-primary-foreground rounded-md px-5 py-2.5 text-sm font-medium"
          >
            Get started
          </Link>
          <Link href="/docs" className="border-fd-border rounded-md border px-5 py-2.5 text-sm font-medium">
            How it works
          </Link>
          <a
            href="https://github.com/BarakChamo/boxer"
            className="border-fd-border rounded-md border px-5 py-2.5 text-sm font-medium"
          >
            GitHub
          </a>
        </div>
      </section>

      <section>
        {/* eslint-disable-next-line @next/next/no-img-element */}
        <img
          src={`${base}/hero.svg`}
          alt="Three coding agents in three git worktrees, each running in its own boxer microVM with its own dev server URL"
          className="w-full rounded-2xl"
        />
      </section>

      <section className="grid gap-8 text-center sm:grid-cols-3">
        {stats.map((s) => (
          <div key={s.value} className="flex flex-col gap-1">
            <div className="text-4xl font-bold tracking-tight">{s.value}</div>
            <div className="text-fd-muted-foreground text-sm">{s.label}</div>
          </div>
        ))}
      </section>

      <section className="flex flex-col gap-8">
        <h2 className="text-center text-3xl font-semibold tracking-tight">Set it up once. Then forget about it.</h2>
        <div className="grid gap-8 sm:grid-cols-3">
          {steps.map((s, i) => (
            <div key={s.title} className="flex flex-col gap-2">
              <div className="text-fd-muted-foreground font-mono text-sm">0{i + 1}</div>
              <h3 className="text-lg font-semibold">{s.title}</h3>
              <p className="text-fd-muted-foreground text-sm">{s.body}</p>
            </div>
          ))}
        </div>
      </section>

      <section className="grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
        {highlights.map(([title, body]) => (
          <div key={title} className="border-fd-border bg-fd-card rounded-xl border p-5">
            <h3 className="mb-2 font-semibold">{title}</h3>
            <p className="text-fd-muted-foreground text-sm">{body}</p>
          </div>
        ))}
      </section>

      <section className="flex flex-col gap-6">
        <h2 className="text-center text-3xl font-semibold tracking-tight">Why not what you already have?</h2>
        <div className="border-fd-border overflow-x-auto rounded-xl border">
          <table className="w-full text-left text-sm">
            <tbody>
              {compare.map((row, r) => (
                <tr key={r} className={r === 0 ? 'bg-fd-card font-semibold' : 'border-fd-border border-t'}>
                  {row.map((cell, c) => (
                    <td
                      key={c}
                      className={`px-4 py-3 ${c === 1 && r > 0 ? 'font-medium' : ''} ${c > 1 && r > 0 ? 'text-fd-muted-foreground' : ''}`}
                    >
                      {cell}
                    </td>
                  ))}
                </tr>
              ))}
            </tbody>
          </table>
        </div>
        <Link href="/docs/comparison" className="text-center text-sm font-medium underline underline-offset-4">
          The full comparison
        </Link>
      </section>

      <section className="flex flex-col items-center gap-4 text-center">
        <h2 className="text-3xl font-semibold tracking-tight">Works with the tools you use</h2>
        <div className="flex max-w-3xl flex-wrap justify-center gap-2">
          {tools.map(([name, slug]) => (
            <Link
              key={name}
              href={`/docs/${slug}`}
              className="border-fd-border hover:bg-fd-accent rounded-full border px-3 py-1 text-sm transition-colors"
            >
              {name}
            </Link>
          ))}
        </div>
      </section>

      <section className="flex flex-col items-center gap-5 text-center">
        <h2 className="text-3xl font-semibold tracking-tight">Try it on your repo</h2>
        <pre className="bg-fd-card border-fd-border max-w-full overflow-x-auto rounded-lg border p-5 text-left text-sm">
          <code>{`curl -sSL https://smolmachines.com/install.sh | bash
${install}
boxer install claude-code
git add .claude .mcp.json && git commit -m "Run agent commands in boxer"`}</code>
        </pre>
        <Link
          href="/docs/quickstart"
          className="bg-fd-primary text-fd-primary-foreground rounded-md px-5 py-2.5 text-sm font-medium"
        >
          Read the quickstart
        </Link>
      </section>
    </main>
  );
}
