import { expect, test } from 'bun:test';
import { blocks } from '../src/lib/prose-blocks.ts';
import { blockRange, transferDraft, saveTransfer, documentID } from '../src/lib/prose-transfer.ts';

test('heading ranges preserve CRLF, spaces, prefixes and nested fenced content', () => {
 const text = '  導入\r\n\r\n# A\r\n本文  \r\n## 子\r\n```md\r\n# not a heading\r\n```still inside\r\n# still inside\r\n```\r\n## 次\r\n末尾\r\n# B\r\n';
 const parts = blocks(text);
 expect(parts.map((p) => p.heading.trim())).toEqual(['本文', 'A', '子', '次', 'B']);
 const [start, end] = blockRange(parts, 1, true);
 const result = transferDraft(text, '宛先\r\n\r\n', start, end, 0, true);
 expect(result.destination).toBe(text.slice(text.indexOf('# A'), text.indexOf('# B')) + '宛先\r\n\r\n');
 expect(result.source).toBe('  導入\r\n\r\n# B\r\n');
 const prefix = blockRange(parts, 0, true);
 expect(text.slice(...prefix)).toBe('  導入\r\n\r\n');
 const own = blockRange(parts, 1, false);
 expect(text.slice(...own)).toBe('# A\r\n本文  \r\n');
 expect(transferDraft(text, '', ...own, 0, false).source).toBe(text);
});

test('move saves destination before source and does not delete after a refused destination', async () => {
 for (const fail of ['', 'destination', 'source']) {
  const called = [];
  const doc = (id) => ({ id, label: id, text: () => '', set: () => {}, save: async () => { called.push(id); if (id === fail) throw new Error('conflict'); return true; } });
  expect(await saveTransfer(doc('destination'), doc('source'), true)).toBe(fail ? fail + '-failed' : 'done');
  expect(called).toEqual(fail === 'destination' ? ['destination'] : ['destination', 'source']);
 }
 const called = [];
 const doc = (id) => ({ id, label: id, text: () => '', set: () => {}, save: async () => { called.push(id); return true; } });
 expect(await saveTransfer(doc('destination'), doc('source'), false)).toBe('done');
 expect(called).toEqual(['destination']);
});

test('global file identity ignores caller project while local files remain distinct', () => {
 expect(documentID('role', 'a', 'global', '/one')).toBe(documentID('role', 'a', 'global', '/two'));
 expect(documentID('role', 'a', 'project', '/one')).not.toBe(documentID('role', 'a', 'project', '/two'));
});


test('insertion supplies only the missing boundary blank line and preserves the copied range', () => {
 const source = '## Heading\r\nbody  \r\n';
 for (const [before, expected] of [['', ''], ['text', 'text\n\n'], ['text\n', 'text\n\n'], ['text\n\n', 'text\n\n'], ['text\r\n', 'text\r\n\r\n']]) {
  expect(transferDraft(source, before, 0, source.length, before.length, false).destination).toBe(expected + source);
 }
 const target = '# Title\n## Next\n';
 expect(transferDraft(source, target, 0, source.length, 8, false).destination).toBe('# Title\n\n' + source + '## Next\n');
});
