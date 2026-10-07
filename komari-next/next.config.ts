import type { NextConfig } from "next";

const nextConfig: NextConfig = {
  distDir: 'dist',
  output: 'export',
  // The Go host also serves the Retro theme below /themes/retro/dist.
  // Keeping Next's immutable chunks on that path avoids colliding with the
  // root SPA route and stale CDN entries previously cached as HTML at /_next.
  assetPrefix: process.env.NODE_ENV === 'development' ? undefined : '/themes/retro/dist',
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
