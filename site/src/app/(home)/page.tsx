import Link from 'next/link';

const install = `curl -sSL https://smolmachines.com/install.sh | bash
curl -fsSL https://raw.githubusercontent.com/BarakChamo/boxer/main/install.sh | sh

cd your-repository
boxer install all
boxer run -c 'uname -a'`;

export default function HomePage() {
  return (
    <main className="flex flex-1 flex-col items-center justify-center gap-10 px-6 py-20">
      <div className="max-w-2xl text-center">
        <h1 className="mb-4 text-4xl font-bold tracking-tight">boxer</h1>
        <p className="text-fd-muted-foreground text-lg">
          Coding agents run shell commands. boxer runs them in a microVM instead — one per git
          worktree, started automatically, with your worktree mounted at the same path.
        </p>
        <p className="text-fd-muted-foreground mt-4">
          The agent is not told. It types <code className="text-fd-foreground">npm test</code>, the
          command runs in the VM, and the output comes back looking exactly as it would have.
        </p>
      </div>

      <pre className="bg-fd-secondary/50 border-fd-border overflow-x-auto rounded-lg border p-5 text-left text-sm">
        <code>{install}</code>
      </pre>

      <div className="flex flex-wrap items-center justify-center gap-4">
        <Link
          href="/docs"
          className="bg-fd-primary text-fd-primary-foreground rounded-md px-5 py-2.5 text-sm font-medium"
        >
          Read the docs
        </Link>
        <Link
          href="/docs/evals/results"
          className="border-fd-border rounded-md border px-5 py-2.5 text-sm font-medium"
        >
          What is measured
        </Link>
        <a
          href="https://github.com/BarakChamo/boxer"
          className="border-fd-border rounded-md border px-5 py-2.5 text-sm font-medium"
        >
          GitHub
        </a>
      </div>
    </main>
  );
}
