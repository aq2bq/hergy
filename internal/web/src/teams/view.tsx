import { ProjectView } from '../lib/project-view';
// teams/view.tsx — チーム定義の画面の一覧とパネル（webui-preact-design 4 章）。
//
// 一覧の行・パネル・足・YAML のタブ・トポロジーの器を描く。class と data-* と
// 文書の表示選択は保存対象から独立させる。状態は state.ts の
// signal を読み、押し込みは自分の onClick / onInput / onKeyDown から state.ts の
// 関数を直接呼ぶ（依頼書 2）。規則の文書の中は共有部品（lib/ruledoc.js）が描く。
import { useEffect, useRef, useState, useCallback } from '../lib/preact.js';
import type { ComponentChildren, TargetedEvent } from 'preact';
import { num } from '../lib/measure.js';
import { YamlTab } from '../lib/yamltab.js';
import { Canvas } from './canvas.js';
import { Rename } from './rename.js';
import { EqualWidths } from '../lib/prose-width.js';
import { transferKeys } from '../lib/prose-transfer-view.js';
import { teamTransfer } from './prose-transfer.js';
import { selectedRules, ruleSelection, selectRule } from './rule-selection.js';
import { RoleColumn, roleSummary } from './role-modal.js';
import { RuleDoc, Counts, Vm, Sev, clone } from '../lib/ruledoc.js';
import { chars } from '../lib/prose-editor.js';
import { Field } from '../lib/field.js';
import { ConflictNotice } from '../lib/conflict.js';
import { RunCell } from '../lib/hq/run.js';
import { state as hq, projectName as hqProjectName } from '../lib/hq.js';
import { copyGroupOf } from '../lib/copy.js';
import { bringUnderBand } from '../lib/band.js';
import { askConfirm, ConfirmDialog } from '../lib/confirm.js';
import {
  PLACES, PLACE, DIR, HQ, GROUPED, GROUPS, CURRENT, state, allRecs, rowsOfGroup, placeGroups,
  dkey, currentProjectName, teamLabel, dtabOf, isOpen, missingOpen,
  rowIssues, dirtyDef, teamIssues, agreeIssues, agreeDiff, changeCount, vlistParts,
  rowOfPanel, okOf, refusalOf, pmodeOf, yfiles, fileOf, fileDirty, yIssues,
  recForFile, chosenFile, scheduleAgree, syncURL, focusAsk, groupOfRecord,
  pressTab, pressToggle, pressNew, pressDup, pressCopy, pressDiscard,
  pressStarterRoles, pressStarterProfile, pressCarry,
  pressDelYes, pressName, pressDtab, pressPmode, bornOf,
  pressToYaml, pressTSave, pressTRevert, pressSave, pressRevert, pressVpick,
  pressToastUndo, pressYfile, yamlState, yamlProblems, yamlRevert, yamlSaved, fileText,
  conflictAt, pressOverwrite, pressRevertConflict, pressUndoConflict,
} from './state.js';
import type { Rec, TeamGroup, YFile } from './state.js';
import type { Schemas } from '../lib/api';

// messagesOf is the sentences a part draws its count from: the server's
// findings carry `text`, the rules panel's own messages carry `msg`, and the
// count reads either (lib/ruledoc's Message).
function messagesOf(list: { sev: string; text?: string }[]): { sev: string; text: string }[] {
  return list.map((m) => ({ sev: m.sev, text: m.text || '' }));
}

// inputValue is what a field holds after a person typed in it.
function inputValue(e: Event): string {
  const t = e.target;
  return t instanceof HTMLInputElement || t instanceof HTMLTextAreaElement || t instanceof HTMLSelectElement ? t.value : '';
}

const other = (p: 'project' | 'global'): 'project' | 'global' => (p === 'project' ? 'global' : 'project');
// placeName is the word a place is drawn by; anything else is this project's.
const placeName = (p: string): string => (p === 'global' ? PLACE.global : PLACE.project);

// TABS are the screen's two tabs: 一覧 and YAML.
const TABS: ['list' | 'yaml', string][] = [['list', '一覧'], ['yaml', 'YAML']];
// MODES are the rules tab's two views: the editor and its Markdown.

// Tabs is the screen's two tabs: 一覧 and YAML.
export function Tabs() {
  const tab = state.value.tab;
  return TABS.map(([t, label]) => (
    <button class={'tab' + (tab === t ? ' on' : '')} role="tab" aria-controls="view" aria-selected={tab === t} tabIndex={tab === t ? 0 : -1} data-act="tab" data-t={t} aria-pressed={tab === t} onClick={() => pressTab(t)}>{label}</button>
  ));
}

export function App() {
  // The 動き column and the group heads are the headquarters' reading: each
  // reads it, so a re-reading draws it without the screen asking (5 章 2).
  return <>{state.value.tab === 'list' ? <List /> : <><ProjectView projects={HQ?.projects} /><YamlPage /></>}<CanvasToast /><ConfirmDialog /></>;
}

// ---- 一覧 --------------------------------------------------------------
// List is the team definitions under one head per place, each head carrying the
// button that adds a definition to that place alone (r-410c, d-e77c). The list
// above carries no add button.
export function List() {
  // A headquarters whose lists could not be read says why, rather than drawing
  // the one project's lists as though the projects were not there (r-1db7 L1).
  if (HQ && HQ.error) return <p class="hq-err" data-role="hq-error">{HQ.error}</p>;
  const head = (
    <div class="head"><span></span><span>チーム定義名</span><span>説明</span><span>メンバー</span><span>ワーカー</span><span>規則とひな形（目安 1,200 字）</span>{HQ ? <span class="hq-runh">状態と操作</span> : null}</div>
  );
  const body = placeGroups().map((g) => {
    const rows = rowsOfGroup(g.key);
    return <>{PlaceHead({ g })}{rows.map((r) => <Row key={r.key.value} r={r} />)}
      {rows.length === 0 ? <div class="hq-empty"><span class="none">{g.name} にはチーム定義がありません。</span></div> : null}
    </>;
  });
  return <>{missingOpen.value.length ? <p role="status">チーム定義 {missingOpen.value.join('、')} が見つかりません。<a href="/teams">一覧を見る</a></p> : null}<div class="list hq-grouped">{head}{body}</div></>;
}

// PlaceHead is one place's head (spec 4 章, d-e77c): its name, where it is,
// how many definitions and running teams it holds, and the button that adds a
// definition to this place alone.
function PlaceHead({ g }: { g: TeamGroup }) {
  const key = g.key;
  const target = key === 'global' ? 'global' : 'project';
  const said = key === 'global' ? 'グローバル' : g.name;
  const count = rowsOfGroup(key).length;
  const live = ((hq.running.value && hq.running.value.teams) || []).filter((t: { project?: string }) => t.project === (key || CURRENT || HQ?.projects?.current?.path)).length;
  return <div class={'hq-gh' + (g.global ? ' global' : '')} data-group={key} data-place={target} aria-label={`置き場所: ${said}`}>
      <span class="hq-gn">{g.name}</span><span class="hq-gp">{g.display || ''}</span>
      {g.global
        ? <span class="hq-gc">どのプロジェクトからも使える。同じ名前があれば、そのプロジェクトのほうが使われる</span>
        : <span class="hq-gc">チーム定義 <b>{count}</b>{HQ ? <> · 動いているチーム <b>{live}</b></> : null}</span>}
      <button class="btn sm hq-add" data-act="new" data-group={key} data-place={target} onClick={() => pressNew(key)}>＋ {said} にチーム定義を追加</button>
    </div>;
}

// Row is one definition: the summary a person presses, and the detail it opens
// under itself (spec-teams B1). The field a press asked for is put in place after
// the draw, by the row itself (5 章 5): nobody puts the focus back later.
function Row({ r }: { r: Rec }) {
  const open = isOpen(r.dom, r.place.value);
  const el = useRef<HTMLDivElement>(null);
  const wasOpen = useRef(false);
  useEffect(() => {
    if (open && !wasOpen.current && dtabOf(r) !== 'agree') bringUnderBand(el.current);
    wasOpen.current = open;
  }, [open]);
  useEffect(() => {
    const ask = focusAsk.value;
    if (!ask || ask.dom !== r.dom || !el.current) return;
    const one = el.current.querySelector<HTMLElement>(ask.sel);
    if (!one) return;
    one.focus({ preventScroll: true });
    if (one instanceof HTMLInputElement) one.select();
    // A copy brings the page to the row it made (d-022a): the row a person
    // pressed stands in the other place, and the copy is off the screen.
    if (ask.scroll) bringUnderBand(el.current);
  }, [focusAsk.value]);
  return (
    <div class={'tdef' + (open ? ' open' : '') + (dirtyDef(r) ? ' unsaved' : '')} data-name={r.dom} data-key={dkey(r)} ref={el}>
      <div class="sum" role="group" data-act="toggle" onClick={(e) => pressSum(r, e)}><Sum r={r} open={open} /></div>
      {open ? <Detail r={r} /> : null}</div>
  );
}

// pressSum opens or closes the row. A press on the row's own field, or on a
// button of the headquarters' 動き column, is not the row's: the field keeps the
// cursor, and the start or the stop acts on the definition rather than opening
// the row (r-5b96 audit 1).
function pressSum(r: Rec, e: TargetedEvent<HTMLElement, MouseEvent>): void {
  const target = e.target instanceof Element ? e.target : null;
  if (!target) return;
  if (target.closest('[data-hq], .in-description, .in-name')) return;
  // A press on another control inside the row (the place's buttons) is that
  // control's, not the row's: only a press on the row itself opens or closes it.
  const own = target.closest('button, a, input, select, [data-act]');
  if (own && own !== e.currentTarget) return;
  pressToggle(r);
}

// nameCell is the row's name column: a field while a definition that is not
// there yet is being named, and the name itself otherwise. A copy keeps the
// name it copies, and its field holds that name with all of it chosen, so typing
// replaces it (spec-teams B3, d-022a).
function NameCell({ r, open }: { r: Rec; open: boolean }) {
  const tag = r.draftRow ? <span class="tag">新規</span> : null;
  const name = r.name.value;
  if (!r.draftRow) return <><span class="nm">{name}</span>{tag}</>;
  if (!open) return <><span class="nm">{name || <span class="none">—</span>}</span>{tag}</>;
  return (
    <>
      <Field id={'nm-' + r.dom} class={'in-name' + ((r.nameIssues.value || []).length ? ' bad' : '')} data-f="name" value={name} placeholder="チーム定義名" spellcheck={false} autocomplete="off" aria-label="チーム定義名" onInput={(e: Event) => pressName(r, inputValue(e))} />{tag}
    </>
  );
}

// groupLabel is the name a place is drawn by inside a row: the project's name
// (proj), グローバル, or プロジェクト when no headquarters reading names it.
const groupLabel = (r: Rec): string => (r.project === 'global' ? 'グローバル' : (r.project ? hqProjectName(r.project) : currentProjectName()));

// placePills is the row's 置き場所 column (spec 4 章, d-e77c): the place's name.
// A project that overrides a global name carries the crossed-out グローバル, and
// the global row says which projects override it.
function placePills(r: Rec): ComponentChildren {
  const label = groupLabel(r);
  const globalGroup = (GROUPS || []).find((g) => g.global);
  const over = (globalGroup && globalGroup.overridden_by) || {};
  const name = r.name.value;
  const shadow = r.project && r.project !== 'global' && (over[name] || []).includes(r.project);
  const overriding = r.project === 'global' ? (over[name] || []) : [];
  return <><span class="pill on">{label}</span>{shadow ? <span class="pill shadow">グローバル</span> : null}{overriding.map((p) => <span class="pill hq-ov">{hqProjectName(p)} で上書き</span>)}</>;
}

// runProject is the project a row's team runs in: without projects:, it is the
// one this serve was started in (r-5b96 audit 5).
const runProject = (r: Rec): string => (r.place.value === 'global' ? '' : (r.project || CURRENT));

function Sum({ r, open }: { r: Rec; open: boolean }) {
  const t = r.draft.value;
  // A team.yaml that does not read has no figure and no numbers to count, so
  // its row says 説明 —, keeps the problems validate named, and leaves the
  // counts blank (spec-teams 1.9). It is fixed in the YAML tab.
  const broken = r.broken.value;
  // A definition that is not there yet stands in the head it was added under:
  // the head, not a switch inside the row, says the place (r-410c). A saved row
  // says which place it is (spec 4 章).
  const pills: ComponentChildren = r.draftRow
    ? <span class="pill pending">{groupLabel(r)}</span>
    : placePills(r);
  const cell = broken ? '—' : t.description || '—';
  const size = chars(r.agreeDraft.value.text) + (r.check.value?.templateSize ?? Math.max(0, r.size.value - chars(r.agreeSaved.value?.text || '')));
  return (
    <>
      <button type="button" class="chev disclosure" aria-label={`${teamLabel(r)} を開閉する`} aria-expanded={open} onClick={(e) => { e.stopPropagation(); pressToggle(r); }}></button>
      <span class="c-name"><NameCell r={r} open={open} /><span class="vcount"><Counts list={messagesOf(rowIssues(r))} /></span></span>
      <span class={'c-description' + (broken || !(t.description || open) ? ' none' : '')}>{cell}</span>
      <span class="c-place">{pills}</span>
      <span class="c-members c-num">{broken ? '—' : t.members.length}</span>
      <span class="c-turns c-num">{broken ? '—' : t.workers.length}</span>

      <span class="c-size">{broken || !r.limit.value ? <span class="none">—</span> : <span class={'amt' + (size > r.limit.value ? ' over' : '')} title={`暫定の目安：${num(r.limit.value)}字。超えても保存・起動できます。`}><span class="v">{num(size)} 字</span></span>}</span>
      {HQ && !r.draftRow ? <span class="hq-runc"><RunCell definition={r.name.value} place={r.place.value} project={runProject(r)} /></span> : null}
    </>
  );
}

// ---- detail ------------------------------------------------------------
function Detail({ r }: { r: Rec }) {
  const hidden = r.hidden.value || [];
  const body = useRef<HTMLDivElement>(null);
  const [floor, setFloor] = useState(0);
  // The tab body never shrinks below the tallest it has been for this
  // definition: switching the tab or picking other documents changes what
  // stands there, not where the page is (r-e9a2, ac-4ba8, ac-c66a). The floor is
  // raised at the press itself, before the switch is drawn, and by its own
  // size afterwards, so the browser never has a shorter page to clamp into.
  const hold = useCallback(() => {
    const node = body.current;
    if (node) setFloor((was) => Math.max(was, node.getBoundingClientRect().height));
  }, []);
  useEffect(() => {
    const node = body.current;
    if (!node) return;
    const grow = () => setFloor((was) => Math.max(was, node.getBoundingClientRect().height));
    grow();
    const observer = new ResizeObserver(grow);
    observer.observe(node);
    return () => observer.disconnect();
  }, []);
  return (
    <div class="detail" onClickCapture={hold} onKeyDownCapture={hold}>
      <ul class="vm mvm"><Vm list={[...(r.nameIssues.value || []), ...r.findings.value.filter((m) => m.field === 'description')].map((m) => ({ sev: m.sev, text: m.text }))} /></ul>
      <nav class="dtabs"><DtabsNav r={r} /></nav>

      <div class="dbody" ref={body} style={floor ? { minHeight: `${floor}px` } : undefined}>
        {dtabOf(r) === 'shape' ? <TeamPanel r={r} /> : <AgreePanel r={r} />}
      </div>
      {hidden.map((h) => (
        <section class="panel shadowed"><header class="ph"><span class="ln">{placeName(h.place)}</span><code class="path">{(DIR[h.place] || '') + r.name.value}/</code><span class="state off">隠れている</span><span class="phr"><button class="btn sm" data-act="to-yaml" data-key={`${r.name.value}.${h.place}.team`} onClick={() => pressToYaml(`${r.name.value}.${h.place}.team`)}>YAML</button></span></header></section>
      ))}
    </div>
  );
}

function DtabsNav({ r }: { r: Rec }) {
  const cur = dtabOf(r);
  const tabs: [string, string, string, boolean][] = [
    ['shape', 'トポロジー', '', changeCount(r) > 0],
    ['agree', '規則', num(chars(r.agreeDraft.value.text)), r.agreeSaved.value ? agreeDiff(r) > 0 : false],
  ];
  return (
    <>
      <span class="dtl" role="tablist" aria-label="チーム定義の編集">{tabs.map(([t, label, n, ch]) => <button class={'dtab' + (cur === t ? ' on' : '')} data-act="dtab" data-t={t} role="tab" aria-selected={cur === t} tabIndex={cur === t ? 0 : -1} onClick={() => pressDtab(r, t)}>{label}{n !== '' ? <span class="n">{n}</span> : null}{ch ? <i class="dot"></i> : null}</button>)}</span>
      <span class="dacts">{!r.draftRow ? <Rename r={r} /> : null}<Dacts r={r} /></span>
    </>
  );
}

// dacts is what one definition's whole folder is offered: 複製, a copy into the
// other place, and 削除 with its own confirm. A definition that is not there
// yet is only taken back (spec-teams B3, B4, B5).
function Dacts({ r }: { r: Rec }) {
  // Deleting cannot be taken back: it is asked for in the shared dialog, away
  // from this button, so the same place pressed twice never deletes (r-d480).
  const del = () => askConfirm({
    title: `${teamLabel(r)} を削除しますか`,
    body: <><p>{`${placeName(r.place.value)} の ${r.name.value}/ から team.yaml と agreement.md を消します。`}</p><p><b>取り消せません。</b></p></>,
    label: '削除する（team.yaml と agreement.md）',
    run: () => pressDelYes(r),
  });
  // The starter's words stand on the base the row was seeded with, so the same
  // rule as the other screens holds: nothing changed since it was made is thrown
  // away at once, and anything the person put in is asked for (r-d480).
  const touched = () => r.born.value !== '' && r.born.value !== bornOf(r);
  const discard = () => {
    if (!touched()) { pressDiscard(r); return; }
    askConfirm({
      title: '入力した内容を捨てますか',
      body: <p>保存していない入力があります。<b>取り消せません。</b></p>,
      label: '破棄する',
      run: () => pressDiscard(r),
    });
  };
  if (r.draftRow) return <button class="btn danger" data-act="discard" onClick={discard}>取り消し</button>;
  const o = other(r.place.value);
  const group = copyGroupOf(o, groupOfRecord(r), GROUPED ? CURRENT : '');
  // A name the side the copy would go to already holds is not offered again:
  // on a grouped screen that side is the copy's own group, and a saved copy
  // there counts as much as one that is not saved yet (r-f781).
  const copying = allRecs().some((x) => x.name.value === r.name.value && x.place.value === o
    && (!GROUPED || groupOfRecord(x) === group));
  return (
    <>
      <button class="btn" data-act="dup" onClick={() => pressDup(r)}>複製</button>
      {!(r.places || []).includes(o) && !copying ? <button class="btn" data-act="copy" onClick={() => pressCopy(r)}>{PLACE[o]}にコピー</button> : null}
      <button class="btn danger" data-act="del" onClick={del}>削除</button>
    </>
  );
}

// ---- トポロジーのタブ --------------------------------------------------
// teamPanel is the トポロジー tab: the canvas and its attribute column
// (canvas.js), the check column under the canvas, and the save's band
// (spec-teams B7). The column's 属性 / YAML tabs pick what stands on the right.
function TeamPanel({ r }: { r: Rec }) {
  const k = dkey(r);
  const both = (r.places || []).length > 1;
  const head = (
    <header class="ph"><code class="path">{r.teamPath.value}</code>
      <span class="phr">{both ? <span class={'state ' + (r.place.value === 'global' ? 'off' : 'on')}>{r.place.value === 'global' ? '隠れている' : '効いている'}</span> : null}</span></header>
  );
  // A team.yaml that does not read draws no figure, and a saved one offers no
  // shape save: the check column alone says what stopped it, and the YAML tab is
  // where the text is put right (spec-teams 1.9). A row that is not there yet is
  // the exception: it stands in no file, so the YAML tab holds nothing to put
  // right, and its save is the only place that row can be kept. Such a row keeps
  // its save, beside the sentence that says what stopped the figure (r-a028,
  // d-9abb).
  if (r.broken.value) {
    return (
      <section class="panel team-p" data-key={k}>
        {head}
        <div class="gbody"><div class="vlist"><VList r={r} /></div></div>
        {r.draftRow ? <Foot r={r} /> : null}
      </section>
    );
  }
  return (
    <section class="panel team-p" data-key={k}>
      {head}
      <StarterNote r={r} />
      <CarryNote r={r} />
      <Canvas r={r} vlist={<VList r={r} />} />
      <Foot r={r} />
    </section>
  );
}

// StarterNote is what a new definition says when no place holds the role
// documents or the profiles its members need: the person chooses from what is
// there, or asks for the starter ones, which the save writes only then
// (r-54b8, d-86b1). A row with nothing missing says nothing.
function StarterNote({ r }: { r: Rec }) {
  // The note is about what a save would write, and only an answer that read the
  // row's base says that. A row whose base was refused offers no save, so it
  // asks for no choice either (r-a028).
  if (!r.draftRow || !r.seeded.value) return null;
  const roles = !r.hasRoles.value, profiles = !r.hasProfiles.value;
  if (!roles && !profiles) return null;
  return (
    <div class="starter">
      {roles ? (
        <div class="st-item">
          <span>プロジェクトにもグローバルにも、ロールがありません。メンバーのロールを選ぶには、ロールが要ります。</span>
          <button class="btn sm" type="button" data-act="starter-roles" aria-pressed={r.starterRoles.value} onClick={() => pressStarterRoles(r)}>{r.starterRoles.value ? 'ひな形のロールを使わない' : 'ひな形のロール lead と builder を使う'}</button>
        </div>
      ) : null}
      {profiles ? (
        <div class="st-item">
          <span>プロジェクトにもグローバルにも、プロファイルがありません。メンバーのプロファイルを選ぶには、プロファイルが要ります。</span>
          <button class="btn sm" type="button" data-act="starter-profile" aria-pressed={r.starterProfile.value} onClick={() => pressStarterProfile(r)}>{r.starterProfile.value ? 'ひな形のプロファイルを使わない' : 'ひな形のプロファイル claude-default を使う'}</button>
        </div>
      ) : null}
    </div>
  );
}

// CarryNote is what a copy of a project's definition says about what it brings
// with it (r-7e4a, d-dc81): the copy is the definition's two files alone, and the
// role documents and profiles the global layer does not hold are the person's to
// choose. A name that is chosen is written with the definition, and a name the
// global layer already holds is not offered, because a copy writes over nothing.
function CarryNote({ r }: { r: Rec }) {
  const c = r.carry.value;
  if (!r.draftRow || !c) return null;
  const missing = c.roles.length + c.profiles.length;
  return (
    <div class="carry">
      <div class="st-item"><span>グローバルに書くのは、このチーム定義（team.yaml と agreement.md）だけです。{missing
        ? 'メンバーが使うロールとプロファイルのうち、グローバルに無いものは、選んだものだけを一緒に書きます。'
        : 'メンバーが使うロールとプロファイルはグローバルにそろっているので、どのプロジェクトからも起動できます。'}</span></div>
      {c.roles.map((name) => (
        <div class="st-item">
          <span>ロール {name} の文書はグローバルにありません。</span>
          <button class="btn sm" type="button" data-act="carry-role" data-name={name} aria-pressed={c.chosenRoles.includes(name)} onClick={() => pressCarry(r, 'role', name)}>{c.chosenRoles.includes(name) ? `ロール ${name} を書かない` : `ロール ${name} をグローバルに書く`}</button>
        </div>
      ))}
      {c.profiles.map((name) => (
        <div class="st-item">
          <span>プロファイル {name} はグローバルにありません。</span>
          <button class="btn sm" type="button" data-act="carry-profile" data-name={name} aria-pressed={c.chosenProfiles.includes(name)} onClick={() => pressCarry(r, 'profile', name)}>{c.chosenProfiles.includes(name) ? `プロファイル ${name} を書かない` : `プロファイル ${name} をグローバルに書く`}</button>
        </div>
      ))}
    </div>
  );
}

// VList is the check column under the canvas: the server's own sentences, each
// one where it belongs (spec-teams X1).
function VList({ r }: { r: Rec }) {
  const { issues, own, out, where, file, roleHref } = vlistParts(r);
  return (
    <>
      <div class="vhead"><span class="vt">設定の検査</span>{issues.length ? <span class="vcount"><Counts list={messagesOf(issues)} /></span> : <span class="vclean">✓ 問題なし</span>}</div>
      {own.map((m) => <button class="vitem" data-act="vpick" data-node={m.node || null} data-worker={m.worker != null && m.worker >= 0 ? m.worker : null} data-name-field={m.field === 'name' ? 1 : null} onClick={() => pressVpick(r, { ...m, node: m.node, worker: m.worker != null && m.worker >= 0 ? m.worker : null, nameField: m.field === 'name' })}><Sev sev={m.sev} /><span class="lno">{where(m)}</span><span class="msg">{m.text}</span></button>)}
      {out.length ? <><div class="vsub">ロールの文書</div>{out.map((m) => <a class="vitem" href={roleHref(m)}><Sev sev={m.sev} /><span class="lno">{file(m)}</span><span class="msg">{m.text}</span></a>)}</> : null}
    </>
  );
}

// Foot is the shape panel's foot: the save of the ops, the count 1.4.6 counts,
// and 元に戻す while there is something to put back. A definition that is not
// there yet shows what its save would write, and no 元に戻す. The 規則 tab
// draws that same row's foot — either tab's 保存 writes the whole folder — so
// the press the save carries and the key its 保存しました stands under are the
// caller's (spec-teams B6, 監査 R9).
function Foot({ r, agree }: { r: Rec; agree?: boolean }) {
  const n = changeCount(r), k = dkey(r);
  // A save refused because the file moved stands above this foot, and the words
  // the person wrote stay where they are (r-3c77, d-a34a).
  const hold = conflictAt(r, agree ? 'a' : 't');
  // mine is the text this screen read for the file: the compare's other side.
  const mine = fileText(`${r.name.value}.${r.place.value}.${agree ? 'agreement' : 'team'}`);
  const notice = hold ? <ConflictNotice c={hold.c} mine={mine} undone={hold.undone}
    onOverwrite={() => pressOverwrite(r, agree ? 'a' : 't')}
    onRevert={() => pressRevertConflict(r, agree ? 'a' : 't')}
    onUndo={() => pressUndoConflict(r, agree ? 'a' : 't')} /> : null;
  const okKey = (agree ? 'a' : 't') + k, saveAct = agree ? 'save' : 't-save';
  // 保存しました stays until the next change writes over it, so a check that
  // answers just after the save does not take the notice away.
  const ok = okOf(okKey) ? <span class="ok">保存しました</span> : null;
  // The gate is the saved team.yaml's own errors (spec-teams 1.3, X3): a save
  // stops only when an error the file did not hold has appeared.
  const issues = teamIssues(r);
  const before = new Set((r.baseFindings.value || []).filter((m) => m.sev === 'err').map((m) => m.text));
  // A member with no turn and a definition no turn reaches the human from
  // still save (r-df77, d-9abb): they stay as the definition's problem and
  // stop up, but a save is where work in progress is kept. Those findings
  // stand on a node (field 'node'); every other new error stops the save.
  const grown = issues.filter((m) => m.sev === 'err' && m.field !== 'node' && !before.has(m.text));
  const grew = grown.length > 0;
  const refusal = refusalOf(r, agree ? 'a' : 't');
  if (r.draftRow) {
    // A row that is not there yet is saved only when its name is one the
    // server will take: the name's problems are the answer's own, and while
    // the first answer is still out nothing is written from the row
    // (spec-teams B2, N2).
    const named = r.seeded.value && !(r.nameIssues.value || []).length;
    // held is why 保存 is shut, in the server's own sentences (ac-456e): the
    // save's refusal, the name's problems, and the errors the row grew since
    // the base it stands on. Every state that shuts the save has one, so a row
    // never stands with its save off and nothing said (r-a028).
    const held: string[] = [];
    const say = (text: string) => { if (text && !held.includes(text)) held.push(text); };
    if (refusal) say(refusal);
    (r.nameIssues.value || []).forEach((m) => say(m.text));
    grown.forEach((m) => say(m.text));
    return (
      <>{notice}<footer class="pf">
        <button class="btn primary" data-act={saveAct} disabled={grew || !named} onClick={() => (agree ? pressSave(r) : pressTSave(r))}>保存</button><span class="dirty">新規</span>{ok}
        <span class="also">{(r.writes.value || []).map((p, i) => <>{i ? '・' : null}<code>{String(p).split('/').pop()}</code></>)}</span>{held.length ? <span class="sev err refusal">{held.join(' ')}</span> : null}
      </footer></>
    );
  }
  if (agree) {
    const s = r.agreeSaved.value, d = agreeDiff(r);
    const errs = agreeIssues(r);
    const was = new Set((s && r.agreeSavedIssues.value ? r.agreeSavedIssues.value : errs).filter((m) => m.sev === 'err').map((m) => m.text));
    const empty = !s && !r.agreeDraft.value.text.length;
    const blocked = empty || (!s ? errs.some((m) => m.sev === 'err') : (errs.some((m) => m.sev === 'err' && !was.has(m.text)) || d === 0));
    return (
      <>{notice}<footer class="pf">
        <button class="btn primary" data-act="save" disabled={blocked} onClick={() => pressSave(r)}>保存</button><span class="errline doc" hidden={!refusal}>{refusal}</span><span class="dirty">{s && d ? `${d} か所` : ''}</span>{ok}
        <span class="acts"><button class="btn" data-act="revert" hidden={empty || !(d > 0)} onClick={() => pressRevert(r)}>元に戻す</button></span>
      </footer></>
    );
  }
  return (
    <>{notice}<footer class="pf">
      <button class="btn primary" data-act="t-save" disabled={!n || grew} onClick={() => pressTSave(r)}>保存</button><span class="dirty">{n ? `${n} か所` : ''}</span>{ok}
      <span class="acts"><button class="btn" data-act="t-revert" hidden={!n} onClick={() => pressTRevert(r)}>元に戻す</button></span>{refusal ? <span class="sev err refusal">{refusal}</span> : null}
    </footer></>
  );
}

// ---- 規則のパネル（ロールの画面と同じ共有の部品） --------------------
// onChange is the shared part saying the document it was handed changed: the row
// gets a new draft (a copy) and the check rides on it.
function onChangeDraft(r: Rec): void {
  r.agreeDraft.value = clone(r.agreeDraft.value);
  scheduleAgree(r);
}

function AgreePanel({ r }: { r: Rec }) {
  const [scanned, setScanned] = useState<Record<string, string[]>>({});
  const onScanned = useCallback((name: string, places: string[] | null) => {
    setScanned((previous) => { const next = { ...previous }; if (places) next[name] = places; else delete next[name]; return next; });
  }, []);
  const places = [...new Set(Object.values(scanned).flat())];
  const shadowed = r.place.value === 'global' && (r.places || []).includes('project');
  const mode = pmodeOf(r);
  const workspace = useRef<HTMLDivElement>(null);
  // Opening the tab, and picking other documents, only changes what stands in
  // the board: the page keeps the place it had (r-e9a2, ac-4ba8, ac-c66a). The
  // row's own floor of height (Detail) keeps the place reachable.
  const names = [...new Set(r.draft.value.members.map(m => m.role).filter(Boolean))];
  const available = ['agreement', ...names.map(name => 'role:' + name)];
  const picked = selectedRules(r.uid, available);
  const active = picked.length ? picked : ['agreement'];
  useEffect(() => {
   const node = workspace.current;
   if (!node || active.length < 2) return;
   const board = node.querySelector<HTMLElement>('.prose-board');
   const tools = node.querySelector<HTMLElement>('.prose-tools');
   const band = document.querySelector<HTMLElement>('.bar');
   const summary = node.closest('.tdef')?.querySelector<HTMLElement>(':scope > .sum');
   const fit = () => {
    const top = (band?.offsetHeight || 0) + (summary?.offsetHeight || 0);
    board?.style.setProperty('--prose-height', `${Math.max(360, window.innerHeight - top - (tools?.offsetHeight || 0) - 20)}px`);
   };
   const frame = requestAnimationFrame(() => {
    fit();
   });
   const observer = new ResizeObserver(fit);
   for (const element of [band, summary, tools]) if (element) observer.observe(element);
   window.addEventListener('resize', fit);
   return () => { cancelAnimationFrame(frame); observer.disconnect(); window.removeEventListener('resize', fit); };
  }, [active.join('|')]);
  const docs = [{key: 'agreement', label: `チームの規則 ${r.name.value}`, members: r.draft.value.members.map(m => m.name).join('、'), status: `${agreeDiff(r) ? '未保存' : '保存済み'}${agreeIssues(r).length ? ` · 問題・警告 ${agreeIssues(r).length}` : ''}`},
   ...names.map(name => ({key: 'role:' + name, label: `ロール ${name}`, members: r.draft.value.members.filter(m => m.role === name).map(m => m.name).join('、'), status: roleSummary(r, name)}))];
  return (
    <div class="prose-workspace" ref={workspace} data-mode={active.length === 1 ? 'single' : active.length === 2 ? 'compare' : 'all'}>
     <div class="prose-documents" aria-label="読む文書">
      {docs.map(doc => <div class={'prose-choice' + (active.includes(doc.key) ? ' on' : '')}>
       <button type="button" class="btn" aria-pressed={active.includes(doc.key)} onClick={() => selectRule(r.uid, doc.key, available)}>{doc.label}<small>{doc.members}</small><small>{doc.status}</small></button>
       <label><input type="checkbox" aria-label={`${doc.label} を並べる`} checked={active.includes(doc.key)} onChange={() => selectRule(r.uid, doc.key, available, true)} />並べる</label>
      </div>)}
      <button class="btn" type="button" onClick={() => { ruleSelection.value = { ...ruleSelection.value, [r.uid]: available }; }}>すべて表示</button>
     </div>
     {active.length > 1 ? <p class="prose-compare-note">比較中 {active.length} 文書。{active.length > 2 ? '横に収まらない文書は、下のスクロールバーで表示できます。' : ''}文書名を押すと、その文書だけを読みます。</p> : null}
     <div class="prose-tools"><EqualWidths /><span class="prose-drag-hint">見出しだけの表示で、行の左の ⠿ をつかんで別の列へ落とすと移します。{transferKeys()}</span><span class="prose-wheel-hint">本文を読み進めるときは、欄を一度押してからホイールを回します。</span>{places.length ? <span class="prose-scope"><span>確認できた範囲</span>{places.map((place) => <span class="scope-place">{place}</span>)}</span> : null}</div><div class="prose-board"><section hidden={!active.includes('agreement')} class={'prose-column panel agree-p' + (shadowed ? ' shadowed' : '')} data-uid={r.uid} data-l={r.place.value} data-key={dkey(r)} data-mode={mode}>
      <div class="pmain">
        <RuleDoc transfer={r.draftRow ? undefined : teamTransfer(r)} row={rowOfPanel(r)} issues={agreeIssues(r)} hasName={false} heading={{ id: `team-prose-${r.uid}`, kind: 'チーム定義', name: r.name.value, title: r.agreeDraft.value.title,
            path: `${placeName(r.place.value)} · teams/${r.name.value}/agreement.md`, members: r.draft.value.members.map((m) => m.name).join('、'),
            template: r.check.value?.templateSize || 0, state: (agreeDiff(r) ? '未保存' : '保存済み') + (shadowed ? '（隠れている）' : '') }}
          onChange={() => onChangeDraft(r)}
          onMode={(v: string) => pressPmode(r, v)}
          onURL={() => syncURL()} />
      </div>
      <Foot r={r} agree={true} />
    </section>{names.map((name) => <RoleColumn key={name} r={r} name={name} visible={active.includes('role:' + name)} onScanned={onScanned} />)}</div></div>
  );
}

// ---- YAML のタブ -------------------------------------------------------
// The tab itself is the shared part (lib/yamltab.js, spec-boundary 7): this
// screen draws the file nav and one section per file, and hands the section to
// the part, which runs the check and the save through /api/team with the file's
// own part.
const keyOfFile = (f: YFile): string => f.file;

// With no team definition at all, the YAML tab has no file to show. It says
// what is missing and the one thing that makes the files (n-f2ef).
function YamlEmpty() {
  return (
    <section class="yed yempty">
      <p>チーム定義はまだありません。</p>
      <p>一覧の『＋ チーム定義を追加』で作ると、ここに team.yaml と agreement.md が並びます。</p>
      <span><button class="btn" data-act="tab" data-t="list" onClick={() => pressTab('list')}>一覧へ</button></span></section>
  );
}

export function YamlPage() {
  const files = yfiles.value;
  if (!files.length) return <YamlEmpty />;
  const chosen = chosenFile();
  const one = chosen ? fileOf(chosen) : null;
  return (
    <div class="ylay"><nav class="files">{PLACES.map((p) => {
      const fs = files.filter((f) => f.place === p);
      if (!fs.length) return null;
      return (
        <div class="fgroup"><div class="gh"><b>{PLACE[p]}</b><code>{DIR[p] || ''}</code></div>{fs.map((f) => {
          const r = recForFile(f);
          const n = f.kind === 'agreement' && r ? Array.from((r.agreeSaved.value || r.agreeDraft.value).text).length : '';
          const on = chosen === f.file;
          return (
            <button class={['file', on ? 'on' : '', f.hidden ? 'hid' : ''].filter(Boolean).join(' ')} data-act="yfile" data-file={f.file} aria-pressed={on} onClick={() => pressYfile(f.file)}><code>{f.name}/{f.kind === 'agreement' ? 'agreement.md' : 'team.yaml'}</code><span class="fs">{fileDirty(f.file) ? <span class="dirty">●</span> : null}{(yIssues(f.file) || []).length ? <Counts list={messagesOf(yIssues(f.file))} /> : (n || '')}</span><span class="ft">{f.hidden ? '隠れている' : f.path}</span></button>
          );
        })}</div>
      );
    })}</nav>
      {one ? <YamlFile key={chosen} f={one} /> : <div class="none">ファイルを選んでください</div>}</div>
  );
}

// YamlFile is one file's section. The section element is the screen's (it
// carries the key the presses read and the hash the save carries); the part
// draws what stands inside it and reads the file and the hash as props.
function YamlFile({ f }: { f: YFile }) {
  const r = recForFile(f);
  const both = r ? (r.places || []).length > 1 : false;
  return (
    <section class="yed" data-key={keyOfFile(f)} data-hash={f.hash}>
      <YamlTab label={f.label} path={f.path}
        badge={both ? { cls: 'state ' + (f.hidden ? 'off' : 'on'), text: f.hidden ? '隠れている' : '効いている' } : null}
        text={f.text} carried={state.value.ytext[f.file]} hash={f.hash} kind="team" place={f.place} name={f.name} extra={{ part: f.kind }}
        problems={yIssues(f.file)}
        onProblems={(probs: Schemas['Problem'][]) => yamlProblems(f.file, probs)}
        onRevert={() => yamlRevert(f.file)}
        onState={({ text }: { text: string }) => yamlState(f.file, { text })}
        onSaved={(answer) => yamlSaved(f.file, answer)} />
    </section>
  );
}

// ---- 下の知らせ（canvas の操作） --------------------------------------
// The words stand at the bottom for four seconds, with 取り消す when the
// operation can be taken back (the mock of 2026-10-23).
function CanvasToast() {
  const t = state.value.toast;
  if (!t) return null;
  return <div class="cv-toast"><span>{t.msg}</span>{t.undo ? <button type="button" data-act="cv-undo" onClick={() => pressToastUndo()}>取り消す</button> : null}</div>;
}
