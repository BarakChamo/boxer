import type { NextConfig } from "next";

// A dev server in a sandbox is reached through a forwarded port or a portless name, so the
// browser's origin is not the one Next.js expects. Without this Next.js 15 logs a cross-origin
// warning for every chunk and says a future version will refuse them.
const nextConfig: NextConfig = {
  allowedDevOrigins: ["127.0.0.1", "localhost", "*.localhost"],
};

export default nextConfig;
