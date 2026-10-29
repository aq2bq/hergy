// A new rule has no screen-made ID: it keeps its internal key in the draft,
// check answer, preview line and switch until the save answer brings the ID.
import { expect, test } from 'bun:test';
import { layout, serve, settle, type } from './page.js';

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
  role: { items: [{
    name: 'builder',
    places: { project: { doc, hash: 'h1', text: 'version: 1\n', path: '.hergy/roles/builder.yaml' } },
    summary: {},
  }] },
  team: { items: [] },
};

test('a rule keeps its draft key without an ID until the save answer supplies one', async () => {
  history.replaceState({}, '', '/roles?open=builder');
  layout('roles-data', island);
  const preview = (body) => ({
    lines: [
      { text: '# 作り方', part: 'title', section: -1, rule: -1 },
      { text: '## 作る', part: 'heading', section: 0, rule: -1 },
      ...body.doc.sections[0].rules.map((rule, i) => ({
        text: `- ${rule.id ? `${rule.id} ` : ''}${rule.text}`,
        part: 'rule', section: 0, rule: i,
      })),
    ],
    chars: 100,
  });
  const problem = { sev: 'warn', at: 'rules[0].text', field: 'text', section: 0, rule: 1, text: '下書きの規則を確認してください' };
  const turn = { section: 0, rule: 1, arrow: 'lead -> builder', definition: 'core', ok: false };
  let saveBody;
  serve((method, url, body) => {
    if (url.startsWith('/api/role/check')) {
      return {
        problems: body.doc.sections[0].rules.length > 1 ? [problem] : [],
        preview: preview(body),
        effect: { diff: { added: [], removedBefore: [], removedAtEnd: false }, turns: body.doc.sections[0].rules.length > 1 ? [turn] : [] },
      };
    }
    if (method === 'PUT' && url.startsWith('/api/role/builder')) {
      saveBody = body;
      const saved = structuredClone(body.doc);
      saved.sections[0].rules[1].id = 'K7QF';
      return { doc: saved, hash: 'h2', text: 'version: 1\n' };
    }
    if (url.startsWith('/api/role')) return { items: [] };
    return undefined;
  });
  await import('../src/roles/main');
  await settle();

  const panel = () => document.querySelector('#rows .role.open .panel[data-l=project]');
  const rules = () => [...panel().querySelectorAll('.sec[data-si="0"] .rule')];
  expect(rules()[0].querySelector('.rid b').textContent.trim()).toBe('BUILD-5');
  click(panel().querySelector('.sec[data-si="0"] [data-act=rule-add]'));
  await settle();

  const added = () => rules()[1];
  const key = added().dataset.k;
  expect(added().querySelector('.rid b').textContent.trim()).toBe('保存で付く');
  expect(added().querySelector('.onsw').getAttribute('aria-checked')).toBe('true');

  // The switch and the problem marker belong to the internal draft key.
  click(added().querySelector('.onsw'));
  await settle();
  expect(added().querySelector('.onsw').getAttribute('aria-checked')).toBe('false');
  click(added().querySelector('.onsw'));
  await settle();
  type(added().querySelector('.redit [data-f=text]'), '保存で ID が付く規則。');
  await new Promise((resolve) => setTimeout(resolve, 350));
  await settle();
  expect(added().querySelector('.rid .sev.warn').textContent).toContain('警告');

  // The check's arrow and the preview line still point at section 0, rule 1;
  // the preview resolves that position to this draft rule's _k.
  const state = await import('../src/roles/state');
  const row = state.roles.value[0];
  const panelRow = state.rowOfPanel(row, 'project');
  expect(panelRow.turns).toEqual([turn]);
  click(panel().querySelector('.pview [data-act=pmode][data-v=md]'));
  await settle();
  const line = panel().querySelector(`.yprev .yl[data-part="rule"][data-k="${key}"]`);
  expect(line, 'the ID-less rule is still the target of its preview line').toBeTruthy();
  click(line);
  await settle();
  expect(panel().getAttribute('data-mode')).toBe('edit');
  expect(document.activeElement).toBe(added().querySelector('.redit [data-f=text]'));

  click(panel().querySelector('.pf [data-act=save]'));
  await settle();
  expect(saveBody, 'the save was sent').toBeTruthy();
  expect(saveBody.doc.sections[0].rules[0].id).toBe('BUILD-5');
  expect(Object.hasOwn(saveBody.doc.sections[0].rules[1], 'id'), 'a new rule sends no empty ID').toBe(false);
  expect(rules()[1].querySelector('.rid b').textContent.trim()).toBe('K7QF');
});
