// SHOWCASE-H2 on the headquarters' road (r-08ea, ac-ad8c, d-c5fe): the roles a
// new member is named after come from /api/hq/roles, not /api/role. Until they
// have come, ＋メンバー cannot be pressed; once they have, the member is named
// after the first role. The answer is held by the test, so the order is the
// test's and nothing waits on a clock. Moved from the browser walk, which held
// the answer in the browser and read the same two things.
import { expect, test } from 'bun:test';
import { later, layout, serve, settle } from './page.js';
import { click } from './teams-page.js';

const projects = {
  mode: 'listed',
  current: { path: '/work/alpha', name: 'alpha' },
  projects: [{ path: '/work/alpha', name: 'alpha' }],
};

const definition = {
  name: 'core', effective: 'project',
  places: { project: { path: '.hergy/teams/core/team.yaml', hash: 'h1', doc: {
    mission: '注文 API を作り直す',
    members: [{ name: 'lead', role: 'lead', profile: 'coder' }],
    workers: [], turns: [['human', 'lead'], ['lead', 'human']],
  } } },
};

test('＋メンバー waits for the headquarters\' roles, and takes the first one', async () => {
  history.replaceState({}, '', '/teams');
  layout('teams-data', {
    dirs: { project: '.hergy/teams', global: '~/.config/hergy/teams' },
    profiles: ['coder'],
    hq: {
      projects,
      teams: [{ project: projects.projects[0], list: { items: [definition] } }],
      roles: [], profiles: [],
    },
    team: { items: [], files: [] },
  });
  const roles = later();
  serve((method, url) => {
    if (url === '/api/hq/roles') return roles.promise;
    if (url === '/api/hq/profiles') return { groups: [] };
    if (url.startsWith('/api/team/check') && url.indexOf('agreement') < 0) {
      return { problems: [], members: [], preview: { lines: [], diff: {} } };
    }
    if (url.startsWith('/api/team/check')) return { problems: [] };
    return undefined;
  });
  await import('../src/teams/main');
  await settle();

  const row = document.querySelector('.tdef');
  expect(row, 'the definition is listed').not.toBe(null);
  click(row.querySelector('.sum'));
  await settle();
  const add = row.querySelector('[data-cv-act="m-add"]');
  expect(add, 'the row is open with its canvas').not.toBe(null);
  expect(add.disabled, '＋メンバー is pressable before the roles came').toBe(true);

  roles.resolve({ groups: [{
    project: projects.projects[0],
    list: { items: [{ name: 'analyst', effective: 'project', places: { project: { doc: { title: '分析' } } } }] },
  }] });
  await settle();
  expect(add.disabled, '＋メンバー is still held after the roles came').toBe(false);

  click(add);
  await settle();
  const name = document.querySelector('.tdef .cv-side [data-cv-f="m-name"]').value;
  expect(name, 'the new member is not named after a role').not.toMatch(/^member-/);
  expect(name).toBe('analyst-1');
});
