// R-1EE4 — a letter a person is typing is the field's, and the screen's own
// redrawing (a check, a preview, the figure, the count of what is unsaved) does
// not take the focus away or lose the letters. Every text field of the team, role
// and profile screens is typed into one letter at a time, sixty letters to a
// field, with one IME conversion committed in the middle (ac-0bcb).
//
// The fields are written by lib/field (one mechanism): the value goes into the
// field only when the field stands somewhere else than the screen says, and
// never while an IME is composing (r-1ee4).
import { test, expect, type Page } from './fixtures';
import { press, q, readTo, there, wait } from './helpers';

// SIXTY is the letters one field takes: sixty of them, with a word in the middle
// made by an IME and committed at once.
const SIXTY = '打ち込んでいる途中で画面が描き直されても、打った字が消えず、フォーカスも外れないことを確かめる文です。あいうえおかきくけ';

// held is the letters one field holds, as the page reads it: read again until it
// is what the walk expects, because a field's value follows the screen's drawing.
const held = (find: string) => `(function(){var el=${find};return el?el.value.length:-1;})()`;

// typist types one text into one field, one letter at a time with a pause
// between them, and answers what went wrong: where the field stopped being the
// page's focus, and where it lost a letter. Three letters at the end go in
// through the browser's own composition events and are committed as one word, the
// way an IME commits 日本語. The answers are collected while the letters go in:
// the focus a walk watches for must not be asked for twice, so the field is read
// as it stands after each letter.
async function typist(page: Page, find: string, text: string, gap = 25): Promise<string[]> {
  const chars = [...text];
  const problems: string[] = [];
  const start = Number(await page.evaluate(held(find)));
  if (start < 0) return [`no field for ${find}`];
  const look = async (want: number): Promise<void> => {
    const said = (await page.evaluate(
      `(function(){var el=${find};if(!el)return null;var a=document.activeElement;return JSON.stringify([a===el,el.value.length]);})()`,
    )) as string | null;
    if (said === null) {
      problems.push(`the field went away at ${want - start} letters`);
      return;
    }
    const [isFocused, length] = JSON.parse(said) as [boolean, number];
    if (!isFocused) problems.push(`the focus left the field after ${want} letters`);
    if (length !== want) problems.push(`the field holds ${length} letters, want ${want}`);
  };
  const cdp = await page.context().newCDPSession(page);
  const imeAt = chars.length - 3;
  for (let i = 0; i < chars.length; i++) {
    if (i === imeAt) {
      const word = chars.slice(i, i + 3).join('');
      for (let n = 1; n <= 3; n++) {
        const composing = chars.slice(i, i + n).join('');
        await cdp.send('Input.imeSetComposition', { text: composing, selectionStart: composing.length, selectionEnd: composing.length });
        await page.waitForTimeout(gap);
        await look(start + i + n);
      }
      await cdp.send('Input.insertText', { text: word });
      await page.waitForTimeout(gap);
      await look(start + i + 3);
      i += 2;
      continue;
    }
    await page.keyboard.type(chars[i]);
    await page.waitForTimeout(gap);
    await look(start + i + 1);
  }
  await cdp.detach();
  return problems;
}

test.describe('the team screen’s text fields', () => {
  test.use({ reads: 'teams' });

  test('TYPING-1 the description and the name hold every letter with the focus', async ({ page, app }) => {
    await page.route('**/api/hq/running', (route) => route.fulfill({ json: { herdr: { ok: true, text: '' }, teams: [] } }));
    await page.goto(`${app.base}/teams?open=core`);
    await wait(page, there('.tdef[data-name="core"] .in-description'));

    // The row's own description, in the list: the check rides on every letter and
    // the preview, the figure and the counts are drawn again under the typing.
    const description = q('.tdef[data-name="core"] .in-description');
    await page.click('.tdef[data-name="core"] .in-description');
    const before = Number(await page.evaluate(held(description)));
    const descriptionProblems = await typist(page, description, SIXTY);
    expect(descriptionProblems, 'the description lost the focus or a letter').toEqual([]);
    await readTo<number>(page, held(description), 'the description does not hold the sixty letters').toBe(before + [...SIXTY].length);
    // Enter is how an IME confirms the word it is making: the row the field
    // stands in does not shut under the person, and the field keeps the focus.
    await page.keyboard.press('Enter');
    await page.waitForTimeout(150);
    await readTo<string>(
      page,
      `(function(){var el=${description};return JSON.stringify([document.activeElement===el,!!document.querySelector('.tdef[data-name="core"].open'),el?el.value.length:-1]);})()`,
      'Enter in the description shut the row or left the field',
    ).toBe(`[true,true,${before + [...SIXTY].length}]`);

    // The same field in the topology's 属性 column: nothing is picked, so the
    // column stands on the team.
    const side = q('.tdef[data-name="core"] .cv-side [data-cv-f=description]');
    await wait(page, `!!${side}`);
    await page.click('.tdef[data-name="core"] .cv-side [data-cv-f=description]');
    const sideBefore = Number(await page.evaluate(held(side)));
    const sideProblems = await typist(page, side, SIXTY);
    expect(sideProblems, 'the 属性 description lost the focus or a letter').toEqual([]);
    await readTo<number>(page, held(side), 'the 属性 description does not hold the sixty letters').toBe(sideBefore + [...SIXTY].length);
  });

  test('TYPING-2 a definition being named holds every letter with the focus', async ({ page, app }) => {
    await page.route('**/api/hq/running', (route) => route.fulfill({ json: { herdr: { ok: true, text: '' }, teams: [] } }));
    await page.goto(`${app.base}/teams`);
    await wait(page, there('[data-act=new]'));
    await press(page, q('[data-act=new]'));
    await wait(page, there('.tdef[data-name^="~new"] .in-name'));

    // The name a person types: every letter asks the server, and the answer
    // draws the paths, the problems and the figure again.
    const name = q('.tdef[data-name^="~new"] .in-name');
    await page.click('.tdef[data-name^="~new"] .in-name');
    const before = Number(await page.evaluate(held(name)));
    const nameProblems = await typist(page, name, SIXTY);
    expect(nameProblems, 'the name field lost the focus or a letter').toEqual([]);
    await readTo<number>(page, held(name), 'the name field does not hold the sixty letters').toBe(before + [...SIXTY].length);
    // The key an IME confirms with is the field's: the row stays open.
    await page.keyboard.press('Enter');
    await page.waitForTimeout(150);
    await readTo<string>(
      page,
      `(function(){var el=${name};return JSON.stringify([document.activeElement===el,!!document.querySelector('.tdef[data-name^="~new"].open'),el?el.value.length:-1]);})()`,
      'Enter in the name shut the row or left the field',
    ).toBe(`[true,true,${before + [...SIXTY].length}]`);

    const description = q('.tdef[data-name^="~new"] .in-description');
    await page.click('.tdef[data-name^="~new"] .in-description');
    const descriptionBefore = Number(await page.evaluate(held(description)));
    const descriptionProblems = await typist(page, description, SIXTY);
    expect(descriptionProblems, 'the new definition’s description lost the focus or a letter').toEqual([]);
    await readTo<number>(page, held(description), 'the new definition’s description does not hold the sixty letters').toBe(descriptionBefore + [...SIXTY].length);
  });
});

test.describe('the role screen’s text fields', () => {
  test.use({ reads: 'roles' });

  test('TYPING-3 the name, the title, the heading, the rule and the search hold every letter', async ({ page, app }) => {
    await page.goto(`${app.base}/roles?open=lead`);
    await wait(page, there('#rows .role.open .panel[data-l=project] .rule'));

    const fields: [string, string][] = [
      ['the name', q('#rows .role.open .panel[data-l=project] [data-f=name]')],
      ['the title', q('#rows .role.open .panel[data-l=project] [data-f=title]')],
      ['the heading', q('#rows .role.open .panel[data-l=project] .sec[data-si="0"] [data-f=heading]')],
      ['the search', q('#q')],
    ];
    for (const [what, find] of fields) {
      await page.evaluate(`(function(){var el=${find};el.focus();el.setSelectionRange(el.value.length,el.value.length);})()`);
      const before = Number(await page.evaluate(held(find)));
      const problems = await typist(page, find, SIXTY);
      expect(problems, `${what} lost the focus or a letter`).toEqual([]);
      await readTo<number>(page, held(find), `${what} does not hold the sixty letters`).toBe(before + [...SIXTY].length);
    }

    // A rule's body and memo: the panel's check comes back while the letters are
    // still going in, and the rule is the same field all the way.
    await page.click('#q');
    await page.keyboard.press('ControlOrMeta+a');
    await page.keyboard.press('Backspace');
    await wait(page, there('#rows .role.open .panel[data-l=project] .rule .rsum'));
    await press(page, `${q('#rows .role.open .panel[data-l=project] .rule')}.querySelector('.rsum')`);
    await wait(page, there('#rows .role.open .rule.open .redit [data-f=text]'));
    for (const [what, sel] of [['the body', '[data-f=text]'], ['the memo', '[data-f=note]']]) {
      const find = q(`#rows .role.open .rule.open .redit ${sel}`);
      await page.click(`#rows .role.open .rule.open .redit ${sel}`);
      const before = Number(await page.evaluate(held(find)));
      const problems = await typist(page, find, SIXTY);
      expect(problems, `${what} lost the focus or a letter`).toEqual([]);
      await readTo<number>(page, held(find), `${what} does not hold the sixty letters`).toBe(before + [...SIXTY].length);
    }
  });
});

test.describe('the profile screen’s text fields', () => {
  test.use({ reads: 'profiles' });

  test('TYPING-4 the name, the argument and the environment hold every letter', async ({ page, app }) => {
    await page.goto(`${app.base}/profiles?open=sonnet-fast`);
    const panel = '[data-key^="sonnet-fast|"]';
    await wait(page, there(`${panel} [data-f=name]`));

    const fields: [string, string][] = [
      ['the name', q(`${panel} [data-f=name]`)],
      ['the argument', q(`${panel} [data-f=arg]`)],
      ['the environment key', q(`${panel} [data-f=ek]`)],
      ['the environment value', q(`${panel} [data-f=ev]`)],
      ['the search', q('#q')],
    ];
    for (const [what, find] of fields) {
      await page.evaluate(`(function(){var el=${find};el.focus();el.setSelectionRange(el.value.length,el.value.length);})()`);
      const before = Number(await page.evaluate(held(find)));
      const problems = await typist(page, find, SIXTY);
      expect(problems, `${what} lost the focus or a letter`).toEqual([]);
      await readTo<number>(page, held(find), `${what} does not hold the sixty letters`).toBe(before + [...SIXTY].length);
    }
  });
});
