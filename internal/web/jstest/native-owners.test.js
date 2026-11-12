// These are ESLint's built-in rules over the actual configuration. No custom
// scanner mirrors the rules: exceptions are checked with the same parser as CI.
import { expect, test } from 'bun:test';
import { ESLint } from '../../../.bin/webtools/node_modules/eslint/lib/api.js';
const lint = new ESLint({ cwd: new URL('../', import.meta.url).pathname, overrideConfigFile: new URL('../../../.bin/webtools/eslint.config.js', import.meta.url).pathname });
async function errors(file, text) {
  return (await lint.lintText(text, { filePath: `src/${file}` }))[0].messages.filter((m) => m.severity === 2).map((m) => m.ruleId);
}
test('native page movements and addresses are rejected outside their owners', async () => {
  expect(await errors('roles/proof.ts', "window.scrollBy(0, 1); el.scrollTop += 2; el.focus(); q.set('open', 'core');")).toEqual(['no-restricted-properties', 'no-restricted-syntax', 'no-restricted-syntax', 'no-restricted-syntax']);
  expect(await errors('lib/page-position.ts', 'window.scrollBy(0, 1); el.scrollTop += 2; el.scrollIntoView();')).toEqual([]);
  expect(await errors('lib/address.ts', "q.get('open'); q.set('open', 'core'); q.delete('open');")).toEqual([]);
});
test('local scroller exceptions do not allow page movement or address writes', async () => {
  expect(await errors('lib/yamltab.tsx', "el.scrollTop = 2; el.scrollLeft = 1; el.focus({preventScroll: true}); window.scrollBy(0, 1); q.get('open');")).toEqual(['no-restricted-properties', 'no-restricted-syntax']);
  expect(await errors('lib/hq/sheets.tsx', 'el.scrollIntoView(); el.scrollTop = 2; el.scrollLeft = 1;')).toEqual(['no-restricted-syntax']);
});
test('resource comparisons stay in editing and transferred prose retains its comparison', async () => {
  const text = 'JSON.stringify(a) === JSON.stringify(b);';
  expect(await errors('roles/proof.ts', text)).toEqual(['no-restricted-syntax', 'no-restricted-syntax']);
  expect(await errors('lib/editing.ts', text)).toEqual([]);
  expect(await errors('lib/prose-transfer.ts', text)).toEqual([]);
});
