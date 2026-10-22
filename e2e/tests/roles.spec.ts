// TestE2ERoles' two 大 items, written again in Playwright (d-3c9b):
// internal/app/e2e_roles_test.go's e2eRolesEditWalk (ROLES-P5, the rule a person
// edits in place) and e2eRolesAssign (ROLES-P14, the person whose role is
// swapped), assertion for assertion.
import { test, expect } from '@playwright/test';
import path from 'node:path';
import {
  changedLines, choose, go, makeFixture, number, press, readFile, removeRoot,
  setField, startServe, text, wait, waitFor, watch,
} from './harness';

// rowJS is a JS expression for the list row one role name stands on.
function rowJS(name: string): string {
  return `[...document.querySelectorAll('#rows .role')].find(function(r){var n=r.querySelector('.c-name .nm');return n&&n.textContent.trim()===${JSON.stringify(name)}})`;
}

// secJS is a JS expression for the section one heading stands on.
function secJS(heading: string): string {
  return `[...document.querySelectorAll('#rows .role.open .panel .sec')].find(function(s){var h=s.querySelector('[data-f=heading]');return h&&h.value===${JSON.stringify(heading)}})`;
}

// vt reads the Markdown preview's "N 行 · N 文字" as the two numbers.
function vt(text0: string): [number, number] {
  const f = text0.split(/\s+/).filter(Boolean);
  return [Number(f[0]), Number(f[3])];
}

// sameLines holds a save to moving lines about rather than changing them.
function sameLines(before: string, after: string): void {
  expect(after.split('\n').sort(), 'the file\'s lines changed, not only their order').toEqual(before.split('\n').sort());
}

// block is the lines from the one a heading stands on, down to the first line
// that is not indented deeper than it.
function block(text0: string, head: string, deeper: string): string {
  const lines = text0.split('\n');
  const start = lines.findIndex((l) => l.trim() === head);
  if (start < 0) throw new Error(`the file no longer holds ${head}`);
  let end = start + 1;
  while (end < lines.length && lines[end].startsWith(deeper)) end++;
  return lines.slice(start, end).join('\n');
}

const itemBlock = (text0: string, id: string) => block(text0, `- id: ${id}`, '        ');
const sectionBlock = (text0: string, heading: string) => block(text0, `- heading: ${heading}`, '    ');

// e2eRolesOpen is S1〜S6, P1〜P3・P15, I6, N1, R1・R4: the row that opens under
// itself, the rule that opens inside it, the checks the screen draws, the deep
// link, and the Markdown line that opens its own rule.
async function rolesOpen(page: import('@playwright/test').Page, base: string): Promise<void> {
  await go(page, `${base}/roles`);
  await waitFor(page, '#rows .role');
  await press(page, `${rowJS('builder')}.querySelector('.sum')`);
  await wait(page, `${rowJS('builder')}.classList.contains('open')`);
  expect(await text(page, 'window.location.search'), 'opening the builder row left open=builder in the URL').toContain('open=builder');
  expect(await number(page, `${rowJS('builder')}.querySelectorAll('.panel').length`), 'builder panels').toBe(1);
  expect(await number(page, `${rowJS('builder')}.querySelectorAll('.usage .uchip').length`), 'builder readers').toBe(3);
  expect(await text(page, `${rowJS('builder')}.querySelector('.uchip .g').textContent`), "the first reader's chip").toBe('core');
  expect(await number(page, `${rowJS('builder')}.querySelectorAll('.dhvm li').length`), 'builder document-level checks').toBe(0);

  // P5: the per-rule badge is the rule's own checks.
  const badge = (id: string) =>
    `(function(){var x=[...document.querySelectorAll('#rows .role.open .panel .rule')].find(function(r){return r.querySelector('.rid b').textContent.trim()===${JSON.stringify(id)}});var s=x.querySelector('.rid .sev');return s?s.textContent.trim():''})()`;
  for (const [id, want] of [['BUILD-1', ''], ['BUILD-3', '警告 1'], ['BUILD-4', '問題 1'], ['BUILD-5', '警告 1']]) {
    expect(await text(page, badge(id)), `the ${id} badge`).toBe(want);
  }
  expect(await text(page, `${rowJS('builder')}.querySelector('.pf .vcount').textContent`),
    "the panel's count").toContain('問題 1');
  expect(await text(page, `${rowJS('builder')}.querySelector('.pf .vcount').textContent`),
    "the panel's count").toContain('警告 2');
  expect(await number(page, `document.querySelectorAll('#rows .role.open .yprev .yl.err').length`), 'preview problems').toBe(1);
  expect(await number(page, `document.querySelectorAll('#rows .role.open .yprev .yl.warn').length`), 'preview warnings').toBe(2);
  expect(await number(page, `document.querySelectorAll('#rows .role.open .panel .sec [data-f=level]').length`), 'level pickers').toBe(2);
  expect(await number(page, `document.querySelectorAll('#rows .role.open .panel .sec [data-f=style]').length`), 'style pickers').toBe(0);
  expect(await text(page,
    `[...document.querySelectorAll('#rows .role.open .panel .sec [data-f=level]')].map(function(b){return b.textContent.trim()}).join(',')`),
    "the sections' depth").toBe('##,##');
  expect(await number(page, `document.querySelectorAll('#rows .role.open .panel select').length`), 'panel selects').toBe(0);
  expect(await number(page,
    `[...document.querySelectorAll('#rows .role.open .yprev .yl .yt')].filter(function(e){return e.textContent.indexOf('- **BUILD-')===0}).length`),
    'preview items with the id at their head').toBe(7);
  expect(await number(page, `document.querySelector('#rows .role.open .c-amount .meter i').getBoundingClientRect().width`),
    "the reader's meter").toBeGreaterThan(0);
  expect(await number(page,
    `Math.min.apply(null,[...document.querySelectorAll('#rows .role.open .yprev .yt')].map(function(e){return parseFloat(getComputedStyle(e).textIndent)}))`),
    'the smallest text-indent').toBeLessThan(0);
  // N1: the origin cell that says — stands one line high.
  const cell = (sel: string) =>
    `(function(){var c=[...document.querySelectorAll(${JSON.stringify(`#rows .role.open .panel .rule ${sel}`)})];return c.length?Math.round(Math.max.apply(null,c.map(function(e){return e.getBoundingClientRect().height}))):-1})()`;
  const emptyCell = await number(page, cell('.org.empty'));
  const namedCell = await number(page, cell('.org:not(.empty)'));
  expect(emptyCell, 'the origin cells to measure').toBeGreaterThan(0);
  expect(namedCell, 'the origin cells to measure').toBeGreaterThan(0);
  expect(emptyCell, 'the empty origin cell against a named one').toBe(namedCell);

  // P3: the rule opens under its own summary, and its place joins the URL.
  await press(page,
    `[...document.querySelectorAll('#rows .role.open .panel .rule')].find(function(r){return r.querySelector('.rid b').textContent.trim()==='BUILD-5'}).querySelector('.rsum')`);
  await wait(page, `!!document.querySelector('#rows .role.open .panel .rule.open .redit')`);
  expect(await text(page, 'window.location.search'), 'opening BUILD-5 left rule= in the URL').toContain('rule=builder.project.BUILD-5');
  expect(await text(page, `document.querySelector('#rows .role.open .rule.open .redit [data-f=id]').value`), 'the opened rule').toBe('BUILD-5');
  expect(await text(page, `document.querySelector('#rows .role.open .rule.open .redit [data-f=text]').value`),
    "the opened rule's text").toMatch(/^大きな設計の判断/);
  expect(await text(page, `${rowJS('builder')}.querySelector('.pedit [data-f=send_why] button.on').textContent`),
    'the why switch').toBe('プロンプトに含める');
  expect(await number(page,
    `document.querySelectorAll('#rows .role.open .rule.open .redit [data-act=rule-revert]:not([hidden])').length`),
    'a rule nothing changed shows 元に戻す').toBe(0);
  expect(await text(page, `${rowJS('builder')}.querySelector('.pf [data-act=save]').disabled+''`),
    'a rule nothing changed can be saved').toBe('true');

  // P1 again: the same row closes.
  await press(page, `${rowJS('builder')}.querySelector('.sum')`);
  await wait(page, `!${rowJS('builder')}.classList.contains('open')`);
  expect(await text(page, 'window.location.search'), 'closing the row left open= in the URL').not.toContain('open=');

  // A document-level check stands above the sections.
  await go(page, `${base}/roles?open=analyst`);
  await waitFor(page, '#rows .role.open .panel');
  expect(await number(page, `document.querySelectorAll('#rows .role.open .panel .dhvm li').length`), 'analyst document-level checks').toBe(1);
  expect(await text(page, `document.querySelector('#rows .role.open .panel .dhvm li span:last-child').textContent`),
    'the analyst check').toContain('core-analyst に渡る規則が 21 件です');
  expect(await number(page, `document.querySelectorAll('#rows .role.open .yprev .yl.warn').length`), "analyst's preview warnings").toBe(1);

  // A deep link opens the row and the rule it names.
  await go(page, `${base}/roles?open=builder&rule=builder.project.BUILD-5`);
  await waitFor(page, '#rows .role.open .rule.open .redit');
  expect(await text(page, `document.querySelector('#rows .role.open .rule.open .redit [data-f=id]').value`), 'the deep-linked rule').toBe('BUILD-5');

  // P15: a Markdown line opens its own rule and puts the cursor in the field.
  await press(page,
    `[...document.querySelectorAll('#rows .role.open .yprev .yl[data-act=yline]')].find(function(l){return l.textContent.indexOf('BUILD-3')>=0})`);
  await wait(page, `document.activeElement && document.activeElement.getAttribute('data-f')==='text'`);
  expect(await text(page, `document.querySelector('#rows .role.open .rule.open .redit [data-f=id]').value`), 'the Markdown line opened').toBe('BUILD-3');
}

// rolesInPrompt is I1・I2・A1: the 規則 switch keeps one rule out of the prompt,
// and the why's own switch dims the why alone.
async function rolesInPrompt(page: import('@playwright/test').Page, base: string, f: ReturnType<typeof makeFixture>): Promise<void> {
  const at = path.join(f.proj, '.hergy', 'roles', 'lead.yaml');
  const before = readFile(at);

  await go(page, `${base}/roles?open=lead&rule=lead.project.LEAD-9`);
  await waitFor(page, '#rows .role.open .rule.open .redit [data-act=in-prompt]');
  const panel = `document.querySelector('#rows .role.open .panel[data-l=project]')`;
  const row = `document.querySelector('#rows .role.open')`;

  // A1: the rule's own switch stands under the field's name 規則.
  expect(await text(page, `${panel}.querySelector('.redit .field.scope .fl').textContent.trim()`),
    "the rule's own switch stands under").toBe('規則');
  const [lines0, chars0] = vt(await text(page, `${panel}.querySelector('.yprev .yinfo .vt').textContent`));
  expect(await text(page, `${panel}.querySelector('.sec[data-si="2"] .sech .n').textContent.trim()`),
    'the last section before the switch').toBe('3 規則');
  expect(await number(page, `${row}.querySelectorAll('.c-count .outn').length`), 'the list marks an 含めない rule').toBe(0);
  expect(await text(page, `${row}.querySelector('.c-amount .amt .v').firstChild.textContent`),
    "the lead reader's amount before the switch").toBe('18');

  // 含めない.
  const outRule = `[...document.querySelectorAll('#rows .role.open .panel .rule')].find(function(r){return r.querySelector('.rid b').textContent.trim()==='LEAD-9'})`;
  await press(page, `${outRule}.querySelector('[data-act=in-prompt][data-v=false]')`);
  await wait(page, `${outRule}.classList.contains('out') && !!${outRule}.querySelector('.marks .mk.out')`);
  await wait(page, `${panel}.querySelector('.sec[data-si="2"] .sech .n').textContent.indexOf('含めない 1')>=0`);
  await wait(page, `${row}.querySelector('.c-count .outn') && ${row}.querySelector('.c-count .outn').textContent.trim()==='含めない 1'`);
  await wait(page, `${row}.querySelector('.c-amount .amt .v').firstChild.textContent==='17'`);
  await wait(page, `!!document.querySelector('#rows .role.open .yprev .yl.ghost .yt s')`);
  expect(await text(page, `document.querySelector('#rows .role.open .yprev .yl.ghost .lno').textContent`),
    "the excluded rule's line carries a number").toBe('');
  const [lines1, chars1] = vt(await text(page, `${panel}.querySelector('.yprev .yinfo .vt').textContent`));
  expect(lines1 < lines0 || chars1 < chars0, 'the preview after the switch').toBe(true);
  expect(await text(page, `${panel}.querySelector('.sec[data-si="2"] .sech .n').textContent.trim()`),
    'the last section after the switch').toBe('3 規則含めない 1');
  expect(await text(page, `${row}.querySelector('.pf .dirty').textContent`), 'the switch in the footer').toBe('1 か所');

  // I1: the save writes the one line and keeps every other line of the file.
  await press(page, `${row}.querySelector('.pf [data-act=save]')`);
  await wait(page, `!!${row}.querySelector('.ok')`);
  const after = readFile(at);
  const changed = changedLines(before, after);
  expect(changed.size, 'the save changed lines').toBe(1);
  for (const line of changed.values()) {
    expect(line, 'the changed line').toBe('        in_prompt: false');
  }
  expect(after, "the save kept LEAD-9's item").toContain('- id: LEAD-9\n        text: ');

  // I2: the why's switch stands under なぜ, and 含めない there dims the why alone.
  await go(page, `${base}/roles?open=builder&rule=builder.project.BUILD-1`);
  await waitFor(page, '#rows .role.open .rule.open .redit .whyrow [data-act=send-why]');
  const bpanel = `document.querySelector('#rows .role.open .panel[data-l=project]')`;
  const whyField = `(function(){var f=[...document.querySelectorAll('#rows .role.open .rule.open .redit > .field')].find(function(d){var l=d.querySelector('label');return l&&l.textContent.trim()==='なぜ'});return f?f.querySelector('.whyrow [data-f=send_why]')?'yes':'no':'none'})()`;
  expect(await text(page, whyField), "the why's switch is not under なぜ").toBe('yes');
  await press(page, `${bpanel}.querySelector('.rule.open .whyrow [data-act=send-why][data-v=false]')`);
  await wait(page, `!!document.querySelector('#rows .role.open .rule.open .redit textarea.held-why')`);
  await wait(page, `!!document.querySelector('#rows .role.open .yprev .yl .held s')`);
  expect(await text(page, `${bpanel}.querySelector('.rule.open .rsum .mk.why').className`),
    "the rule's なぜ mark after its own 含めない").toContain('held');
  expect(await number(page, `document.querySelectorAll('#rows .role.open .panel .rule.out').length`),
    "the why's 含めない dimmed whole rules").toBe(0);
  expect(await text(page, `${bpanel}.querySelector('.rule.open .redit [data-f=why]').value`),
    "the why's 含めない emptied the why").not.toBe('');
}

// rolesHeadingDepth is I3・A2: ### makes the Markdown's heading deeper and the
// save add the section's one level: 3 line, and nothing else.
async function rolesHeadingDepth(page: import('@playwright/test').Page, base: string, f: ReturnType<typeof makeFixture>): Promise<void> {
  const at = path.join(f.proj, '.hergy', 'roles', 'builder.yaml');
  const before = readFile(at);

  await go(page, `${base}/roles?open=builder`);
  await waitFor(page, '#rows .role.open .panel .sec [data-act=sec-pick]');
  const make = secJS('作り方');
  expect(await text(page, `${make}.querySelector('.sech [data-f=level]').textContent.trim()`), "作り方's depth").toBe('##');
  await press(page, `${make}.querySelector('.sech [data-act=sec-pick][data-w=lvl]')`);
  await wait(page, `${make}.querySelectorAll('.pickrow .lvl').length===3`);
  expect(await text(page, `[...${make}.querySelectorAll('.pickrow .lvl')].map(function(b){return b.textContent.trim().split(' ')[0]}).join(',')`),
    "the depth's three marks").toBe('##,###,####');
  await press(page, `${make}.querySelector('.pickrow [data-act=sec-level][data-v="3"]')`);
  await wait(page, `${make}.querySelector('.sech [data-f=level]').textContent.trim()==='###'`);
  await wait(page, `[...document.querySelectorAll('#rows .role.open .yprev .yl .yt')].some(function(e){return e.textContent.trim()==='### 作り方'})`);
  expect(await text(page, `document.querySelector('#rows .role.open .pf .dirty').textContent`), 'the depth change in the footer').toBe('1 か所');
  expect(await text(page, `${make}.querySelector('.sech [data-f=level]').className`), 'the depth mark after the change').toContain('changed');

  await press(page, `document.querySelector('#rows .role.open .pf [data-act=save]')`);
  await wait(page, `!!document.querySelector('#rows .role.open .ok')`);
  const after = readFile(at);
  expect(after.replace('    level: 3\n', ''), 'the save added the one level: 3 line and nothing else').toBe(before);
  expect(after.split('\n').length - before.split('\n').length, 'the save added lines').toBe(1);
}

// rolesSectionOrder is I4・A4: a section's ↑ moves the whole section up, the
// Markdown follows, and the save comes back as the same lines swapped.
async function rolesSectionOrder(page: import('@playwright/test').Page, base: string, f: ReturnType<typeof makeFixture>): Promise<void> {
  const at = path.join(f.proj, '.hergy', 'roles', 'builder.yaml');
  const before = readFile(at);

  await go(page, `${base}/roles?open=builder`);
  await waitFor(page, '#rows .role.open .panel .sec [data-act=sec-up]');
  expect(await text(page, `document.querySelectorAll('#rows .role.open .panel .sec')[0].querySelector('[data-act=sec-up]').disabled+''`),
    "the first section's ↑ is not shut").toBe('true');
  expect(await text(page, `document.querySelectorAll('#rows .role.open .panel .sec')[1].querySelector('[data-act=sec-down]').disabled+''`),
    "the last section's ↓ is not shut").toBe('true');
  await press(page, `document.querySelectorAll('#rows .role.open .panel .sec')[1].querySelector('[data-act=sec-up]')`);
  await wait(page, `document.querySelectorAll('#rows .role.open .panel .sec [data-f=heading]')[0].value==='確かめ方'`);
  expect(await text(page, `document.querySelector('#rows .role.open .pf .dirty').textContent`), 'moving the sections in the footer').toBe('1 か所');
  await wait(page,
    `(function(){var t=[...document.querySelectorAll('#rows .role.open .yprev .yl .yt')].map(function(e){return e.textContent});var a=t.findIndex(function(s){return s.indexOf('確かめ方')>=0});var b=t.findIndex(function(s){return s.indexOf('作り方')>=0});return a>=0&&b>=0&&a<b})()`);

  await press(page, `document.querySelector('#rows .role.open .pf [data-act=save]')`);
  await wait(page, `!!document.querySelector('#rows .role.open .ok')`);
  const after = readFile(at);
  sameLines(before, after);
  expect(after.indexOf('  - heading: 確かめ方'), 'the moved section did not land first').toBeLessThan(after.indexOf('  - heading: 作り方'));
  expect(sectionBlock(after, '確かめ方'), "the moved section's own lines changed").toBe(sectionBlock(before, '確かめ方'));
  expect(sectionBlock(after, '作り方'), "the other section's own lines changed").toBe(sectionBlock(before, '作り方'));
}

// rolesRuleDrag is I5・A5: a rule is dragged by its ⋮ handle below another rule
// and the save comes back as the same lines with the rule's item moved.
async function rolesRuleDrag(page: import('@playwright/test').Page, base: string, f: ReturnType<typeof makeFixture>): Promise<void> {
  const at = path.join(f.proj, '.hergy', 'roles', 'builder.yaml');
  const before = readFile(at);

  await go(page, `${base}/roles?open=builder`);
  await waitFor(page, '#rows .role.open .panel .sec .rule .grip');
  const makeOrder = `[...${secJS('作り方')}.querySelectorAll('.rule .rid b')].map(function(b){return b.textContent.trim()}).join(',')`;
  expect(await text(page, makeOrder), "作り方's rules before the drag").toBe('BUILD-1,BUILD-2,BUILD-3,BUILD-4');
  const drag = `(function(){
		var make=${secJS('作り方')};
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
  expect(await text(page, drag), 'the drag did not start').toBe('dragged');
  await wait(page, `!!document.querySelector('#rows .role.open .panel .sec .rule.flash')`);
  expect(await text(page, makeOrder), 'the dropped rule landed').toBe('BUILD-2,BUILD-3,BUILD-1,BUILD-4');
  expect(await text(page, `document.querySelector('#rows .role.open .pf .dirty').textContent`), 'the drag in the footer').toBe('1 か所');

  await press(page, `document.querySelector('#rows .role.open .pf [data-act=save]')`);
  await wait(page, `!!document.querySelector('#rows .role.open .ok')`);
  const after = readFile(at);
  sameLines(before, after);
  expect(itemBlock(after, 'BUILD-1'), "the moved rule's own lines changed").toBe(itemBlock(before, 'BUILD-1'));
  let previous = 0;
  for (const id of ['BUILD-2', 'BUILD-3', 'BUILD-1', 'BUILD-4']) {
    const i = after.indexOf(`- id: ${id}`);
    expect(i, `${id} left the file`).toBeGreaterThanOrEqual(0);
    expect(i, `the file puts ${id} above the rule before it`).toBeGreaterThanOrEqual(previous);
    previous = i;
  }
}

// rolesSaveRule is P4・C4: one rule's text is written on the line it stands on,
// and every other line of the file comes back byte for byte.
async function rolesSaveRule(page: import('@playwright/test').Page, base: string, f: ReturnType<typeof makeFixture>): Promise<void> {
  const at = path.join(f.proj, '.hergy', 'roles', 'builder.yaml');
  const before = readFile(at);
  const want = 'E2E が書き換えた本文。';

  await go(page, `${base}/roles?open=builder`);
  await waitFor(page, '#rows .role.open .panel .rule');
  await press(page,
    `[...document.querySelectorAll('#rows .role.open .panel .rule')].find(function(r){return r.querySelector('.rid b').textContent.trim()==='BUILD-5'}).querySelector('.rsum')`);
  await wait(page, `!!document.querySelector('#rows .role.open .rule.open .redit [data-f=text]')`);
  await setField(page, `document.querySelector('#rows .role.open .rule.open .redit [data-f=text]')`, want);
  await press(page, `${rowJS('builder')}.querySelector('.pf [data-act=save]')`);
  await wait(page, `!!document.querySelector('#rows .role.open .ok')`);
  expect(await text(page, `document.querySelector('#rows .role.open .ok').textContent`), 'the saved line').toBe('保存しました');

  const after = readFile(at);
  const changed = changedLines(before, after);
  expect(changed.size, 'the save changed lines').toBe(1);
  for (const line of changed.values()) {
    expect(line, "the rule's new text").toBe(`        text: ${want}`);
  }
  expect(after, "the save lost BUILD-4's derived origin").toContain('refs: [REQ-5]');
  expect(await text(page,
    `(function(){var x=[...document.querySelectorAll('#rows .role.open .rule')].find(function(r){return r.querySelector('.rid b').textContent.trim()==='BUILD-5'});return x.querySelector('.rtx').textContent})()`),
    "the saved rule's summary").toContain(want);
}

test('ROLES-P5 the rule a person edits', async ({ page }) => {
  const f = makeFixture('roles');
  const serve = await startServe(f);
  try {
    const said = watch(page, false);
    await rolesOpen(page, serve.url);
    await rolesInPrompt(page, serve.url, f);
    await rolesHeadingDepth(page, serve.url, f);
    await rolesSectionOrder(page, serve.url, f);
    await rolesRuleDrag(page, serve.url, f);
    await rolesSaveRule(page, serve.url, f);
    expect(said, 'the role screen logged console errors').toEqual([]);
  } finally {
    await serve.stop();
    removeRoot(f);
  }
});

test('ROLES-P14 the person whose role is swapped', async ({ page }) => {
  const f = makeFixture('roles');
  const serve = await startServe(f);
  try {
    const said = watch(page, false);
    // P6・V2: one person's role is picked in its card, the card says how many
    // changed, and the save writes that one line of team.yaml.
    const at = path.join(f.proj, '.hergy', 'teams', 'research', 'team.yaml');
    const before = readFile(at);
    await go(page, `${serve.url}/roles/assign`);
    await waitFor(page, '.team[data-t=research]');
    expect(await number(page, `document.querySelectorAll('.team[data-t=research] [data-act=t-save]').length`),
      'a card nothing changed shows a save button').toBe(0);
    await choose(page, `document.querySelector('.team[data-t=research] .arow[data-m=scout-2] select')`, 'writer');
    await wait(page, `!!document.querySelector('.team[data-t=research] [data-act=t-save]')`);
    expect(await text(page, `document.querySelector('.team[data-t=research] .tside .dirty').textContent`),
      'the changed card').toBe('1 人');
    expect(await text(page, `document.querySelector('.team[data-t=research] .arow[data-m=scout-2] s.was').textContent`),
      'the old role').toBe('scout');
    await press(page, `document.querySelector('.team[data-t=research] [data-act=t-save]')`);
    await wait(page, `!!document.querySelector('.team[data-t=research] .ok')`);
    expect(await text(page, `document.querySelector('.team[data-t=research] .ok').textContent`), "the card's saved line").toBe('保存しました');
    const after = readFile(at);
    const changed = changedLines(before, after);
    expect(changed.size, 'the assign save changed lines').toBe(1);
    for (const line of changed.values()) {
      expect(line, "scout-2's new role").toBe('    role: writer');
    }
    expect(await text(page, `document.querySelector('.team[data-t=research] .arow[data-m=scout-2] select').value`),
      'the card after the save').toBe('writer');
    expect(said, 'the role screen logged console errors').toEqual([]);
  } finally {
    await serve.stop();
    removeRoot(f);
  }
});
