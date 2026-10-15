import { spawnSync } from 'node:child_process';
const audience = process.argv[2];
if (!['owner', 'guest'].includes(audience)) throw new Error('Unknown audience');
const env = { ...process.env, KOMARI_AUDIENCE: audience };
for (const [file, args] of [
  ['./script/generate-brand-assets.mjs', []],
  ['./node_modules/next/dist/bin/next', ['build', '--webpack']],
  ['./script/protect-komari-placeholders.mjs', []],
  ['./script/verify-theme-export.mjs', []],
  ...(audience === 'guest' ? [['../script/verify-guest-build.mjs', ['retro']]] : []),
]) {
  const result = spawnSync(process.execPath, [file, ...args], { env, stdio: 'inherit' });
  if (result.status !== 0) process.exit(result.status || 1);
}
