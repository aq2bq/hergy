// r-0b69: the prompt column builds its text where the whole-text panel builds
// it, with the same running team, and says which name it used. A team booted
// under another name (up --as) then shows the same member name in both places.
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

test('the column builds with the running team\'s name, follows the panel\'s pick, and falls back with a reason', async () => {
  history.replaceState({}, '', '/teams?open=core');
  layout('teams-data', island());
  const asked = serve((method, url, body) => {
    if (url.startsWith('/api/team/prompt-comparison')) {
      if (mode === 'fail') throw new Error('プロジェクトを読めません');
      if (mode === 'one') return comparison({ team: 'core-hotfix', selected: { team: 'core-hotfix', project: '/example' }, runs: [{ team: 'core-hotfix', project: '/example' }] });
      const picked = body && body.running_team === 'core-hotfix';
      return comparison({
        runs: [{ team: 'core', project: '/example' }, { team: 'core-hotfix', project: '/example' }],
        selected: picked ? { team: 'core-hotfix', project: '/example' } : null,
        team: picked ? 'core-hotfix' : 'core',
        segments: picked ? aliasSegments : memberSegments,
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
    click(row().querySelector('[data-cv-side="prompt"]')); await settle(); await settle();
  };
  click(row().querySelector('.cv-n.member[data-cv-n="lead"]')); await settle();
  await openTab();

  // One running team under another name: the column takes its name and says so.
  expect(panel().querySelector('.cv-note').textContent).toContain('実行中のチーム core-hotfix の名前で作っています。');
  expect(panel().querySelector('.cv-topology-section[data-kind=roster] pre').textContent).toContain('core-hotfix-builder');
  const firstAsk = asked.filter((one) => one.url.startsWith('/api/team/prompt-comparison'));
  expect(firstAsk.length).toBe(1);
  expect(firstAsk[0].body.member).toBe('lead');
  expect(firstAsk[0].body.running_team).toBe('');

  // Two running teams and none picked: the column stands on the definition's
  // name, and takes the team the whole-text panel picks.
  mode = 'two';
  click(row().querySelector('[data-cv-side="attr"]')); await settle();
  await openTab();
  expect(panel().querySelector('.cv-note').textContent).toContain('動いているチームが 2 つあるため、定義名 core で作っています。');
  click(panel().querySelector('.cv-prompt button')); await settle(); await settle();
  const select = document.querySelector('.prompt-controls select');
  expect(select).not.toBe(null);
  choose(select, 'core-hotfix'); await settle(); await settle();
  click([...document.querySelectorAll('.prompt-comparison button')].find((x) => x.textContent === '閉じる')); await settle(); await settle();
  expect(panel().querySelector('.cv-note').textContent).toContain('実行中のチーム core-hotfix の名前で作っています。');
  expect(panel().querySelector('.cv-topology-section[data-kind=roster] pre').textContent).toContain('core-hotfix-builder');

  // A text the running name cannot build falls back to the check's segments
  // and says why.
  mode = 'fail';
  click(row().querySelector('[data-cv-side="attr"]')); await settle();
  await openTab();
  expect(panel().querySelector('.cv-note').textContent).toContain('定義名 core で作っています。');
  expect(panel().querySelector('.cv-topology-section[data-kind=roster] pre').textContent).toContain('core-builder');
});
