// r-3d54 / ac-824b — the table walked in a browser. Every event the person can
// start from each column (属性・YAML・プロンプト) leaves the picked thing and the
// right column where teams/selection-table.ts says. The expectation is read from
// that table, not written a second time here; this file says only which press is
// which event.
//
// Each row opens the page afresh: the canvas keeps nothing between two rows.
import { test, expect } from './fixtures';
import type { Page } from '@playwright/test';
import { ready } from './helpers';
import { SIDE_RULES } from '../../internal/web/src/teams/selection-table';
import type { PickHow, Side } from '../../internal/web/src/teams/selection-table';

test.use({ reads: 'teams' });

const SIDES: Side[] = ['attr', 'yaml', 'prompt'];
const NAME: Record<Side, string> = { attr: '属性', yaml: 'YAML', prompt: 'プロンプト' };
const ORIGINAL = ['lead', 'analyst', 'impl-a', 'impl-b', 'reviewer'];

const row = (page: Page) => page.locator('.tdef[data-name="core"]');
const panel = (page: Page) => row(page).locator('.panel.team-p');
const side = (page: Page) => panel(page).locator('.cv-side');
const heading = (page: Page, one: Side) => side(page).getByRole('button', { name: NAME[one], exact: true });
const svg = (page: Page) => panel(page).locator('.cv-canvas svg');

// ---- what the walk looks at ------------------------------------------------
const noPick = async (page: Page): Promise<void> => {
  await expect(panel(page).locator('.cv-n.sel, .cv-e.sel')).toHaveCount(0);
};
const memberPick = async (page: Page, id: string): Promise<void> => {
  await expect(panel(page).locator(`.cv-n.member.sel[data-cv-n="${id}"]`)).toHaveCount(1);
};
const workerPick = async (page: Page, i: number): Promise<void> => {
  await expect(panel(page).locator(`.cv-n.worker.sel[data-cv-w="${i}"]`)).toHaveCount(1);
};
// the picked line is the one carrying its × (data-k), and it is the only lit one.
const edgePick = async (page: Page, k: string): Promise<void> => {
  await expect(panel(page).locator(`.cv-ex[data-k="${k}"]`)).toHaveCount(1);
  await expect(panel(page).locator('.cv-e.sel')).toHaveCount(1);
};
// a new point: the picked member is one the fixture did not have.
const newMemberPick = async (page: Page): Promise<void> => {
  const picked = panel(page).locator('.cv-n.member.sel');
  await expect(picked).toHaveCount(1);
  expect(ORIGINAL).not.toContain(await picked.getAttribute('data-cv-n'));
};

// ---- one press on the figure ------------------------------------------------
const pressMember = async (page: Page, id = 'analyst'): Promise<void> => {
  await panel(page).locator(`.cv-n.member[data-cv-n="${id}"]`).click();
};
const pressWorker = async (page: Page, i = 0): Promise<void> => {
  await panel(page).locator(`.cv-n.worker[data-cv-w="${i}"]`).click();
};
const pickWithKey = async (page: Page, css: string, key: string): Promise<void> => {
  const one = panel(page).locator(css);
  await one.focus();
  await one.press(key);
};

// ---- the rows ---------------------------------------------------------------
// from is where the press stands: a column the screen offers it in. A control
// that lives in the 属性 panel (複製, the name field) is pressed from 属性 alone,
// and the YAML preview's lines from YAML: the screen does not offer them from
// the other columns, so a walk that started there would press nothing.
type Walk = {
  name: string;
  how: PickHow;
  what?: 'member' | 'worker' | 'edge' | 'human';
  pressed?: Side; // the heading a side event pressed
  from?: Side[];
  setup?: (page: Page) => Promise<void>;
  press: (page: Page, start: Side) => Promise<void>;
  pick: (page: Page) => Promise<void>;
};

const WALKS: Walk[] = [
  {
    name: 'a member pressed', how: 'press', what: 'member',
    press: async (page) => { await pressMember(page); },
    pick: async (page) => { await memberPick(page, 'analyst'); },
  },
  {
    name: 'a member chosen with Enter', how: 'press', what: 'member',
    press: async (page) => { await pickWithKey(page, '.cv-n.member[data-cv-n="analyst"]', 'Enter'); },
    pick: async (page) => { await memberPick(page, 'analyst'); },
  },
  {
    name: 'a worker chosen with Space', how: 'press', what: 'worker',
    press: async (page) => { await pickWithKey(page, '.cv-n.worker[data-cv-w="0"]', ' '); },
    pick: async (page) => { await workerPick(page, 0); },
  },
  {
    name: 'a line pressed', how: 'press', what: 'edge',
    // The hit stroke is transparent and a straight line has an empty box, so the
    // press is sent without the visibility check: the box's middle is on the line.
    press: async (page) => { await panel(page).locator('.cv-hit[data-cv-e="analyst|lead"]').click({ force: true }); },
    pick: async (page) => { await edgePick(page, 'analyst|lead'); },
  },
  {
    name: 'a line chosen with Enter', how: 'press', what: 'edge',
    press: async (page) => { await pickWithKey(page, '.cv-hit[data-cv-e="analyst|lead"]', 'Enter'); },
    pick: async (page) => { await edgePick(page, 'analyst|lead'); },
  },
  {
    name: 'a line drawn from a point', how: 'press', what: 'edge',
    setup: async (page) => { await pressMember(page, 'impl-a'); },
    press: async (page) => {
      const handle = panel(page).locator('[data-cv-h="impl-a"]').first();
      const target = panel(page).locator('.cv-n.member[data-cv-n="impl-b"]');
      const hb = await handle.boundingBox();
      const tb = await target.boundingBox();
      expect(hb, 'the drawn line needs the handle of the picked point').not.toBeNull();
      expect(tb, 'the drawn line needs the point it is drawn to').not.toBeNull();
      await page.mouse.move(hb!.x + hb!.width / 2, hb!.y + hb!.height / 2);
      await page.mouse.down();
      await page.mouse.move(tb!.x + tb!.width / 2, tb!.y + tb!.height / 2, { steps: 4 });
      await page.mouse.up();
    },
    pick: async (page) => { await edgePick(page, 'impl-a|impl-b'); },
  },
  {
    name: 'the blank space pressed', how: 'let',
    setup: async (page) => { await pressMember(page, 'lead'); },
    press: async (page) => {
      const box = await svg(page).boundingBox();
      expect(box, 'the blank space needs the figure').not.toBeNull();
      await page.mouse.click(box!.x + 12, box!.y + 12);
    },
    pick: noPick,
  },
  {
    name: 'Esc', how: 'let',
    setup: async (page) => { await pressMember(page, 'lead'); },
    press: async (page) => { await page.keyboard.press('Escape'); },
    pick: noPick,
  },
  {
    name: 'Delete', how: 'let',
    setup: async (page) => { await pressMember(page, 'lead'); },
    press: async (page) => { await page.keyboard.press('Delete'); },
    pick: noPick,
  },
  {
    // 保存 puts the figure back on the file: the pick cannot stand (d-a575). The
    // description is changed first so the save has something to write.
    name: 'a save', how: 'reset',
    setup: async (page) => {
      await side(page).locator('[data-cv-f="description"]').fill('保存のあとの選択');
      await pressMember(page, 'lead');
    },
    press: async (page) => {
      await panel(page).locator('[data-act="t-save"]').click();
      await expect(panel(page).locator('.ok')).toBeVisible();
    },
    pick: noPick,
  },
  {
    name: 'a member added', how: 'field',
    press: async (page) => { await panel(page).locator('[data-cv-act="m-add"]').click(); },
    pick: async (page) => { await newMemberPick(page); },
  },
  {
    name: 'a member copied', how: 'field', from: ['attr'],
    setup: async (page) => { await pressMember(page, 'lead'); },
    press: async (page) => { await panel(page).locator('[data-cv-act="m-copy"]').click(); },
    pick: async (page) => { await newMemberPick(page); },
  },
  {
    name: 'a member renamed', how: 'field', from: ['attr'],
    setup: async (page) => { await pressMember(page, 'lead'); },
    press: async (page) => { await side(page).locator('[data-cv-f="m-name"]').fill('leadx'); },
    pick: async (page) => { await memberPick(page, 'leadx'); },
  },
  {
    // a worker with no purpose is a finding; its row is under the figure, where
    // every column reads it.
    name: 'a finding row pressed', how: 'field',
    setup: async (page) => {
      await pressMember(page, 'lead');
      await panel(page).locator('.cv-tools [data-cv-act="w-add"]').click();
      await expect(panel(page).locator('.vlist .vitem[data-worker="2"]')).toHaveCount(1);
    },
    press: async (page) => { await panel(page).locator('.vlist .vitem[data-worker="2"]').click(); },
    pick: async (page) => { await workerPick(page, 2); },
  },
  {
    name: 'a YAML line pressed', how: 'field', from: ['yaml'],
    press: async (page) => { await side(page).locator('.yl[data-node="analyst"]').first().click(); },
    pick: async (page) => { await memberPick(page, 'analyst'); },
  },
  {
    name: 'the team description edited', how: 'field',
    press: async (page) => { await panel(page).getByRole('button', { name: 'チームの説明を編集' }).click(); },
    pick: noPick,
  },
  {
    name: 'the 属性 heading', how: 'side', pressed: 'attr',
    setup: async (page) => { await pressMember(page, 'lead'); },
    press: async (page) => { await heading(page, 'attr').click(); },
    pick: async (page) => { await memberPick(page, 'lead'); },
  },
  {
    name: 'the YAML heading', how: 'side', pressed: 'yaml',
    setup: async (page) => { await pressMember(page, 'lead'); },
    press: async (page) => { await heading(page, 'yaml').click(); },
    pick: async (page) => { await memberPick(page, 'lead'); },
  },
  {
    name: 'the プロンプト heading', how: 'side', pressed: 'prompt',
    setup: async (page) => { await pressMember(page, 'lead'); },
    press: async (page) => { await heading(page, 'prompt').click(); },
    pick: async (page) => { await memberPick(page, 'lead'); },
  },
];

// NO_PRESS is the one row of the table the screen has no press for: YAML へ
// (to-yaml, teams/canvas/act.ts) is the old button the tabs replaced (r-f0be),
// and no element carries data-cv-act="to-yaml" any more. The row is measured in
// internal/web/jstest/selection.test.js. When a press comes back it belongs in
// WALKS and this list empties; a new row of SIDE_RULES with no press fails the
// count below, which is the point of that test.
const NO_PRESS: PickHow[] = ['yaml'];

// ruleOf is the row of the table an event falls in: by the event, and by the
// thing it picked when the row names one.
const ruleOf = (how: PickHow, what?: Walk['what'] | null) =>
  SIDE_RULES.find((one) => one.how === how && (!one.what || one.what.includes(what ?? null)));

// wantSide is the column the table answers for one row, from one starting column.
const wantSide = (walk: Walk, start: Side): Side => {
  const rule = ruleOf(walk.how, walk.what ?? null);
  expect(rule, `the table has no row for ${walk.name}`).toBeDefined();
  if (rule!.side === 'keep') return start;
  if (rule!.side === 'pressed') return walk.pressed!;
  return rule!.side;
};

test('the table has the row the walk needs for every event', () => {
  const unnamed = WALKS.filter((walk) => !ruleOf(walk.how, walk.what ?? null));
  expect(unnamed.map((walk) => walk.name), 'an event the table does not name').toEqual([]);
  const measured = SIDE_RULES.filter((one) => !NO_PRESS.includes(one.how));
  const hit = new Set(WALKS.map((walk) => ruleOf(walk.how, walk.what ?? null)));
  expect(measured.filter((one) => !hit.has(one)).map((one) => one.how), 'a row of the table no walk measures').toEqual([]);
});

for (const walk of WALKS) {
  for (const start of walk.from ?? SIDES) {
    test(`${walk.name}, starting at ${start}`, async ({ page, app }) => {
      await page.goto(`${app.base}/teams?open=core`);
      await ready(page);
      if (walk.setup) await walk.setup(page);
      await heading(page, start).click();
      await expect(heading(page, start)).toHaveClass(/on/);
      await walk.press(page, start);
      await walk.pick(page);
      await expect(heading(page, wantSide(walk, start))).toHaveClass(/on/);
    });
  }
}
