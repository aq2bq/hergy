// The rule document's own state (lib/ruledoc.js): a rule a person opens stands
// open with the caret in its body, the 編集 / Markdown switch is the screen's own
// state, and a press on a line of the Markdown puts the caret in the field that
// line names. The part keeps this state in signals of its own now: nothing raises
// a number to have its trees drawn again (webui-components 1 章).
import { expect, test } from 'bun:test';
import { layout, serve, settle } from './page.js';

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

const preview = {
  lines: [
    { text: '# 作り方', part: 'title', section: -1, rule: -1 },
    { text: '## 作る', part: 'heading', section: 0, rule: -1 },
    { text: '- HAAA 古い本文。', part: 'rule', section: 0, rule: 0 },
  ],
  chars: 100,
};

test('a rule opens with the caret in it, the switch is the panel\'s, and a preview line sends the caret', async () => {
  history.replaceState({}, '', '/roles?open=builder');
  layout('roles-data', island);
  serve((method, url) => {
    if (url.startsWith('/api/role/check')) return { problems: [], preview, effect: { diff: { added: [], removedBefore: [] }, turns: [] } };
    if (url.startsWith('/api/role')) return { items: [] };
    return undefined;
  });
  await import('../src/roles/main');
  await settle();

  const panel = () => document.querySelector('#rows .role.open .panel[data-l=project]');
  const rule = () => panel().querySelector('.rule');
  expect(rule().classList.contains('open')).toBe(false);

  // Opening a rule: it stands open, and the caret stays on the press's own
  // summary (5 章 5).
  click(rule().querySelector('.rsum'));
  await settle();
  expect(rule().classList.contains('open')).toBe(true);
  expect(document.activeElement).toBe(rule().querySelector('.rsum'));

  // The switch: Markdown and back, drawn from the screen's own state.
  click(panel().querySelector('.pview [data-act=pmode][data-v=md]'));
  await settle();
  expect(panel().getAttribute('data-mode')).toBe('md');
  click(panel().querySelector('.pview [data-act=pmode][data-v=edit]'));
  await settle();
  expect(panel().getAttribute('data-mode')).toBe('edit');

  // Closing the rule takes its body away; opening it again brings the caret back.
  click(rule().querySelector('.rsum'));
  await settle();
  expect(rule().classList.contains('open')).toBe(false);
  click(rule().querySelector('.rsum'));
  await settle();
  expect(document.activeElement).toBe(rule().querySelector('.rsum'));

  // A press on a line of the Markdown goes to the field that line names, in the
  // edit where a person types (A2, P15).
  click(panel().querySelector('.pview [data-act=pmode][data-v=md]'));
  await settle();
  const line = [...panel().querySelectorAll('.yprev .yl')].find((el) => el.dataset.part === 'rule');
  expect(line).toBeTruthy();
  click(line);
  await settle();
  expect(panel().getAttribute('data-mode')).toBe('edit');
  expect(document.activeElement).toBe(rule().querySelector('.redit [data-f=text]'));
});
