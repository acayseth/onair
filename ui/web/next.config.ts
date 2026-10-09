import type { NextConfig } from "next";

const uid = new Date().getTime();

const nextConfig: NextConfig = {
  poweredByHeader: false,
  logging: {
    browserToTerminal: true,
    fetches: {
      fullUrl: true,
      hmrRefreshes: true,
    },
    serverFunctions: true,
  },
  images: {
    unoptimized: true,
  },
  generateBuildId: () => `build@${uid}`,
  deploymentId: `deploy@${uid}`,
  output: "standalone",
  cacheComponents: true,
  partialPrefetching: true,
  reactCompiler: true,
  turbopack: {
    rules: {
      "*.css": {
        loaders: ["@tailwindcss/turbopack"],
        as: "*.css",
      },
    },
  },
};

export default nextConfig;
