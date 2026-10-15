import { readFile, readdir, writeFile, rename } from 'node:fs/promises';
import { resolve, join } from 'node:path';
import { createRequire } from 'node:module';
const require = createRequire(import.meta.url);
const postcss = require('../komari-web/node_modules/postcss');
const theme = process.argv[2];
if (!['default', 'retro'].includes(theme)) throw new Error('Unknown theme');
const root = resolve(import.meta.dirname, '..', theme === 'default' ? 'komari-web' : 'komari-next', 'dist-guest');
if (theme === 'default') await rename(join(root, 'guest.html'), join(root, 'index.html')).catch(error => { if (error.code !== 'ENOENT') throw error; });
const blocked = /\/api\/(?:login|logout|me|admin)\b|\/admin(?:[/?"'`]|$)|session_token|admin_entrance_token|logged_in|LoginDialog|loginRedirect|password_login|\boauth\b|\boidc\b|登录|登陆|登入|登出|密码|密碼|ログイン|パスワード|\bsign[ -]?in\b|\blog[ -]?in\b|\blog[ -]?out\b/i;
let files = 0;
async function visit(folder) {
  for (const entry of await readdir(folder, { withFileTypes: true })) {
    const file = join(folder, entry.name);
    if (entry.isDirectory()) { await visit(file); continue; }
    if (entry.name.endsWith('.map')) throw new Error(`Guest source map exported: ${file}`);
    if (!/\.(?:js|css|html|json|txt|svg)$/.test(entry.name)) continue;
    let text = await readFile(file, 'utf8');
    if (entry.name.endsWith('.css')) {
      const css = postcss.parse(text);
      css.walkRules(rule => { if (/login|log-in|admin|account|onboarding|terminal|password/i.test(rule.selector)) rule.remove(); });
      text = css.toString(); await writeFile(file, text);
    }
    const decoded = text.replace(/\\u([\da-f]{4})/gi, (_, value) => String.fromCharCode(parseInt(value, 16))).replace(/\\x([\da-f]{2})/gi, (_, value) => String.fromCharCode(parseInt(value, 16)));
    const match = decoded.match(blocked);
    if (match) throw new Error(`Privileged content in guest artifact ${file}: ${match[0]}`);
    if (entry.name.endsWith('.svg') && /<script\b|\bonload\s*=/i.test(text)) throw new Error(`Executable SVG in viewer export: ${file}`);
    files++;
  }
}
await visit(root);
console.log(`Verified ${theme} visitor export: ${files} text resources, no application sign-in code or source maps.`);
