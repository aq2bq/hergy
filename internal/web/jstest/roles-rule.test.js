import { expect, test } from 'bun:test';
import { layout, serve, settle, type } from './page.js';

const text = '# 作り方\r\n\r\n本文  \r\n## 検証\r\n→ は本文。\r\n';
test('the full prose is editable and saved without IDs or whitespace rewriting', async () => {
  history.replaceState({}, '', '/roles?open=builder&rule=OLD');
  layout('roles-data', {
    places: { project: '.hergy/roles/', global: '/global/roles/' }, texts: {},
    role: { items: [{ name: 'builder', places: { project: {
      doc: { name: 'builder', title: '作り方', text }, text, hash: 'h1', path: '.hergy/roles/builder.md',
    } }, summary: {} }] }, team: { items: [] },
  });
  const asked = serve((method, url, body) => {
    if (url.includes('/check')) return { problems: [], preview: { lines: [], chars: 0 }, effect: { diff: {} } };
    if (method === 'PUT') return { doc: body.doc, text: body.doc.text, hash: 'h2' };
    if (url.startsWith('/api/role')) return { items: [] };
    return undefined;
  });
  await import('../src/roles/main');
  await settle();
  const panel = document.querySelector('.role.open .panel');
  const input = panel.querySelector('textarea[aria-label$="の規則の全文"]');
  expect(input).not.toBeNull();
  const changed = '# 作り方\n\n 空白を残す  \n\n## 検証\n→ は本文。\n';
  type(input, changed);
  await settle();
  panel.querySelector('[data-act=save]').click();
  await settle();
  const put = asked.find((r) => r.method === 'PUT');
  expect(put.body.doc.text).toBe(changed);
  expect(Object.keys(put.body.doc).sort()).toEqual(['name', 'text', 'title']);
  expect(put.body.hash).toBe('h1');
});
