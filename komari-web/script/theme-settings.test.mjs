import { readFileSync } from 'node:fs';
import ts from 'typescript';
import { test } from 'node:test';
import assert from 'node:assert/strict';
const source = readFileSync(new URL('../src/utils/themeSettingValues.ts', import.meta.url), 'utf8');
const code = ts.transpileModule(source, { compilerOptions: { module: ts.ModuleKind.ES2022, target: ts.ScriptTarget.ES2022 } }).outputText;
const { readThemeSetting, mergeThemeSettings } = await import('data:text/javascript;base64,' + Buffer.from(code).toString('base64'));

test('Retro managed settings read nested false values and legacy dotted keys', () => {
  assert.equal(readThemeSetting({ guestDisplay: { showPrice: false }, 'guestDisplay.showPrice': true }, 'guestDisplay.showPrice', 'retro'), false);
  assert.equal(readThemeSetting({ 'guestDisplay.showPrice': true }, 'guestDisplay.showPrice', 'retro'), true);
});
test('managed edits update canonical nested state without dropping undeclared settings', () => {
  const original = { guestDisplay: { showPrice: false, showExpiredAt: true }, 'guestDisplay.showPrice': false, future: { preserved: true } };
  const next = mergeThemeSettings(original, { 'guestDisplay.showPrice': true, 'statusCardsVisibility.currentTime': false }, 'retro');
  assert.deepEqual(next, { guestDisplay: { showPrice: true, showExpiredAt: true }, statusCardsVisibility: { currentTime: false }, future: { preserved: true } });
  assert.equal(original.guestDisplay.showPrice, false);
  assert.equal(readThemeSetting(next, 'guestDisplay.showPrice', 'retro'), true);
});
test('frontend nested edits clear stale dotted aliases; custom theme literal keys stay literal', () => {
  assert.deepEqual(mergeThemeSettings({ 'guestDisplay.showPrice': false }, { guestDisplay: { showPrice: true } }, 'retro'), { guestDisplay: { showPrice: true } });
  assert.deepEqual(mergeThemeSettings({}, { 'guestDisplay.showPrice': true }, 'custom'), { 'guestDisplay.showPrice': true });
});
