// ROLES-P5 and ROLES-P14 — checks/roles.md's 大 items, walked in a browser.
// They are internal/app e2eRolesEditWalk (the rule a person edits: e2eRolesOpen,
// e2eRolesInPrompt, e2eRolesHeadingDepth, e2eRolesSectionOrder, e2eRolesRuleDrag,
// e2eRolesSaveRule, in that order over one project) and e2eRolesAssign, test for
// test and assertion for assertion, over the same real-scale fixture
// (testdata/roles-v2: 6 rows, 51 rules, 4 team definitions, 15 people).
import { test, expect } from './fixtures';
import { readFileSync } from 'node:fs';
import path from 'node:path';
import {
  changedLines,
  choose,
  focus,
  press,
  q,
  read,
  readTo,
  set,
  there,
  wait,
} from './helpers';

test.use({ reads: 'roles' });

// row is one list row, found by the role's name rather than by a column it may
// not have (e2eRolesRowJS).
const row = (name: string) =>
  `[...document.querySelectorAll('#rows .role')].find(function(r){var n=r.querySelector('.c-name .nm');return n&&n.textContent.trim()===${JSON.stringify(name)}})`;

// sec is one section of the open role's project document, found by its heading:
// a section is reached by its heading rather than by its place, because the walk
// moves the sections (e2eRolesSecJS).
const sec = (heading: string) =>
  `[...document.querySelectorAll('#rows .role.open .panel .sec')].find(function(s){var h=s.querySelector('[data-f=heading]');return h&&h.value===${JSON.stringify(heading)}})`;

// rule is one rule of the open role's panel, found by its id.
const rule = (id: string) =>
  `[...document.querySelectorAll('#rows .role.open .panel .rule')].find(function(r){return r.querySelector('.rid b').textContent.trim()===${JSON.stringify(id)}})`;

const panel = `document.querySelector('#rows .role.open .panel[data-l=project]')`;

// sameLines holds a save to moving lines about rather than changing them: the
// file comes back with the same lines in another order (e2eRolesSameLines).
function sameLines(before: string, after: string): void {
  const b = before.split('\n');
  const a = after.split('\n');
  expect(a.length, `the file went from ${b.length} lines to ${a.length}`).toBe(b.length);
  expect([...a].sort(), "the file's lines changed, not only their order").toEqual([...b].sort());
}

// block is the lines from the one a heading stands on, down to the first line
// that is not indented deeper than it (e2eRolesBlock).
function block(text: string, head: string, deeper: string): string {
  const lines = text.split('\n');
  const start = lines.findIndex((l) => l.trim() === head);
  expect(start, `the file no longer holds ${head}`).toBeGreaterThanOrEqual(0);
  let end = start + 1;
  while (end < lines.length && lines[end].startsWith(deeper)) end++;
  return lines.slice(start, end).join('\n');
}

const itemBlock = (text: string, id: string) => block(text, `- id: ${id}`, '        ');
const sectionBlock = (text: string, heading: string) => block(text, `- heading: ${heading}`, '    ');

// vt reads the Markdown preview's "N 行 · N 文字" as the two numbers.
function vt(text: string): [number, number] {
  const f = text.trim().split(/\s+/);
  expect(f.length, `the Markdown preview's counts read ${text}`).toBeGreaterThanOrEqual(4);
  return [Number(f[0]), Number(f[3])];
}

test('ROLES-P5 the rule a person edits', async ({ page, app }) => {
  const project = (name: string) => path.join(app.proj, '.hergy', 'roles', name);

  // ---- e2eRolesOpen: P1・P3・P5 ----
  await page.goto(`${app.base}/roles`);
  await wait(page, there('#rows .role'));
  await press(page, `${row('builder')}.querySelector('.sum')`);
  await wait(page, `${row('builder')}.classList.contains('open')`);
  await readTo<string>(
    page,
    `window.location.search`,
    'opening the builder row left in the URL',
  ).toContain('open=builder');
  await readTo<number>(
    page,
    `${row('builder')}.querySelectorAll('.panel').length`,
    'builder shows panels, want the project\'s alone',
  ).toBe(1);
  await readTo<number>(
    page,
    `${row('builder')}.querySelectorAll('.usage .uchip').length`,
    'builder is read by chips, want 3',
  ).toBe(3);
  await readTo<string>(
    page,
    `${row('builder')}.querySelector('.uchip .g').textContent`,
    "the first reader's chip reads, want core",
  ).toBe('core');
  await readTo<number>(
    page,
    `${row('builder')}.querySelectorAll('.dhvm li').length`,
    'builder shows document-level checks, want none',
  ).toBe(0);

  // P5: the per-rule badge is the rule's own checks.
  const badge = (id: string, said: string) =>
    readTo<string>(
      page,
      `(function(){var x=${rule(id)};var s=x.querySelector('.rid .sev');return s?s.textContent.trim():''})()`,
      said,
    );
  for (const [id, want] of [
    ['BUILD-1', ''],
    ['BUILD-3', '警告 1'],
    ['BUILD-4', '問題 1'],
    ['BUILD-5', '警告 1'],
  ]) {
    await badge(id, `the ${id} badge reads, want ${want}`).toBe(want);
  }
  const countScript = `${row('builder')}.querySelector('.pf .vcount').textContent`;
  await readTo<string>(
    page,
    countScript,
    "the panel's count reads, want 問題 1 and 警告 2",
  ).toContain('問題 1');
  await readTo<string>(
    page,
    countScript,
    "the panel's count reads, want 問題 1 and 警告 2",
  ).toContain('警告 2');
  await readTo<number>(
    page,
    `document.querySelectorAll('#rows .role.open .yprev .yl.err').length`,
    'the Markdown preview marks lines as a problem, want 1',
  ).toBe(1);
  await readTo<number>(
    page,
    `document.querySelectorAll('#rows .role.open .yprev .yl.warn').length`,
    'the Markdown preview marks lines as a warning, want 2',
  ).toBe(2);
  // The layout of a section is one way now (d-f4f6).
  await readTo<number>(
    page,
    `document.querySelectorAll('#rows .role.open .panel .sec [data-f=level]').length`,
    "builder shows level pickers, want the 2 sections'",
  ).toBe(2);
  await readTo<number>(
    page,
    `document.querySelectorAll('#rows .role.open .panel .sec [data-f=style]').length`,
    'the section header offers style pickers, want none',
  ).toBe(0);
  // I6: nowhere the level says —, and no control of the panel is a select any more.
  await readTo<string>(
    page,
    `[...document.querySelectorAll('#rows .role.open .panel .sec [data-f=level]')].map(function(b){return b.textContent.trim()}).join(',')`,
    "the sections' depth reads, want ##,##",
  ).toBe('##,##');
  await readTo<number>(
    page,
    `document.querySelectorAll('#rows .role.open .panel select').length`,
    'the panel still shows selects',
  ).toBe(0);
  await readTo<number>(
    page,
    `[...document.querySelectorAll('#rows .role.open .yprev .yl .yt')].filter(function(e){return e.textContent.indexOf('- **BUILD-')===0}).length`,
    "the Markdown preview shows items with the id at their head, want the 7 rules'",
  ).toBe(7);
  // R1: the meter paints, and a kept line is pulled back to its bullet's head.
  await readTo<number>(
    page,
    `document.querySelector('#rows .role.open .c-amount .meter i').getBoundingClientRect().width`,
    "the reader's meter is wide, want a painted fill",
  ).toBeGreaterThan(0);
  await readTo<number>(
    page,
    `Math.min.apply(null,[...document.querySelectorAll('#rows .role.open .yprev .yt')].map(function(e){return parseFloat(getComputedStyle(e).textIndent)}))`,
    'the smallest text-indent in the Markdown preview is, want a kept line pulled back',
  ).toBeLessThan(0);
  // N1: the origin cell that says — stands one line high, exactly as high as a
  // cell that names an origin. The two heights are read as one "empty|named", so
  // the reading is the pair the walk compares and not one cell at a time.
  const cell = (find: string) =>
    `(function(){var c=[...document.querySelectorAll(${JSON.stringify(`#rows .role.open .panel .rule ${find}`)})];return c.length?Math.round(Math.max.apply(null,c.map(function(e){return e.getBoundingClientRect().height}))):-1})()`;
  const heights = `(${cell('.org.empty')})+'|'+(${cell('.org:not(.empty)')})`;
  await readTo<string>(
    page,
    heights,
    `the cell that says — stands as high as one that names an origin, want one line each`,
  ).toMatch(/^([1-9]\d*)\|\1$/);

  // P3: the rule opens under its own summary, and its place joins the URL.
  await press(page, `${rule('BUILD-5')}.querySelector('.rsum')`);
  await wait(page, there('#rows .role.open .panel .rule.open .redit'));
  await readTo<string>(
    page,
    `window.location.search`,
    'opening BUILD-5 left in the URL',
  ).toContain('rule=builder.project.BUILD-5');
  await readTo<string>(
    page,
    q('#rows .role.open .rule.open .redit [data-f=id]') + '.value',
    'the opened rule reads, want BUILD-5',
  ).toBe('BUILD-5');
  await readTo<string>(
    page,
    q('#rows .role.open .rule.open .redit [data-f=text]') + '.value',
    "the opened rule's text reads",
  ).toContain('大きな設計の判断');
  await readTo<string>(
    page,
    `${row('builder')}.querySelector('.pedit [data-f=send_why] button.on').textContent`,
    'the why switch reads, want the default プロンプトに含める',
  ).toBe('プロンプトに含める');
  await readTo<number>(
    page,
    `document.querySelectorAll('#rows .role.open .rule.open .redit [data-act=rule-revert]:not([hidden])').length`,
    'a rule nothing changed shows 元に戻す',
  ).toBe(0);
  await readTo<string>(
    page,
    `${row('builder')}.querySelector('.pf [data-act=save]').disabled+''`,
    'a rule nothing changed can be saved',
  ).toBe('true');

  // P1 again: the same row closes.
  await press(page, `${row('builder')}.querySelector('.sum')`);
  await wait(page, `!${row('builder')}.classList.contains('open')`);
  await readTo<string>(
    page,
    `window.location.search`,
    'closing the row left in the URL',
  ).not.toContain('open=');

  // A document-level check stands above the sections.
  await page.goto(`${app.base}/roles?open=analyst`);
  await wait(page, there('#rows .role.open .panel'));
  await readTo<number>(
    page,
    `document.querySelectorAll('#rows .role.open .panel .dhvm li').length`,
    'analyst shows document-level checks, want 1',
  ).toBe(1);
  await readTo<string>(
    page,
    q('#rows .role.open .panel .dhvm li span:last-child') + '.textContent',
    'the analyst check reads',
  ).toContain('core-analyst に渡る規則が 21 件です');
  // R4: the same document-wide sentence marks the first line of the preview alone.
  await readTo<number>(
    page,
    `document.querySelectorAll('#rows .role.open .yprev .yl.warn').length`,
    "analyst's preview marks lines as a warning, want the first line alone",
  ).toBe(1);

  // A deep link opens the row and the rule it names.
  await page.goto(`${app.base}/roles?open=builder&rule=builder.project.BUILD-5`);
  await wait(page, there('#rows .role.open .rule.open .redit'));
  await readTo<string>(
    page,
    q('#rows .role.open .rule.open .redit [data-f=id]') + '.value',
    'the deep-linked rule reads, want BUILD-5',
  ).toBe('BUILD-5');

  // P15: a Markdown line opens its own rule and puts the cursor in the field the
  // line stands for — the rule's 本文.
  await press(
    page,
    `[...document.querySelectorAll('#rows .role.open .yprev .yl[data-act=yline]')].find(function(l){return l.textContent.indexOf('BUILD-3')>=0})`,
  );
  await wait(
    page,
    `document.activeElement && document.activeElement.getAttribute('data-f')==='text'`,
  );
  await readTo<string>(
    page,
    q('#rows .role.open .rule.open .redit [data-f=id]') + '.value',
    'the Markdown line opened, want BUILD-3',
  ).toBe('BUILD-3');

  // ---- e2eRolesInPrompt: I1・I2・A1 ----
  const lead = project('lead.yaml');
  const beforeLead = readFileSync(lead, 'utf8');
  await page.goto(`${app.base}/roles?open=lead&rule=lead.project.LEAD-9`);
  await wait(page, there('#rows .role.open .rule.open .redit [data-act=in-prompt]'));
  // A1: the rule's own switch stands under the field's name 規則.
  await readTo<string>(
    page,
    `${panel}.querySelector('.redit .field.scope .fl').textContent.trim()`,
    "the rule's own switch stands under, want 規則",
  ).toBe('規則');
  const [lines0, chars0] = vt(
    await read<string>(page, `${panel}.querySelector('.yprev .yinfo .vt').textContent`),
  );
  await readTo<string>(
    page,
    `${panel}.querySelector('.sec[data-si="2"] .sech .n').textContent.trim()`,
    'the last section before the switch reads, want 3 規則',
  ).toBe('3 規則');
  await readTo<number>(
    page,
    `document.querySelectorAll('#rows .role.open .c-count .outn').length`,
    'the list marks an 含めない rule before any was made',
  ).toBe(0);
  await readTo<string>(
    page,
    `document.querySelector('#rows .role.open .c-amount .amt .v').firstChild.textContent`,
    "the lead reader's amount before the switch reads, want 18",
  ).toBe('18');

  // 含めない.
  const outRule = rule('LEAD-9');
  await press(page, `${outRule}.querySelector('[data-act=in-prompt][data-v=false]')`);
  await wait(
    page,
    `${outRule}.classList.contains('out') && !!${outRule}.querySelector('.marks .mk.out')`,
  );
  await wait(
    page,
    `${panel}.querySelector('.sec[data-si="2"] .sech .n').textContent.indexOf('含めない 1')>=0`,
  );
  await wait(
    page,
    `document.querySelector('#rows .role.open .c-count .outn') && document.querySelector('#rows .role.open .c-count .outn').textContent.trim()==='含めない 1'`,
  );
  await wait(
    page,
    `document.querySelector('#rows .role.open .c-amount .amt .v').firstChild.textContent==='17'`,
  );
  await wait(page, there('#rows .role.open .yprev .yl.ghost .yt s'));
  await readTo<string>(
    page,
    `${q('#rows .role.open .yprev .yl.ghost .lno')}.textContent`,
    "the excluded rule's line carries the number, want none",
  ).toBe('');
  // The preview's two counts are read as the two numbers vt writes, each read
  // again until it is below where the walk left it.
  const vtNumber = (at: number) =>
    `Number(${panel}.querySelector('.yprev .yinfo .vt').textContent.trim().split(/\\s+/)[${at}])`;
  await readTo<number>(
    page,
    vtNumber(0),
    `the preview reads ${lines0} 行 or more after the switch, want below`,
  ).toBeLessThan(lines0);
  await readTo<number>(
    page,
    vtNumber(3),
    `the preview reads ${chars0} 文字 or more after the switch, want below`,
  ).toBeLessThan(chars0);
  await readTo<string>(
    page,
    `${panel}.querySelector('.sec[data-si="2"] .sech .n').textContent.trim()`,
    'the last section after the switch reads, want 3 規則含めない 1',
  ).toBe('3 規則含めない 1');
  await readTo<string>(
    page,
    `document.querySelector('#rows .role.open .pf .dirty').textContent`,
    'the switch reads in the footer, want 1 か所',
  ).toBe('1 か所');

  // I1: the save writes the one line and keeps every other line of the file.
  await press(page, `document.querySelector('#rows .role.open .pf [data-act=save]')`);
  await wait(page, there('#rows .role.open .ok'));
  const afterLead = readFileSync(lead, 'utf8');
  const changedLead = changedLines(beforeLead, afterLead);
  expect(
    changedLead.size,
    `the save changed lines, want the rule's in_prompt line alone: ${JSON.stringify([...changedLead])}`,
  ).toBe(1);
  for (const [line, text] of changedLead) {
    expect(text, `line ${line} became, want the rule's in_prompt: false`).toBe(
      '        in_prompt: false',
    );
  }
  expect(afterLead, "the save did not keep LEAD-9's item").toContain('- id: LEAD-9\n        text: ');

  // I2: the why's switch stands under なぜ, and 含めない there dims the why's own
  // line alone — the rule stays in the prompt.
  await page.goto(`${app.base}/roles?open=builder&rule=builder.project.BUILD-1`);
  await wait(page, there('#rows .role.open .rule.open .redit .whyrow [data-act=send-why]'));
  const whyField = `(function(){var f=[...document.querySelectorAll('#rows .role.open .rule.open .redit > .field')].find(function(d){var l=d.querySelector('label');return l&&l.textContent.trim()==='なぜ'});return f?f.querySelector('.whyrow [data-f=send_why]')?'yes':'no':'none'})()`;
  await readTo<string>(page, whyField, "the why's switch is not under なぜ").toBe('yes');
  await press(page, `${panel}.querySelector('.rule.open .whyrow [data-act=send-why][data-v=false]')`);
  await wait(page, there('#rows .role.open .rule.open .redit textarea.held-why'));
  await wait(page, there('#rows .role.open .yprev .yl .held s'));
  await readTo<string>(
    page,
    `${panel}.querySelector('.rule.open .rsum .mk.why').className`,
    "the rule's なぜ mark reads after its own 含めない, want held",
  ).toContain('held');
  await readTo<number>(
    page,
    `document.querySelectorAll('#rows .role.open .panel .rule.out').length`,
    "the why's 含めない dimmed whole rules, want none",
  ).toBe(0);
  await readTo<string>(
    page,
    `${panel}.querySelector('.rule.open .redit [data-f=why]').value`,
    "the why's 含めない emptied the why, want the text kept and writable",
  ).not.toBe('');

  // ---- e2eRolesHeadingDepth: I3・A2 ----
  const builder = project('builder.yaml');
  let before = readFileSync(builder, 'utf8');
  await page.goto(`${app.base}/roles?open=builder`);
  await wait(page, there('#rows .role.open .panel .sec [data-act=sec-pick]'));
  const make = sec('作り方');
  await readTo<string>(
    page,
    `${make}.querySelector('.sech [data-f=level]').textContent.trim()`,
    "作り方's depth reads, want ##",
  ).toBe('##');
  await press(page, `${make}.querySelector('.sech [data-act=sec-pick][data-w=lvl]')`);
  await wait(page, `${make}.querySelectorAll('.pickrow .lvl').length===3`);
  await readTo<string>(
    page,
    `[...${make}.querySelectorAll('.pickrow .lvl')].map(function(b){return b.textContent.trim().split(' ')[0]}).join(',')`,
    "the depth's three marks read, want ##,###,####",
  ).toBe('##,###,####');
  await press(page, `${make}.querySelector('.pickrow [data-act=sec-level][data-v="3"]')`);
  await wait(page, `${make}.querySelector('.sech [data-f=level]').textContent.trim()==='###'`);
  await wait(
    page,
    `[...document.querySelectorAll('#rows .role.open .yprev .yl .yt')].some(function(e){return e.textContent.trim()==='### 作り方'})`,
  );
  await readTo<string>(
    page,
    q('#rows .role.open .pf .dirty') + '.textContent',
    'the depth change reads in the footer, want 1 か所',
  ).toBe('1 か所');
  await readTo<string>(
    page,
    `${make}.querySelector('.sech [data-f=level]').className`,
    'the depth mark reads after the change, want changed',
  ).toContain('changed');
  await press(page, `document.querySelector('#rows .role.open .pf [data-act=save]')`);
  await wait(page, there('#rows .role.open .ok'));
  let after = readFileSync(builder, 'utf8');
  expect(
    after.replace('    level: 3\n', ''),
    'the save did not add the one level: 3 line to 作り方, and nothing else',
  ).toBe(before);
  expect(
    after.split('\n').length - before.split('\n').length,
    "the save added lines, want the level's one",
  ).toBe(1);

  // ---- e2eRolesSectionOrder: I4・A4 ----
  before = readFileSync(builder, 'utf8');
  await page.goto(`${app.base}/roles?open=builder`);
  await wait(page, there('#rows .role.open .panel .sec [data-act=sec-up]'));
  // The first section's ↑ and the last's ↓ are shut.
  await readTo<string>(
    page,
    `document.querySelectorAll('#rows .role.open .panel .sec')[0].querySelector('[data-act=sec-up]').disabled+''`,
    "the first section's ↑ is not shut",
  ).toBe('true');
  await readTo<string>(
    page,
    `document.querySelectorAll('#rows .role.open .panel .sec')[1].querySelector('[data-act=sec-down]').disabled+''`,
    "the last section's ↓ is not shut",
  ).toBe('true');
  await press(
    page,
    `document.querySelectorAll('#rows .role.open .panel .sec')[1].querySelector('[data-act=sec-up]')`,
  );
  await wait(
    page,
    `document.querySelectorAll('#rows .role.open .panel .sec [data-f=heading]')[0].value==='確かめ方'`,
  );
  await readTo<string>(
    page,
    q('#rows .role.open .pf .dirty') + '.textContent',
    'moving the sections reads in the footer, want 1 か所',
  ).toBe('1 か所');
  await wait(
    page,
    `(function(){var t=[...document.querySelectorAll('#rows .role.open .yprev .yl .yt')].map(function(e){return e.textContent});var a=t.findIndex(function(s){return s.indexOf('確かめ方')>=0});var b=t.findIndex(function(s){return s.indexOf('作り方')>=0});return a>=0&&b>=0&&a<b})()`,
  );
  await press(page, `document.querySelector('#rows .role.open .pf [data-act=save]')`);
  await wait(page, there('#rows .role.open .ok'));
  after = readFileSync(builder, 'utf8');
  sameLines(before, after);
  expect(
    after.indexOf('  - heading: 確かめ方'),
    'the moved section did not land first',
  ).toBeLessThan(after.indexOf('  - heading: 作り方'));
  expect(
    sectionBlock(after, '確かめ方'),
    "the moved section's own lines changed",
  ).toBe(sectionBlock(before, '確かめ方'));
  expect(
    sectionBlock(after, '作り方'),
    "the other section's own lines changed",
  ).toBe(sectionBlock(before, '作り方'));

  // ---- e2eRolesRuleDrag: I5・A5 ----
  before = readFileSync(builder, 'utf8');
  await page.goto(`${app.base}/roles?open=builder`);
  await wait(page, there('#rows .role.open .panel .sec .rule .grip'));
  const makeOrder = `[...${sec('作り方')}.querySelectorAll('.rule .rid b')].map(function(b){return b.textContent.trim()}).join(',')`;
  await readTo<string>(
    page,
    makeOrder,
    "作り方's rules stand before the drag, want BUILD-1..BUILD-4",
  ).toBe('BUILD-1,BUILD-2,BUILD-3,BUILD-4');
  // The drag is the events a person's mouse fires: the ⋮ makes the row
  // draggable, the row is lifted, and it is dropped on BUILD-3's lower half.
  const drag = `(function(){
		var make=${sec('作り方')};
		var byId=function(id){return [...make.querySelectorAll('.rule')].find(function(r){return r.querySelector('.rid b').textContent.trim()===id})};
		var a=byId('BUILD-1'), b=byId('BUILD-3');
		if(!a||!b){return 'missing';}
		a.querySelector('.grip').dispatchEvent(new PointerEvent('pointerdown',{bubbles:true}));
		a.dispatchEvent(new DragEvent('dragstart',{bubbles:true,dataTransfer:new DataTransfer()}));
		var rc=b.getBoundingClientRect(), y=rc.top+rc.height*0.75;
		b.dispatchEvent(new DragEvent('dragover',{bubbles:true,clientY:y,dataTransfer:new DataTransfer()}));
		b.dispatchEvent(new DragEvent('drop',{bubbles:true,clientY:y,dataTransfer:new DataTransfer()}));
		a.dispatchEvent(new DragEvent('dragend',{bubbles:true,dataTransfer:new DataTransfer()}));
		return 'dragged';
	})()`;
  await readTo<string>(page, drag, 'the drag did not start').toBe('dragged');
  await wait(page, there('#rows .role.open .panel .sec .rule.flash'));
  await readTo<string>(
    page,
    makeOrder,
    'the dropped rule landed as, want it below BUILD-3',
  ).toBe('BUILD-2,BUILD-3,BUILD-1,BUILD-4');
  await readTo<string>(
    page,
    q('#rows .role.open .pf .dirty') + '.textContent',
    'the drag reads in the footer, want 1 か所',
  ).toBe('1 か所');
  await press(page, `document.querySelector('#rows .role.open .pf [data-act=save]')`);
  await wait(page, there('#rows .role.open .ok'));
  after = readFileSync(builder, 'utf8');
  sameLines(before, after);
  expect(itemBlock(after, 'BUILD-1'), "the moved rule's own lines changed").toBe(
    itemBlock(before, 'BUILD-1'),
  );
  let at = 0;
  for (const id of ['BUILD-2', 'BUILD-3', 'BUILD-1', 'BUILD-4']) {
    const i = after.indexOf(`- id: ${id}`);
    expect(i, `${id} left the file`).toBeGreaterThanOrEqual(0);
    expect(i, `the file puts ${id} above the rule before it`).toBeGreaterThanOrEqual(at);
    at = i;
  }

  // ---- e2eRolesSaveRule: P4・C4 ----
  before = readFileSync(builder, 'utf8');
  const want = 'E2E が書き換えた本文。';
  await page.goto(`${app.base}/roles?open=builder`);
  await wait(page, there('#rows .role.open .panel .rule'));
  await press(page, `${rule('BUILD-5')}.querySelector('.rsum')`);
  await wait(page, there('#rows .role.open .rule.open .redit [data-f=text]'));
  await set(page, q('#rows .role.open .rule.open .redit [data-f=text]'), want);
  await press(page, `${row('builder')}.querySelector('.pf [data-act=save]')`);
  await wait(page, there('#rows .role.open .ok'));
  await readTo<string>(
    page,
    q('#rows .role.open .ok') + '.textContent',
    'the saved line reads, want 保存しました',
  ).toBe('保存しました');
  after = readFileSync(builder, 'utf8');
  const changed = changedLines(before, after);
  expect(
    changed.size,
    `the save changed lines, want the rule's text line alone: ${JSON.stringify([...changed])}`,
  ).toBe(1);
  for (const [line, text] of changed) {
    expect(text, `line ${line} became, want the rule's new text`).toBe(`        text: ${want}`);
  }
  expect(after, "the save lost BUILD-4's derived origin").toContain('refs: [REQ-5]');
  await readTo<string>(
    page,
    `${rule('BUILD-5')}.querySelector('.rtx').textContent`,
    "the saved rule's summary reads",
  ).toContain(want);
});

test('ROLES-P14 the person whose role is swapped', async ({ page, app }) => {
  // P14・P6・V2: one person's role is picked in its card, the card says how many
  // changed, and the save writes that one line of team.yaml.
  const file = path.join(app.proj, '.hergy', 'teams', 'research', 'team.yaml');
  const before = readFileSync(file, 'utf8');

  await page.goto(`${app.base}/roles/assign`);
  await wait(page, there('.team[data-t=research]'));
  await readTo<number>(
    page,
    `document.querySelectorAll('.team[data-t=research] [data-act=t-save]').length`,
    'a card nothing changed shows a save button',
  ).toBe(0);
  await choose(page, q('.team[data-t=research] .arow[data-m=scout-2] select'), 'writer');
  await wait(page, there('.team[data-t=research] [data-act=t-save]'));
  await readTo<string>(
    page,
    q('.team[data-t=research] .tside .dirty') + '.textContent',
    'the changed card says, want 1 人',
  ).toBe('1 人');
  await readTo<string>(
    page,
    q('.team[data-t=research] .arow[data-m=scout-2] s.was') + '.textContent',
    'the old role reads, want scout',
  ).toBe('scout');
  await press(page, q('.team[data-t=research] [data-act=t-save]'));
  await wait(page, there('.team[data-t=research] .ok'));
  await readTo<string>(
    page,
    q('.team[data-t=research] .ok') + '.textContent',
    "the card's saved line reads",
  ).toBe('保存しました');

  const changed = changedLines(before, readFileSync(file, 'utf8'));
  expect(
    changed.size,
    `the assign save changed lines, want scout-2's role alone: ${JSON.stringify([...changed])}`,
  ).toBe(1);
  for (const [line, text] of changed) {
    expect(text, `line ${line} became, want scout-2's new role`).toBe('    role: writer');
  }
  await readTo<string>(
    page,
    q('.team[data-t=research] .arow[data-m=scout-2] select') + '.value',
    'the card reads after the save, want writer',
  ).toBe('writer');
});
