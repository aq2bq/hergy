// R-3C77 — a save refused because the file moved keeps what the person wrote.
// The walk is ac-ccf0's: the person is in the middle of a change, the test
// writes the file, and the save is pressed. The save does not write over the
// file, the words on the screen stay letter for letter, the notice says what the
// file holds now, and the person chooses: read it again (and go back), compare,
// or write what they wrote over it. No sentence sends them to a fresh page.
import { test, expect } from './fixtures';
import { appendFileSync, readFileSync } from 'node:fs';
import path from 'node:path';
import { press, q, read, readTo, set, there, wait } from './helpers';

// noReload is the sentence the screens no longer say (d-a34a): a refused save
// keeps the draft, so nothing asks for a page to be read again.
async function noReload(page: Parameters<typeof wait>[0]): Promise<void> {
  await readTo<string>(page, 'document.body.innerText', 'a sentence sends the person back to a fresh page').not.toContain('開き直して');
  await readTo<string>(page, 'document.body.innerText', 'a sentence says ディスク where a person reads ファイル').not.toContain('ディスク');
}

test.describe('the role screen', () => {
  test.use({ reads: 'roles' });

  test('R3C77-1 a refused save keeps the draft, shows the file, and writes it over', async ({ page, app }) => {
    const file = path.join(app.proj, '.hergy', 'roles', 'builder.yaml');
    const panel = '#rows .role.open .panel[data-l=project]';
    await page.goto(`${app.base}/roles?open=builder`);
    await wait(page, there(`${panel} [data-f=title]`));
    const title = await read<string>(page, `${q(`${panel} [data-f=title]`)}.value`);
    const typed = `${title} を直した`;
    await set(page, q(`${panel} [data-f=title]`), typed);

    // The test is the other place: it writes the file while the person is here.
    const other = '\n# ほかのところが書いた行\n';
    appendFileSync(file, other);
    const theirs = readFileSync(file, 'utf8');

    await press(page, q(`${panel} [data-act=save]`));
    await wait(page, there(`${panel} .conflict`));

    // Nothing was written over the other place's text, and the words on the
    // screen did not move.
    expect(readFileSync(file, 'utf8'), 'the refused save wrote the file').toBe(theirs);
    await readTo<string>(page, `${q(`${panel} [data-f=title]`)}.value`, 'the refused save lost the words on the screen').toBe(typed);
    // The notice names the file and the hour it was written.
    await readTo<boolean>(page, `/${'builder\\.yaml'}/.test(${q(`${panel} .conflict .cf-text`)}.textContent)`, 'the notice does not name the file').toBe(true);
    await readTo<boolean>(page, `/（\\d{1,2}:\\d{2}）/.test(${q(`${panel} .conflict .cf-text`)}.textContent)`, 'the notice does not say when the file was written').toBe(true);

    // 見比べる reads the file's text beside the draft: the other place's line is
    // one of the lines only the file holds.
    await press(page, q(`${panel} [data-act=cf-compare]`));
    await wait(page, there(`${panel} .conflict .cf-compare tbody tr`));
    await readTo<string>(page, `[...document.querySelectorAll(${JSON.stringify(`${panel} .conflict .cf-compare tbody tr.cf-del td`)})].map(function(td){return td.textContent}).join('|')`, 'the compare does not show the line the file has now').toContain('ほかのところが書いた行');

    // 上書き writes what the person wrote over the file they were shown.
    await press(page, q(`${panel} [data-act=cf-overwrite]`));
    await wait(page, there(`${panel} .ok`));
    await readTo<boolean>(page, `${q(`${panel} .conflict`)}===null`, 'the notice stayed after the overwrite').toBe(true);
    const written = readFileSync(file, 'utf8');
    expect(written, 'the overwrite did not write the draft').toContain(typed);
    await noReload(page);
  });

  test('R3C77-2 元に戻す reads the file in, and 取り消す brings the words back', async ({ page, app }) => {
    const file = path.join(app.proj, '.hergy', 'roles', 'builder.yaml');
    const panel = '#rows .role.open .panel[data-l=project]';
    await page.goto(`${app.base}/roles?open=builder`);
    await wait(page, there(`${panel} [data-f=title]`));
    const title = await read<string>(page, `${q(`${panel} [data-f=title]`)}.value`);
    const typed = `${title} を直した`;
    await set(page, q(`${panel} [data-f=title]`), typed);
    appendFileSync(file, '\n# ほかのところが書いた行\n');
    await press(page, q(`${panel} [data-act=save]`));
    await wait(page, there(`${panel} .conflict`));

    // 元に戻す reads the file's own text in: the draft is the file as it is.
    await press(page, q(`${panel} [data-act=cf-revert]`));
    await readTo<string>(page, `${q(`${panel} [data-f=title]`)}.value`, '元に戻す did not read the file in').toBe(title);
    await readTo<boolean>(page, `!!${q(`${panel} .conflict`)}.querySelector('[data-act=cf-undo]')`, '元に戻す offers no way back').toBe(true);

    // 取り消す puts the words the person wrote back.
    await press(page, q(`${panel} [data-act=cf-undo]`));
    await readTo<string>(page, `${q(`${panel} [data-f=title]`)}.value`, '取り消す did not bring the words back').toBe(typed);
    await readTo<boolean>(page, `${q(`${panel} .conflict`)}===null`, 'the notice stayed after 取り消す').toBe(true);
  });
});

test.describe('the team screen', () => {
  test.use({ reads: 'teams' });

  test('R3C77-3 the definition’s description is kept and written over', async ({ page, app }) => {
    const file = path.join(app.proj, '.hergy', 'teams', 'core', 'team.yaml');
    const shape = '.tdef[data-name="core"] .panel.team-p';
    await page.goto(`${app.base}/teams?open=core`);
    await wait(page, there(`${shape} [data-act=t-save]`));
    const typed = 'ほかのところと重ならない説明の書き直し。';
    await set(page, q('.tdef[data-name="core"] .in-description'), typed);
    appendFileSync(file, '\n# ほかのところが書いた行\n');
    const theirs = readFileSync(file, 'utf8');
    await press(page, q(`${shape} [data-act=t-save]`));
    await wait(page, there(`${shape} .conflict`));
    expect(readFileSync(file, 'utf8'), 'the refused save wrote the file').toBe(theirs);
    await readTo<string>(page, `${q('.tdef[data-name="core"] .in-description')}.value`, 'the refused save lost the description').toBe(typed);
    await press(page, q(`${shape} [data-act=cf-overwrite]`));
    await wait(page, there(`${shape} .ok`));
    await readTo<boolean>(page, `${q(`${shape} .conflict`)}===null`, 'the notice stayed after the overwrite').toBe(true);
    expect(readFileSync(file, 'utf8'), 'the overwrite did not write the description').toContain(typed);
  });

  test('R3C77-4 the team’s rules are kept and read in', async ({ page, app }) => {
    const file = path.join(app.proj, '.hergy', 'teams', 'core', 'agreement.yaml');
    await page.goto(`${app.base}/teams?open=core`);
    await wait(page, there('.tdef[data-name="core"] .panel.team-p'));
    await press(page, q('.tdef[data-name="core"] .dtab[data-t=agree]'));
    const panel = '.tdef[data-name="core"] .panel.agree-p';
    await wait(page, there(`${panel} [data-act=save]`));
    await press(page, q(`${panel} .sec[data-si="0"] [data-act=rule-add]`));
    const added = `[...document.querySelectorAll(${JSON.stringify(`${panel} .sec[data-si="0"] .rule`)})].at(-1)`;
    await wait(page, `!!${added}`);
    const typed = 'ほかのところと重ならない規則。';
    await set(page, `${added}.querySelector('[data-f=text]')`, typed);
    appendFileSync(file, '\n# ほかのところが書いた行\n');
    const theirs = readFileSync(file, 'utf8');
    await press(page, q(`${panel} [data-act=save]`));
    await wait(page, there(`${panel} .conflict`));
    expect(readFileSync(file, 'utf8'), 'the refused save wrote the rules file').toBe(theirs);
    await readTo<string>(page, `${added}.querySelector('[data-f=text]').value`, 'the refused save lost the rule').toBe(typed);
    await press(page, q(`${panel} [data-act=cf-overwrite]`));
    await wait(page, there(`${panel} .ok`));
    await readTo<boolean>(page, `${q(`${panel} .conflict`)}===null`, 'the notice stayed after the overwrite').toBe(true);
    expect(readFileSync(file, 'utf8'), 'the overwrite did not write the rule').toContain(typed);
  });

  test('R3C77-5 the YAML tab keeps its text and writes it over', async ({ page, app }) => {
    const file = path.join(app.proj, '.hergy', 'teams', 'core', 'team.yaml');
    await page.goto(`${app.base}/teams/yaml`);
    await wait(page, there('.yed .ytext'));
    const before = await read<string>(page, `document.querySelector('.yed .ytext').value`);
    const typed = `${before}\n# 書きかけの行\n`;
    await set(page, q('.yed .ytext'), typed);
    appendFileSync(file, '\n# ほかのところが書いた行\n');
    const theirs = readFileSync(file, 'utf8');
    await wait(page, `!${q('.yed [data-act=yaml-save]')}.disabled`);
    await press(page, q('.yed [data-act=yaml-save]'));
    await wait(page, there('.yed .conflict'));
    expect(readFileSync(file, 'utf8'), 'the refused save wrote the YAML file').toBe(theirs);
    await readTo<string>(page, `${q('.yed .ytext')}.value`, 'the refused save lost the text').toBe(typed);
    await press(page, q('.yed [data-act=cf-overwrite]'));
    await wait(page, there('.yed .ok'));
    const written = readFileSync(file, 'utf8');
    expect(written, 'the overwrite did not write the text').toContain('# 書きかけの行');
    await noReload(page);
  });
});

test.describe('the profile screen', () => {
  test.use({ reads: 'profiles' });

  test('R3C77-6 a profile is kept and written over', async ({ page, app }) => {
    const file = path.join(app.proj, '.hergy', 'profiles.yaml');
    const panel = '[data-key^="sonnet-fast|"]';
    await page.goto(`${app.base}/profiles?open=sonnet-fast`);
    await wait(page, there(`${panel} [data-act=save]`));
    const typed = '21000';
    await set(page, `${q(panel)}.querySelector('[data-f=ev]')`, typed);
    appendFileSync(file, '# ほかのところが書いた行\n');
    const theirs = readFileSync(file, 'utf8');
    await press(page, q(`${panel} [data-act=save]`));
    await wait(page, there(`${panel} .conflict`));
    expect(readFileSync(file, 'utf8'), 'the refused save wrote profiles.yaml').toBe(theirs);
    await readTo<string>(page, `${q(panel)}.querySelector('[data-f=ev]').value`, 'the refused save lost the value').toBe(typed);
    await press(page, q(`${panel} [data-act=cf-overwrite]`));
    await wait(page, there(`${panel} .ok`));
    await readTo<boolean>(page, `${q(`${panel} .conflict`)}===null`, 'the notice stayed after the overwrite').toBe(true);
    expect(readFileSync(file, 'utf8'), 'the overwrite did not write the value').toContain(typed);
  });
});
