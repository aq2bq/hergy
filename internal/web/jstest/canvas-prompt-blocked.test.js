// CANVAS-PROMPT, the blocked definition (d-20ac): a definition hergy will not
// build a prompt for shows the reason and no lines; when the last problem goes,
// the lines with the unsaved ops appear. ⑤ r-ba9b: the answer is the comparison
// owner's, and the check no longer carries a second copy of the lines.
import { expect, test } from 'bun:test';
import { layout, serve, settle } from './page.js';
import { choose, click, island } from './teams-page.js';

const flush = async () => { await settle(); await new Promise((r) => setTimeout(r, 5)); await settle(); };

test('a definition with a problem says why, keeps the history entry available after it is fixed', async () => {
  history.replaceState({}, '', '/teams?open=core');
  layout('teams-data', island({ turns: [['human', 'lead']] }));
  const blocked = 'このチーム定義には問題があるので、hergy はプロンプトを組みません';
  // A new member that hands the turn to no one stops the prompt; once both
  // directions are there, the answer carries the lines of the unsaved draft.
  const answerFor = (body) => {
    const ops = body.doc.ops || [];
    const turns = ops.filter((op) => op.kind === 'addTurn').length;
    if (ops.some((op) => op.kind === 'addMember') && turns < 2) {
      return { problems: [{ sev: 'err', at: 'turns', text: 'このメンバーは誰にもターンを渡していません' }], segments: [] };
    }
    if (turns >= 2) {
      const added = ops.find((op) => op.kind === 'addMember').member;
      return { problems: [], segments: [
        { kind: 'template', text: 'あなたは core-lead です。\n', origin: { kind: 'hergy' } },
        { kind: 'roster', text: `- core-${added}（ロール builder）\n`, origin: { kind: 'roster' } },
        { kind: 'turns', text: `- core-${added}（ロール builder）\n`, origin: { kind: 'turns' } },
      ] };
    }
    return { problems: [], segments: [] };
  };
  const asked = serve((method, url, body) => {
    if (url.startsWith('/api/team/prompt-comparison')) {
      const one = answerFor(body);
      return {
        runs: [], selected: null, status: 'stopped', reason: '', recorded: null,
        current: one.segments.length ? one.segments.map((s) => s.text).join('') : null,
        segments: one.segments, problems: one.problems, diff: [],
        project: '/example', executable: 'hergy', language: 'ja', team: 'core', member: 'lead', draft: true,
      };
    }
    if (url.startsWith('/api/team/check') && body && body.part === 'team') return { problems: answerFor(body).problems };
    if (url.startsWith('/api/team/check')) return { problems: [] };
    if (url.startsWith('/api/role')) return { items: [{ name: 'builder' }] };
    return undefined;
  });
  await import('../src/teams/main');
  const { PROMPT_READ } = await import('../src/teams/prompt-source.js');
  PROMPT_READ.wait = 0;
  await flush();

  // (a) the new member hands the turn to no one: the lead's column says why and
  // holds no line.
  click(document.querySelector('.tdef[data-name="core"] [data-cv-act="m-add"]'));
  await flush();
  const name = document.querySelector('.tdef[data-name="core"] .cv-side [data-cv-f="m-name"]').value;
  expect(name).not.toBe('');
  click(document.querySelector('.tdef[data-name="core"] .cv-n.member[data-cv-n="lead"]'));
  await flush();
  // The attribute tab no longer carries the prompt (r-f0be).
  expect(document.querySelector('.tdef[data-name="core"] .cv-prompt')).toBe(null);
  click(document.querySelector('.tdef[data-name="core"] [data-cv-side="prompt"]'));
  await flush(); await flush();
  let box = document.querySelector('.tdef[data-name="core"] .cv-prompt');
  expect(box.textContent).toContain(blocked);
  expect(box.querySelectorAll('.yl').length).toBe(0);

  // (b) both directions are drawn (not saved): the lines name the new member,
  // and the blocked words go. The attribute fields are read in 属性.
  click(document.querySelector('.tdef[data-name="core"] [data-cv-side="attr"]'));
  await flush();
  choose(document.querySelector('.tdef[data-name="core"] .cv-side select[data-cv-f=turn-add]'), name);
  await flush();
  click(document.querySelector(`.tdef[data-name="core"] .cv-n.member[data-cv-n="${name}"]`));
  await flush();
  choose(document.querySelector('.tdef[data-name="core"] .cv-side select[data-cv-f=turn-add]'), 'lead');
  await flush();
  click(document.querySelector('.tdef[data-name="core"] .cv-n.member[data-cv-n="lead"]'));
  await flush();
  click(document.querySelector('.tdef[data-name="core"] [data-cv-side="prompt"]'));
  await flush(); await flush();
  box = document.querySelector('.tdef[data-name="core"] .cv-prompt');
  expect(box.querySelector('button').textContent).toBe('全文と差分を読む');
  expect(box.textContent).not.toContain(blocked);

  // The lines came from the comparison of the unsaved draft: both turns were sent.
  const last = asked.filter((one) => one.url.startsWith('/api/team/prompt-comparison')).pop();
  expect(last.body.doc.ops.filter((op) => op.kind === 'addTurn').length).toBe(2);
});
