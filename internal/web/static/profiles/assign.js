// profiles/assign.js — 割り当てのタブ（spec 2 節、R6）。
//
// チーム定義ごとの札と、メンバー・ワーカーのプロファイルの選択を描く。押し込みは
// main.js が actions へ渡す（webui-preact-design 5 章 1）。
import { html } from '../vendor/preact.js';
import { assign, assignKind, pickChanged, teamDirty, teamMissing, ISLAND, classOf } from './state.js';

const optionKind = (row) => {
  const group = (row.options || []).find((g) => (g.names || []).includes(row.profile));
  return group ? group.kind : '';
};

// kindCounts is the bar's numbers: how many picks each kind holds now.
function kindCounts() {
  const counts = {};
  (assign.value.teams || []).forEach((team) => (team.rows || []).forEach((row) => {
    const kind = optionKind(row);
    if (kind) counts[kind] = (counts[kind] || 0) + 1;
  }));
  return counts;
}

export function Kindbar() {
  const kinds = (ISLAND.assign && ISLAND.assign.kinds) || [];
  const counts = kindCounts();
  return html`<div class="kindbar">${kinds.map((k) => {
    const count = counts[k.name] || 0;
    const grow = Math.max(1, Math.min(20, count));
    const on = assignKind.value !== null && assignKind.value === k.name;
    const dim = assignKind.value !== null && !on;
    return html`<button class=${'kseg ' + (k.kindClass || '') + ' g' + grow + (dim ? ' dim' : '')} data-act="kind-hi" data-k=${k.name} aria-pressed=${on ? 'true' : 'false'}><span>${k.name}</span><b>${count}</b></button>`;
  })}</div>`;
}

function Pick({ row }) {
  const kind = optionKind(row);
  return html`<div class="pick">${pickChanged(row) ? html`<s class="was">${row.saved}</s>` : ''}<i class=${'sw ' + (kind ? 'k-' + kind : row.kindCls || '')}></i><span class="sel"><select data-k=${row.key} data-act="pick" aria-label="${row.name} のプロファイル">${row.missing ? html`<option selected value=${row.profile} data-kind="">${row.profile}</option>` : ''}${(row.options || []).map((g) => html`<optgroup label=${g.kind}>${(g.names || []).map((name) => html`<option value=${name} data-kind=${g.kind} selected=${name === row.profile}>${name}</option>`)}</optgroup>`)}</select></span></div>`;
}

function AssignRow({ row }) {
  const dim = assignKind.value !== null && optionKind(row) !== assignKind.value;
  return html`<div class=${'arow' + (row.missing ? ' missing' : '') + (dim ? ' dim' : '') + (pickChanged(row) ? ' changed' : '')} data-k=${row.key} data-saved=${row.saved || ''}>
    <div class="who">${row.kind === 'm'
      ? html`<code>${row.name}</code><span class="role"><code>role ${row.role}</code></span>`
      : html`<span class="purpose">${row.name}</span><span class="role"><code>from ${row.role}</code></span>`}</div>
    <${Pick} row=${row} />
    ${row.missing ? html`<ul class="vm"><li><span class="sev err">問題</span><span>${row.error}</span></li></ul>` : ''}
  </div>`;
}

function Team({ team }) {
  const dirty = teamDirty(team);
  const missing = teamMissing(team);
  const rows = team.rows || [];
  return html`<section class="team" data-g=${team.name} data-hash=${team.hash}>
    <header class="th"><span class="tl">チーム定義</span><code class="tid">${team.name}</code><span class="tn">${team.members}</span>
      <span class="tside">${team.err > 0 ? html`<span class="sev err">問題 ${team.err}</span>` : ''}${team.saved && !dirty ? html`<span class="ok">保存しました</span>` : ''}${dirty > 0 ? html`<span class="dirty">${dirty} 人</span><button class="btn primary" data-act="g-save" disabled=${missing > 0}>保存</button>` : ''}</span>
      <code class="path">${team.path}</code>
    </header>
    ${rows.filter((row) => row.kind === 'm').map((row) => html`<${AssignRow} key=${row.key} row=${row} />`)}
    ${team.workers > 0 ? html`<div class="sub">ワーカー</div>${rows.filter((row) => row.kind === 'w').map((row) => html`<${AssignRow} key=${row.key} row=${row} />`)}` : ''}
    ${team.refusal ? html`<span class="sev err refusal">${team.refusal}</span>` : ''}
  </section>`;
}

export function AssignPage() {
  return html`<${Kindbar} /><div class="teams">${assign.value.empty ? html`<p class="none">チーム定義はありません</p>` : ''}${(assign.value.teams || []).map((team) => html`<${Team} key=${team.name} team=${team} />`)}</div>`;
}
