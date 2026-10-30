// ROLES-S1, the 保存しました a save answers stands until the person's next change
// (r-b41c): a check that answers a beat after the save — the one the typing
// scheduled (scheduleCheck) — draws the panel again, but it is not a change, so
// it does not take the notice away. The check's answer used to go through touch,
// which is the change's own way: the notice was drawn and erased a few
// milliseconds later, and the walk that waits for it (RB41C-5) then never saw it.
import { expect, test } from 'bun:test';
import { layout, serve, settle, type } from './page.js';

function click(el) {
  el.dispatchEvent(new Event('click', { bubbles: true }));
}

const doc = {
  name: 'builder', title: '作り方', revision: 1,
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

const check = {
  problems: [],
  preview: { lines: [{ text: '# 作り方', part: 'title', section: -1, rule: -1 }], chars: 10 },
  effect: { diff: { added: [], removedBefore: [] }, turns: [] },
};

test('a check that answers after the save does not take away 保存しました, and a change does', async () => {
  history.replaceState({}, '', '/roles?open=builder');
  layout('roles-data', island);
  serve((method, url, body) => {
    if (url.startsWith('/api/role/check')) return check;
    if (method === 'PUT' && url.startsWith('/api/role/builder')) return { doc: body.doc, hash: 'h2', text: 'version: 1\n' };
    if (url.startsWith('/api/role')) return { items: [] };
    return undefined;
  });
  await import('../src/roles/main');
  await settle();

  const panel = () => document.querySelector('#rows .role.open .panel[data-l=project]');
  const notice = () => panel().querySelector('.ok');
  const rule = () => panel().querySelector('.rule');

  // Type into the rule and save at once: the typing scheduled a check for 300ms
  // later, and that check answers after the save's own.
  click(rule().querySelector('.rsum'));
  await settle();
  type(panel().querySelector('.rule.open .redit [data-f=text]'), '書き直した本文。');
  await settle();
  expect(notice()).toBe(null);
  click(panel().querySelector('[data-act=save]'));
  await settle();
  expect(notice(), 'the save answered 保存しました').not.toBe(null);

  // The scheduled check answers now: the panel is drawn again, and the notice is
  // still the drawing's.
  await new Promise((resolve) => setTimeout(resolve, 400));
  await settle();
  expect(notice(), 'a check that answers is not a change, so the notice stands').not.toBe(null);

  // A change of the person's own takes it away.
  click(rule().querySelector('.rsum'));
  await settle();
  type(panel().querySelector('.rule.open .redit [data-f=text]'), 'もう一度書き直した本文。');
  await settle();
  expect(notice(), 'a change takes the notice away').toBe(null);
});
