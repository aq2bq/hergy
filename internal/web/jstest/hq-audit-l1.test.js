// r-1db7 L1: a headquarters whose lists could not be read says why, rather than
// falling back to the one-project lists (no project pills, the 置き場所 column,
// no 動き column). The island is the server's, so what the list makes of it is
// what is read.
import { expect, test } from 'bun:test';
import { layout, serve, settle } from './page.js';

test('a headquarters read that failed says the sentence instead of the old lists', async () => {
  history.replaceState({}, '', '/teams');
  layout('teams-data', {
    dirs: { project: '.hergy/teams', global: '~/.config/hergy/teams' },
    profiles: ['coder'],
    hq: {
      projects: {
        mode: 'listed',
        current: { path: '/work/alpha', name: 'alpha' },
        projects: [{ path: '/work/alpha', name: 'alpha', listed: true }],
      },
      teams: [], roles: [], profiles: [],
      error: 'そのプロジェクトは本部に並んでいません',
    },
    team: { items: [], files: [] },
  });
  serve(() => undefined);
  await import('../src/teams/main');
  await settle();

  const error = document.querySelector('[data-role=hq-error]');
  expect(error).not.toBe(null);
  expect(error.textContent).toBe('そのプロジェクトは本部に並んでいません');
  // The lists of one project are not drawn as though the projects were not there.
  expect(document.querySelector('.list')).toBe(null);
});
