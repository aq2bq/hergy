// PROFILES-P1, PROFILES-P11, PROFILES-C1 and PROFILES-ADB6 — walked in a
// browser. Only what the browser alone can see is compared here (d-3e43): the
// row a person presses opens inside the view (d-d475), the box that rewrites the
// people using a renamed profile is ticked from the start, a person's chip puts
// the cursor on the person's row of the assign tab, the YAML text a person
// types is still theirs after the check answered, and the delete of a profile
// goes through whatever shape the file wrote profiles: in (ac-adb6).
import { test } from './fixtures';
import { press, q, read, readTo, set, there, wait } from './helpers';
import { readFileSync, writeFileSync } from 'node:fs';
import path from 'node:path';

test.use({ reads: 'profiles' });

// row is the list row one profile name stands on.
const row = (name: string) => `#rows .row[data-name="${name}"]`;

test('PROFILES-P1 rows open in place', async ({ page, app }) => {
  // P1: the row opens under itself, and the detail it opens stands in the view
  // (d-d475: a press changed something off screen).
  await page.goto(`${app.base}/profiles`);
  await wait(page, there('#rows .row'));
  await press(page, `${q(`${row('sonnet-fast')} .sum`)}`);
  await wait(page, `!${q(`${row('sonnet-fast')} .detail`)}.hidden`);
  const visible = `(function(){var d=${q(`${row('sonnet-fast')} .detail`)};if(!d){return;}var r=d.getBoundingClientRect();return r.top < window.innerHeight && r.bottom > 0;})()`;
  await readTo<boolean>(page, visible, 'the opened detail is not in the view').toBe(true);

  // R1・P1: a name both files hold still counts the readers of the panel's own
  // file, and the box that rewrites them is ticked at first — the write the
  // screen starts with, which no 中 test can see.
  await page.goto(`${app.base}/profiles?open=sonnet-fast`);
  await wait(page, there(row('sonnet-fast')));
  await set(page, q(`${row('sonnet-fast')} .panel[data-file=project] [data-f=name]`), 'sonnet-5.5');
  await wait(page, `!${q(`${row('sonnet-fast')} .panel[data-file=project] .rename`)}.hidden`);
  await readTo<boolean>(
    page,
    `${q(`${row('sonnet-fast')} .panel[data-file=project] .rename input`)}.checked`,
    'the rename box is not ticked at first',
  ).toBe(true);
});

test('PROFILES-P11 usage jumps', async ({ page, app }) => {
  // P11: a person's chip moves to the assign tab, centers the person's row and
  // lights it.
  await page.goto(`${app.base}/profiles?open=opus-plan`);
  await wait(page, there('.uchip'));
  await press(page, q(`${row('opus-plan')} .uchip`));
  await wait(page, there('.team[data-g=core] .arow[data-k="m-0"].flash'));
  await readTo<string>(
    page,
    `document.activeElement.closest('.arow').dataset.k`,
    'the focus is on, want m-0',
  ).toBe('m-0');
});

test('PROFILES-C1 typing in the YAML tab survives the check, and the save writes it', async ({ page, app }) => {
  // C1 is the rewrite's own reason (webui-preact-design 1 章): the check answers
  // while the person is in the field, and a screen drawn again from a string put
  // its own text back and lost the typing. So this walk holds four things the
  // drawing must not take away: the typed text, the problem's press, 元に戻す,
  // and the save that writes the file.
  const yed = `document.querySelector('.yed')`;
  await page.goto(`${app.base}/profiles/yaml`);
  await wait(page, there('.yed .vlist .vhead'));
  const file = await read<string>(page, `${yed}.querySelector('.path').textContent`);
  const before = await read<string>(page, `${yed}.querySelector('.ytext').value`);
  const typed = 'profiles:\n  x: [\n';

  // The server answers with the problem on its line, and the field still holds
  // what the person typed: no draw put the page's own text back. The save shuts
  // only when the check has answered about this text (the file's own warning
  // does not shut it), so that wait is what says the answer is here.
  await set(page, `document.querySelector('.yed .ytext')`, typed);
  await readTo<boolean>(page, `${yed}.querySelector('[data-act=yaml-save]').disabled`, 'the save stands open over an error the file did not have').toBe(true);
  await readTo<string>(page, `${yed}.querySelector('.ytext').value`, 'the drawing put the text it was built with back').toBe(typed);
  await readTo<number>(page, `${yed}.querySelectorAll('.vitem').length`, 'the check said nothing about the text').toBeGreaterThan(0);

  // A problem's press puts the caret on the line it names: line 2 is the one the
  // server called out.
  await press(page, `${yed}.querySelector('.vitem')`);
  await readTo<number>(page, `(function(){var t=${yed}.querySelector('.ytext');return t.value.slice(0, t.selectionStart).split('\\n').length;})()`, 'the caret did not move to the problem').toBe(2);

  // 元に戻す puts the file's own text back.
  await press(page, `${yed}.querySelector('[data-act=yaml-revert]')`);
  await readTo<string>(page, `${yed}.querySelector('.ytext').value`, 'revert did not put the file text back').toBe(before);

  // A change the check takes saves, and the sentence and the file follow it.
  await set(page, `document.querySelector('.yed .ytext')`, `${before}\n# a line the check takes\n`);
  await wait(page, `!${yed}.querySelector('[data-act=yaml-save]').disabled`);
  await press(page, `${yed}.querySelector('[data-act=yaml-save]')`);
  await readTo<string>(page, `${yed}.querySelector('.ok').textContent`, 'the save said nothing').toBe('保存しました');
  const written = readFileSync(path.join(app.proj, file), 'utf8');
  if (!written.endsWith('# a line the check takes\n')) {
    throw new Error(`the save did not write the file: ${JSON.stringify(written.slice(-40))}`);
  }
});

test('PROFILES-ADB6 a profile is deleted from a file hergy could not rewrite', async ({ page, app }) => {
  // ac-adb6, r-430b: the master deleted a profile and hergy stopped with its own
  // bug, so the profile stayed. The file wrote profiles: in a shape the line edit
  // could not take an entry out of: the only profile of a block mapping (the
  // lines left a bare `profiles:`, which reads as a null value and not as the
  // empty mapping the delete means), and a flow mapping on the key's own line
  // (the entry stands on the line the key stands on, so the line edit took the
  // key's line). Both deletes go through, and the file keeps every line the
  // delete did not have to change.
  const file = path.join(app.proj, '.hergy', 'profiles.yaml');
  const cases = [
    { name: 'adb6-one', text: 'version: 1\nprofiles:\n  adb6-one:\n    kind: claude\n', want: 'version: 1\nprofiles: {}\n' },
    { name: 'adb6-doc', text: '---\nversion: 1\nprofiles:\n  adb6-doc:\n    kind: claude\n', want: '---\nversion: 1\nprofiles: {}\n' },
    { name: 'adb6-flow', text: 'version: 1\nprofiles: {adb6-flow: {kind: claude}, adb6-keep: {kind: codex}}\n', want: 'version: 1\nprofiles: {adb6-keep: {kind: codex}}\n' },
  ];
  for (const one of cases) {
    writeFileSync(file, one.text, 'utf8');
    await page.goto(`${app.base}/profiles?open=${one.name}`);
    const panel = `${row(one.name)} .panel[data-file=project]`;
    await wait(page, there(`${panel} [data-act=del]`));
    await press(page, q(`${panel} [data-act=del]`));
    const answer = page.waitForResponse((r) => r.request().method() === 'DELETE' && r.url().includes(`/api/profile/${one.name}`));
    await press(page, q(`${panel} [data-act=del-yes]`));
    const res = await answer;
    if (res.status() !== 200) {
      throw new Error(`the delete of ${one.name} was refused (${res.status()}): ${await res.text()}`);
    }
    if (readFileSync(file, 'utf8') !== one.want) {
      throw new Error(`the delete of ${one.name} wrote ${JSON.stringify(readFileSync(file, 'utf8'))}`);
    }
    await wait(page, `!${q(row(one.name))}`);
  }
});
