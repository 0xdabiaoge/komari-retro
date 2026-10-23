import type { NextConfig } from "next";
import path from "node:path";

const guest = process.env.KOMARI_AUDIENCE === "guest";

const nextConfig: NextConfig = {
  distDir: guest ? 'dist-guest' : 'dist',
  output: 'export',
  // Separate resource namespaces let the Go host enforce the audience before
  // serving chunks, without collisions with root routes or old /_next caches.
  assetPrefix: process.env.NODE_ENV === 'development' ? undefined : (guest ? '/share-assets/retro' : '/private-assets/retro'),
  webpack(config, { webpack }) {
    if (guest) {
      const replacements: Record<string, string> = {
        'components/Footer': 'Footer.tsx',
        'components/NavBar': 'NavBar.tsx', 'hooks/useThemeWriter': 'useThemeWriter.ts',
        'components/PWAInstallPrompt': 'PWA.tsx', 'components/PWAUpdatePrompt': 'PWA.tsx',
      };
      for (const [name, replacement] of Object.entries(replacements)) {
        const target = path.resolve('src/guest', replacement);
        config.resolve.alias['@/' + name] = target;
        config.resolve.alias[path.resolve('src', name + '.tsx')] = target;
        config.resolve.alias[path.resolve('src', name + '.ts')] = target;
        const expression = new RegExp('(?:^@/|[/\\\\])' + name.replaceAll('/', '[/\\\\]') + '(?:\\.tsx?)?$');
        config.plugins.push(new webpack.NormalModuleReplacementPlugin(expression, (resource: { request: string }) => { resource.request = target; }));
      }
      config.module.rules.push({
        test: /[\\/]src[\\/]i18n[\\/]locales[\\/].+\.json$/,
        type: 'javascript/auto',
        use: [path.resolve('../script/guest-locales-loader.cjs')],
      });
      config.module.rules.push({ test: /[\\/]src[\\/].+\.tsx?$/, enforce: 'pre', use: [path.resolve('../script/viewer-source-loader.cjs')] });
      config.plugins.push(new webpack.NormalModuleReplacementPlugin(/(?:^|[\\/])(?:Login|AccountContext|loginRedirect)(?:\.tsx?)?$|[\\/]components[\\/]admin[\\/]|^@\/lib\/api$/, (resource: { request: string }) => {
        throw new Error('Privileged module reached viewer build: ' + resource.request);
      }));
    }
    return config;
  },
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
