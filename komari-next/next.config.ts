import type { NextConfig } from "next";

const nextConfig: NextConfig = {
  distDir: 'dist',
  output: 'export',
  productionBrowserSourceMaps: false,
  images: {
    unoptimized: true,
  },
  ...(process.env.NODE_ENV === 'development'
    ? {
        // API rewrites are only for local development. Production is served by the Go host.
        async rewrites() {
          const apiTarget = process.env.NEXT_PUBLIC_API_TARGET || 'http://127.0.0.1:25774';
          return [
            { source: '/api/:path*', destination: `${apiTarget}/api/:path*` },
            { source: '/themes/:path*', destination: `${apiTarget}/themes/:path*` },
          ];
        },
      }
    : {}),
};

export default nextConfig;
