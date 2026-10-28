// ROLES-N1b, the same save with the rename's mark off (r-b0bc, ac-2abf): the
// screen sends rename_users: false, so the teams that name the old role are left
// as they are. The renaming carries into the teams only while the box beside the
// name stands checked. The answers are the test's, so the body the screen sends
// is what is read.
import { expect, test } from 'bun:test';
import { layout, serve, settle } from './page.js';

const roleDoc = (name) => ({
  name, title: '', revision: 1,
  sections: [{ heading: '進め方', level: 0, intro: '', rules: [{ id: 'B-1', text: '仕様を確かめる。', note: '', in_prompt: true }] }],
});

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

test('a role renamed with the rename box off sends rename_users: false', async () => {
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
  const box = () => panel().querySelector('.rename input[type=checkbox]');
  const field = panel().querySelector('[data-f=name]');

  // Rename the role, then take the mark off: the teams keep the old name.
  field.value = 'builder-2';
  field.dispatchEvent(new Event('input', { bubbles: true }));
  await settle();
  expect(box()).not.toBe(null);
  box().checked = false;
  box().dispatchEvent(new Event('change', { bubbles: true }));
  await settle();
  expect(box().checked).toBe(false);

  const save = panel().querySelector('[data-act=save]');
  expect(save.disabled).toBe(false);
  save.dispatchEvent(new Event('click', { bubbles: true }));
  await settle();

  const put = asked.filter((one) => one.method === 'PUT').pop();
  expect(put).not.toBe(undefined);
  expect(put.body.rename_users).toBe(false);
});
