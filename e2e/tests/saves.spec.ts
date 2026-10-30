// R-B41C — a save of one's own is not refused at the next one. The master's
// steps are the first walk: add a rule, save, add another rule, save. Every
// screen that writes a file does it three times without opening the page again,
// and the file holds the last words that were typed (ac-cca6).
//
// The cause was hergy's own: a save writes the person's lines, gives the rules
// their ids after it, and answered with the text from before the ids, so the
// screen's next save carried a hash the file no longer had. A save's answer is
// the file the write left, and every screen puts the answer's files under its
// feet (r-b41c).
import { test, expect } from './fixtures';
import { readFileSync, writeFileSync } from 'node:fs';
import path from 'node:path';
import { press, q, readTo, set, there, wait } from './helpers';

// save presses one 保存: the request is out, and it was taken.
async function save(page: Parameters<typeof wait>[0], find: string, at: string): Promise<void> {
  const answer = page.waitForResponse((one) => one.request().method() === 'PUT' && one.url().includes(at));
  await press(page, find);
  const answer0 = await answer;
  expect(answer0.status(), `the save at ${at} was refused: ${await answer0.text()}`).toBe(200);
}

test.describe('the role screen', () => {
  test.use({ reads: 'roles' });

  test('RB41C-1 a role is saved three times in a row, the master’s steps first', async ({ page, app }) => {
    const file = path.join(app.proj, '.hergy', 'roles', 'builder.yaml');
    const panel = '#rows .role.open .panel[data-l=project]';
    await page.goto(`${app.base}/roles?open=builder`);
    await wait(page, there(`${panel} [data-act=save]`));
    const bodies = ['規則を 1 つ足した。', '規則をもう 1 つ足した。', '規則を 3 つ目まで足した。'];
    for (const [i, body] of bodies.entries()) {
      // ＋規則, then the words: the rule the press added is the one open.
      await press(page, q(`${panel} .sec[data-si="0"] [data-act=rule-add]`));
      const added = `[...document.querySelectorAll(${JSON.stringify(`${panel} .sec[data-si="0"] .rule`)})].at(-1)`;
      await wait(page, `!!${added}`);
      await set(page, `${added}.querySelector('[data-f=text]')`, body);
      await wait(page, `!${q(`${panel} [data-act=save]`)}.disabled`);
      await save(page, q(`${panel} [data-act=save]`), '/api/role/builder');
      await wait(page, there(`${panel} .ok`));
      // The words are still the screen's, and the file holds them: a save the
      // screen could not hold on to would have dropped them all by now.
      expect(readFileSync(file, 'utf8'), `the save ${i + 1} did not write ${body}`).toContain(body);
    }
    const text = readFileSync(file, 'utf8');
    for (const body of bodies) expect(text, 'a rule a save wrote went away').toContain(body);
    expect(text.match(/足した。/g)?.length ?? 0, 'the rules the writes gave an id to').toBe(3);
  });
  test('RB41C-5 a save that gives another file its ids does not refuse that file’s own save', async ({ page, app }) => {
    // The master's ありとあらゆる場所: a save gives the rules their ids in every
    // file, so a document nobody touched moves when another one is saved. The
    // screen takes the answer's files, and the row that moved is saved next
    // without opening the page again.
    const lead = path.join(app.proj, '.hergy', 'roles', 'lead.yaml');
    const was = readFileSync(lead, 'utf8');
    writeFileSync(lead, was.replace('id: PAAF', 'id: C1'), 'utf8');

    await page.goto(`${app.base}/roles?open=builder`);
    const builder = '#rows .role.open .panel[data-l=project]';
    await wait(page, there(`${builder} [data-act=save]`));
    await press(page, q(`${builder} .sec[data-si="0"] [data-act=rule-add]`));
    const added = `[...document.querySelectorAll(${JSON.stringify(`${builder} .sec[data-si="0"] .rule`)})].at(-1)`;
    await wait(page, `!!${added}`);
    await set(page, `${added}.querySelector('[data-f=text]')`, 'id を配るきっかけの規則。');
    await wait(page, `!${q(`${builder} [data-act=save]`)}.disabled`);
    await save(page, q(`${builder} [data-act=save]`), '/api/role/builder');

    // lead moved with that save: its own save is the next one, on the row the
    // list already holds. The list draws a head per place and a row per place
    // (r-410c), so the row is the one under the project's head, not the first row
    // named lead.
    const row = `(() => {
      let on = null;
      for (const el of document.querySelector('#rows').children) {
        if (el.classList.contains('hq-gh')) { on = el.dataset.place || null; continue; }
        if (on !== 'project' || !el.classList.contains('role')) continue;
        const n = el.querySelector('.nm');
        if (n && n.textContent.trim() === 'lead') return el;
      }
      return null;
    })()`;
    await wait(page, `!!${row}`);
    await press(page, `${row}.querySelector('.sum')`);
    const leadPanel = `${row}.querySelector('.panel[data-l=project]')`;
    await wait(page, `${leadPanel}&&!!${leadPanel}.querySelector('[data-act=save]')`);
    await press(page, `${leadPanel}.querySelector('.rule .rsum')`);
    await wait(page, `!!${leadPanel}.querySelector('.rule.open .redit [data-f=text]')`);
    await set(page, `${leadPanel}.querySelector('.rule.open .redit [data-f=text]')`, 'lead の規則を書き直した。');
    await wait(page, `!${leadPanel}.querySelector('[data-act=save]').disabled`);
    await save(page, `${leadPanel}.querySelector('[data-act=save]')`, '/api/role/lead');
    await wait(page, `!!${leadPanel}.querySelector('.ok')`);
    expect(readFileSync(lead, 'utf8'), 'the save on the row the other save moved did not write').toContain('lead の規則を書き直した。');
  });
});

test.describe('the profile screen', () => {
  test.use({ reads: 'profiles' });

  test('RB41C-2 a profile is saved three times in a row', async ({ page, app }) => {
    const file = path.join(app.proj, '.hergy', 'profiles.yaml');
    await page.goto(`${app.base}/profiles?open=sonnet-fast`);
    const panel = '[data-key^="sonnet-fast|"]';
    await wait(page, there(`${panel} [data-act=save]`));
    const values = ['11001', '12001', '13001'];
    for (const [i, value] of values.entries()) {
      await set(page, `${q(panel)}.querySelector('[data-f=ev]')`, value);
      await wait(page, `!${q(`${panel} [data-act=save]`)}.disabled`);
      await save(page, q(`${panel} [data-act=save]`), '/api/profile/sonnet-fast');
      await wait(page, there(`${panel} .ok`));
      expect(readFileSync(file, 'utf8'), `the save ${i + 1} did not write ${value}`).toMatch(new RegExp(`MAX_THINKING_TOKENS: ["']?${value}`));
    }
  });
});

test.describe('the team screen', () => {
  test.use({ reads: 'teams' });

  // shape is the row's topology panel: the save that writes team.yaml.
  const shape = '.tdef[data-name="core"] .panel.team-p';

  test('RB41C-3 a team definition’s description is saved three times in a row', async ({ page, app }) => {
    const file = path.join(app.proj, '.hergy', 'teams', 'core', 'team.yaml');
    await page.goto(`${app.base}/teams?open=core`);
    await wait(page, there(`${shape} [data-act=t-save]`));
    const descriptions = ['説明を 1 度書き直した。', '説明を 2 度書き直した。', '説明を 3 度書き直した。'];
    for (const [i, description] of descriptions.entries()) {
      await set(page, q('.tdef[data-name="core"] .in-description'), description);
      await wait(page, `!${q(`${shape} [data-act=t-save]`)}.disabled`);
      await save(page, q(`${shape} [data-act=t-save]`), '/api/team/core');
      await wait(page, there(`${shape} .ok`));
      expect(readFileSync(file, 'utf8'), `the save ${i + 1} did not write the description`).toContain(`description: "${description}"`);
    }
  });

  test('RB41C-4 a team’s rules are saved three times in a row', async ({ page, app }) => {
    const file = path.join(app.proj, '.hergy', 'teams', 'core', 'agreement.yaml');
    await page.goto(`${app.base}/teams?open=core`);
    await wait(page, there(`${shape} [data-act=t-save]`));
    await press(page, q('.tdef[data-name="core"] .dtab[data-t=agree]'));
    const panel = '.tdef[data-name="core"] .panel.agree-p';
    await wait(page, there(`${panel} [data-act=save]`));
    const bodies = ['チームの規則を 1 つ足した。', 'チームの規則をもう 1 つ足した。', 'チームの規則を 3 つ目まで足した。'];
    for (const [i, body] of bodies.entries()) {
      await press(page, q(`${panel} .sec[data-si="0"] [data-act=rule-add]`));
      const added = `[...document.querySelectorAll(${JSON.stringify(`${panel} .sec[data-si="0"] .rule`)})].at(-1)`;
      await wait(page, `!!${added}`);
      await set(page, `${added}.querySelector('[data-f=text]')`, body);
      await wait(page, `!${q(`${panel} [data-act=save]`)}.disabled`);
      await save(page, q(`${panel} [data-act=save]`), '/api/team/core');
      await wait(page, there(`${panel} .ok`));
      expect(readFileSync(file, 'utf8'), `the save ${i + 1} did not write ${body}`).toContain(body);
    }
  });
});
