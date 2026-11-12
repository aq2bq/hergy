import { promptClock } from './prompt-clock.js';
// ⑤ r-ba9b: the prompt's key is everything a change of which changes the text.
// A start, a stop and a pick read again without pressing anything; the old
// answer does not stand while the new one is read.
import { expect, test } from 'bun:test';
import { layout, serve, settle } from './page.js';
import { checkAnswer, choose, click, island, memberSegments } from './teams-page.js';
import { setRunning } from '../src/lib/hq/state.js';

const flush = async () => { await settle(); await new Promise((r) => setTimeout(r, 5)); await settle(); };

function comparison(runs, sent) {
  const selected = sent ? runs.find((run) => run.team === sent) || null : (runs.length === 1 ? runs[0] : null);
  return {
    runs, selected, status: selected ? 'stopped' : runs.length ? 'choose' : 'stopped',
    reason: '', recorded: null,
    current: memberSegments.map((s) => s.text).join(''), segments: memberSegments,
    problems: [], diff: [], project: '/example', executable: 'hergy', language: 'ja',
    team: selected ? selected.team : 'core', member: 'lead', draft: true,
  };
}

test('a start, a stop and a pick change the key and read again', async () => {
  history.replaceState({}, '', '/teams?open=core');
  layout('teams-data', island());
  let live = [];
  const asked = serve((method, url, body) => {
    if (url.startsWith('/api/team/prompt-comparison')) return comparison(live, (body && body.running_team) || '');
    if (url.startsWith('/api/team/check') && body && body.part === 'team') return checkAnswer(memberSegments);
    if (url.startsWith('/api/team/check')) return { problems: [] };
    if (url.startsWith('/api/role')) return { items: [] };
    return undefined;
  });
  await promptClock();
 await import('../src/teams/main');
  await flush();
  const row = () => document.querySelector('.tdef[data-name="core"]');
  const panel = () => row().querySelector('.cv-topology-prompt');
  const note = () => panel().querySelector('.cv-note').textContent;
  const reads = () => asked.filter((one) => one.url.startsWith('/api/team/prompt-comparison')).length;

  click(row().querySelector('.cv-n.member[data-cv-n="lead"]')); await flush();
  click(row().querySelector('[data-cv-side="prompt"]')); await flush(); await flush();
  const first = reads();
  expect(note()).toContain('この定義から動いているチームが無いため、定義名 core で作っています。');

  // A team starts: the live reading changes, so the key does, and the column
  // reads again with no press.
  live = [{ team: 'core', project: '', members: [{ name: 'core-lead', live: true }] }];
  setRunning({ herdr: { ok: true }, teams: [{ team: 'core', definition: 'core', place: 'project', project: '', members: [{ name: 'core-lead', live: true }] }] });
  await flush(); await flush();
  expect(reads()).toBeGreaterThan(first);
  expect(note()).toContain('実行中のチーム core の名前で作っています。');

  // It stops: the key moves again, and the definition's name comes back.
  live = [];
  setRunning({ herdr: { ok: true }, teams: [] });
  await flush(); await flush();
  expect(note()).toContain('定義名 core で作っています。');

  // Two run and a person picks one in the dialog: the pick is part of the key.
  live = [{ team: 'core', project: '' }, { team: 'core-hotfix', project: '' }];
  setRunning({ herdr: { ok: true }, teams: [
    { team: 'core', definition: 'core', place: 'project', project: '', members: [{ name: 'core-lead', live: true }] },
    { team: 'core-hotfix', definition: 'core', place: 'project', project: '', members: [{ name: 'core-hotfix-lead', live: true }] },
  ] });
  await flush(); await flush();
  expect(note()).toContain('動いているチームが 2 つあるため、定義名 core で作っています。');
  click(panel().querySelector('.cv-prompt button')); await flush();
  choose(document.querySelector('.prompt-controls select'), 'core-hotfix'); await flush(); await flush();
  expect(note()).toContain('実行中のチーム core-hotfix の名前で作っています。');
  const afterPick = asked.filter((one) => one.url.startsWith('/api/team/prompt-comparison'));
  expect(afterPick[afterPick.length - 1].body.running_team).toBe('core-hotfix');
});

test('prompt components keep independent private picks and expose no arbitrary-key setter', async () => {
  const { createPromptSource } = await import('../src/teams/prompt-source');
  const a = createPromptSource({ wait: 0 }), b = createPromptSource({ wait: 0 });
  const row = { dom: 'core', place: { value: 'project' } };
  a.pickRun(row, 'core-hotfix');
  expect(a.pickedRun(row)).toBe('core-hotfix');
  expect(b.pickedRun(row)).toBe('');
  expect(a.setRun).toBeUndefined();
  expect(Object.isFrozen(a)).toBe(true);
});
