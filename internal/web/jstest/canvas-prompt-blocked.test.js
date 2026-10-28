// CANVAS-PROMPT, the blocked definition (d-20ac): a definition hergy will not
// build a prompt for shows the reason and no lines; when the last problem goes,
// the lines with the unsaved ops appear. The answer is the test's, so what the
// column says and asks is what is read.
import { expect, test } from 'bun:test';
import { layout, serve, settle } from './page.js';
import { choose, click, island } from './teams-page.js';

test('a definition with a problem says why, and gives the lines once it is fixed', async () => {
  history.replaceState({}, '', '/teams?open=core');
  layout('teams-data', island({ turns: [['human', 'lead']] }));
  const blocked = 'このチーム定義には問題があるので、hergy はプロンプトを組みません';
  // A new member that hands the turn to no one stops the prompt; once both
  // directions are there, the answer carries the lines of the unsaved draft.
  const asked = serve((method, url, body) => {
    if (url.startsWith('/api/team/check') && body && body.part === 'team') {
      const ops = body.doc.ops || [];
      const turns = ops.filter((op) => op.kind === 'addTurn').length;
      if (ops.some((op) => op.kind === 'addMember') && turns < 2) {
        return { problems: [{ sev: 'err', at: 'turns', text: 'このメンバーは誰にもターンを渡していません' }], members: [] };
      }
      if (turns >= 2) {
        const added = ops.find((op) => op.kind === 'addMember').member;
        return { problems: [], members: [{ name: 'lead', segments: [
          { kind: 'template', text: 'あなたは core-lead です。\n', origin: { kind: 'hergy' } },
          { kind: 'roster', text: `- core-${added}（ロール builder）\n`, origin: { kind: 'roster' } },
          { kind: 'turns', text: `- core-${added}（ロール builder）\n`, origin: { kind: 'turns' } },
        ] }] };
      }
    }
    if (url.startsWith('/api/team/check') && body && body.part === 'agreement') return { problems: [] };
    if (url.startsWith('/api/team/check')) return { problems: [] };
    if (url.startsWith('/api/role')) return { items: [{ name: 'builder' }] };
    return undefined;
  });
  await import('../static/teams/main.js');
  await settle();

  // (a) the new member hands the turn to no one: the lead's column says why and
  // holds no line.
  click(document.querySelector('.tdef[data-name="core"] [data-cv-act="m-add"]'));
  await settle();
  const name = document.querySelector('.tdef[data-name="core"] .cv-side [data-cv-f="m-name"]').value;
  expect(name).not.toBe('');
  click(document.querySelector('.tdef[data-name="core"] .cv-n.member[data-cv-n="lead"]'));
  await settle();
  let box = document.querySelector('.tdef[data-name="core"] .cv-prompt');
  expect(box.textContent).toContain(blocked);
  expect(box.querySelectorAll('.yl').length).toBe(0);

  // (b) both directions are drawn (not saved): the lines name the new member,
  // and the blocked words go.
  choose(document.querySelector('.tdef[data-name="core"] .cv-side select[data-cv-f=turn-add]'), name);
  await settle();
  click(document.querySelector(`.tdef[data-name="core"] .cv-n.member[data-cv-n="${name}"]`));
  await settle();
  choose(document.querySelector('.tdef[data-name="core"] .cv-side select[data-cv-f=turn-add]'), 'lead');
  await settle();
  click(document.querySelector('.tdef[data-name="core"] .cv-n.member[data-cv-n="lead"]'));
  await settle();
  box = document.querySelector('.tdef[data-name="core"] .cv-prompt');
  expect(box.querySelectorAll('.yl').length).toBeGreaterThan(0);
  expect(box.textContent).toContain(`core-${name}`);
  expect(box.textContent).toContain(`core-${name}（ロール`);
  expect(box.textContent).not.toContain(blocked);

  // The lines came from the check of the unsaved draft: both turns were sent.
  const last = asked.filter((one) => one.url.startsWith('/api/team/check') && one.body.part === 'team').pop();
  expect(last.body.doc.ops.filter((op) => op.kind === 'addTurn').length).toBe(2);
});
