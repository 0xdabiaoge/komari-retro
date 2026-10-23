import fs from 'node:fs';
import ts from 'typescript';
import { test } from 'node:test';
import assert from 'node:assert/strict';
const source = fs.readFileSync(new URL('../src/pages/terminal/terminalCommand.ts', import.meta.url), 'utf8');
const js = ts.transpileModule(source, { compilerOptions: { module: ts.ModuleKind.ES2022, target: ts.ScriptTarget.ES2022 } }).outputText;
const { directoryCommand } = await import('data:text/javascript;base64,' + Buffer.from(js).toString('base64'));
test('directory commands quote shell metacharacters and never append Enter', () => {
  assert.equal(directoryCommand('/var/log'), "cd -- '/var/log'");
  assert.equal(directoryCommand("/tmp/a'$(echo bad)"), "cd -- '/tmp/a'\\''$(echo bad)'");
  assert.equal(directoryCommand("C:/Users/O'Brien"), "Set-Location -LiteralPath 'C:/Users/O''Brien'");
  for (const path of ['/root', 'C:\\Temp', '/tmp/中文 space']) assert.ok(!/[\r\n]/.test(directoryCommand(path)));
  for (const path of ['', '/tmp/a\nb', '/tmp/a\rb', '/tmp/a\x1bb']) assert.throws(() => directoryCommand(path));
});
