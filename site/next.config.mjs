import { createMDX } from 'fumadocs-mdx/next';

const withMDX = createMDX();

// GitHub Pages serves a project site from a subdirectory, so the build needs to know its prefix.
// The workflow sets BOXER_BASE_PATH=/boxer; a custom domain sets it to "" and a local `next dev`
// leaves it unset, which is the same thing.
const basePath = process.env.BOXER_BASE_PATH ?? '';

/** @type {import('next').NextConfig} */
const config = {
  reactStrictMode: true,
  // Pages serves files, not a Node server: everything has to be prerendered. The only route that
  // was not is search, which becomes a static index below.
  output: 'export',
  basePath,
  // Pages has no image optimiser behind it.
  images: { unoptimized: true },
  // Without this, /docs/start/install is a file with no extension and Pages serves it as a
  // download rather than a page.
  trailingSlash: true,
};

export default withMDX(config);
