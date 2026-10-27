// roles/view.js — 一覧のタブと、パネル（webui-preact-design 4 章）。
//
// 行・パネル・足を描く。class と data-* と入れ子は、移す前のマークアップから 1 つ
// も変えない（5 章 6）。押し込みは持たず（main.js が state.js へ渡す）、状態は
// state.js を読むだけ。規則の文書の中は共通部品（lib/ruledoc.js）が描く。
import { html } from '../vendor/preact.js';
import { Measure } from '../lib/measure.js';
import {
  PLACES, LNAME, HQ, GROUPED, state, rev, roles, teams, currentProjectName, groupKeyOf,
  live, eff, cur, curName, whoReads, projectNameOf, pathOfRow, issuesOf, rowIssues, checks,
  rowOfPanel, filtering, ruleShown, filterCount, FILTERS, okOf, refusalOf,
} from './state.js';
import { RuleDoc, Counts, Badge, rulesOf, inP, diffCount } from '../lib/ruledoc.js';
import { AssignPage } from './assign.js';
import { YamlPage } from './yaml.js';

const other = (l) => (l === 'project' ? 'global' : 'project');

// Tabs is the three tabs of the screen: 一覧, 割り当て and YAML. Its numbers are
// the list's own and the people the assign tab holds.
export function Tabs() {
  rev.value;
  const people = teams.reduce((n, t) => n + t.members.length, 0);
  return [['roles', '一覧', ''], ['assign', '割り当て', people], ['yaml', 'YAML', '']].map(([t, label, n]) => html`<button class=${'tab' + (state.tab === t ? ' on' : '')} data-act="tab" data-t=${t} aria-pressed=${state.tab === t}>${label}${n !== '' ? html`<span class="n">${n}</span>` : ''}</button>`);
}

export function App() {
  rev.value;
  if (state.tab === 'assign') return html`<${AssignPage} />`;
  if (state.tab === 'yaml') return html`<${YamlPage} />`;
  return html`<${List} />`;
}

// List is the roles list: the toolbar (＋ ロールを追加, the filters, the search),
// the column names, and the rows.
export function List() {
  return html`<div class="toolbar">
      <button class="btn primary" data-act="new">＋ ロールを追加</button>
      <div class="filters">${FILTERS.map(([f, label]) => html`<button class=${'fchip' + (state.filter === f ? ' on' : '')} data-act="filter" data-f=${f || ''} aria-pressed=${state.filter === f}>${label}<span class="n">${filterCount(f)}</span></button>`)}</div>
      <input id="q" class="search" type="search" placeholder="規則を探す" value=${state.q} autocomplete="off" />
    </div>
    <div class="list">
      <div class="head"><span></span><span>名前</span><span>${HQ ? 'プロジェクト' : '置き場所'}</span><span>規則</span><span>オン / オフ</span><span>大きさ</span><span>使っているチーム</span></div>
      <div id="rows">${rowsHTML()}</div>
    </div>`;
}

function rowsHTML() {
  const shown = roles.filter((r) => !filtering() || r.isNew || r.markdown
    || PLACES.some((l) => r[l] && rulesOf(r[l].draft).some((x) => ruleShown(x, issuesOf(r, l)))));
  if (GROUPED) {
    const out = (HQ.roles || []).map((g) => html`${groupHeader(g)}${shown.filter((r) => r.group === groupKeyOf(g)).map((r) => html`<${Role} key=${r.uid} r=${r} />`)}`);
    if (!shown.length) return html`<div class="sum"><span></span><span class="none">${roles.length ? '0 件' : 'ロールはありません'}</span></div>`;
    return out;
  }
  if (!shown.length) return html`<div class="sum"><span></span><span class="none">${roles.length ? '0 件' : 'ロールはありません'}</span></div>`;
  return shown.map((r) => html`<${Role} key=${r.uid} r=${r} />`);
}

// groupHeader is one project's group's head (spec 4 章): its name, where it is,
// and how many roles it holds. The global layer comes last with the one sentence
// about being used from every project.
function groupHeader(g) {
  const rows = (g.list && g.list.items) || [];
  if (g.global) return html`<div class="hq-gh global"><span class="hq-gn">グローバル</span><span class="hq-gp">${g.display || ''}</span><span class="hq-gc">どのプロジェクトからも使える。同じ名前があれば、そのプロジェクトのほうが使われる</span></div>`;
  return html`<div class="hq-gh"><span class="hq-gn">${groupNameOf(g)}</span><span class="hq-gp">${g.project ? g.project.display : ''}</span><span class="hq-gc">ロール <b>${rows.length}</b></span></div>`;
}

// Role is one row: the summary a person presses, and the detail it opens under
// itself (P1).
function Role({ r }) {
  const open = filtering() || state.open.has(r.uid);
  return html`<div class=${'role' + (open ? ' open' : '') + (unsaved(r) ? ' unsaved' : '')} data-uid=${r.uid}>
      <div class="sum" role="button" tabindex="0" aria-expanded=${open} data-act="toggle">${sumInner(r)}</div>
      ${open ? detail(r) : ''}
    </div>`;
}

// unsaved is whether a row holds more than the files do.
const unsaved = (r) => PLACES.some((l) => r[l] && (!r[l].saved || diffCount(r[l]) > 0));

const readersCell = (r) => {
  if (!GROUPED) {
    const ids = [...new Set(whoReads(curName(r), r.group).map((x) => x.t.id))];
    return ids.length ? html`<b>${ids.length}</b><span>${ids.join(' · ')}</span>` : html`<span class="none">0</span>`;
  }
  const name = curName(r);
  const rs = whoReads(name, r.group);
  if (r.group === 'global') {
    const byProject = new Map();
    rs.forEach(({ t }) => {
      const key = t.group;
      if (!byProject.has(key)) byProject.set(key, []);
      if (!byProject.get(key).includes(t.id)) byProject.get(key).push(t.id);
    });
    if (!byProject.size) return html`<span class="none">0</span>`;
    const total = [...byProject.values()].reduce((n, xs) => n + xs.length, 0);
    const parts = [...byProject.entries()].map(([key, ids]) => (key === 'global' ? ids.join(' · ') : `${projectNameOf(key)}: ${ids.join(' · ')}`));
    return html`<b>${total}</b><span>${parts.join(' / ')}</span>`;
  }
  const ids = [...new Set(rs.map(({ t }) => t.id))];
  return ids.length ? html`<b>${ids.length}</b><span>${ids.join(' · ')}</span>` : html`<span class="none">0</span>`;
};

// placePills is the row's プロジェクト column (spec 4 章): the project's name, or
// グローバル. A project that overrides a global name carries the crossed-out
// グローバル, and the global row says which projects override it.
function placePills(r) {
  if (!GROUPED) {
    return PLACES.map((l) => (live(r, l)
      ? html`<span class=${'pill ' + (l === 'global' && live(r, 'project') ? 'shadow' : 'on')}>${l === 'global' ? LNAME.global : currentProjectName()}</span>`
      : r[l] ? html`<span class="pill pending">${l === 'global' ? LNAME.global : currentProjectName()}</span>` : ''));
  }
  const label = r.group === 'global' ? 'グローバル' : projectNameOf(r.group);
  const globalGroup = (HQ.roles || []).find((g) => g.global);
  const over = (globalGroup && globalGroup.overridden_by) || {};
  const shadow = r.group && r.group !== 'global' && (over[curName(r)] || []).includes(r.group);
  const overriding = r.group === 'global' ? (over[curName(r)] || []) : [];
  return html`<span class="pill on">${label}</span>${shadow ? html`<span class="pill shadow">グローバル</span>` : ''}${overriding.map((p) => html`<span class="pill hq-ov">${projectNameOf(p)} で上書き</span>`)}`;
}

function sumInner(r) {
  if (r.markdown) {
    return html`<span class="chev"></span>
        <span class="c-name"><span class="nm">${r.name}</span><span class="sev err">問題 1</span></span>
        <span class="c-place">${placePills(r)}</span>
        <span class="c-count"><span class="none">—</span></span>
        <span class="c-backing"></span>
        <span class="c-amount"><span class="none">—</span></span>
        <span class="c-readers">${readersCell(r)}</span>`;
  }
  const d = cur(r), saved = live(r, 'project') || live(r, 'global');
  const readers = whoReads(curName(r), r.group);
  const issues = rowIssues(r);
  const rs = rulesOf(d);
  const on = rs.filter(inP).length, off = rs.length - on;
  const size = checks[r.uid + eff(r)];
  const teamIds = [...new Set(readers.map((x) => x.t.id))];
  return html`<span class="chev"></span>
      <span class="c-name"><span class="nm">${d.name ? d.name : html`<span class="none">—</span>`}</span><span class="ttl">${d.title}</span>${r.isNew ? html`<span class="tag">新規</span>` : ''}${saved ? html`<${Badge} list=${issues} />` : readers.length ? html`<span class="sev err">問題 ${readers.length}</span>` : ''}</span>
      <span class="c-place">${r.noDoc ? '' : placePills(r)}</span>
      <span class="c-count">${saved ? rs.length : html`<span class="none">—</span>`}</span>
      <span class="c-backing">${saved ? html`<span class="onoff">オン <b>${on}</b>${off ? html` · <span class="offn">オフ ${off}</span>` : ''}</span>` : html`<span class="none">—</span>`}</span>
      <span class="c-amount">${size && size.limit ? html`<${Measure} chars=${size.size} limit=${size.limit} />` : html`<span class="none">—</span>`}</span>
      <span class="c-readers">${GROUPED ? readersCell(r) : teamIds.length ? html`<b>${teamIds.length}</b><span>${teamIds.join(' · ')}</span>` : html`<span class="none">0</span>`}</span>`;
}

function detail(r) {
  if (r.markdown) {
    return html`<div class="detail">
      <div class="usage">${usage(r)}</div>
      <div class="panels"><div class="dh"><ul class="vm dhvm"><li><span class="sev err">問題</span><span>${r.markdown}</span></li></ul></div></div>
    </div>`;
  }
  const ls = PLACES.filter((l) => r[l]);
  return html`<div class="detail">
      <div class="usage">${usage(r)}</div>
      <div class="panels">${ls.map((l) => html`<${Panel} key=${l} r=${r} l=${l} />`)}</div>
    </div>`;
}

function usage(r) {
  const readers = whoReads(curName(r), r.group);
  if (!readers.length) return '';
  return html`<span class="lbl">使っているチーム</span>${readers.map(({ t, m }) => html`<button class="uchip" data-act="goto" data-t=${t.id} data-m=${m.name} data-i=${t.members.indexOf(m)}><span class="g">${r.group === 'global' ? `${projectNameOf(t.group)}: ` : ''}${t.id}</span>${m.name}</button>`)}`;
}

// Panel is one place's document: the head (where it stands, the 編集 / Markdown
// switch), the shared part's drawing of the document, and the foot (保存, what
// changed, 複製 / 削除).
function Panel({ r, l }) {
  const e = r[l], d = e.draft, key = r.uid + l, both = live(r, 'project') && live(r, 'global');
  const shadowed = l === 'global' && live(r, 'project');
  const row = rowOfPanel(r, l);
  const issues = row.issues;
  const nread = e.saved && !r[other(l)] ? whoReads(e.saved.name, r.group).length : 0;
  const mode = state.pmode[key] || 'edit';
  const placeLabel = GROUPED && r.group && r.group !== 'global' ? projectNameOf(r.group) : LNAME[l];
  const refusal = refusalOf(key);
  const nameError = refusal && refusal.at === 'name' ? refusal.text : '';
  const docError = refusal && refusal.at === 'doc' ? refusal.text : '';
  const n = diffCount(e);
  const before = new Set((e.saved && e.__savedIssues ? e.__savedIssues : issues).filter((m) => m.sev === 'err').map((m) => m.msg));
  const blocked = !e.saved ? issues.some((m) => m.sev === 'err') : (issues.some((m) => m.sev === 'err' && !before.has(m.msg)) || n === 0);
  return html`<section class=${'panel' + (shadowed ? ' shadowed' : '')} data-uid=${r.uid} data-l=${l} data-mode=${mode}>
      <header class="ph">
        ${r.isNew
          ? html`<div class="seg">${PLACES.map((x) => html`<button class=${l === x ? 'on' : ''} data-act="pick-place" data-x=${x}>${LNAME[x]}</button>`)}</div>`
          : html`<span class="ln">${placeLabel}</span>`}
        <code class="path">${pathOfRow(r, l, d)}</code>
        <span class="phr">
          ${both ? html`<span class=${'state ' + (shadowed ? 'off' : 'on')}>${shadowed ? '隠れている' : '効いている'}</span>` : ''}
          <span class="seg pview">${[['edit', '編集'], ['md', 'Markdown']].map(([v, label]) => html`<button class=${mode === v ? 'on' : ''} data-act="pmode" data-v=${v} aria-pressed=${mode === v}>${label}</button>`)}</span>
        </span>
      </header>
      <div class="pmain"><${RuleDoc} row=${row} issues=${issues} nameError=${nameError} /></div>
      <footer class="pf">
        <button class="btn primary" data-act="save" disabled=${blocked}>保存</button><span class="errline doc" hidden=${!docError}>${docError}</span><span class="dirty">${e.saved && n ? `${n} か所` : ''}</span><span class="vcount"><${Counts} list=${issues} /></span>${okOf(key) ? html`<span class="ok">保存しました</span>` : ''}
        <span class="acts">
          ${e.saved ? html`<button class="btn" data-act="dup">複製</button>` : ''}
          ${e.saved && !r[other(l)] ? html`<button class="btn" data-act="copy">${LNAME[other(l)]}にコピー</button>` : ''}
          ${!e.saved ? html`<button class="btn danger" data-act="discard">取り消し</button>`
            : state.confirm[key] ? html`<button class="btn danger solid" data-act="del-yes">削除する${nread ? `（${nread} 人が読んでいる）` : ''}</button><button class="btn" data-act="del-no">やめる</button>`
            : html`<button class="btn danger" data-act="del">削除</button>`}
        </span>
      </footer>
    </section>`;
}
