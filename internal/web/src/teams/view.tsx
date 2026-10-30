// teams/view.tsx — チーム定義の画面の一覧とパネル（webui-preact-design 4 章）。
//
// 一覧の行・パネル・足・YAML のタブ・トポロジーの器を描く。class と data-* と
// 入れ子は、移す前のマークアップから 1 つも変えない（5 章 6）。状態は state.ts の
// signal を読み、押し込みは自分の onClick / onInput / onKeyDown から state.ts の
// 関数を直接呼ぶ（依頼書 2）。規則の文書の中は共有部品（lib/ruledoc.js）が描く。
import { useEffect, useRef } from '../lib/preact.js';
import type { ComponentChildren, TargetedEvent } from 'preact';
import { Measure } from '../lib/measure.js';
import { YamlTab } from '../lib/yamltab.js';
import { Canvas } from './canvas.js';
import { RuleDoc, Counts, Vm, OutNote, Sev, rulesOf, outCount, clone } from '../lib/ruledoc.js';
import { Field, inText } from '../lib/field.js';
import { RunCell } from '../lib/hq/run.js';
import { state as hq, projectName as hqProjectName } from '../lib/hq.js';
import { copyGroupOf } from '../lib/copy.js';
import {
  PLACES, PLACE, DIR, HQ, GROUPED, GROUPS, CURRENT, state, allRecs, rowsOfGroup,
  dkey, groupKeyOf, groupNameOf, rowsIn, currentProjectName, teamLabel, dtabOf, isOpen,
  rowIssues, dirtyDef, teamIssues, agreeIssues, agreeDiff, changeCount, vlistParts,
  rowOfPanel, okOf, refusalOf, pmodeOf, yfiles, fileOf, fileDirty, yIssues,
  recForFile, chosenFile, scheduleAgree, syncURL, focusAsk, groupOfRecord,
  pressTab, pressToggle, pressNew, pressDup, pressCopy, pressPlace, pressDiscard,
  pressStarterRoles, pressStarterProfile,
  pressDel, pressDelNo, pressDelYes, pressName, pressMission, pressDtab, pressPmode,
  pressToYaml, pressTSave, pressTRevert, pressSave, pressRevert, pressVrule, pressVpick,
  pressToastUndo, pressYfile, yamlState, yamlProblems, yamlRevert, yamlSaved,
} from './state.js';
import type { Rec, YFile } from './state.js';
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

// keyOf is the key a person pressed.
function keyOf(e: Event): string {
  return e instanceof KeyboardEvent ? e.key : '';
}

const other = (p: 'project' | 'global'): 'project' | 'global' => (p === 'project' ? 'global' : 'project');
// placeName is the word a place is drawn by; anything else is this project's.
const placeName = (p: string): string => (p === 'global' ? PLACE.global : PLACE.project);

// TABS are the screen's two tabs: 一覧 and YAML.
const TABS: ['list' | 'yaml', string][] = [['list', '一覧'], ['yaml', 'YAML']];
// MODES are the rules tab's two views: the editor and its Markdown.
const MODES: [string, string][] = [['edit', '編集'], ['md', 'Markdown']];

// Tabs is the screen's two tabs: 一覧 and YAML.
export function Tabs() {
  const tab = state.value.tab;
  return TABS.map(([t, label]) => (
    <button class={'tab' + (tab === t ? ' on' : '')} data-act="tab" data-t={t} aria-pressed={tab === t} onClick={() => pressTab(t)}>{label}</button>
  ));
}

export function App() {
  // The 動き column and the group heads are the headquarters' reading: each
  // reads it, so a re-reading draws it without the screen asking (5 章 2).
  return <>{state.value.tab === 'list' ? <List /> : <YamlPage />}<CanvasToast /></>;
}

// ---- 一覧 --------------------------------------------------------------
export function List() {
  const toolbar = <div class="toolbar"><button class="btn primary" data-act="new" onClick={() => pressNew()}>＋ チーム定義を追加</button></div>;
  // A headquarters whose lists could not be read says why, rather than drawing
  // the one project's lists as though the projects were not there (r-1db7 L1).
  if (HQ && HQ.error) return <>{toolbar}<p class="hq-err" data-role="hq-error">{HQ.error}</p></>;
  const recs = allRecs();
  if (!recs.length && !GROUPED) return <>{toolbar}<div class="list"><p class="none">チーム定義はありません</p></div></>;
  const head = (
    <div class="head"><span></span><span>チーム定義名</span><span>使命</span><span>{HQ ? 'プロジェクト' : '置き場所'}</span><span>メンバー</span><span>ワーカー</span><span>ルール</span><span>大きさ</span>{HQ ? <span class="hq-runh">動き</span> : null}</div>
  );
  if (!GROUPED) {
    return <>{toolbar}<div class="list">{head}{recs.map((r) => <Row key={r.key.value} r={r} />)}</div></>;
  }
  // The rows stand in groups: every listed project in the config's order, then
  // the global layer once (spec 4 章).
  let any = false;
  const body = (GROUPS || []).map((g) => {
    const rows = rowsOfGroup(groupKeyOf(g));
    if (rows.length) any = true;
    return <><GroupHead key={'g' + groupKeyOf(g)} g={g} />{rows.map((r) => <Row key={r.key.value} r={r} />)}</>;
  });
  return <>{toolbar}<div class="list hq-grouped">{head}{body}{any ? null : <p class="none">チーム定義はありません</p>}</div></>;
}

// groupHeader is one project's group's head (spec 4 章): its name, where it
// is, and how many definitions and running teams it holds. The global layer
// comes last, with the one sentence about being used from every project.
function GroupHead({ g }: { g: Schemas['HQTeamGroup'] }) {
  const key = groupKeyOf(g);
  if (g.global) {
    return <div class="hq-gh global" data-group="global"><span class="hq-gn">グローバル</span><span class="hq-gp">{g.display || ''}</span><span class="hq-gc">どのプロジェクトからも使える。同じ名前があれば、そのプロジェクトのほうが使われる</span></div>;
  }
  const live = ((hq.running.value && hq.running.value.teams) || []).filter((t: { project?: string }) => t.project === key).length;
  return <div class="hq-gh" data-group={key}><span class="hq-gn">{groupNameOf(g)}</span><span class="hq-gp">{g.project ? g.project.display : ''}</span><span class="hq-gc">チーム定義 <b>{rowsIn(g).length}</b> · 動いているチーム <b>{live}</b></span></div>;
}

// bringUnderBand moves the page so an opened row's head stands under the band
// (d-943f, r-e39c): the figure under it has the room the window has, and a row
// opened low in the list is not opened half off the screen.
function bringUnderBand(row: HTMLElement | null): void {
  const head = row ? row.querySelector<HTMLElement>(':scope > .sum') : null;
  if (!head) return;
  const stick = Number.parseFloat(getComputedStyle(document.documentElement).getPropertyValue('--stick')) || 62;
  const delta = head.getBoundingClientRect().top - stick;
  if (Math.abs(delta) < 2) return;
  const quiet = typeof window.matchMedia === 'function' && window.matchMedia('(prefers-reduced-motion: reduce)').matches;
  window.scrollBy({ top: delta, behavior: quiet ? 'auto' : 'smooth' });
}
// Row is one definition: the summary a person presses, and the detail it opens
// under itself (spec-teams B1). The field a press asked for is put in place after
// the draw, by the row itself (5 章 5): nobody puts the focus back later.
function Row({ r }: { r: Rec }) {
  const open = isOpen(r.dom);
  const el = useRef<HTMLDivElement>(null);
  const wasOpen = useRef(false);
  useEffect(() => {
    if (open && !wasOpen.current) bringUnderBand(el.current);
    wasOpen.current = open;
  }, [open]);
  useEffect(() => {
    const ask = focusAsk.value;
    if (!ask || ask.dom !== r.dom || !el.current) return;
    const one = el.current.querySelector<HTMLElement>(ask.sel);
    if (!one) return;
    one.focus({ preventScroll: true });
    if (one instanceof HTMLInputElement) one.select();
  }, [focusAsk.value]);
  return (
    <div class={'tdef' + (open ? ' open' : '') + (dirtyDef(r) ? ' unsaved' : '')} data-name={r.dom} data-key={dkey(r)} ref={el}>
      <div class="sum" role="button" tabindex={0} aria-expanded={open} data-act="toggle" onClick={(e) => pressSum(r, e)} onKeyDown={(e: KeyboardEvent) => { if (inText(e)) return; if (keyOf(e) === 'Enter' || keyOf(e) === ' ') { e.preventDefault(); pressToggle(r); } }}><Sum r={r} open={open} /></div>
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
  if (target.closest('[data-hq], .in-mission, .in-name')) return;
  // A press on another control inside the row (the place's buttons) is that
  // control's, not the row's: only a press on the row itself opens or closes it.
  const own = target.closest('button, a, input, select, [data-act]');
  if (own && own !== e.currentTarget) return;
  pressToggle(r);
}

// nameCell is the row's name column: a field while a definition that is not
// there yet is being named, and the name itself otherwise. A copy keeps the
// name it copies, so it shows the name rather than a field (spec-teams B3).
function NameCell({ r, open }: { r: Rec; open: boolean }) {
  const tag = r.draftRow ? <span class="tag">新規</span> : null;
  const name = r.name.value;
  if (!r.draftRow) return <><span class="nm">{name}</span>{tag}</>;
  if (r.how === 'copy' || !open) return <><span class="nm">{name || <span class="none">—</span>}</span>{tag}</>;
  return (
    <>
      <Field id={'nm-' + r.dom} class={'in-name' + ((r.nameIssues.value || []).length ? ' bad' : '')} data-f="name" value={name} placeholder="チーム定義名" spellcheck={false} autocomplete="off" aria-label="チーム定義名" onInput={(e: Event) => pressName(r, inputValue(e))} />{tag}
    </>
  );
}

// placePills is the row's プロジェクト column (spec 4 章): the project's name,
// or グローバル. A project that overrides a global name carries the crossed-out
// グローバル, and the global row says which projects override it.
function placePills(r: Rec): ComponentChildren {
  // Without projects: the list stays one, and its プロジェクト column still
  // names the project or グローバル (d-8a45, r-5b96 audit 5).
  const place = r.place.value;
  if (!GROUPED) {
    return (r.places || []).map((p) => <span class={'pill ' + (p === place ? 'on' : 'shadow')}>{p === 'global' ? 'グローバル' : currentProjectName()}</span>);
  }
  const label = r.project === 'global' ? 'グローバル' : hqProjectName(r.project);
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
  const t = r.draft.value, saved = r.saved.value, rules = rulesOf(r.agreeDraft.value);
  // A team.yaml that does not read has no figure and no numbers to count, so
  // its row says 使命 —, keeps the problems validate named, and leaves the
  // counts blank (spec-teams 1.9). It is fixed in the YAML tab.
  const broken = r.broken.value;
  // A definition that is not there yet chooses its place: a row added from the
  // scaffold can go either way, and a copy is bound to the place it goes to
  // (spec-teams B2, B3). A saved row says which project it is (spec 4 章).
  let pills: ComponentChildren;
  if (r.draftRow && r.how === 'copy') pills = <span class="pill pending">{PLACE[r.place.value]}</span>;
  else if (r.draftRow) {
    pills = (
      <span class="seg" role="group" aria-label="置き場所">{PLACES.map((p) => <button class={p === r.place.value ? 'on' : ''} data-act="pick-place" data-x={p} aria-pressed={p === r.place.value} onClick={() => pressPlace(r, p)}>{PLACE[p]}</button>)}</span>
    );
  } else pills = placePills(r);
  const cell = broken ? '—'
    : open
      ? <Field id={'m-' + r.dom} class={'in-mission' + (t.mission !== saved.mission ? ' changed' : '')} data-f="mission" value={t.mission} autocomplete="off" aria-label={`${teamLabel(r)} の使命`} onInput={(e: Event) => pressMission(r, inputValue(e))} />
      : t.mission ? t.mission : '—';
  return (
    <>
      <span class="chev"></span>
      <span class="c-name"><NameCell r={r} open={open} /><span class="vcount"><Counts list={messagesOf(rowIssues(r))} /></span></span>
      <span class={'c-mission' + (broken || !(t.mission || open) ? ' none' : '')}>{cell}</span>
      <span class="c-place">{pills}</span>
      <span class="c-members c-num">{broken ? '—' : t.members.length}</span>
      <span class="c-turns c-num">{broken ? '—' : t.workers.length}</span>
      <span class="c-agree c-num">{rules.length}<OutNote n={outCount(rules)} /></span>
      <span class="c-size">{broken || !r.limit.value ? <span class="none">—</span> : <Measure chars={r.size.value} limit={r.limit.value} />}</span>
      {HQ && !r.draftRow ? <span class="hq-runc"><RunCell definition={r.name.value} place={r.place.value} project={runProject(r)} /></span> : null}
    </>
  );
}

// ---- detail ------------------------------------------------------------
function Detail({ r }: { r: Rec }) {
  const hidden = r.hidden.value || [];
  return (
    <div class="detail">
      <ul class="vm mvm"><Vm list={[...(r.nameIssues.value || []), ...r.findings.value.filter((m) => m.field === 'mission')].map((m) => ({ sev: m.sev, text: m.text }))} /></ul>
      <nav class="dtabs"><DtabsNav r={r} /></nav>
      {dtabOf(r) === 'shape' ? <TeamPanel r={r} /> : <AgreePanel r={r} />}
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
    ['agree', 'ルール', String(rulesOf(r.agreeDraft.value).length), r.agreeSaved.value ? agreeDiff(r) > 0 : false],
  ];
  return (
    <>
      <span class="dtl">{tabs.map(([t, label, n, ch]) => <button class={'dtab' + (cur === t ? ' on' : '')} data-act="dtab" data-t={t} aria-pressed={cur === t} onClick={() => pressDtab(r, t)}>{label}{n !== '' ? <span class="n">{n}</span> : null}{ch ? <i class="dot"></i> : null}</button>)}</span>
      <span class="dacts"><Dacts r={r} /></span>
    </>
  );
}

// dacts is what one definition's whole folder is offered: 複製, a copy into the
// other place, and 削除 with its own confirm. A definition that is not there
// yet is only taken back (spec-teams B3, B4, B5).
function Dacts({ r }: { r: Rec }) {
  if (r.draftRow) return <button class="btn danger" data-act="discard" onClick={() => pressDiscard(r)}>取り消し</button>;
  const o = other(r.place.value);
  const group = GROUPED ? copyGroupOf(o, r.project, CURRENT) : r.project;
  // A name the side the copy would go to already holds is not offered again:
  // on a grouped screen that side is the copy's own group, and a saved copy
  // there counts as much as one that is not saved yet (r-f781).
  const copying = allRecs().some((x) => x.name.value === r.name.value && x.place.value === o
    && (!GROUPED || groupOfRecord(x) === group));
  const ck = 'del' + dkey(r);
  return (
    <>
      <button class="btn" data-act="dup" onClick={() => pressDup(r)}>複製</button>
      {!(r.places || []).includes(o) && !copying ? <button class="btn" data-act="copy" onClick={() => pressCopy(r)}>{PLACE[o]}にコピー</button> : null}
      {state.value.confirm[ck]
        ? <><button class="btn danger solid" data-act="del-yes" onClick={() => pressDelYes(r)}>削除する（team.yaml と agreement.yaml）</button><button class="btn" data-act="del-no" onClick={() => pressDelNo(r)}>やめる</button></>
        : <button class="btn danger" data-act="del" onClick={() => pressDel(r)}>削除</button>}
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
  // A team.yaml that does not read draws no figure and offers no shape save:
  // the check column alone says what stopped it, and the YAML tab is where the
  // text is put right (spec-teams 1.9).
  if (r.broken.value) {
    return (
      <section class="panel team-p" data-key={k}>
        {head}
        <div class="gbody"><div class="vlist"><VList r={r} /></div></div>
      </section>
    );
  }
  return (
    <section class="panel team-p" data-key={k}>
      {head}
      <StarterNote r={r} />
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
  if (!r.draftRow) return null;
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

// VList is the check column under the canvas: the server's own sentences, each
// one where it belongs (spec-teams X1).
function VList({ r }: { r: Rec }) {
  const { issues, own, teamOut, out, where, file, roleHref, ruleID } = vlistParts(r);
  return (
    <>
      <div class="vhead"><span class="vt">validate</span>{issues.length ? <span class="vcount"><Counts list={messagesOf(issues)} /></span> : <span class="vclean">✓ 問題なし</span>}</div>
      {own.map((m) => <button class="vitem" data-act="vpick" data-node={m.node || null} data-worker={m.worker != null && m.worker >= 0 ? m.worker : null} data-name-field={m.field === 'name' ? 1 : null} onClick={() => pressVpick(r, { ...m, node: m.node, worker: m.worker != null && m.worker >= 0 ? m.worker : null, nameField: m.field === 'name' })}><Sev sev={m.sev} /><span class="lno">{where(m)}</span><span class="msg">{m.text}</span></button>)}
      {teamOut.length ? <><div class="vsub">チームのルール</div>{teamOut.map((m) => <button class="vitem" data-act="vrule" data-name={r.dom} data-id={ruleID(m)} data-k={m.ruleKey || 0} onClick={() => pressVrule(r, m.ruleKey || ruleID(m))}><Sev sev={m.sev} /><span class="lno">{file(m)}</span><span class="msg">{m.text}</span></button>)}</> : null}
      {out.length ? <><div class="vsub">ロールの文書</div>{out.map((m) => <a class="vitem" href={roleHref(m)}><Sev sev={m.sev} /><span class="lno">{file(m)}</span><span class="msg">{m.text}</span></a>)}</> : null}
    </>
  );
}

// Foot is the shape panel's foot: the save of the ops, the count 1.4.6 counts,
// and 元に戻す while there is something to put back. A definition that is not
// there yet shows what its save would write, and no 元に戻す. The ルール tab
// draws that same row's foot — either tab's 保存 writes the whole folder — so
// the press the save carries and the key its 保存しました stands under are the
// caller's (spec-teams B6, 監査 R9).
function Foot({ r, agree }: { r: Rec; agree?: boolean }) {
  const n = changeCount(r), k = dkey(r);
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
  const grew = issues.some((m) => m.sev === 'err' && m.field !== 'node' && !before.has(m.text));
  const refusal = refusalOf(r, agree ? 'a' : 't');
  if (r.draftRow) {
    // A row that is not there yet is saved only when its name is one the
    // server will take: the name's problems are the answer's own, and while
    // the first answer is still out nothing is written from the row
    // (spec-teams B2, N2).
    const named = r.seeded.value && !(r.nameIssues.value || []).length;
    return (
      <footer class="pf">
        <button class="btn primary" data-act={saveAct} disabled={grew || !named} onClick={() => (agree ? pressSave(r) : pressTSave(r))}>保存</button><span class="dirty">新規</span>{ok}
        <span class="also">{(r.writes.value || []).map((p, i) => <>{i ? '・' : null}<code>{String(p).split('/').pop()}</code></>)}</span>{refusal ? <span class="sev err refusal">{refusal}</span> : null}
      </footer>
    );
  }
  if (agree) {
    const s = r.agreeSaved.value, d = agreeDiff(r);
    const errs = agreeIssues(r);
    const was = new Set((s && r.agreeSavedIssues.value ? r.agreeSavedIssues.value : errs).filter((m) => m.sev === 'err').map((m) => m.text));
    const blocked = !s ? errs.some((m) => m.sev === 'err') : (errs.some((m) => m.sev === 'err' && !was.has(m.text)) || d === 0);
    return (
      <footer class="pf">
        <button class="btn primary" data-act="save" disabled={blocked} onClick={() => pressSave(r)}>保存</button><span class="errline doc" hidden={!refusal}>{refusal}</span><span class="dirty">{s && d ? `${d} か所` : ''}</span><span class="vcount"><Counts list={messagesOf(errs)} /></span>{ok}
        <span class="acts"><button class="btn" data-act="revert" hidden={!(d > 0)} onClick={() => pressRevert(r)}>元に戻す</button></span>
      </footer>
    );
  }
  return (
    <footer class="pf">
      <button class="btn primary" data-act="t-save" disabled={!n || grew} onClick={() => pressTSave(r)}>保存</button><span class="dirty">{n ? `${n} か所` : ''}</span>{ok}
      <span class="acts"><button class="btn" data-act="t-revert" hidden={!n} onClick={() => pressTRevert(r)}>元に戻す</button></span>{refusal ? <span class="sev err refusal">{refusal}</span> : null}
    </footer>
  );
}

// ---- ルールのパネル（ロールの画面と同じ共有の部品） --------------------
// onChange is the shared part saying the document it was handed changed: the row
// gets a new draft (a copy) and the check rides on it.
function onChangeDraft(r: Rec): void {
  r.agreeDraft.value = clone(r.agreeDraft.value);
  scheduleAgree(r);
}

function AgreePanel({ r }: { r: Rec }) {
  const both = (r.places || []).length > 1, shadowed = r.place.value === 'global' && (r.places || []).includes('project');
  const mode = pmodeOf(r);
  return (
    <section class={'panel agree-p' + (shadowed ? ' shadowed' : '')} data-uid={r.uid} data-l={r.place.value} data-key={dkey(r)} data-mode={mode}>
      <header class="ph"><code class="path">{r.agreePath.value}</code>
        <span class="phr">
          {both ? <span class={'state ' + (shadowed ? 'off' : 'on')}>{shadowed ? '隠れている' : '効いている'}</span> : null}
          <span class="seg pview">{MODES.map(([v, label]) => <button class={mode === v ? 'on' : ''} data-act="pmode" data-v={v} aria-pressed={mode === v} onClick={() => pressPmode(r, v)}>{label}</button>)}</span>
        </span></header>
      <div class="pmain">
        <RuleDoc row={rowOfPanel(r)} issues={agreeIssues(r)} hasName={false}
          onChange={() => onChangeDraft(r)}
          onMode={(v: string) => pressPmode(r, v)}
          onURL={() => syncURL()} />
      </div>
      <Foot r={r} agree={true} />
    </section>
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
      <p>一覧の『＋ チーム定義を追加』で作ると、ここに team.yaml と agreement.yaml が並びます。</p>
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
          const n = f.kind === 'agreement' && r ? rulesOf(r.agreeSaved.value || r.agreeDraft.value).length : '';
          const on = chosen === f.file;
          return (
            <button class={['file', on ? 'on' : '', f.hidden ? 'hid' : ''].filter(Boolean).join(' ')} data-act="yfile" data-file={f.file} aria-pressed={on} onClick={() => pressYfile(f.file)}><code>{f.name}/{f.kind === 'agreement' ? 'agreement.yaml' : 'team.yaml'}</code><span class="fs">{fileDirty(f.file) ? <span class="dirty">●</span> : null}{(yIssues(f.file) || []).length ? <Counts list={messagesOf(yIssues(f.file))} /> : (n || '')}</span><span class="ft">{f.hidden ? '隠れている' : f.path}</span></button>
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
        text={f.text} hash={f.hash} kind="team" place={f.place} name={f.name} extra={{ part: f.kind }}
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
