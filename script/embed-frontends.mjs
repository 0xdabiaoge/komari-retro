import { cp, mkdir, rm, access } from 'node:fs/promises';
import { resolve, join } from 'node:path';
const root = resolve(import.meta.dirname, '..');
for (const [project, theme, guest] of [['komari-web', 'defaultTheme', 'default'], ['komari-next', 'retroTheme', 'retro']]) {
  for (const [source, relative] of [['dist', `web/public/${theme}/dist`], ['dist-guest', `web/public/guest/${guest}/dist`]]) {
    const from = join(root, project, source), to = join(root, relative);
    await access(join(from, 'index.html'));
    if (!to.startsWith(join(root, 'web/public') + '/'.replace('/', process.platform === 'win32' ? '\\' : '/'))) throw new Error('Unexpected output');
    await rm(to, { recursive: true, force: true });
    await mkdir(to, { recursive: true });
    await cp(from, to, { recursive: true });
    console.log('Embedded ' + relative);
  }
  await cp(join(root,project,'komari-theme.json'),join(root,'web/public',theme,'komari-theme.json'));
}
