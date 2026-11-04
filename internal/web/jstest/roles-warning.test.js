import { expect, test } from 'bun:test';
import { layout, serve, settle, type } from './page.js';

// click is a person's press on one element.
function click(el) {
  el.dispatchEvent(new Event('click', { bubbles: true }));
}

const doc = {
  name: 'builder', title: '作り方', text: '# 作り方\n本文\n', revision: 1,
  sections: [{ heading: '作る', level: 0, intro: '', rules: [
    { id: 'HAAA', text: '古い本文。', note: '', in_prompt: true },
  ] }],
};

const island = {
  places: { project: '.hergy/roles/', global: '~/.config/hergy/roles/' },
  texts: {},
  role: {
    items: [{
      name: 'builder',
      places: { project: { doc, hash: 'h1', text: 'version: 1\n', path: '.hergy/roles/builder.yaml' } },
      summary: {},
    }],
  },
  team: { items: [] },
};

// preview is the document as the screen draws it: the title, a blank line, the
// heading, a blank line, and the rule's own line (rules.PreviewLines).
const preview = {
  lines: [
    { text: '# 作り方', part: 'title', section: -1, rule: -1 },
    { text: '', part: 'blank', section: -1, rule: -1 },
    { text: '## 作る', part: 'heading', section: 0, rule: -1 },
    { text: '', part: 'blank', section: -1, rule: -1 },
    { text: '- HAAA 古い本文。', part: 'rule', section: 0, rule: 0 },
  ],
  chars: 1201,
};

test('a document-wide budget warning leaves prose saving available', async () => {
  history.replaceState({}, '', '/roles?open=builder');
  layout('roles-data', island);
  const asked = serve((method, url, body) => {
    if (url.startsWith('/api/role/check')) {
      return {
        // d-00a9: the document as a whole is over the provisional budget, so
        // the warning names no rule of its own (section -1).
        problems: [{ sev: 'warn', section: -1, rule: -1, field: 'text', text: 'この文書は 1201 文字です（仮置きの基準は 1200 文字）' }],
        preview, effect: { diff: { added: [], removedBefore: [] }, turns: [] },
      };
    }
    if (method === 'PUT' && url.startsWith('/api/role/builder')) return { doc: body.doc, text: body.doc.text, hash: 'h2' };
    if (url.startsWith('/api/role')) return { items: [] };
    return undefined;
  });
  await import('../src/roles/main');
  await settle();

  const panel = document.querySelector('#rows .role.open .panel');
  expect(panel.querySelectorAll('.vm .sev.warn').length).toBe(1);
  const want = '字'.repeat(1201);
  type(panel.querySelector('textarea[aria-label$="の散文の全文"]'), want);
  await settle();
  const button = panel.querySelector('[data-act=save]');
  expect(button.disabled).toBe(false);
  click(button); await settle();
  const save = asked.find((one) => one.method === 'PUT');
  expect(save.body.doc.text).toBe(want);
});
