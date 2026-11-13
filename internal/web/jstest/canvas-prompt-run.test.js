import { promptClock } from './prompt-clock.js';
// r-0b69 / ⑤ r-ba9b: one owner reads the prompt (prompt-source.ts). The column
// and the whole-text panel draw its answer, a pick is one pick, a start/stop or
// an edit reads again through the key, and a failed read leaves no old text.
import { expect, test } from 'bun:test';
import { layout, serve, settle } from './page.js';

// flush lets a read (and the pick it may clear) finish: the owner's pause is 0
// in this file, and a short timer lets its promise and signals settle.
const flush = async () => { await settle(); await new Promise((r) => setTimeout(r, 5)); await settle(); };
import { checkAnswer, choose, click, island, memberSegments } from './teams-page.js';
import { setRunning } from '../src/lib/hq/state.js';

const aliasSegments = [
  { kind: 'template', text: 'あなたは core-hotfix チームの core-hotfix-lead です。\n', origin: { kind: 'hergy' } },
  { kind: 'roster', text: '- core-hotfix-builder（ロール builder）\n', origin: { kind: 'roster' } },
  { kind: 'turns', text: '## ターンを渡す相手\n- core-hotfix-builder\n', origin: { kind: 'turns' } },
];
const textOf = (segments) => segments.map((segment) => segment.text).join('');

function comparison(over = {}) {
  const segments = over.segments || aliasSegments;
  return {
    runs: [], selected: null, status: 'stopped', reason: '動いているチームがないため、今の文だけを表示します',
    recorded: null, current: textOf(segments), segments, problems: [], diff: [],
    project: '/example', executable: 'hergy', language: 'ja', team: 'core', member: 'lead', draft: true,
    ...over,
  };
}

let mode = 'two';

test('the note names the team the shown text was built with, in every case', async () => {
  history.replaceState({}, '', '/teams?open=core');
  layout('teams-data', island());
  let liveState = { herdr: { ok: true }, teams: [] };
  const live = (teams) => {
    liveState = { herdr: { ok: true }, teams: teams.map((team) => ({ team, definition: 'core', place: 'project', project: '', members: [{ name: `${team}-lead`, live: true }] })) };
    setRunning(liveState);
  };
  live(['core', 'core-hotfix']);
  const asked = serve((method, url, body) => {
    if (url === '/api/hq/running') return liveState;
    if (url.startsWith('/api/team/prompt-comparison')) {
      if (mode === 'netfail') throw new Error('プロジェクトを読めません');
      const sent = (body && body.running_team) || '';
      if (mode === 'one') return comparison({ team: 'core-hotfix', selected: { team: 'core-hotfix', project: '/example' }, runs: [{ team: 'core-hotfix', project: '/example' }] });
      if (mode === 'stale') {
        // core-hotfix stopped: only core runs now, and a stale pick is not
        // taken by the server, so the owner has to let it go itself.
        return comparison({ runs: [{ team: 'core', project: '/example' }], selected: sent === 'core-hotfix' ? null : { team: 'core', project: '/example' }, team: 'core', segments: memberSegments, status: sent === 'core-hotfix' ? 'choose' : 'stopped', reason: sent === 'core-hotfix' ? '実行中のチームを選んでください' : '' });
      }
      if (mode === 'error') return comparison({ status: 'error', reason: 'チームの状態を読めません', runs: [], selected: null, team: 'core', segments: memberSegments });
      const alias = sent === 'core-hotfix';
      return comparison({
        runs: [{ team: 'core', project: '/example' }, { team: 'core-hotfix', project: '/example' }],
        selected: sent ? { team: sent, project: '/example' } : null,
        team: sent || 'core',
        segments: alias ? aliasSegments : memberSegments,
      });
    }
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
  const roster = () => panel().querySelector('.cv-topology-section[data-kind=roster] pre').textContent;
  const openTab = async () => {
    click(row().querySelector('[data-cv-side="prompt"]')); await flush();
  };
  const openDialog = async () => {
    click(panel().querySelector('.cv-prompt button')); await flush();
  };
  const closeDialog = async () => {
    click([...document.querySelectorAll('.prompt-comparison button')].find((x) => x.textContent === '閉じる')); await flush();
  };
  // The server's answer is changed by the test (mode); a read is asked for by
  // the panel's 再取得. The key did not move, so this is what asks again.
  const reread = async () => {
    await openDialog();
    click([...document.querySelectorAll('.prompt-comparison button')].find((x) => x.textContent === '再取得')); await flush(); await flush();
    await closeDialog();
  };

  click(row().querySelector('.cv-n.member[data-cv-n="lead"]')); await flush();
  await openTab();

  // Two runs, none picked: the definition's name. The panel's pick is taken,
  // and the column builds with that team's name.
  expect(note()).toContain('動いているチームが 2 つあるため、チーム定義名 core で作っています。');
  await openDialog();
  choose(document.querySelector('.prompt-controls select'), 'core-hotfix'); await flush(); await flush();
  await closeDialog();
  expect(note()).toContain('実行中のチーム core-hotfix の名前で作っています。');
  expect(roster()).toContain('core-hotfix-builder');

  // A failed reading of the team state says nothing about the runs, so the pick
  // stands: the reason is shown, and the next reading still names the pick.
  mode = 'error';
  await reread();
  expect(note()).toContain('チームの状態を読めません そのため、チーム定義名 core で作っています。');
  mode = 'two';
  await reread();
  const afterError = asked.filter((one) => one.url.startsWith('/api/team/prompt-comparison'));
  expect(afterError[afterError.length - 1].body.running_team).toBe('core-hotfix');
  expect(note()).toContain('実行中のチーム core-hotfix の名前で作っています。');

  // One running team: its name is taken and shown.
  mode = 'one';
  live(['core-hotfix']);
  await reread();
  expect(note()).toContain('実行中のチーム core-hotfix の名前で作っています。');
  expect(roster()).toContain('core-hotfix-builder');

  // The picked team stops: the owner lets the pick go, and the next read takes
  // the one team still running.
  mode = 'stale';
  live(['core']);
  await reread();
  expect(note()).toContain('実行中のチーム core の名前で作っています。');
  const afterStale = asked.filter((one) => one.url.startsWith('/api/team/prompt-comparison'));
  expect(afterStale[afterStale.length - 1].body.running_team).toBe('');

  // The pick was let go, so two running teams ask for a choice again (the owner
  // does not adopt the single team into the person's pick).
  mode = 'two';
  live(['core', 'core-hotfix']);
  await reread();
  expect(note()).toContain('動いているチームが 2 つあるため、チーム定義名 core で作っています。');
  await openDialog();
  choose(document.querySelector('.prompt-controls select'), 'core-hotfix'); await flush(); await flush();
  await closeDialog();
  expect(note()).toContain('実行中のチーム core-hotfix の名前で作っています。');

  // A failed re-read leaves no old text: the note says so, and no roster stands.
  mode = 'netfail';
  await reread();
  expect(note()).toContain('文を表示できません。');
  expect(panel().querySelector('.cv-topology-section[data-kind=roster]')).toBeNull();

  // A failed first read has no text of its own: the column says it cannot show
  // one (the check's segments are not a second reading any more).
  click(row().querySelector('[data-cv-side="attr"]')); await flush();
  await openTab();
  expect(note()).toContain('文を表示できません。');
  expect(panel().querySelector('.cv-topology-section[data-kind=roster]')).toBeNull();

  // 再取得 reads again, and the column is whole.
  click(row().querySelector('[data-cv-side="attr"]')); await flush();
  await openTab();
  mode = 'two';
  await reread();
  expect(roster()).toContain('core-hotfix-builder');
});
