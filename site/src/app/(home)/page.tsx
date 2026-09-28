import Link from 'next/link';

// Real output from two worktrees of one app, each serving port 3000 from its own sandbox.
const demo = `# Claude Code, in the fix-ui worktree, runs:   npm test
# boxer's hook runs it in fix-ui's sandbox:     boxer run -c 'npm test'

$ boxer ls        # two agent sessions, two worktrees, two sandboxes
NAME         STATE    BRANCH    SERVES
mild-lynx    running  add-auth  https://add-auth.myapp.localhost:1355
olive-comet  running  fix-ui    https://fix-ui.myapp.localhost:1355`;

const steps = [
  {
    title: '1. Install it into your agent',
    body: 'boxer install claude-code writes hooks, an MCP server, a skill and a subagent into your repository. Commit them once.',
  },
  {
    title: '2. Work as usual',
    body: 'When a session starts, boxer creates that worktree\'s sandbox and starts your dev server. Build commands like npm, node and make run inside it.',
  },
  {
    title: '3. Run as many as you like',
    body: 'Every new worktree gets its own sandbox, whoever created it: you, the agent, or an orchestrator. Deleted worktrees are cleaned up.',
  },
];

const features = [
  {
    title: 'One sandbox per worktree',
    body: 'Each git worktree gets its own microVM when an agent session starts in it. Delete the worktree and boxer cleans the sandbox up.',
  },
  {
    title: 'Your agent does not change',
    body: 'A hook rewrites npm test to boxer run before the shell sees it. Output and exit codes come back unchanged.',
  },
  {
    title: 'Each dev server gets its own URL',
    body: 'Every worktree runs its dev server on the same port inside its sandbox, and gets its own URL outside.',
  },
  {
    title: 'Isolated from your machine',
    body: 'System packages, global tools and running processes stay in the sandbox. It sees only the worktree. On smolvm it reaches only the hosts you allow.',
  },
];

const agents = [
  ['Claude Code', 'claude-code'],
  ['Codex', 'codex'],
  ['Gemini CLI', 'gemini-cli'],
  ['Copilot CLI', 'copilot'],
  ['OpenCode', 'opencode'],
  ['pi', 'pi'],
  ['Grok', 'grok'],
  ['Kimi Code', 'kimi-and-dsh'],
];

const orchestrators = [
  ['Conductor', 'conductor'],
  ['T3 Code', 't3-code'],
  ['Paperclip', 'paperclip'],
  ['herdr', 'herdr'],
  ['Multica', 'multica'],
  ['OpenHands', 'openhands'],
];

function Pill({ href, children }: { href: string; children: React.ReactNode }) {
  return (
    <Link
      href={href}
      className="border-fd-border hover:bg-fd-accent rounded-full border px-3 py-1 text-sm transition-colors"
    >
      {children}
    </Link>
  );
}

export default function HomePage() {
  return (
    <main className="mx-auto flex w-full max-w-5xl flex-1 flex-col gap-20 px-6 py-20">
      <section className="flex flex-col items-center gap-6 text-center">
        <h1 className="text-4xl font-bold tracking-tight sm:text-5xl">
          Run coding agents in parallel
          <br />
          without them colliding
        </h1>
        <p className="text-fd-muted-foreground max-w-2xl text-lg">
          Install boxer into your coding agent once. From then on, every worktree your agents work
          in gets its own sandbox, with its own dev server and its own URL. You keep working as
          before.
        </p>
        <div className="flex flex-wrap justify-center gap-3">
          <Link
            href="/docs/quickstart"
            className="bg-fd-primary text-fd-primary-foreground rounded-md px-5 py-2.5 text-sm font-medium"
          >
            Quickstart
          </Link>
          <Link href="/docs" className="border-fd-border rounded-md border px-5 py-2.5 text-sm font-medium">
            What is boxer
          </Link>
          <a
            href="https://github.com/BarakChamo/boxer"
            className="border-fd-border rounded-md border px-5 py-2.5 text-sm font-medium"
          >
            GitHub
          </a>
        </div>
      </section>

      <section className="flex flex-col gap-3">
        <pre className="bg-fd-card border-fd-border overflow-x-auto rounded-xl border p-5 text-left text-sm leading-relaxed">
          <code>{demo}</code>
        </pre>
        <p className="text-fd-muted-foreground text-center text-sm">
          Each session created its own sandbox. Both dev servers run on port 3000 without clashing.
        </p>
      </section>

      <section className="flex flex-col gap-6">
        <h2 className="text-center text-2xl font-semibold">How it works</h2>
        <div className="grid gap-4 sm:grid-cols-3">
          {steps.map((s) => (
            <div key={s.title} className="flex flex-col gap-2">
              <h3 className="font-semibold">{s.title}</h3>
              <p className="text-fd-muted-foreground text-sm">{s.body}</p>
            </div>
          ))}
        </div>
      </section>

      <section className="grid gap-4 sm:grid-cols-2">
        {features.map((f) => (
          <div key={f.title} className="border-fd-border bg-fd-card rounded-xl border p-5">
            <h2 className="mb-2 font-semibold">{f.title}</h2>
            <p className="text-fd-muted-foreground text-sm">{f.body}</p>
          </div>
        ))}
      </section>

      <section className="grid gap-10 sm:grid-cols-2">
        <div className="flex flex-col gap-3">
          <h2 className="text-lg font-semibold">Works with your agent</h2>
          <div className="flex flex-wrap gap-2">
            {agents.map(([name, slug]) => (
              <Pill key={slug} href={`/docs/setup/${slug}`}>
                {name}
              </Pill>
            ))}
          </div>
        </div>
        <div className="flex flex-col gap-3">
          <h2 className="text-lg font-semibold">And your orchestrator</h2>
          <div className="flex flex-wrap gap-2">
            {orchestrators.map(([name, slug]) => (
              <Pill key={slug} href={`/docs/orchestrators/${slug}`}>
                {name}
              </Pill>
            ))}
          </div>
        </div>
      </section>

      <section className="flex flex-col items-center gap-3 text-center">
        <h2 className="text-lg font-semibold">One command per agent, then commit</h2>
        <pre className="bg-fd-card border-fd-border overflow-x-auto rounded-xl border p-5 text-left text-sm">
          <code>{`boxer install claude-code
git add .claude .mcp.json && git commit -m "Run agent commands in boxer"`}</code>
        </pre>
        <Link href="/docs/quickstart" className="text-sm font-medium underline underline-offset-4">
          Full quickstart
        </Link>
      </section>
    </main>
  );
}
