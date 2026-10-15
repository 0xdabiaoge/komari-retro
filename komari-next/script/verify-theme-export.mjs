import { access, readFile } from 'node:fs/promises';
import { join } from 'node:path';

const outputDir = join(process.cwd(), process.env.KOMARI_AUDIENCE === 'guest' ? 'dist-guest' : 'dist');
const indexPath = join(outputDir, 'index.html');
const html = await readFile(indexPath, 'utf8');
const references = [...html.matchAll(/(?:src|href)="([^"]*_next\/static\/[^"]+)"/g)]
  .map((match) => match[1]);
const rootReferences = references.filter((reference) => reference.startsWith('/_next/static/'));
const assetRoot = process.env.KOMARI_AUDIENCE === 'guest' ? '/share-assets/retro/' : '/private-assets/retro/';
const prefix = assetRoot + '_next/static/';
const prefixedReferences = references.filter((reference) => reference.startsWith(prefix));

if (rootReferences.length > 0) {
  throw new Error(`Theme export still uses root-level Next.js asset paths: ${rootReferences.join(', ')}`);
}

if (prefixedReferences.length === 0) {
  throw new Error(`Theme export has no assets under ${prefix}`);
}

for (const reference of prefixedReferences) {
  const assetPath = join(outputDir, reference.slice(assetRoot.length));
  await access(assetPath);
}

console.log(`Verified ${prefixedReferences.length} namespaced Next.js assets in the theme export.`);
