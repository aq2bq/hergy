import { expect, test } from 'bun:test';
import { blocks, chars } from '../src/lib/prose-editor';

test('heading ranges preserve raw text and ignore fenced headings', () => {
  const text = '前文\r\n# 題\r\n本文\r\n```md\r\n# コード\r\n```\r\n## 子\r\n末尾  \r\n';
  const found = blocks(text);
  expect(found.map((b) => b.heading.trim())).toEqual(['本文', '題', '子']);
  expect(found.map((b) => text.slice(b.start, b.end)).join('')).toBe(text);
  expect(chars('😀\r\n ')).toBe(3);
  expect(chars('字'.repeat(1200))).toBe(1200);
  expect(chars('字'.repeat(1201))).toBe(1201);
});
