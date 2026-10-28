// activity/board.js — 活動の画面の 3 段（spec 6 章、r-8a67、d-8a45）。
//
// 上から 3 段: 人間を待っている（許可待ち・人間の番）と止まっているチーム、
// 動いているチームの図（1 チーム 1 枚）、選んだチームの中。行の一覧は使わない。
// ここは state.js の signal を読んで描くだけで、DOM を自分では書かない
// （webui-preact-design 5 章 1）。class と data-* は CSS と e2e が読むので、
// 移す前のマークアップから 1 つも変えない（5 章 6）。
import { html, useState } from '../vendor/preact.js';
import * as A from '../lib/hq.js';
import { openSheet } from '../lib/deliver/state.js';
import { teams, note, current, noneText, ST, NS, lastMoment, pick } from './state.js';
import { Figure } from './figure.js';

// A workspace herdr did not name is left out whole, so no separator stands
// without a word (r-5b96 audit 11).
const whereText = (t, node) => [
  t.workspace ? `ワークスペース ${t.workspace}` : '',
  `タブ ${t.name}`,
  `ペイン ${(node && node.herdr) || t.name}`,
].filter(Boolean).join(' · ');
const tabWhere = (t) => [t.workspace ? `ワークスペース ${t.workspace}` : '', `タブ ${t.name}`].filter(Boolean).join(' · ');

// attention is the first tier: every member waiting for a human, with where to
// answer, and every team none of whose members holds the turn (spec 6 章).
function Attention({ rows }) {
  const waiting = [];
  for (const t of rows) {
    for (const m of t.crew) {
      if (m.state !== 'blocked' && m.state !== 'handed') continue;
      const q = t.question || (t.starting ? '起動の途中で、このフォルダを信頼するかを聞いています' : '');
      waiting.push(html`<button type="button" class="hq-at" data-pick=${t.name} onClick=${() => pick(t.name)}><span class=${'st ' + (m.state === 'blocked' ? 'answer' : 'turn')}>${m.state === 'blocked' ? '許可待ち' : '人間の番'}</span>
        <span class="hq-aw"><code>${m.herdr || (t.name + '-' + m.name)}</code><span class="hq-pw">${t.projectName}</span></span>
        <span class="hq-aq">${q}</span>
        <span class="hq-where">${whereText(t, m)}</span>
        <span class="hq-tl">${m.since ? m.since : ''}</span></button>`);
    }
  }
  const stalled = rows.filter((t) => t.state === 'stalled').map((t) => html`<button type="button" class="hq-at stall" data-pick=${t.name} onClick=${() => pick(t.name)}><span class="st stalled">止まっている</span>
    <span class="hq-aw"><code>${t.name}</code><span class="hq-pw">${t.projectName}</span></span>
    <span class="hq-aq">誰もターンを持っていない</span>
    <span class="hq-where">${tabWhere(t)}</span>
    <span class="hq-tl">${(t.since && t.since.len) || ''}</span></button>`);
  if (!waiting.length && !stalled.length) return html`<section class="hq-att calm"><p>人間を待っているチームも、止まっているチームもありません。</p></section>`;
  return html`<section class="hq-att"><div class="hq-ah"><b>人間を待っている</b><span class="n">${waiting.length}</span>${stalled.length ? html`<b class="sp">止まっている</b><span class="n">${stalled.length}</span>` : ''}</div><div class="hq-al">${waiting}${stalled}</div></section>`;
}

function countsLine(t) {
  const n = (k) => t.crew.filter((m) => m.state === k).length;
  // The figure's counts add up to the crew: a member with no agent is
  // 動いていない, as the opened list calls it (r-5b96 audit K1).
  return html`<div class="hq-tc"><span><i class="dot active"></i>仕事中 ${n('active')}</span><span><i class="dot idle"></i>止まっている ${n('idle')}</span>${n('blocked') ? html`<span><i class="dot blocked"></i>許可待ち ${n('blocked')}</span>` : ''}${n('absent') ? html`<span><i class="dot absent"></i>動いていない ${n('absent')}</span>` : ''}${t.workers.length ? html`<span class="wk">ワーカー ${t.workers.length}</span>` : ''}</div>`;
}

function Tile({ team: t }) {
  const sel = t.name === current.value.name;
  const stall = t.state === 'stalled'
    ? html`<p class="hq-why">誰もターンを持っていません（人間の番でもない）。最後に動いたのは ${lastMoment(t) || (t.since && t.since.at) || ''}。lead に続きを渡すか、<code>hergy nudge</code> でうながす</p>`
    : '';
  // 定義を組み立てられないチームは、理由も直し方もこの行に出す: 本部の動きの欄と
  // 同じ文で、hergy status が書くもの（r-fc16）。
  const broken = t.error ? html`<p class="hq-why" data-role="definition-error">${t.error}</p>` : '';
  return html`<article class=${'hq-tile ' + t.state + (sel ? ' sel' : '')} data-team=${t.name}>
    <button type="button" class="hq-th" data-pick=${t.name} aria-pressed=${sel ? 'true' : 'false'} onClick=${() => pick(t.name)}>
      <span class="hq-tn">${t.name}</span><span class="hq-pw">${t.projectName} · ${t.workspace}</span>
      <span class=${'st ' + t.state}>${ST[t.state] || ''}</span>
      <span class="hq-tl">${t.since ? html`<b>${t.since.len}</b> ${t.since.at}` : ''}</span>
    </button>
    <${Figure} team=${t} />
    ${broken}
    ${stall}
    ${countsLine(t)}
  </article>`;
}

// Detail is the third tier: the opened team's last screen, where to answer, and
// every member and worker with its state (spec 6 章). A member the definition has
// and no agent is live for carries 起動 (r-5d1a): the member is started in the
// team's tab, and the progress stands in the start's own sheet.
function Detail({ team: t }) {
  const [error, setError] = useState('');
  const press = async (name) => { setError(await A.raiseMember(t.name, name)); };
  // The headquarters' reading is a module: the board imports it (5 章 1).
  // The screen's last lines are drawn when there are some: a team whose pane
  // could not be read (a working one, often) has no heading and no empty box
  // (r-5b96 audit 10). The place to answer stands on its own then.
  const answerBox = t.wait ? html`<div class="hq-ans"><span>答える場所</span><span class="hq-where">${whereText(t, t.wait)}</span></div>` : '';
  const hasTail = t.tail.length > 0;
  const screenBox = hasTail ? html`<div class="hq-dq"><div class="th">画面の最後（<code>${t.wait ? (t.wait.herdr || (t.name + '-' + t.wait.name)) : t.last ? (t.name + '-' + t.last) : t.name}</code>）</div>
    <pre class="term">${t.tail.join('\n')}</pre>${answerBox}</div>`
    : (answerBox ? html`<div class="hq-dq">${answerBox}</div>` : '');
  const rows = t.crew.map((m) => html`<li class=${m.state}><i class=${'dot ' + m.state}></i><span class="nm">${m.name}</span><span class="sw">${NS[m.state] || ''}${m.state === 'absent' && !t.starting ? html`<button type="button" class="hq-btn sm" data-hq="raise" data-team=${t.name} data-member=${m.name} aria-label=${`${m.name} を起動`} onClick=${() => press(m.name)}>起動</button>` : ''}</span><span class="t">${m.at || ''}</span>${m.doing ? html`<span class="hq-doing">${m.doing}</span>` : ''}${m.stale ? html`<span class="dlv-sub"><button class="dlv-badge" type="button" data-deliver-open title="伝えていない変更があります" onClick=${() => openSheet()}>定義が変わった</button></span>` : ''}</li>`);
  const wk = t.workers.length ? [html`<li class="sub">ワーカー</li>`, t.workers.map((w) => html`<li class=${w.state}><i class=${'dot ' + w.state}></i><span class="nm wk">${w.name}</span><span class="sw">${NS[w.state] || ''}</span><span class="t">${w.from ? w.from + ' が追加' : ''}</span>${w.doing ? html`<span class="hq-doing">${w.doing}</span>` : ''}</li>`)] : '';
  const defHref = `/teams?open=${encodeURIComponent(A ? A.token(t.definition, t.project) : t.definition)}`;
  return html`<section class="hq-detail" aria-label="${t.name} の中">
    <div class="hq-dh"><b>${t.name}</b><span class="hq-pw">${[t.projectName, t.workspace ? 'ワークスペース ' + t.workspace : ''].filter(Boolean).join(' · ')}</span><span class=${'st ' + t.state}>${ST[t.state] || ''}</span>
      <span class="hq-dacts"><a class="hq-btn sm" href=${defHref}>チーム定義を開く</a><button type="button" class="hq-btn sm danger" data-hq="stop" data-team=${t.name} onClick=${() => A.stopSheet(t.name)}>チームを止める…</button></span></div>
    <div class="hq-db">${screenBox}<ul class="nodes">${rows}${wk}</ul>${error ? html`<p class="hq-err" data-role="raise-error">${error}</p>` : ''}</div></section>`;
}

// Board is the whole screen: the three tiers, or the one line a board with
// nothing to show carries (2.6, 2.7).
export function Board() {
  const rows = teams.value;
  const n = note.value;
  if (!rows.length) return html`<p class="gone">${n.ok ? noneText : n.text}</p>`;
  if (!n.ok) return html`<p class="gone">${n.text}</p>`;
  const sel = current.value;
  return html`<div class="hq-board"><${Attention} rows=${rows} /><div class="hq-tiles">${rows.map((t) => html`<${Tile} key=${t.name} team=${t} />`)}</div>${sel ? html`<${Detail} team=${sel} />` : ''}</div>`;
}
