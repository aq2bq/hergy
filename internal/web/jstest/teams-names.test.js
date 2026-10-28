// TEAMS-V1, a definition whose names do not resolve: the row opens under
// itself, its head carries the file's path as the project reads it (TEAMS-V4),
// the agreement offers no 元に戻す with nothing changed (R5), and a definition
// whose worker names a profile that is not there is drawn like one that reads,
// its one problem standing on that worker (R10). None of this needs a browser:
// the answer is the test's, so the screen's own reading of it is what is read.
import { expect, test } from 'bun:test';
import { layout, serve, settle } from './page.js';
import { click } from './teams-page.js';

const agreement = (name) => ({
  path: `.hergy/teams/${name}/agreement.yaml`, hash: 'a1', has: true,
  doc: {
    name: '', title: `${name} チームのルール`, revision: 1,
    sections: [{ heading: '使命', rules: [{
      id: 'TEAM-1', text: 'このチームの約束。', why: 'だから。',
      send_why: true, in_prompt: true, note: '', report: '', tests: [], origin: [],
    }] }],
  },
});

const teams = {
  dirs: { project: '.hergy/teams', global: '~/.config/hergy/teams' },
  profiles: ['coder', 'gemini-flash'],
  team: {
    items: [
      {
        name: 'core', effective: 'project',
        places: { project: { path: '.hergy/teams/core/team.yaml', hash: 'h1', doc: {
          mission: '注文 API を作り直す',
          members: [{ name: 'lead', role: 'lead', profile: 'coder' }, { name: 'builder', role: 'builder', profile: 'coder' }],
          workers: [{ from: 'lead', purpose: '実装', profile: 'coder' }],
          turns: [['human', 'lead'], ['lead', 'human']],
          agreement: agreement('core'),
        } } },
      },
      {
        name: 'docs', effective: 'project',
        places: { project: { path: '.hergy/teams/docs/team.yaml', hash: 'h2', doc: {
          mission: '利用者向けの文書を整える',
          members: [{ name: 'lead', role: 'lead', profile: 'coder' }, { name: 'writer', role: 'writer', profile: 'coder' }],
          // 翻訳 names a profile profiles.yaml does not hold: the definition is
          // still drawn, and its one problem stands on this worker (R10).
          workers: [{ from: 'writer', purpose: '翻訳', profile: 'gemini-flash' }],
          turns: [['human', 'lead'], ['lead', 'human']],
          agreement: agreement('docs'),
        } } },
      },
    ],
    files: [],
  },
};

test('a definition whose names do not resolve opens with its path, no revert, and its own figure', async () => {
  history.replaceState({}, '', '/teams');
  layout('teams-data', teams);
  serve((method, url, body) => {
    if (url.startsWith('/api/team/check') && body && body.part === 'team') {
      const problems = body.name === 'docs'
        ? [{ sev: 'err', at: 'workers[0].profile', text: 'プロファイル gemini-flash がありません' }]
        : [];
      return { problems, members: [], preview: { lines: [], diff: {} } };
    }
    if (url.startsWith('/api/team/check')) return { problems: [] };
    if (url.startsWith('/api/role')) return { items: [{ name: 'lead' }, { name: 'builder' }, { name: 'writer' }] };
    return undefined;
  });
  await import('../src/teams/main');
  await settle();

  // P1: the press opens the row, and the panel stands under its own summary —
  // not in another row's place.
  const core = document.querySelector('.tdef[data-name="core"]');
  click(core.querySelector('.sum'));
  await settle();
  expect(core.classList.contains('open')).toBe(true);
  const detail = core.querySelector('.sum').nextElementSibling;
  expect(detail).not.toBe(null);
  expect(detail.classList.contains('detail')).toBe(true);
  expect(detail.querySelector('.panel.team-p')).not.toBe(null);

  // TEAMS-V4: the head is the path alone, as the project reads it.
  expect(core.querySelector('.panel.team-p .ph .path').textContent).toBe('.hergy/teams/core/team.yaml');

  // R5: nothing has changed, so the agreement offers no 元に戻す.
  click(core.querySelector('.dtabs .dtab[data-t=agree]'));
  await settle();
  const revert = core.querySelector('.panel.agree-p .pf [data-act=revert]');
  expect(revert).not.toBe(null);
  expect(revert.hidden).toBe(true);

  // R10: docs — a worker names a profile that is not there — is drawn like one
  // that reads, saves, and its one problem stands on the worker it names.
  const docs = document.querySelector('.tdef[data-name="docs"]');
  click(docs.querySelector('.sum'));
  await settle();
  expect(docs.querySelector('.panel.team-p .cv-canvas svg')).not.toBe(null);
  expect(docs.querySelector('.panel.team-p footer.pf [data-act=t-save]')).not.toBe(null);
  expect(docs.querySelector('.vlist .vitem .msg').textContent).toContain('gemini-flash');
  const workers = [...docs.querySelectorAll('.panel.team-p .cv-n.worker.err')];
  expect(workers.map((el) => el.dataset.cvW)).toEqual(['0']);
});
