// ROLES-P5's bug (n-d0c9, ac-cdff, r-7ea6): a project role whose file name is
// Japanese (roles/進行役.yaml, no title) is copied to the global layer; the
// check's answer stands on the name field, in the screen's own words, and the
// save is held before it is pressed. The answer is the test's, so what the panel
// makes of it is what is read (the server's answer is internal/web's
// TestRoleNameThatIsNotAnIdentifierIsANameProblem).
import { expect, test } from 'bun:test';
import { layout, serve, settle } from './page.js';

const doc = {
  name: '進行役', title: '', revision: 1,
  sections: [{
    heading: '進め方', level: 0, intro: '',
    rules: [{ id: 'PM-1', text: '今日決めることを 1 つ書く。', note: '', in_prompt: true }],
  }],
};

const island = {
  places: { project: '.hergy/roles/', global: '~/.config/hergy/roles/' },
  texts: {},
  role: {
    items: [{
      name: '進行役',
      places: { project: { doc, hash: 'h1', text: 'version: 1\n', path: '.hergy/roles/進行役.yaml' } },
      summary: {},
    }],
  },
  team: { items: [] },
};

const order = [
  'ロール名「進行役」は使えません。ロール名は英小文字で始め、英小文字・数字・-・_ で 32 文字までにします。',
  '日本語の呼び名はタイトルに書きます。ロールの画面でロール名を変えて保存すると、そのロールを使うチーム定義も書き換わります',
].join('');

// answer is the check's answer for a document named 進行役: the problem stands
// on the name field.
const answer = {
  problems: [{ sev: 'err', at: 'name', field: 'name', section: -1, rule: -1, text: order }],
  preview: { lines: [], chars: 0 },
  effect: { diff: { added: [], removedBefore: [] }, turns: [] },
};

function click(el) {
  el.dispatchEvent(new Event('click', { bubbles: true }));
}

test('a copied role named 進行役 shows the name problem and holds the save', async () => {
  history.replaceState({}, '', '/roles?open=進行役');
  layout('roles-data', island);
  const asked = serve((method, url, body) => {
    if (url.startsWith('/api/role/check')) {
      return body && body.doc && body.doc.name === '進行役' ? answer : { problems: [], preview: { lines: [], chars: 0 } };
    }
    if (url.startsWith('/api/role')) return { items: [] };
    return undefined;
  });
  await import('../src/roles/main');
  await settle();

  const row = document.querySelector('#rows .role.open');
  expect(row, 'the role named 進行役 is open').not.toBe(null);

  // The saved project document already says it: the sentence stands under ロール名.
  const project = row.querySelector('.panel[data-l=project]');
  const projectName = project.querySelector('.errline.name');
  expect(projectName).not.toBe(null);
  expect(projectName.hidden).toBe(false);
  expect(projectName.textContent).toBe(order);
  expect(project.querySelector('[data-act=save]').disabled, 'the save of a role named 進行役 is pressable').toBe(true);

  // Copy to the global layer: the copy's check answers at once, and the new
  // panel says the same before the save is pressed.
  click(project.querySelector('[data-act=copy]'));
  await settle();
  const global = row.querySelector('.panel[data-l=global]');
  expect(global, 'the copy made a global panel').not.toBe(null);
  const globalName = global.querySelector('.errline.name');
  expect(globalName).not.toBe(null);
  expect(globalName.hidden).toBe(false);
  expect(globalName.textContent).toBe(order);
  expect(global.querySelector('[data-act=save]').disabled, 'the copy of a role named 進行役 is pressable').toBe(true);

  // The name's sentence stands once, under the field — not again in the
  // document-wide list.
  const docList = [...global.querySelectorAll('.dhvm .vm-item')].map((el) => el.textContent).join('\n');
  expect(docList).not.toContain('ロール名「進行役」');

  // Nothing was written: the screen held the save before it was pressed.
  expect(asked.filter((one) => one.method === 'PUT')).toEqual([]);
});
