// r-0b69: the prompt column builds its text where the whole-text panel builds
// it, with the same running team, and says which name it used. A team booted
// under another name (up --as) then shows the same member name in both places,
// and the note always names the team the shown text was built with.
import { expect, test } from 'bun:test';
import { layout, serve, settle } from './page.js';
import { checkAnswer, choose, click, island, memberSegments } from './teams-page.js';

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

// mode is what the comparison API answers with, changed between the phases.
let mode = 'one';

test('the note names the team the shown text was built with, in every case', async () => {
  history.replaceState({}, '', '/teams?open=core');
  layout('teams-data', island());
  const asked = serve((method, url, body) => {
    if (url.startsWith('/api/team/prompt-comparison')) {
      if (mode === 'netfail') throw new Error('プロジェクトを読めません');
      const sent = (body && body.running_team) || '';
      if (mode === 'one') return comparison({ team: 'core-hotfix', selected: { team: 'core-hotfix', project: '/example' }, runs: [{ team: 'core-hotfix', project: '/example' }] });
      if (mode === 'stale') {
        // core-hotfix stopped: only core runs now, and a stale pick is not
        // taken by the server, so the column has to let it go itself.
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
  await import('../src/teams/main'); await settle();
  const row = () => document.querySelector('.tdef[data-name="core"]');
  const panel = () => row().querySelector('.cv-topology-prompt');
  const openTab = async () => {
    click(row().querySelector('[data-cv-side="prompt"]')); await settle(); await settle(); await settle(); await settle();
  };
  const reopen = async () => {
    click(row().querySelector('[data-cv-side="attr"]')); await settle();
    await openTab();
  };
  const note = () => panel().querySelector('.cv-note').textContent;
  const roster = () => panel().querySelector('.cv-topology-section[data-kind=roster] pre').textContent;

  click(row().querySelector('.cv-n.member[data-cv-n="lead"]')); await settle();
  await openTab();

  // One running team under another name.
  expect(note()).toContain('実行中のチーム core-hotfix の名前で作っています。');
  expect(roster()).toContain('core-hotfix-builder');
  const firstAsk = asked.filter((one) => one.url.startsWith('/api/team/prompt-comparison'));
  expect(firstAsk.length).toBeGreaterThan(0);
  expect(firstAsk[0].body.member).toBe('lead');
  // The first reading names no team: a single running team is the server's pick.
  expect(firstAsk[0].body.running_team).toBe('');
  expect(firstAsk.every((one) => one.body.member === 'lead')).toBe(true);

  // The team state cannot be read: the reason is shown, not "no team runs".
  // The answer also lets the picked team go, which the next phase needs.
  mode = 'error';
  await reopen();
  expect(note()).toContain('チームの状態を読めません そのため、定義名 core で作っています。');

  // Two runs, none picked: the definition's name. The panel's pick is taken.
  mode = 'two';
  await reopen();
  expect(note()).toContain('動いているチームが 2 つあるため、定義名 core で作っています。');
  click(panel().querySelector('.cv-prompt button')); await settle(); await settle();
  choose(document.querySelector('.prompt-controls select'), 'core-hotfix'); await settle(); await settle();
  click([...document.querySelectorAll('.prompt-comparison button')].find((x) => x.textContent === '閉じる')); await settle(); await settle();
  expect(note()).toContain('実行中のチーム core-hotfix の名前で作っています。');

  // The picked team stops, and the full panel is not opened: the column lets
  // the pick go and takes the one team still running.
  mode = 'stale';
  await reopen();
  expect(note()).toContain('実行中のチーム core の名前で作っています。');
  const afterStale = asked.filter((one) => one.url.startsWith('/api/team/prompt-comparison'));
  expect(afterStale[afterStale.length - 1].body.running_team).toBe('');

  // A failed re-read keeps the text it had, and the note names that text's team.
  // The dialog stays open so its own last answer still names the teams to pick.
  mode = 'two';
  await reopen();
  expect(note()).toContain('実行中のチーム core の名前で作っています。');
  click(panel().querySelector('.cv-prompt button')); await settle(); await settle();
  choose(document.querySelector('.prompt-controls select'), 'core-hotfix'); await settle(); await settle();
  expect(note()).toContain('実行中のチーム core-hotfix の名前で作っています。');
  mode = 'netfail';
  choose(document.querySelector('.prompt-controls select'), 'core'); await settle(); await settle(); await settle();
  expect(note()).toContain('直前に作った文は、実行中のチーム core-hotfix の名前で作ったものです。');
  expect(roster()).toContain('core-hotfix-builder');
  click([...document.querySelectorAll('.prompt-comparison button')].find((x) => x.textContent === '閉じる')); await settle(); await settle();

  // A failed first read has no text of its own: the check's segments stand in,
  // built on the definition's name.
  await reopen();
  expect(note()).toContain('定義名 core で作っています。');
  expect(roster()).toContain('core-builder');
});
