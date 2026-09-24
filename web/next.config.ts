import path from "node:path";
import type { NextConfig } from "next";

const nextConfig: NextConfig = {
  // The repo root has its own package-lock.json; pin the app root so Next.js
  // doesn't guess the wrong workspace.
  turbopack: {
    root: path.join(__dirname),
  },
};

export default nextConfig;
