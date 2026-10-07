# Komari Retro

Komari Retro is an additional built-in public dashboard theme for Komari. It is based on the Komari-Next `1.4.19` source release (`914761b838d4b0f5c03a040debc0b79535be0669`) and keeps the upstream MIT license in `LICENSE`.

The theme keeps the node dashboard, detail charts, search, localization, appearance controls, and administrator-managed settings. The world map component, its data, map-only settings, and map-specific dependencies have been removed. The header, footer, theme metadata, favicon, and application manifest use Komari Retro branding.

## Build

```sh
npm ci
npm run build
```

The static export is written to `dist/`. The release workflow copies it into `web/public/retroTheme/`, where the Go server embeds it alongside the default theme. The theme ID is `retro`.

The release build supplies `NEXT_PUBLIC_THEME_VERSION` so the footer and embedded theme metadata show the same generated product version.

## Attribution

This derivative includes source code from [Komari-Next](https://github.com/tonyliuzj/komari-next), copyright Tony Liu, under the MIT License. See `LICENSE` for the complete notice.
