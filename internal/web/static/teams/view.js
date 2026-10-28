// teams/view.js — チーム定義の画面の一覧とパネル（webui-preact-design 4 章）。
//
// 一覧の行・パネル・足・YAML のタブ・トポロジーの器を描く。class と data-* と
// 入れ子は、移す前のマークアップから 1 つも変えない（5 章 6）。状態は state.js の
// signal を読み、押し込みは自分の onClick / onInput / onKeyDown から state.js の
// 関数を直接呼ぶ（依頼書 2）。規則の文書の中は共通部品（lib/ruledoc.js）が描く。
import { html, useEffect, useRef } from '../vendor/preact.js';
import { Measure } from '../lib/measure.js';
import { YamlTab } from '../lib/yamltab.js';
import { Canvas } from './canvas.js';
import { RuleDoc, Counts, Vm, OutNote, Sev, rulesOf, outCount, clone } from '../lib/ruledoc.js';
import { RunCell } from '../lib/hq/run.js';
import { state as hq, projectName as hqProjectName, rev as hqRev } from '../lib/hq.js';
import {
  PLACES, PLACE, DIR, HQ, GROUPED, GROUPS, CURRENT, state, allRecs, rowsOfGroup,
  dkey, groupKeyOf, groupNameOf, rowsIn, currentProjectName, teamLabel, dtabOf, isOpen,
  rowIssues, dirtyDef, teamIssues, agreeIssues, agreeDiff, changeCount, vlistParts,
  rowOfPanel, okOf, refusalOf, pmodeOf, yfiles, fileOf, fileDirty, yIssues,
  recForFile, chosenFile, scheduleAgree, syncURL, focusAsk,
  pressTab, pressToggle, pressNew, pressDup, pressCopy, pressPlace, pressDiscard,
  pressDel, pressDelNo, pressDelYes, pressName, pressMission, pressDtab, pressPmode,
  pressToYaml, pressTSave, pressTRevert, pressSave, pressRevert, pressVrule, pressVpick,
  pressToastUndo, pressYfile, yamlState, yamlProblems, yamlRevert, yamlSaved,
} from './state.js';

const other = (p) => (p === 'project' ? 'global' : 'project');

// Tabs is the screen's two tabs: 一覧 and YAML.
export function Tabs() {
  const tab = state.value.tab;
  return [['list', '一覧'], ['yaml', 'YAML']].map(([t, label]) => html`<button class=${'tab' + (tab === t ? ' on' : '')} data-act="tab" data-t=${t} aria-pressed=${tab === t} onClick=${() => pressTab(t)}>${label}</button>`);
}

export function App() {
  // The 動き column and the group heads are the headquarters' reading: a
  // re-reading draws them without the screen asking (5 章 2).
  hqRev.value;
  return html`${state.value.tab === 'list' ? html`<${List} />` : html`<${YamlPage} />`}<${CanvasToast} />`;
}

// ---- 一覧 --------------------------------------------------------------
export function List() {
  const toolbar = html`<div class="toolbar"><button class="btn primary" data-act="new" onClick=${() => pressNew()}>＋ チーム定義を追加</button></div>`;
  const recs = allRecs();
  if (!recs.length && !GROUPED) return html`${toolbar}<div class="list"><p class="none">チーム定義はありません</p></div>`;
  const head = html`<div class="head"><span></span><span>チーム定義名</span><span>使命</span><span>${HQ ? 'プロジェクト' : '置き場所'}</span><span>メンバー</span><span>ワーカー</span><span>ルール</span><span>大きさ</span>${HQ ? html`<span class="hq-runh">動き</span>` : ''}</div>`;
  if (!GROUPED) {
    return html`${toolbar}<div class="list">${head}${recs.map((r) => html`<${Row} key=${r.key.value} r=${r} />`)}</div>`;
  }
  // The rows stand in groups: every listed project in the config's order, then
  // the global layer once (spec 4 章).
  let any = false;
  const body = GROUPS.map((g) => {
    const rows = rowsOfGroup(groupKeyOf(g));
    if (rows.length) any = true;
    return html`<${GroupHead} g=${g} />${rows.map((r) => html`<${Row} key=${r.key.value} r=${r} />`)}`;
  });
  return html`${toolbar}<div class="list hq-grouped">${head}${body}${any ? '' : html`<p class="none">チーム定義はありません</p>`}</div>`;
}

// groupHeader is one project's group's head (spec 4 章): its name, where it
// is, and how many definitions and running teams it holds. The global layer
// comes last, with the one sentence about being used from every project.
function GroupHead({ g }) {
  const key = groupKeyOf(g);
  hqRev.value;
  if (g.global) {
    return html`<div class="hq-gh global" data-group="global"><span class="hq-gn">グローバル</span><span class="hq-gp">${g.display || ''}</span><span class="hq-gc">どのプロジェクトからも使える。同じ名前があれば、そのプロジェクトのほうが使われる</span></div>`;
  }
  const live = ((hq.running && hq.running.teams) || []).filter((t) => t.project === key).length;
  return html`<div class="hq-gh" data-group=${key}><span class="hq-gn">${groupNameOf(g)}</span><span class="hq-gp">${g.project ? g.project.display : ''}</span><span class="hq-gc">チーム定義 <b>${rowsIn(g).length}</b> · 動いているチーム <b>${live}</b></span></div>`;
}

// Row is one definition: the summary a person presses, and the detail it opens
// under itself (spec-teams B1). The field a press asked for is put in place after
// the draw, by the row itself (5 章 5): nobody puts the focus back later.
function Row({ r }) {
  const open = isOpen(r.dom);
  const el = useRef(null);
  useEffect(() => {
    const ask = focusAsk.value;
    if (!ask || ask.dom !== r.dom || !el.current) return;
    const one = el.current.querySelector(ask.sel);
    if (!one) return;
    one.focus({ preventScroll: true });
    if (one.select) one.select();
  }, [focusAsk.value]);
  return html`<div class=${'tdef' + (open ? ' open' : '') + (dirtyDef(r) ? ' unsaved' : '')} data-name=${r.dom} data-key=${dkey(r)} ref=${el}>
    <div class="sum" role="button" tabindex="0" aria-expanded=${open} data-act="toggle" onClick=${(e) => pressSum(r, e)} onKeyDown=${(e) => { if (e.key === 'Enter' || e.key === ' ') { e.preventDefault(); pressToggle(r); } }}>${html`<${Sum} r=${r} open=${open} />`}</div>
    ${open ? html`<${Detail} r=${r} />` : ''}</div>`;
}

// pressSum opens or closes the row. A press on the row's own field, or on a
// button of the headquarters' 動き column, is not the row's: the field keeps the
// cursor, and the start or the stop acts on the definition rather than opening
// the row (r-5b96 audit 1).
function pressSum(r, e) {
  if (e.target.closest('[data-hq], .in-mission, .in-name')) return;
  // A press on another control inside the row (the place's buttons) is that
  // control's, not the row's: only a press on the row itself opens or closes it.
  const own = e.target.closest('button, a, input, select, [data-act]');
  if (own && own !== e.currentTarget) return;
  pressToggle(r);
}

// nameCell is the row's name column: a field while a definition that is not
// there yet is being named, and the name itself otherwise. A copy keeps the
// name it copies, so it shows the name rather than a field (spec-teams B3).
function NameCell({ r, open }) {
  const tag = r.draftRow ? html`<span class="tag">新規</span>` : '';
  const name = r.name.value;
  if (!r.draftRow) return html`<span class="nm">${name}</span>${tag}`;
  if (r.how === 'copy' || !open) return html`<span class="nm">${name || html`<span class="none">—</span>`}</span>${tag}`;
  return html`<input id=${'nm-' + r.dom} class=${'in-name' + ((r.nameIssues.value || []).length ? ' bad' : '')} data-f="name" value=${name} placeholder="チーム定義名" spellcheck="false" autocomplete="off" aria-label="チーム定義名" onInput=${(e) => pressName(r, e.target.value)} onKeyDown=${(e) => { if (e.key === 'Enter') e.preventDefault(); }} />${tag}`;
}

// placePills is the row's プロジェクト column (spec 4 章): the project's name,
// or グローバル. A project that overrides a global name carries the crossed-out
// グローバル, and the global row says which projects override it.
function placePills(r) {
  // Without projects: the list stays one, and its プロジェクト column still
  // names the project or グローバル (d-8a45, r-5b96 audit 5).
  const place = r.place.value;
  if (!GROUPED) {
    return (r.places || []).map((p) => html`<span class=${'pill ' + (p === place ? 'on' : 'shadow')}>${p === 'global' ? 'グローバル' : currentProjectName()}</span>`);
  }
  const label = r.project === 'global' ? 'グローバル' : hqProjectName(r.project);
  const globalGroup = (GROUPS || []).find((g) => g.global);
  const over = (globalGroup && globalGroup.overridden_by) || {};
  const name = r.name.value;
  const shadow = r.project && r.project !== 'global' && (over[name] || []).includes(r.project);
  const overriding = r.project === 'global' ? (over[name] || []) : [];
  return html`<span class="pill on">${label}</span>${shadow ? html`<span class="pill shadow">グローバル</span>` : ''}${overriding.map((p) => html`<span class="pill hq-ov">${hqProjectName(p)} で上書き</span>`)}`;
}

// runProject is the project a row's team runs in: without projects:, it is the
// one this serve was started in (r-5b96 audit 5).
const runProject = (r) => (r.place.value === 'global' ? '' : (r.project || CURRENT));

function Sum({ r, open }) {
  const t = r.draft.value, saved = r.saved.value, rules = rulesOf(r.agreeDraft.value);
  // A team.yaml that does not read has no figure and no numbers to count, so
  // its row says 使命 —, keeps the problems validate named, and leaves the
  // counts blank (spec-teams 1.9). It is fixed in the YAML tab.
  const broken = r.broken.value;
  // A definition that is not there yet chooses its place: a row added from the
  // scaffold can go either way, and a copy is bound to the place it goes to
  // (spec-teams B2, B3). A saved row says which project it is (spec 4 章).
  let pills;
  if (r.draftRow && r.how === 'copy') pills = html`<span class="pill pending">${PLACE[r.place.value]}</span>`;
  else if (r.draftRow) pills = html`<span class="seg" role="group" aria-label="置き場所">${PLACES.map((p) => html`<button class=${p === r.place.value ? 'on' : ''} data-act="pick-place" data-x=${p} aria-pressed=${p === r.place.value} onClick=${() => pressPlace(r, p)}>${PLACE[p]}</button>`)}</span>`;
  else pills = placePills(r);
  const cell = broken ? '—'
    : open
      ? html`<input id=${'m-' + r.dom} class=${'in-mission' + (t.mission !== saved.mission ? ' changed' : '')} data-f="mission" value=${t.mission} autocomplete="off" aria-label=${`${teamLabel(r)} の使命`} onInput=${(e) => pressMission(r, e.target.value)} onKeyDown=${(e) => { if (e.key === 'Enter') e.preventDefault(); }} />`
      : t.mission ? t.mission : '—';
  return html`<span class="chev"></span>
    <span class="c-name"><${NameCell} r=${r} open=${open} /><span class="vcount"><${Counts} list=${rowIssues(r)} /></span></span>
    <span class=${'c-mission' + (broken || !(t.mission || open) ? ' none' : '')}>${cell}</span>
    <span class="c-place">${pills}</span>
    <span class="c-members c-num">${broken ? '—' : t.members.length}</span>
    <span class="c-turns c-num">${broken ? '—' : t.workers.length}</span>
    <span class="c-agree c-num">${rules.length}<${OutNote} n=${outCount(rules)} /></span>
    <span class="c-size">${broken || !r.limit.value ? html`<span class="none">—</span>` : html`<${Measure} chars=${r.size.value} limit=${r.limit.value} />`}</span>
    ${HQ && !r.draftRow ? html`<span class="hq-runc"><${RunCell} definition=${r.name.value} place=${r.place.value} project=${runProject(r)} /></span>` : ''}`;
}

// ---- detail ------------------------------------------------------------
function Detail({ r }) {
  const hidden = r.hidden.value || [];
  return html`<div class="detail">
    <ul class="vm mvm"><${Vm} list=${[...(r.nameIssues.value || []), ...r.findings.value.filter((m) => m.field === 'mission')].map((m) => ({ sev: m.sev, msg: m.text }))} /></ul>
    <nav class="dtabs"><${DtabsNav} r=${r} /></nav>
    ${dtabOf(r) === 'shape' ? html`<${TeamPanel} r=${r} />` : html`<${AgreePanel} r=${r} />`}
    ${hidden.map((h) => html`<section class="panel shadowed"><header class="ph"><span class="ln">${PLACE[h.place]}</span><code class="path">${(DIR[h.place] || '') + r.name.value}/</code><span class="state off">隠れている</span><span class="phr"><button class="btn sm" data-act="to-yaml" data-key=${`${r.name.value}.${h.place}.team`} onClick=${() => pressToYaml(`${r.name.value}.${h.place}.team`)}>YAML</button></span></header></section>`)}
  </div>`;
}

function DtabsNav({ r }) {
  const cur = dtabOf(r);
  const tabs = [
    ['shape', 'トポロジー', '', changeCount(r) > 0],
    ['agree', 'ルール', String(rulesOf(r.agreeDraft.value).length), r.agreeSaved.value ? agreeDiff(r) > 0 : false],
  ];
  return html`<span class="dtl">${tabs.map(([t, label, n, ch]) => html`<button class=${'dtab' + (cur === t ? ' on' : '')} data-act="dtab" data-t=${t} aria-pressed=${cur === t} onClick=${() => pressDtab(r, t)}>${label}${n !== '' ? html`<span class="n">${n}</span>` : ''}${ch ? html`<i class="dot"></i>` : ''}</button>`)}</span><span class="dacts"><${Dacts} r=${r} /></span>`;
}

// dacts is what one definition's whole folder is offered: 複製, a copy into the
// other place, and 削除 with its own confirm. A definition that is not there
// yet is only taken back (spec-teams B3, B4, B5).
function Dacts({ r }) {
  if (r.draftRow) return html`<button class="btn danger" data-act="discard" onClick=${() => pressDiscard(r)}>取り消し</button>`;
  const o = other(r.place.value);
  const copying = allRecs().some((x) => x.draftRow && x.how === 'copy' && x.name.value === r.name.value && x.place.value === o);
  const ck = 'del' + dkey(r);
  return html`<button class="btn" data-act="dup" onClick=${() => pressDup(r)}>複製</button>
    ${!(r.places || []).includes(o) && !copying ? html`<button class="btn" data-act="copy" onClick=${() => pressCopy(r)}>${PLACE[o]}にコピー</button>` : ''}
    ${state.value.confirm[ck] ? html`<button class="btn danger solid" data-act="del-yes" onClick=${() => pressDelYes(r)}>削除する（team.yaml と agreement.yaml）</button><button class="btn" data-act="del-no" onClick=${() => pressDelNo(r)}>やめる</button>`
    : html`<button class="btn danger" data-act="del" onClick=${() => pressDel(r)}>削除</button>`}`;
}

// ---- トポロジーのタブ --------------------------------------------------
// teamPanel is the トポロジー tab: the canvas and its attribute column
// (canvas.js), the check column under the canvas, and the save's band
// (spec-teams B7). The column's 属性 / YAML tabs pick what stands on the right.
function TeamPanel({ r }) {
  const k = dkey(r);
  const both = (r.places || []).length > 1;
  const head = html`<header class="ph"><code class="path">${r.teamPath.value}</code>
      <span class="phr">${both ? html`<span class=${'state ' + (r.place.value === 'global' ? 'off' : 'on')}>${r.place.value === 'global' ? '隠れている' : '効いている'}</span>` : ''}</span></header>`;
  // A team.yaml that does not read draws no figure and offers no shape save:
  // the check column alone says what stopped it, and the YAML tab is where the
  // text is put right (spec-teams 1.9).
  if (r.broken.value) {
    return html`<section class="panel team-p" data-key=${k}>
    ${head}
    <div class="gbody"><div class="vlist"><${VList} r=${r} /></div></div>
  </section>`;
  }
  return html`<section class="panel team-p" data-key=${k}>
    ${head}
    <${Canvas} r=${r} vlist=${html`<${VList} r=${r} />`} />
    <${Foot} r=${r} />
  </section>`;
}

// VList is the check column under the canvas: the server's own sentences, each
// one where it belongs (spec-teams X1).
function VList({ r }) {
  const { issues, own, teamOut, out, where, file, roleHref, ruleID } = vlistParts(r);
  return html`<div class="vhead"><span class="vt">validate</span>${issues.length ? html`<span class="vcount"><${Counts} list=${issues} /></span>` : html`<span class="vclean">✓ 問題なし</span>`}</div>
  ${own.map((m) => html`<button class="vitem" data-act="vpick" data-node=${m.node || null} data-worker=${m.worker != null && m.worker >= 0 ? m.worker : null} data-name-field=${m.field === 'name' ? 1 : null} onClick=${() => pressVpick(r, { node: m.node, worker: m.worker != null && m.worker >= 0 ? m.worker : null, nameField: m.field === 'name' })}><${Sev} sev=${m.sev} /><span class="lno">${where(m)}</span><span class="msg">${m.text}</span></button>`)}
  ${teamOut.length ? html`<div class="vsub">チームのルール</div>${teamOut.map((m) => html`<button class="vitem" data-act="vrule" data-name=${r.dom} data-id=${ruleID(m)} onClick=${() => pressVrule(r, ruleID(m))}><${Sev} sev=${m.sev} /><span class="lno">${file(m)}</span><span class="msg">${m.text}</span></button>`)}` : ''}
  ${out.length ? html`<div class="vsub">ロールの文書</div>${out.map((m) => html`<a class="vitem" href=${roleHref(m)}><${Sev} sev=${m.sev} /><span class="lno">${file(m)}</span><span class="msg">${m.text}</span></a>`)}` : ''}`;
}

// Foot is the shape panel's foot: the save of the ops, the count 1.4.6 counts,
// and 元に戻す while there is something to put back. A definition that is not
// there yet shows what its save would write, and no 元に戻す. The ルール tab
// draws that same row's foot — either tab's 保存 writes the whole folder — so
// the press the save carries and the key its 保存しました stands under are the
// caller's (spec-teams B6, 監査 R9).
function Foot({ r, agree }) {
  const n = changeCount(r), k = dkey(r);
  const okKey = (agree ? 'a' : 't') + k, saveAct = agree ? 'save' : 't-save';
  // 保存しました stays until the next change writes over it, so a check that
  // answers just after the save does not take the notice away.
  const ok = okOf(okKey) ? html`<span class="ok">保存しました</span>` : '';
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
    return html`<footer class="pf">
      <button class="btn primary" data-act=${saveAct} disabled=${grew || !named} onClick=${() => (agree ? pressSave(r) : pressTSave(r))}>保存</button><span class="dirty">新規</span>${ok}
      <span class="also">${(r.writes.value || []).map((p, i) => html`${i ? '・' : ''}<code>${String(p).split('/').pop()}</code>`)}</span>${refusal ? html`<span class="sev err refusal">${refusal}</span>` : ''}
    </footer>`;
  }
  if (agree) {
    const s = r.agreeSaved.value, d = agreeDiff(r);
    const errs = agreeIssues(r);
    const was = new Set((s && r.agreeSavedIssues.value ? r.agreeSavedIssues.value : errs).filter((m) => m.sev === 'err').map((m) => m.msg));
    const blocked = !s ? errs.some((m) => m.sev === 'err') : (errs.some((m) => m.sev === 'err' && !was.has(m.msg)) || d === 0);
    return html`<footer class="pf">
      <button class="btn primary" data-act="save" disabled=${blocked} onClick=${() => pressSave(r)}>保存</button><span class="errline doc" hidden=${!refusal}>${refusal}</span><span class="dirty">${s && d ? `${d} か所` : ''}</span><span class="vcount"><${Counts} list=${errs} /></span>${ok}
      <span class="acts"><button class="btn" data-act="revert" hidden=${!(d > 0)} onClick=${() => pressRevert(r)}>元に戻す</button></span>
    </footer>`;
  }
  return html`<footer class="pf">
      <button class="btn primary" data-act="t-save" disabled=${!n || grew} onClick=${() => pressTSave(r)}>保存</button><span class="dirty">${n ? `${n} か所` : ''}</span>${ok}
      <span class="acts"><button class="btn" data-act="t-revert" hidden=${!n} onClick=${() => pressTRevert(r)}>元に戻す</button></span>${refusal ? html`<span class="sev err refusal">${refusal}</span>` : ''}
    </footer>`;
}

// ---- ルールのパネル（ロールの画面と同じ共有の部品） --------------------
// onChange is the shared part saying the document it was handed changed: the row
// gets a new draft (a copy) and the check rides on it.
function onChangeDraft(r) {
  r.agreeDraft.value = clone(r.agreeDraft.value);
  scheduleAgree(r);
}

function AgreePanel({ r }) {
  const both = (r.places || []).length > 1, shadowed = r.place.value === 'global' && (r.places || []).includes('project');
  const mode = pmodeOf(r);
  return html`<section class=${'panel agree-p' + (shadowed ? ' shadowed' : '')} data-uid=${r.uid} data-l=${r.place.value} data-key=${dkey(r)} data-mode=${mode}>
    <header class="ph"><code class="path">${r.agreePath.value}</code>
      <span class="phr">
        ${both ? html`<span class=${'state ' + (shadowed ? 'off' : 'on')}>${shadowed ? '隠れている' : '効いている'}</span>` : ''}
        <span class="seg pview">${[['edit', '編集'], ['md', 'Markdown']].map(([v, label]) => html`<button class=${mode === v ? 'on' : ''} data-act="pmode" data-v=${v} aria-pressed=${mode === v} onClick=${() => pressPmode(r, v)}>${label}</button>`)}</span>
      </span></header>
    <div class="pmain"><${RuleDoc} row=${rowOfPanel(r)} issues=${agreeIssues(r)} hasName=${false}
      onChange=${() => onChangeDraft(r)}
      onMode=${(v) => pressPmode(r, v)}
      onURL=${() => syncURL()} /></div>
    <${Foot} r=${r} agree=${true} />
  </section>`;
}

// ---- YAML のタブ -------------------------------------------------------
// The tab itself is the shared part (lib/yamltab.js, spec-boundary 7): this
// screen draws the file nav and one section per file, and hands the section to
// the part, which runs the check and the save through /api/team with the file's
// own part.
const keyOfFile = (f) => f.file;

// With no team definition at all, the YAML tab has no file to show. It says
// what is missing and the one thing that makes the files (n-f2ef).
function YamlEmpty() {
  return html`<section class="yed yempty">
    <p>チーム定義はまだありません。</p>
    <p>一覧の『＋ チーム定義を追加』で作ると、ここに team.yaml と agreement.yaml が並びます。</p>
    <span><button class="btn" data-act="tab" data-t="list" onClick=${() => pressTab('list')}>一覧へ</button></span></section>`;
}

export function YamlPage() {
  const files = yfiles.value;
  if (!files.length) return html`<${YamlEmpty} />`;
  const chosen = chosenFile();
  const one = fileOf(chosen);
  return html`<div class="ylay"><nav class="files">${PLACES.map((p) => {
    const fs = files.filter((f) => f.place === p);
    if (!fs.length) return '';
    return html`<div class="fgroup"><div class="gh"><b>${PLACE[p]}</b><code>${DIR[p] || ''}</code></div>${fs.map((f) => {
      const r = recForFile(f);
      const n = f.kind === 'agreement' && r ? rulesOf(r.agreeSaved.value || r.agreeDraft.value).length : '';
      const on = chosen === f.file;
      return html`<button class=${['file', on ? 'on' : '', f.hidden ? 'hid' : ''].filter(Boolean).join(' ')} data-act="yfile" data-file=${f.file} aria-pressed=${on} onClick=${() => pressYfile(f.file)}><code>${f.name}/${f.kind === 'agreement' ? 'agreement.yaml' : 'team.yaml'}</code><span class="fs">${fileDirty(f.file) ? html`<span class="dirty">●</span>` : ''}${(yIssues(f.file) || []).length ? html`<${Counts} list=${yIssues(f.file)} />` : (n || '')}</span><span class="ft">${f.hidden ? '隠れている' : f.path}</span></button>`;
    })}</div>`;
  })}</nav>
      ${one ? html`<${YamlFile} key=${chosen} f=${one} />` : html`<div class="none">ファイルを選んでください</div>`}</div>`;
}

// YamlFile is one file's section. The section element is the screen's (it
// carries the key the presses read and the hash the save carries); the part
// draws what stands inside it and reads the file and the hash as props.
function YamlFile({ f }) {
  const r = recForFile(f);
  const both = r ? (r.places || []).length > 1 : false;
  return html`<section class="yed" data-key=${keyOfFile(f)} data-hash=${f.hash}>
    <${YamlTab} label=${f.label} path=${f.path}
      badge=${both ? { cls: 'state ' + (f.hidden ? 'off' : 'on'), text: f.hidden ? '隠れている' : '効いている' } : null}
      text=${f.text} hash=${f.hash} kind="team" place=${f.place} name=${f.name} extra=${{ part: f.kind }}
      problems=${yIssues(f.file)}
      onProblems=${(probs) => yamlProblems(f.file, probs)}
      onRevert=${() => yamlRevert(f.file)}
      onState=${({ text }) => yamlState(f.file, { text })}
      onSaved=${(answer) => yamlSaved(f.file, answer)} />
  </section>`;
}

// ---- 下の知らせ（canvas の操作） --------------------------------------
// The words stand at the bottom for four seconds, with 取り消す when the
// operation can be taken back (the mock of 2026-10-23).
function CanvasToast() {
  const t = state.value.toast;
  if (!t) return null;
  return html`<div class="cv-toast"><span>${t.msg}</span>${t.undo ? html`<button type="button" data-act="cv-undo" onClick=${() => pressToastUndo()}>取り消す</button>` : ''}</div>`;
}
