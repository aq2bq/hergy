// roles/view.js — 一覧のタブと、パネル（webui-preact-design 4 章）。
//
// 行・パネル・足を描く。class と data-* と入れ子は、移す前のマークアップから 1 つ
// も変えない（5 章 6）。状態は state.js の signal を読み、押し込みは自分の
// onClick / onInput / onKeyDown から state.js の関数を直接呼ぶ（5 章 1）。規則の
// 文書の中は共有部品（lib/ruledoc.js）が描き、その押し込みも部品のもの。
import { useEffect, useRef } from '../lib/preact.js';
import { Measure } from '../lib/measure.js';
import { Field, inText } from '../lib/field.js';
import {
  PLACES, LNAME, HQ, GROUPED, state, roles, teams, currentProjectName, CURRENT, roleGroups, groupOfRow,
  live, eff, cur, curName, whoReads, projectNameOf, pathOfRow, issuesOf, rowIssues, checks,
  rowOfPanel, filtering, ruleShown, filterCount, FILTERS, okOf, refusalOf, touch, scheduleCheck,
  focusAsk, withEntry, clone, pressToggle, pressFilter, pressNew, pressPmode,
  pressDup, pressCopy, pressDel, pressDelNo, pressDiscard, pressSearch, pressTab, pressGoto, canCopy,
  doSave, doDelete, syncURL, conflictAt, pressOverwrite, pressRevertConflict, pressUndoConflict,
} from './state.js';
import { ConflictNotice } from '../lib/conflict.js';
import { bringUnderBand } from '../lib/band.js';
import { RuleDoc, Counts, Badge, rulesOf, inP, diffCount } from '../lib/ruledoc.js';
import type { RuleIssue } from '../lib/ruledoc.js';
import type { Place, RoleGroup, RoleRow } from './state.js';
import { AssignPage } from './assign.js';
import { YamlPage } from './yaml.js';

const other = (l: Place): Place => (l === 'project' ? 'global' : 'project');

// Tabs is the three tabs of the screen: 一覧, 割り当て and YAML. Its numbers are
// the list's own and the people the assign tab holds.
export function Tabs() {
  const s = state.value;
  const people = teams.value.reduce((n, t) => n + t.members.length, 0);
  const tabs: Array<['roles' | 'assign' | 'yaml', string, string | number]> = [['roles', '一覧', ''], ['assign', '割り当て', people], ['yaml', 'YAML', '']];
  return tabs.map(([t, label, n]) => <button class={'tab' + (s.tab === t ? ' on' : '')} data-act="tab" data-t={t} aria-pressed={s.tab === t} onClick={() => pressTab(t)}>{label}{n !== '' ? <span class="n">{n}</span> : ''}</button>);
}

export function App() {
  const tab = state.value.tab;
  if (tab === 'assign') return <AssignPage />;
  if (tab === 'yaml') return <YamlPage />;
  return <List />;
}

// List is the roles list: the filters and the search, the column names, and the
// rows under each place's head. The head's own button adds a role to that place
// (r-410c); the list above carries no add button.
export function List() {
  const s = state.value;
  return <><div class="toolbar">
      <div class="filters">{FILTERS.map(([f, label]) => <button class={'fchip' + (s.filter === f ? ' on' : '')} data-act="filter" data-f={f || ''} aria-pressed={s.filter === f} onClick={() => pressFilter(f)}>{label}<span class="n">{filterCount(f)}</span></button>)}</div>
      <Field id="q" class="search" type="search" placeholder="規則を探す" value={s.q} autocomplete="off" onInput={(e: Event) => { if (e.currentTarget instanceof HTMLInputElement) pressSearch(e.currentTarget.value); }} />
    </div>
    {HQ && HQ.error
      ? <p class="hq-err" data-role="hq-error">{HQ.error}</p>
      : <div class="list">
        <div class="head"><span></span><span>ロール名</span><span>置き場所</span><span>規則</span><span>オン / オフ</span><span>大きさ</span><span>使っているチーム</span></div>
        <div id="rows">{rowsHTML()}</div>
      </div>}</>;
}

function rowsHTML() {
  const shown = roles.value.filter((r) => !filtering() || r.isNew || r.markdown
    || PLACES.some((l) => r[l] && rulesOf(r[l].draft).some((x) => ruleShown(x, issuesOf(r, l)))));
  return roleGroups().map((g) => {
    const rows = shown.filter((r) => groupOfRow(r) === g.key);
    return <>{groupHeader(g)}{rows.map((r) => <Role key={r.uid} r={r} />)}
      {rows.length === 0 && !filtering() && !state.value.q ? <div class="hq-empty"><span class="none">{g.name} にはロールがありません。</span></div> : null}
    </>;
  });
}

// groupHeader is one place's head (spec 4 章, d-e77c): its name, where it is,
// how many roles it holds, and the button that adds a role to this place alone.
function groupHeader(g: RoleGroup) {
  const target = g.key === 'global' ? 'global' : 'project';
  const said = g.key === 'global' ? 'グローバル' : g.name;
  return <div class={'hq-gh' + (g.global ? ' global' : '')} data-group={g.key} data-place={target} aria-label={`置き場所: ${said}`}>
      <span class="hq-gn">{g.name}</span><span class="hq-gp">{g.display || ''}</span>
      {g.global
        ? <span class="hq-gc">どのプロジェクトからも使える。同じ名前があれば、そのプロジェクトのほうが使われる</span>
        : <span class="hq-gc">ロール <b>{g.count}</b></span>}
      <button class="btn sm hq-add" data-act="new" data-group={g.key} data-place={target} onClick={() => pressNew(g.key)}>＋ {said} にロールを追加</button>
    </div>;
}

// Role is one row: the summary a person presses, and the detail it opens under
// itself (P1). The caret a press asked for is put in the row's own fields after
// the draw (5 章 5): the field is the row's, and nobody puts it back later.
function Role({ r }: { r: RoleRow }) {
  const open = filtering() || state.value.open.has(r.uid);
  const el = useRef<HTMLDivElement | null>(null);
  useEffect(() => {
    const ask = focusAsk.value;
    if (!ask || ask.uid !== r.uid || !el.current) return;
    const one = el.current.querySelector<HTMLElement>(ask.sel);
    if (!one) return;
    one.focus({ preventScroll: true });
    // A copy's name is chosen, so the first letter a person types replaces it
    // rather than landing beside it (d-022a).
    if (ask.select && (one instanceof HTMLInputElement || one instanceof HTMLTextAreaElement)) one.select();
    // A copy brings the page to the row it made (d-022a): the row a person
    // pressed stands in the other place, and the copy is off the screen.
    if (ask.scroll) bringUnderBand(el.current);
  }, [focusAsk.value]);
  return <div class={'role' + (open ? ' open' : '') + (unsaved(r) ? ' unsaved' : '')} data-uid={r.uid} ref={el}>
      <div class="sum" role="button" tabIndex={0} aria-expanded={open} data-act="toggle" onClick={() => pressToggle(r.uid)} onKeyDown={(e: KeyboardEvent) => { if (inText(e)) return; if (e.key === 'Enter' || e.key === ' ') { e.preventDefault(); pressToggle(r.uid); } }}>{sumInner(r)}</div>
      {open ? detail(r) : ''}
    </div>;
}

// unsaved is whether a row holds more than the files do.
const unsaved = (r: RoleRow): boolean => PLACES.some((l) => r[l] && (!r[l]!.saved || diffCount(r[l]!) > 0));

const readersCell = (r: RoleRow) => {
  if (!GROUPED) {
    const ids = [...new Set(whoReads(curName(r), r.group).map((x) => x.t.id))];
    return ids.length ? <><b>{ids.length}</b><span>{ids.join(' · ')}</span></> : <span class="none">0</span>;
  }
  const name = curName(r);
  const rs = whoReads(name, r.group);
  if (r.group === 'global') {
    const byProject = new Map<string, string[]>();
    rs.forEach(({ t }) => {
      const key = t.group;
      const ids = byProject.get(key) || [];
      if (!byProject.has(key)) byProject.set(key, ids);
      if (!ids.includes(t.id)) ids.push(t.id);
    });
    if (!byProject.size) return <span class="none">0</span>;
    const total = [...byProject.values()].reduce((n, xs) => n + xs.length, 0);
    const parts = [...byProject.entries()].map(([key, ids]) => `${projectNameOf(key)}: ${ids.join(' · ')}`);
    return <><b>{total}</b><span>{parts.join(' / ')}</span></>;
  }
  const ids = [...new Set(rs.map(({ t }) => t.id))];
  return ids.length ? <><b>{ids.length}</b><span>{ids.join(' · ')}</span></> : <span class="none">0</span>;
};

// placePills is the row's 置き場所 column (spec 4 章, d-e77c): the place's name.
// A project that overrides a global name also carries the crossed-out グローバル,
// and a global row says which projects override it.
function placePills(r: RoleRow) {
  const label = r.group === 'global' ? LNAME.global : (r.group ? projectNameOf(r.group) : currentProjectName());
  const globalGroup = (HQ && HQ.roles || []).find((g) => g.global);
  const over: Record<string, string[]> = (globalGroup && globalGroup.overridden_by) || {};
  const shadow = r.group && r.group !== 'global' && (over[curName(r)] || []).includes(r.group);
  const overriding = r.group === 'global' ? (over[curName(r)] || []) : [];
  return <><span class="pill on">{label}</span>{shadow ? <span class="pill shadow">グローバル</span> : ''}{overriding.map((p) => <span class="pill hq-ov">{projectNameOf(p)} で上書き</span>)}</>;
}

function sumInner(r: RoleRow) {
  if (r.markdown) {
    return <><span class="chev"></span>
        <span class="c-name"><span class="nm">{r.name}</span><span class="sev err">問題 1</span></span>
        <span class="c-place">{placePills(r)}</span>
        <span class="c-count"><span class="none">—</span></span>
        <span class="c-backing"></span>
        <span class="c-amount"><span class="none">—</span></span>
        <span class="c-readers">{readersCell(r)}</span></>;
  }
  const d = cur(r), saved = live(r, 'project') || live(r, 'global');
  const readers = whoReads(curName(r), r.group);
  const issues = rowIssues(r);
  const rs = rulesOf(d);
  const on = rs.filter(inP).length, off = rs.length - on;
  const size = checks.value[r.uid + eff(r)];
  const teamIds = [...new Set(readers.map((x) => x.t.id))];
  return <><span class="chev"></span>
      <span class="c-name"><span class="nm">{d.name ? d.name : <span class="none">—</span>}</span><span class="ttl">{d.title}</span>{r.isNew ? <span class="tag">新規</span> : ''}{saved ? <Badge list={issues} /> : readers.length ? <span class="sev err">問題 {readers.length}</span> : ''}</span>
      <span class="c-place">{r.noDoc ? '' : placePills(r)}</span>
      <span class="c-count">{saved ? rs.length : <span class="none">—</span>}</span>
      <span class="c-backing">{saved ? <span class="onoff">オン <b>{on}</b>{off ? <>{' · '}<span class="offn">オフ {off}</span></> : ''}</span> : <span class="none">—</span>}</span>
      <span class="c-amount">{size && size.limit ? <Measure chars={size.size || 0} limit={size.limit} /> : <span class="none">—</span>}</span>
      <span class="c-readers">{GROUPED ? readersCell(r) : teamIds.length ? <><b>{teamIds.length}</b><span>{teamIds.join(' · ')}</span></> : <span class="none">0</span>}</span></>;
}

function detail(r: RoleRow) {
  if (r.markdown) {
    return <div class="detail">
      <div class="usage">{usage(r)}</div>
      <div class="panels"><div class="dh"><ul class="vm dhvm"><li><span class="sev err">問題</span><span>{r.markdown}</span></li></ul></div></div>
    </div>;
  }
  const ls = PLACES.filter((l) => r[l]);
  return <div class="detail">
      <div class="usage">{usage(r)}</div>
      <div class="panels">{ls.map((l) => <Panel key={l} r={r} l={l} />)}</div>
    </div>;
}

function usage(r: RoleRow) {
  const readers = whoReads(curName(r), r.group);
  if (!readers.length) return '';
  return <><span class="lbl">使っているチーム</span>{readers.map(({ t, m }) => <button class="uchip" data-act="goto" data-t={t.id} data-m={m.name} data-i={t.members.indexOf(m)} onClick={() => pressGoto(t.id, m.name, t.members.indexOf(m))}><span class="g">{r.group === 'global' ? `${projectNameOf(t.group)}: ` : ''}{t.id}</span>{m.name}</button>)}</>;
}

// onChange is the shared part saying the document it was handed changed: the row
// gets a new draft (a copy), the band's dot moves, and the check rides on it.
function onChangeDraft(r: RoleRow, l: Place): void {
  withEntry(r.uid, l, (e) => ({ ...e, draft: clone(e.draft) }));
  touch();
  scheduleCheck(r, l);
}

// Panel is one place's document: the head (where it stands, the 編集 / Markdown
// switch), the shared part's drawing of the document, and the foot (保存, what
// changed, 複製 / 削除).
function Panel({ r, l }: { r: RoleRow; l: Place }) {
  const e = r[l];
  if (!e) return '';
  const d = e.draft, key = r.uid + l, both = live(r, 'project') && live(r, 'global');
  const shadowed = l === 'global' && live(r, 'project');
  const row = rowOfPanel(r, l);
  if (!row) return '';
  const issues: RuleIssue[] = row.issues || [];
  const nread = e.saved && !r[other(l)] ? whoReads(e.saved.name, r.group).length : 0;
  const mode = state.value.pmode[key] || 'edit';
  const placeLabel = r.group === 'global' ? LNAME.global : (r.group ? projectNameOf(r.group) : currentProjectName());
  const refusal = refusalOf(key);
  const hold = conflictAt(key);
  // The name field's problem is the check's own (d-841e): the sentence stands
  // under ロール名, and while it is there the save is held. A save's refusal is the
  // same answer until the next check speaks.
  const nameIssue = issues.find((m) => m.f === 'name' && m.sev === 'err');
  const nameError = nameIssue ? nameIssue.text : (refusal && refusal.at === 'name' ? refusal.text : '');
  const docError = refusal && refusal.at === 'doc' ? refusal.text : '';
  const n = diffCount(e);
  const before = new Set((e.saved && e.__savedIssues ? e.__savedIssues : issues).filter((m) => m.sev === 'err').map((m) => m.text));
  const blocked = !!nameIssue || (!e.saved ? issues.some((m) => m.sev === 'err') : (issues.some((m) => m.sev === 'err' && !before.has(m.text)) || n === 0));
  return <section class={'panel' + (shadowed ? ' shadowed' : '')} data-uid={r.uid} data-l={l} data-mode={mode}>
      <header class="ph">
        <span class="ln">{placeLabel}</span>
        <code class="path">{pathOfRow(r, l, d)}</code>
        <span class="phr">
          {both ? <span class={'state ' + (shadowed ? 'off' : 'on')}>{shadowed ? '隠れている' : '効いている'}</span> : ''}
          <span class="seg pview">{[['edit', '編集'], ['md', 'Markdown']].map(([v, label]) => <button class={mode === v ? 'on' : ''} data-act="pmode" data-v={v} aria-pressed={mode === v} onClick={() => pressPmode(r.uid, l, v)}>{label}</button>)}</span>
        </span>
      </header>
      <div class="pmain"><RuleDoc row={row} issues={issues} nameError={nameError}
        filtering={filtering()} shown={ruleShown} query={state.value.q}
        readerCount={(name: string) => whoReads(name, GROUPED ? CURRENT : '').length}
        onChange={() => onChangeDraft(r, l)}
        onMode={(v: 'edit' | 'md') => pressPmode(r.uid, l, v)}
        onURL={() => syncURL(false)} /></div>
      {hold ? <ConflictNotice c={hold.c} mine={e.text || ''} undone={hold.undone}
        onOverwrite={() => pressOverwrite(r.uid, l)}
        onRevert={() => pressRevertConflict(r.uid, l)}
        onUndo={() => pressUndoConflict(r.uid, l)} /> : null}
      <footer class="pf">
        <button class="btn primary" data-act="save" disabled={blocked} onClick={() => doSave(r.uid, l)}>保存</button><span class="errline doc" hidden={!docError}>{docError}</span><span class="dirty">{e.saved && n ? `${n} か所` : ''}</span><span class="vcount"><Counts list={issues} /></span>{okOf(key) ? <span class="ok">保存しました</span> : ''}
        <span class="acts">
          {e.saved ? <button class="btn" data-act="dup" onClick={() => pressDup(r.uid, l)}>複製</button> : ''}
          {canCopy(r, l) ? <button class="btn" data-act="copy" onClick={() => pressCopy(r.uid, l)}>{LNAME[other(l)]}にコピー</button> : ''}
          {!e.saved ? <button class="btn danger" data-act="discard" onClick={() => pressDiscard(r.uid, l)}>取り消し</button>
            : state.value.confirm[key] ? <><button class="btn danger solid" data-act="del-yes" onClick={() => doDelete(r.uid, l)}>削除する{nread ? `（${nread} 人が読んでいる）` : ''}</button><button class="btn" data-act="del-no" onClick={() => pressDelNo(r.uid, l)}>やめる</button></>
            : <button class="btn danger" data-act="del" onClick={() => pressDel(r.uid, l)}>削除</button>}
        </span>
      </footer>
    </section>;
}
