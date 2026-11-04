import { expect, test } from 'bun:test';
import { h, render } from '../src/lib/preact';
import { ProseEditor } from '../src/lib/prose-editor';
import { settle } from './page.js';

test('the outline has block counts and returns to one full-text editor', async () => {
  document.body.innerHTML = '<main></main>';
  const text = '# 題\n本文\n## 次\n続き\n';
  render(h(ProseEditor, { text, onChange() {} }), document.querySelector('main'));
  await settle();
  expect(document.querySelectorAll('textarea').length).toBe(1);
  const toggle = document.querySelector('button');
  expect(toggle.title).toBe(toggle.getAttribute('aria-label'));
  toggle.click(); await settle();
  expect(document.querySelectorAll('textarea').length).toBe(0);
  expect(document.querySelectorAll('.prose-outline li').length).toBe(2);
  document.querySelector('button').click(); await settle();
  expect(document.querySelector('textarea').value).toBe(text);
});
