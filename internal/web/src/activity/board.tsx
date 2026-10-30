// activity/board.tsx — アクティビティの画面（spec 6 章、r-8a67、d-8a45）。
//
// 上から、人間を待っている（許可待ち・人間の返事待ち）と止まっているチーム、
// 動いているチームの図（1 チーム 1 枚）。行の一覧は使わない。
// ここは state.ts の signal を読んで描くだけで、DOM を自分では書かない
// （webui-preact-design 5 章 1）。class と data-* は CSS と e2e が読むので、
// 移す前のマークアップから 1 つも変えない（5 章 6）。
import type { JSX } from 'preact';
import * as A from '../lib/hq.js';
import { openSheet, teamCount } from '../lib/apply/state.js';
import { teams, note, current, noneText, ST, NS, lastMoment, pick } from './state.js';
import type { ActivityTeam, CrewMember } from './state.js';
import { Figure } from './figure.js';

// A workspace herdr did not name is left out whole, so no separator stands
// without a word (r-5b96 audit 11).
const whereText = (t: ActivityTeam, node: Pick<CrewMember, 'herdr' | 'name'> | null): string => [
  t.workspace ? `ワークスペース ${t.workspace}` : '',
  `タブ ${t.name}`,
  `ペイン ${(node && node.herdr) || t.name}`,
].filter(Boolean).join(' · ');
const tabWhere = (t: ActivityTeam): string => [t.workspace ? `ワークスペース ${t.workspace}` : '', `タブ ${t.name}`].filter(Boolean).join(' · ');

function pickTile(name: string): void {
  pick(name);
  const tile = Array.from(document.querySelectorAll<HTMLElement>('.hq-tile[data-team]')).find((el) => el.dataset.team === name);
  if (tile && typeof tile.scrollIntoView === 'function') tile.scrollIntoView({ block: 'nearest', behavior: 'smooth' });
}

// attention is the first tier: every member waiting for a human, with where to
// answer, and every team none of whose members holds the turn (spec 6 章).
function Attention({ rows }: { rows: ActivityTeam[] }): JSX.Element {
  const waiting: JSX.Element[] = [];
  for (const t of rows) {
    for (const m of t.crew) {
      if (m.state !== 'blocked' && m.state !== 'handed') continue;
      const q = t.question || (t.starting ? '起動の途中で、このフォルダを信頼するかを聞いています' : '');
      waiting.push(<button type="button" class="hq-at" data-pick={t.name} onClick={() => pickTile(t.name)}><span class={'st ' + (m.state === 'blocked' ? 'answer' : 'turn')}>{m.state === 'blocked' ? '許可待ち' : ST.turn}</span>
        <span class="hq-aw"><code>{m.herdr || (t.name + '-' + m.name)}</code><span class="hq-pw">{t.projectName}</span></span>
        <span class="hq-aq">{q}</span>
        <span class="hq-where">{whereText(t, m)}</span>
        <span class="hq-tl">{m.since ? m.since : ''}</span></button>);
    }
  }
  const stalled: JSX.Element[] = rows.filter((t) => t.state === 'stalled').map((t) => <button type="button" class="hq-at stall" data-pick={t.name} onClick={() => pickTile(t.name)}><span class="st stalled">止まっている</span>
    <span class="hq-aw"><code>{t.name}</code><span class="hq-pw">{t.projectName}</span></span>
    <span class="hq-aq">誰もターンを持っていない</span>
    <span class="hq-where">{tabWhere(t)}</span>
    <span class="hq-tl">{(t.since && t.since.len) || ''}</span></button>);
  if (!waiting.length && !stalled.length) return <section class="hq-att calm"><p>人間を待っているチームも、止まっているチームもありません。</p></section>;
  return <section class="hq-att"><div class="hq-ah"><b>人間を待っている</b><span class="n">{waiting.length}</span>{stalled.length ? <><b class="sp">止まっている</b><span class="n">{stalled.length}</span></> : ''}</div><div class="hq-al">{waiting}{stalled}</div></section>;
}

function countsLine(t: ActivityTeam): JSX.Element {
  const states: [string, string, boolean][] = [
    ['active', NS.active, true], ['idle', NS.idle, true], ['blocked', NS.blocked, false],
    ['handed', NS.handed, false], ['absent', NS.absent, false],
  ];
  const group = (label: string, entries: { state: string }[]): JSX.Element => <div class="hq-tgroup">
    <b>{label} {entries.length}人</b>
    {states.map(([key, word, always]) => {
      const count = entries.filter((entry) => entry.state === key).length;
      return always || count ? <span><i class={'dot ' + key}></i>{word} {count}</span> : '';
    })}
  </div>;
  return <div class="hq-tc">{group('メンバー', t.crew)}{group('ワーカー', t.workers)}</div>;
}

function Tile({ team: t }: { team: ActivityTeam }): JSX.Element {
  const sel = t.name === current.value?.name;
  const projectWorkspace = [t.projectName, t.workspace].filter(Boolean).join(' · ');
  const stall = t.state === 'stalled'
    ? <p class="hq-why">誰もターンを持っていません。人間の返事待ちのメンバーもいません。最後に動いたのは {lastMoment(t) || (t.since && t.since.at) || ''}。lead に続きを渡すか、<code>hergy nudge</code> でうながす</p>
    : '';
  // 定義を組み立てられないチームは、理由も直し方もこの行に出す: 本部の動きの欄と
  // 同じ文で、hergy status が書くもの（r-fc16）。
  const broken = t.error ? <p class="hq-why" data-role="definition-error">{t.error}</p> : '';
  const defHref = `/teams?open=${encodeURIComponent(A ? A.token(t.definition, t.project) : t.definition)}`;
  const changed = teamCount(t.name);
  return <article class={'hq-tile ' + t.state + (sel ? ' sel' : '')} data-team={t.name}>
    <button type="button" class="hq-th" data-pick={t.name} aria-pressed={sel ? 'true' : 'false'} onClick={() => pick(t.name)}>
      <span class="hq-tn">{t.name}</span>{projectWorkspace ? <span class="hq-pw">{projectWorkspace}</span> : ''}
      <span class={'st ' + t.state}>{t.starting ? '起動の途中 · 許可待ち' : (ST[t.state] || '')}</span>
      <span class="hq-tl">{t.since ? <><b>{t.since.len}</b> {t.since.at}</> : ''}</span>
    </button>
    <Figure team={t} />
    {broken}
    {stall}
    {countsLine(t)}
    <div class="hq-tactions" aria-label={`${t.name} の操作`}>
      <a class="hq-btn sm" href={defHref}>チーム定義を開く</a>
      {changed > 0 ? <button type="button" class="hq-btn sm" data-hq="apply" data-team={t.name} onClick={() => openSheet(t.name)}>反映していない変更 <b>{changed}</b></button> : ''}
      <button type="button" class="hq-btn sm danger" data-hq="stop" data-team={t.name} onClick={() => A.stopSheet(t.name)}>チームを終了する…</button>
    </div>
  </article>;
}

// Board is the whole screen: attention rows and team tiles, or the one line a board with
// nothing to show carries (2.6, 2.7).
export function Board(): JSX.Element {
  const rows = teams.value;
  const n = note.value;
  if (!rows.length) return <p class="gone">{n.ok ? noneText : n.text}</p>;
  if (!n.ok) return <p class="gone">{n.text}</p>;
  return <div class="hq-board"><Attention rows={rows} /><div class="hq-tiles">{rows.map((t) => <Tile key={t.name} team={t} />)}</div></div>;
}
