// A rule gets its ID from the save answer: the draft shows the placeholder,
// its switch works before the save, and the returned ID is the one on screen.
import { test, expect } from './fixtures';
import { readFileSync } from 'node:fs';
import path from 'node:path';
import { press, q, read, readTo, set, wait } from './helpers';

test.use({ reads: 'roles' });

test('a new rule displays the ID the save returns', async ({ page, app }) => {
  const file = path.join(app.proj, '.hergy', 'roles', 'lead.yaml');
  const body = '保存の答えで ID が付く画面テスト。';
  await page.goto(`${app.base}/roles?open=lead`);
  const section = '#rows .role.open .panel[data-l=project] .sec[data-si="0"]';
  await wait(page, `!!document.querySelector(${JSON.stringify(`${section} [data-act=rule-add]`)})`);
  const before = await read<number>(page, `document.querySelectorAll(${JSON.stringify(`${section} .rule`)}).length`);
  await press(page, q(`${section} [data-act=rule-add]`));
  await wait(page, `document.querySelectorAll(${JSON.stringify(`${section} .rule`)}).length===${before + 1}`);

  const added = `[...document.querySelectorAll(${JSON.stringify(`${section} .rule`)})].at(-1)`;
  await readTo<string>(page, `${added}.querySelector('.rid b').textContent.trim()`, 'the unsaved rule ID').toBe('保存で付く');
  await readTo<string>(page, `${added}.querySelector('.redit .ridv').textContent.trim()`, 'the opened rule ID').toBe('保存で付く');

  await press(page, `${added}.querySelector('.onsw')`);
  await readTo<string>(page, `${added}.querySelector('.onsw').getAttribute('aria-checked')`, 'the switch before save').toBe('false');
  await press(page, `${added}.querySelector('.onsw')`);
  await readTo<string>(page, `${added}.querySelector('.onsw').getAttribute('aria-checked')`, 'the switch turned back on').toBe('true');
  await set(page, `${added}.querySelector('.redit [data-f=text]')`, body);

  await wait(page, '!document.querySelector("#rows .role.open .panel[data-l=project] .pf [data-act=save]").disabled');
  const response = page.waitForResponse((one) => one.request().method() === 'PUT' && one.url().includes('/api/role/lead'));
  await press(page, q('#rows .role.open .panel[data-l=project] .pf [data-act=save]'));
  await wait(page, '!!document.querySelector("#rows .role.open .ok")');
  const saved = await (await response).json() as { doc: { sections: { rules: { id?: string; text?: string }[] }[] } };
  const savedRule = saved.doc.sections.flatMap((one) => one.rules).find((one) => one.text === body);
  const id = savedRule?.id;
  expect(id, 'hergy returned the assigned ID for the new rule').toMatch(/^[A-HJ-KM-NP-TV-Z][A-HJ-KM-NP-TV-Z0-9]{3}$/);
  const savedRow = `[...document.querySelectorAll(${JSON.stringify(`${section} .rule`)})].find((r)=>r.querySelector('.rtx').textContent.includes(${JSON.stringify(body)}))`;
  await wait(page, `${savedRow}&&${savedRow}.querySelector('.rid b').textContent.trim()!=='保存で付く'`);
  await readTo<string>(page, `${savedRow}.querySelector('.rid b').textContent.trim()`, 'the screen shows the ID in hergy’s save answer').toBe(id);
  expect(readFileSync(file, 'utf8'), 'hergy wrote the assigned ID to the file').toContain(`id: ${id}`);
});
