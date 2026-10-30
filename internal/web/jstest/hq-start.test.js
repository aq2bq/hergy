// r-df61 / d-b023: the 動き column's start button says what it does, and the
// sheet it opens always takes the team's name (empty for the definition's name)
// and a name the server may refuse beside the name's own field.
//
// The answers are the test's, so what the sheet makes of them is what is read;
// the server's own check is TestStartNameFollowsUpAs (internal/app/hq_test.go).
import { expect, test } from 'bun:test';
import { layout, serve, settle, type } from './page.js';
import { h, render } from '../src/lib/preact.js';
import { RunCell } from '../src/lib/hq/run.js';
import { setRunning } from '../src/lib/hq/state.js';
import { startSheet } from '../src/lib/hq/sheets.js';

const options = (over) => Object.assign({
  definition: 'core', place: 'project',
  project: { path: '/work/alpha', name: 'alpha' },
  members: [{ name: 'lead', herdr: 'core-lead' }],
  clash: [], suggested_name: 'core-alpha', new_workspace_name: 'core',
  last_workspace: { id: 'w1', label: 'main' },
  workspaces: [{ id: 'w1', label: 'main', selectable: true, why: 'last' }],
}, over);

const sheet = () => document.querySelector('.hq-sheet');
const nameField = () => sheet().querySelector('#hq-as');

// open presses one start and waits for its options: the sheet's effect reads
// after its first frame, as it does in a browser.
async function open(definition, place, project) {
  startSheet(definition, place, project);
  for (let i = 0; i < 50; i++) {
    await settle();
    if (nameField()) return;
    await new Promise((resolve) => setTimeout(resolve, 10));
  }
}

function page() {
  layout('teams-data', {});
  setRunning({ herdr: { ok: true, text: '' }, teams: [] });
}

test('the start button says what it does, and a team of its own name is shown', async () => {
  history.replaceState({}, '', '/teams');
  page();
  const host = document.createElement('div');
  document.getElementById('view').appendChild(host);

  render(h(RunCell, { definition: 'core', place: 'project', project: '/work/alpha' }), host);
  expect(host.querySelector('[data-hq=start]').textContent).toBe('このプロジェクトで起動する');

  render(h(RunCell, { definition: 'shared', place: 'global', project: '' }), host);
  expect(host.querySelector('[data-hq=start]').textContent).toBe('プロジェクトを選んで起動する');

  // A team started under a name of its own stands in the 動き column, beside
  // the definition's row (r-df61).
  setRunning({
    herdr: { ok: true, text: '' },
    teams: [{
      team: 'dev2', definition: 'core', place: 'project', project: '/work/alpha', project_name: 'alpha',
      workspace: { id: 'w1', label: 'main' }, members: [], workers: [], waiting: [], starting: false,
    }],
  });
  await settle();
  render(h(RunCell, { definition: 'core', place: 'project', project: '/work/alpha' }), host);
  expect(host.querySelector('[data-role=team-name]').textContent).toBe('dev2');
});

test('a project definition opens the named start and takes the team name', async () => {
  history.replaceState({}, '', '/teams');
  page();
  const asked = serve((method, url) => {
    if (url.startsWith('/api/hq/launch/options')) return options({});
    if (url === '/api/hq/launch') return {};
    return undefined;
  });
  await open('core', 'project', '/work/alpha');

  expect(sheet().querySelector('.hq-h b').textContent).toBe('このプロジェクトで起動する');
  expect(nameField().value).toBe('');
  expect(sheet().querySelector('[data-role=clash]')).toBe(null);
  expect(sheet().querySelector('[data-role=same]').textContent).toContain('hergy up core');
  expect(sheet().querySelector('[data-role=same]').textContent).not.toContain('--as');

  type(nameField(), 'dev2');
  await settle();
  expect(sheet().querySelector('[data-role=same]').textContent).toContain('hergy up core --as dev2');

  sheet().querySelector('[data-hq=go]').click();
  await settle();
  const launch = asked.filter((one) => one.url === '/api/hq/launch').pop();
  expect(launch.method).toBe('POST');
  expect(launch.body.name).toBe('dev2');
});

test('an empty team name starts the definition by its own name', async () => {
  history.replaceState({}, '', '/teams');
  page();
  const asked = serve((method, url) => {
    if (url.startsWith('/api/hq/launch/options')) return options({});
    if (url === '/api/hq/launch') return {};
    return undefined;
  });
  await open('core', 'project', '/work/alpha');
  sheet().querySelector('[data-hq=go]').click();
  await settle();
  const launch = asked.filter((one) => one.url === '/api/hq/launch').pop();
  expect(launch.body.name).toBe(undefined);
});

test('a global definition opens the project start, and a clash seeds the suggested name', async () => {
  history.replaceState({}, '', '/teams');
  page();
  serve((method, url) => {
    if (url.startsWith('/api/hq/launch/options')) {
      return options({
        definition: 'shared', place: 'global', project: null,
        members: [], suggested_name: 'shared-alpha',
        clash: [{ team: 'shared', project: '/work/beta' }],
        projects: [{ path: '/work/alpha', name: 'alpha', available: true }],
        last_workspace: undefined,
        workspaces: [{ id: 'w1', label: 'main', selectable: true, why: 'free' }],
      });
    }
    return undefined;
  });
  await open('shared', 'global', '');

  expect(sheet().querySelector('.hq-h b').textContent).toBe('プロジェクトを選んで起動する');
  expect(nameField().value).toBe('shared-alpha');
  expect(sheet().querySelector('[data-role=clash]').textContent).toContain('shared という名前のチームが');
  expect(sheet().querySelector('[data-role=same]').textContent).toContain('hergy up shared --as shared-alpha');
});

test('a name the server refuses stands beside the name field, not with the sheet error', async () => {
  history.replaceState({}, '', '/teams');
  page();
  serve((method, url) => {
    if (url.startsWith('/api/hq/launch/options')) return options({});
    return undefined;
  });
  await open('core', 'project', '/work/alpha');

  const answer = () => globalThis.fetch;
  const was = answer();
  globalThis.fetch = async (url, opts = {}) => {
    if (String(url) === '/api/hq/launch' && opts.method === 'POST') {
      const text = JSON.stringify({ text: 'チームの名前は [a-z][a-z0-9_-]{0,24} の形にしてください: "Dev 2"', at: 'name' });
      return { ok: false, status: 422, text: async () => text, headers: { get: () => 'application/json' } };
    }
    return was(url, opts);
  };
  type(nameField(), 'Dev 2');
  await settle();
  sheet().querySelector('[data-hq=go]').click();
  await settle();

  const why = sheet().querySelector('[data-role=name-error]');
  expect(why).not.toBe(null);
  expect(why.textContent).toContain('チームの名前は');
  expect(sheet().querySelector('[data-role=error]').textContent).toBe('');
});
