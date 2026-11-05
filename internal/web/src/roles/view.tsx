// roles/view.js — 一覧のタブと、パネル（webui-preact-design 4 章）。
//
// 行・パネル・足を描く。class と data-* と入れ子は、移す前のマークアップから 1 つ
// も変えずに移した構成を基に、編集操作を上部へそろえる。状態は state.js の signal を読み、押し込みは自分の
// onClick / onInput / onKeyDown から state.js の関数を直接呼ぶ（5 章 1）。規則の
// 文書の中は共有部品（lib/ruledoc.js）が描き、その押し込みも部品のもの。
import { useEffect, useRef } from '../lib/preact.js';
import { Measure } from '../lib/measure.js';
import { Field } from '../lib/field.js';
import {
  PLACES, LNAME, HQ, GROUPED, state, roles, teams, currentProjectName, CURRENT, roleGroups, groupOfRow,
  live, eff, cur, curName, whoReads, projectNameOf, pathOfRow, issuesOf, rowIssues, checks,
  rowOfPanel, filtering, ruleShown, filterCount, FILTERS, okOf, refusalOf, touch, scheduleCheck,
  focusAsk, withEntry, clone, pressToggle, pressFilter, pressNew, pressPmode,
  pressDup, pressCopy, pressDiscard, pressSearch, pressTab, pressGoto, canCopy,
  doSave, doDelete, syncURL, conflictAt, pressOverwrite, pressRevertConflict, pressUndoConflict,
} from './state.js';
import { ConflictNotice } from '../lib/conflict.js';
import { askConfirm, ConfirmDialog } from '../lib/confirm.js';
import { bringUnderBand } from '../lib/band.js';
import { RuleDoc, Counts, Badge } from '../lib/ruledoc.js';
import { dirty as changedDirty, roleDirty, ROLE_DOC } from '../lib/changed.js';
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
  const tabs: Array<['roles' | 'assign' | 'yaml', string, string | number]> = [['roles', '一覧', ''], ['assign', '割り当て', people], ['yaml', 'Markdown', '']];
  return tabs.map(([t, label, n]) => <button class={'tab' + (s.tab === t ? ' on' : '')} role="tab" aria-controls="view" aria-selected={s.tab === t} tabIndex={s.tab === t ? 0 : -1} data-act="tab" data-t={t} aria-pressed={s.tab === t} onClick={() => pressTab(t)}>{label}{n !== '' ? <span class="n">{n}</span> : ''}</button>);
}

export function App() {
  const tab = state.value.tab;
  if (tab === 'assign') return <><AssignPage /><ConfirmDialog /></>;
  if (tab === 'yaml') return <><YamlPage /><ConfirmDialog /></>;
  return <><List /><ConfirmDialog /></>;
}

// List is the roles list: the filters and the search, the column names, and the
// rows under each place's head. The head's own button adds a role to that place
// (r-410c); the list above carries no add button.
export function List() {
  const s = state.value;
  return <><div class="toolbar">
      <div class="filters">{FILTERS.filter(([f]) => !f || filterCount(f) > 0 || s.filter === f).map(([f, label]) => <button class={'fchip' + (s.filter === f ? ' on' : '')} data-act="filter" data-f={f || ''} aria-pressed={s.filter === f} onClick={() => pressFilter(f)}>{label}<span class="n">{filterCount(f)} ロール</span></button>)}</div>
      <Field id="q" class="search" type="search" placeholder="文書を探す" value={s.q} autocomplete="off" onInput={(e: Event) => { if (e.currentTarget instanceof HTMLInputElement) pressSearch(e.currentTarget.value); }} />
    </div>
    {HQ && HQ.error
      ? <p class="hq-err" data-role="hq-error">{HQ.error}</p>
      : <div class="list">
        <div class="head"><span></span><span>ロール名</span><span>置き場所</span><span>大きさ</span><span>使っているチーム</span></div>
        <div id="rows">{rowsHTML()}</div>
      </div>}</>;
}

function rowsHTML() {
  const shown = roles.value.filter((r) => !filtering() || r.isNew || r.markdown
    || PLACES.some((l) => r[l] && ruleShown(r[l].draft, issuesOf(r, l))));
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
  const wasOpen = useRef(false);
  useEffect(() => {
    if (open && !wasOpen.current && !filtering()) {
      const frame = requestAnimationFrame(() => {
        const top = document.querySelector<HTMLElement>('.bar')?.offsetHeight || 0;
        if (el.current) window.scrollBy({ top: el.current.getBoundingClientRect().top - top, behavior: 'instant' });
      });
      wasOpen.current = open;
      return () => cancelAnimationFrame(frame);
    }
    wasOpen.current = open;
  }, [open]);
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
      <div class="sum" role="group" data-act="toggle" onClick={() => pressToggle(r.uid)}>{sumInner(r)}</div>
      {open ? detail(r) : ''}
    </div>;
}

// unsaved is whether a row holds more than the files do.
const unsaved = (r: RoleRow): boolean => PLACES.some((l) => r[l] && roleDirty(r[l]!));

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
  // The crossed-out グローバル is the owner's mark for a name both layers hold
  // (r-ea17), not the screen's comparison of the two places.
  const shadow = r.mark === 'in_effect';
  const overriding = r.group === 'global' ? (over[curName(r)] || []) : [];
  return <><span class="pill on">{label}</span>{shadow ? <span class="pill shadow">グローバル</span> : ''}{overriding.map((p) => <span class="pill hq-ov">{projectNameOf(p)} で上書き</span>)}</>;
}

function sumInner(r: RoleRow) {
  if (r.markdown) {
    return <><button type="button" class="chev disclosure" aria-label={`${r.markdown ? r.name : curName(r)} を開閉する`} aria-expanded={state.value.open.has(r.uid)} onClick={(e) => { e.stopPropagation(); pressToggle(r.uid); }}></button>
        <span class="c-name"><span class="nm">{r.name}</span><span class="sev err">問題 1</span></span>
        <span class="c-place">{placePills(r)}</span>
        <span class="c-amount"><span class="none">—</span></span>
        <span class="c-readers">{readersCell(r)}</span></>;
  }
  const d = cur(r), saved = live(r, 'project') || live(r, 'global');
  const readers = whoReads(curName(r), r.group);
  const issues = rowIssues(r);
  const size = checks.value[r.uid + eff(r)];
  const teamIds = [...new Set(readers.map((x) => x.t.id))];
  return <><button type="button" class="chev disclosure" aria-label={`${r.markdown ? r.name : curName(r)} を開閉する`} aria-expanded={state.value.open.has(r.uid)} onClick={(e) => { e.stopPropagation(); pressToggle(r.uid); }}></button>
      <span class="c-name"><span class="nm">{d.name ? d.name : <span class="none">—</span>}</span>{r.mark ? <span class={'state ' + (r.mark === 'hidden' ? 'off' : 'on')}>{r.mark === 'hidden' ? '隠れている' : '効いている'}</span> : ''}<span class="ttl">{d.title}</span>{r.isNew ? <span class="tag">新規</span> : ''}{saved ? <Badge list={issues} /> : readers.length ? <span class="sev err">問題 {readers.length}</span> : ''}</span>
      <span class="c-place">{r.noDoc ? '' : placePills(r)}</span>
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
      <div class="panels">{ls.map((l) => r[l]!.problem
        ? <div class="dh" data-problem={l}><ul class="vm dhvm"><li><span class="sev err">問題</span><span>{`${r.paths?.[l] || ''} を読めません: ${r[l]!.problem}`}</span></li></ul></div>
        : <Panel key={l} r={r} l={l} />)}</div>
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
  const d = e.draft, key = r.uid + l;
  const shadowed = r.mark === 'hidden' && l === 'global';
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
  const n = roleDirty(e) ? 1 : 0;
  const before = new Set((e.saved && e.__savedIssues ? e.__savedIssues : issues).filter((m) => m.sev === 'err').map((m) => m.text));
  const grew = issues.filter((m) => m.sev === 'err' && !before.has(m.text));
  const blocked = !!nameIssue || (!e.saved ? issues.some((m) => m.sev === 'err') : (grew.length > 0 || n === 0));
  // held is why 保存 is shut, in the server's own sentences (ac-456e): the save's
  // refusal, the name's problem, and the errors the row grew over the document it
  // reads. A rule's problem is drawn where the rule stands and a folded rule shows
  // its count alone, so the way to fix a shut save stands beside the save too.
  const held: string[] = [];
  const say = (text: string | undefined) => { if (text && !held.includes(text)) held.push(text); };
  say(refusal ? refusal.text : '');
  say(nameIssue ? nameIssue.text : '');
  (e.saved ? grew : issues.filter((m) => m.sev === 'err')).forEach((m) => say(m.text));
  return <section class={'panel' + (shadowed ? ' shadowed' : '')} data-uid={r.uid} data-l={l} data-mode={mode}>
      <header class="edit-actions"><b>ロール {d.name}</b>        <span class="acts">
          {e.saved ? <button class="btn" data-act="dup" onClick={() => pressDup(r.uid, l)}>複製</button> : ''}
          {canCopy(r, l) ? <button class="btn" data-act="copy" onClick={() => pressCopy(r.uid, l)}>{LNAME[other(l)]}にコピー</button> : ''}
          {!e.saved ? <button class="btn danger" data-act="discard" onClick={() => {
            // Compared with what the row was made with (a new one, a duplicate
            // or a copy), so a fresh copy that nobody touched goes at once
            // (r-d480).
            // Compared with what the row was made with, through the one owner
            // (r-d480, r-1e9a). A new row's born is its template.
            const untouched = !changedDirty(ROLE_DOC, r.born || { name: '', title: '', text: '## 責務\n' }, { name: e.draft.name, title: e.draft.title || '', text: e.draft.text });
            if (untouched) { pressDiscard(r.uid, l); return; }
            askConfirm({ title: '入力した内容を捨てますか', body: <p>保存していない入力があります。<b>取り消せません。</b></p>, label: '破棄する', run: () => pressDiscard(r.uid, l) });
          }}>取り消し</button>
            : <button class="btn danger" data-act="del" onClick={() => askConfirm({
              title: `ロール ${d.name} を削除しますか`,
              body: <><p>{`${placeLabel} の roles/${d.name}.md を消します。`}{nread ? `${nread} 人が読んでいます。` : ''}</p><p><b>取り消せません。</b></p></>,
              label: `削除する${nread ? `（${nread} 人が読んでいる）` : ''}`,
              run: () => doDelete(r.uid, l),
            })}>削除</button>}
        </span></header>
      <div class="pmain"><RuleDoc row={row} issues={issues} nameError={nameError} heading={{ kind: 'ロール', name: row.draft.name || '新しいロール', title: row.draft.title,
          id: `role-${r.uid}-${l}`, compact: true, path: `${placeLabel} · roles/${d.name}.md`, fullPath: pathOfRow(r, l, d), state: (n ? '未保存' : '保存済み') + (shadowed ? '（隠れている）' : ''), members: whoReads(row.draft.name, r.group).map((u) => `${u.t.id}: ${u.m.name}`).join('、') }}
        filtering={filtering()} query={state.value.q}
        readerCount={(name: string) => whoReads(name, GROUPED ? CURRENT : '').length}
        onChange={() => onChangeDraft(r, l)}
        onMode={(v: 'edit' | 'md') => pressPmode(r.uid, l, v)}
        onURL={() => syncURL(false)} /></div>
      {hold ? <ConflictNotice c={hold.c} mine={e.text || ''} undone={hold.undone}
        onOverwrite={() => pressOverwrite(r.uid, l)}
        onRevert={() => pressRevertConflict(r.uid, l)}
        onUndo={() => pressUndoConflict(r.uid, l)} /> : null}
      <footer class="pf">
        <button class="btn primary" data-act="save" disabled={blocked} onClick={() => doSave(r.uid, l)}>保存</button>{held.length ? <span class="sev err refusal">{held.join(' ')}</span> : null}<span class="dirty">{e.saved && n ? `${n} か所` : ''}</span><span class="vcount"><Counts list={issues.filter((p) => p.f !== 'size')} /></span>{okOf(key) ? <span class="ok">保存しました</span> : ''}

      </footer>
    </section>;
}
