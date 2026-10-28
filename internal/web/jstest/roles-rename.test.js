// ROLES-N1, the role screen sends the rename's mark by the name the server reads
// (r-b0bc, ac-2abf, n-b06c): a save whose role name has changed carries
// rename_users: true — the field apiIn holds — so the teams that name the old
// role are rewritten with it. The answers are the test's, so the body the screen
// sends is what is read.
import { expect, test } from 'bun:test';
import { layout, serve, settle } from './page.js';

const roleDoc = (name) => ({
  name, title: '', revision: 1,
  sections: [{ heading: '進め方', level: 0, intro: '', rules: [{ id: 'B-1', text: '仕様を確かめる。', note: '', in_prompt: true }] }],
});

// island is the roles page read: one role, builder, named by one member of a team.
const island = {
  places: { project: '.hergy/roles/', global: '~/.config/hergy/roles/' },
  texts: {},
  role: {
    items: [{
      name: 'builder',
      places: { project: { doc: roleDoc('builder'), hash: 'h1', text: 'version: 1\n', path: '.hergy/roles/builder.yaml' } },
      summary: { rules: 1 },
    }],
  },
  team: {
    items: [{
      name: 'core', effective: 'project', summary: { members: [{ name: 'impl', role: 'builder' }] },
      places: { project: { doc: { mission: '注文 API を作り直す', members: [{ name: 'impl', role: 'builder', profile: 'coder' }], workers: [], turns: [] } } },
    }],
  },
};

test('a role renamed and saved sends rename_users: true, the field the server reads', async () => {
  history.replaceState({}, '', '/roles?open=builder');
  layout('roles-data', island);
  const asked = serve((method, url, body) => {
    if (method === 'PUT' && url.startsWith('/api/role/')) return { doc: body.doc, hash: 'h2', text: 'version: 1\n' };
    if (url.startsWith('/api/role/check')) return { problems: [], preview: { lines: [], chars: 0 }, effect: { diff: { added: [], removedBefore: [] }, turns: [] } };
    if (url.startsWith('/api/role')) return { items: [] };
    return undefined;
  });
  await import('../src/roles/main');
  await settle();

  const panel = () => document.querySelector('#rows .role.open .panel[data-l=project]');
  const field = () => panel().querySelector('[data-f=name]');
  const box = () => panel().querySelector('.rename input[type=checkbox]');

  // Rename the role: the box that says the teams are rewritten stands, checked.
  field().value = 'builder-2';
  field().dispatchEvent(new Event('input', { bubbles: true }));
  await settle();
  expect(box()).not.toBe(null);
  expect(box().checked).toBe(true);

  const save = panel().querySelector('[data-act=save]');
  expect(save.disabled).toBe(false);
  save.dispatchEvent(new Event('click', { bubbles: true }));
  await settle();

  const put = asked.filter((one) => one.method === 'PUT').pop();
  expect(put).not.toBe(undefined);
  expect(put.body.rename_users).toBe(true);
  // The server's field is rename_users; a body that names it renameUsers is the
  // shape of the bug this holds out (n-b06c).
  expect('renameUsers' in put.body).toBe(false);
});
