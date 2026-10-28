// ROLES-P5, the rule a person edits: a sentence about the document as a whole
// marks the preview's first line alone, not every heading and blank line (R4),
// and the text a person types is the one the save carries (P4, C4). The file the
// save writes, one line of it, is the server's (internal/web's
// TestROLES_F1_ASaveWritesTheChangedLineAlone); here the check's answer and the
// save's answer are the test's, so what the screen makes of them is what is read.
import { expect, test } from 'bun:test';
import { layout, serve, settle, type } from './page.js';

// click is a person's press on one element.
function click(el) {
  el.dispatchEvent(new Event('click', { bubbles: true }));
}

const doc = {
  name: 'builder', title: '作り方', revision: 1,
  sections: [{ heading: '作る', level: 0, intro: '', rules: [
    { id: 'BUILD-5', text: '古い本文。', note: '', in_prompt: true },
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
    { text: '- BUILD-5 古い本文。', part: 'rule', section: 0, rule: 0 },
  ],
  chars: 3100,
};

test('a document-wide warning marks the first line alone, and the typed text is what the save carries', async () => {
  history.replaceState({}, '', '/roles?open=builder');
  layout('roles-data', island);
  const asked = serve((method, url, body) => {
    if (url.startsWith('/api/role/check')) {
      return {
        // d-00a9: the document as a whole is over the provisional budget, so
        // the warning names no rule of its own (section -1).
        problems: [{ sev: 'warn', section: -1, rule: -1, field: 'text', text: 'この文書は 3100 文字です（仮置きの基準は 3000 文字）' }],
        preview, effect: { diff: { added: [], removedBefore: [] }, turns: [] },
      };
    }
    if (method === 'PUT' && url.startsWith('/api/role/builder')) return { hash: 'h2' };
    if (url.startsWith('/api/role')) return { items: [] };
    return undefined;
  });
  await import('../src/roles/main');
  await settle();

  // R4: the warning about the document as a whole stands on the first line
  // alone; no heading or blank line carries it.
  const lines = [...document.querySelectorAll('#rows .role.open .yprev .yl')];
  expect(lines.length).toBe(preview.lines.length);
  const warned = lines.map((el, i) => (el.classList.contains('warn') ? i : -1)).filter((i) => i >= 0);
  expect(warned).toEqual([0]);

  // P4・C4: the typed text is the one the save carries.
  const rule = [...document.querySelectorAll('#rows .role.open .panel .rule')]
    .find((el) => el.querySelector('.rid b').textContent.trim() === 'BUILD-5');
  expect(rule).toBeTruthy();
  click(rule.querySelector('.rsum'));
  await settle();
  const want = '単体テストが書き換えた本文。';
  type(document.querySelector('#rows .role.open .rule.open .redit [data-f=text]'), want);
  await settle();
  click(document.querySelector('#rows .role.open .pf [data-act=save]'));
  await settle();

  const save = asked.filter((one) => one.method === 'PUT' && one.url.startsWith('/api/role/builder')).pop();
  expect(save, 'the save was not sent').toBeTruthy();
  expect(save.body.doc.sections[0].rules[0].text).toBe(want);
  // Nothing else of the document moved: the id and the other places the panel
  // carries are the ones the island gave.
  expect(save.body.doc.sections[0].rules[0].id).toBe('BUILD-5');
  expect(save.body.doc.sections[0].heading).toBe('作る');
});
