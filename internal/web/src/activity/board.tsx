// activity/board.tsx — 活動の画面の 3 段（spec 6 章、r-8a67、d-8a45）。
//
// 上から 3 段: 人間を待っている（許可待ち・人間の番）と止まっているチーム、
// 動いているチームの図（1 チーム 1 枚）、選んだチームの中。行の一覧は使わない。
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

// attention is the first tier: every member waiting for a human, with where to
// answer, and every team none of whose members holds the turn (spec 6 章).
function Attention({ rows }: { rows: ActivityTeam[] }): JSX.Element {
  const waiting: JSX.Element[] = [];
  for (const t of rows) {
    for (const m of t.crew) {
      if (m.state !== 'blocked' && m.state !== 'handed') continue;
      const q = t.question || (t.starting ? '起動の途中で、このフォルダを信頼するかを聞いています' : '');
      waiting.push(<button type="button" class="hq-at" data-pick={t.name} onClick={() => pick(t.name)}><span class={'st ' + (m.state === 'blocked' ? 'answer' : 'turn')}>{m.state === 'blocked' ? '許可待ち' : '人間の番'}</span>
        <span class="hq-aw"><code>{m.herdr || (t.name + '-' + m.name)}</code><span class="hq-pw">{t.projectName}</span></span>
        <span class="hq-aq">{q}</span>
        <span class="hq-where">{whereText(t, m)}</span>
        <span class="hq-tl">{m.since ? m.since : ''}</span></button>);
    }
  }
  const stalled: JSX.Element[] = rows.filter((t) => t.state === 'stalled').map((t) => <button type="button" class="hq-at stall" data-pick={t.name} onClick={() => pick(t.name)}><span class="st stalled">止まっている</span>
    <span class="hq-aw"><code>{t.name}</code><span class="hq-pw">{t.projectName}</span></span>
    <span class="hq-aq">誰もターンを持っていない</span>
    <span class="hq-where">{tabWhere(t)}</span>
    <span class="hq-tl">{(t.since && t.since.len) || ''}</span></button>);
  if (!waiting.length && !stalled.length) return <section class="hq-att calm"><p>人間を待っているチームも、止まっているチームもありません。</p></section>;
  return <section class="hq-att"><div class="hq-ah"><b>人間を待っている</b><span class="n">{waiting.length}</span>{stalled.length ? <><b class="sp">止まっている</b><span class="n">{stalled.length}</span></> : ''}</div><div class="hq-al">{waiting}{stalled}</div></section>;
}

function countsLine(t: ActivityTeam): JSX.Element {
  const states = [
    ['active', NS.active, true], ['idle', NS.idle, true], ['blocked', NS.blocked, false],
    ['handed', NS.handed, false], ['absent', NS.absent, false],
  ] as const;
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
  const stall = t.state === 'stalled'
    ? <p class="hq-why">誰もターンを持っていません（人間の番でもない）。最後に動いたのは {lastMoment(t) || (t.since && t.since.at) || ''}。lead に続きを渡すか、<code>hergy nudge</code> でうながす</p>
    : '';
  // 定義を組み立てられないチームは、理由も直し方もこの行に出す: 本部の動きの欄と
  // 同じ文で、hergy status が書くもの（r-fc16）。
  const broken = t.error ? <p class="hq-why" data-role="definition-error">{t.error}</p> : '';
  return <article class={'hq-tile ' + t.state + (sel ? ' sel' : '')} data-team={t.name}>
    <button type="button" class="hq-th" data-pick={t.name} aria-pressed={sel ? 'true' : 'false'} onClick={() => pick(t.name)}>
      <span class="hq-tn">{t.name}</span><span class="hq-pw">{t.projectName} · {t.workspace}</span>
      <span class={'st ' + t.state}>{ST[t.state] || ''}</span>
      <span class="hq-tl">{t.since ? <><b>{t.since.len}</b> {t.since.at}</> : ''}</span>
    </button>
    <Figure team={t} />
    {broken}
    {stall}
    {countsLine(t)}
  </article>;
}

// Detail is the third tier: the opened team's last screen, where to answer, and
// every member and worker with its state (spec 6 章). A team with a change that
// is not applied yet carries 反映していない変更 の数 here (n-4b22): the press opens
// the sheet narrowed to this team. Starting one member by hand is not offered any
// more — a member the definition has and no agent is live for is 起動する の反映
// (d-36ef).
function Detail({ team: t }: { team: ActivityTeam }): JSX.Element {
  // The headquarters' reading is a module: the board imports it (5 章 1).
  // The screen's last lines are drawn when there are some: a team whose pane
  // could not be read (a working one, often) has no heading and no empty box
  // (r-5b96 audit 10). The place to answer stands on its own then.
  const answerBox = t.wait ? <div class="hq-ans"><span>答える場所</span><span class="hq-where">{whereText(t, t.wait)}</span></div> : '';
  const hasTail = t.tail.length > 0;
  const screenBox = hasTail ? <div class="hq-dq"><div class="th">画面の最後（<code>{t.wait ? (t.wait.herdr || (t.name + '-' + t.wait.name)) : t.last ? (t.name + '-' + t.last) : t.name}</code>）</div>
    <pre class="term">{t.tail.join('\n')}</pre>{answerBox}</div>
    : (answerBox ? <div class="hq-dq">{answerBox}</div> : '');
  const rows = t.crew.map((m) => <li class={m.state}><i class={'dot ' + m.state}></i><span class="nm">{m.name}</span><span class="sw">{NS[m.state] || ''}</span><span class="t">{m.at || ''}</span>{m.doing ? <span class="hq-doing">{m.doing}</span> : ''}</li>);
  const workers = t.workers.length ? <><li class="sub">ワーカー</li>{t.workers.map((w) => <li class={w.state}><i class={'dot ' + w.state}></i><span class="nm wk">{w.name}</span><span class="sw">{NS[w.state] || ''}</span><span class="t">{w.from ? w.from + ' が追加' : ''}</span>{w.doing ? <span class="hq-doing">{w.doing}</span> : ''}</li>)}</> : '';
  const defHref = `/teams?open=${encodeURIComponent(A ? A.token(t.definition, t.project) : t.definition)}`;
  const stale = teamCount(t.name);
  const apply = stale ? <button type="button" class="hq-btn sm" data-hq="apply" data-team={t.name} onClick={() => openSheet(t.name)}>反映していない変更 <b>{stale}</b></button> : '';
  return <section class="hq-detail" aria-label={`${t.name} の中`}>
    <div class="hq-dh"><b>{t.name}</b><span class="hq-pw">{[t.projectName, t.workspace ? 'ワークスペース ' + t.workspace : ''].filter(Boolean).join(' · ')}</span><span class={'st ' + t.state}>{ST[t.state] || ''}</span>
      <span class="hq-dacts"><a class="hq-btn sm" href={defHref}>チーム定義を開く</a>{apply}<button type="button" class="hq-btn sm danger" data-hq="stop" data-team={t.name} onClick={() => A.stopSheet(t.name)}>チームを止める…</button></span></div>
    <div class="hq-db"><div class="hq-dleft"><Figure team={t} large />{screenBox}</div><ul class="nodes">{rows}{workers}</ul></div></section>;
}

// Board is the whole screen: the three tiers, or the one line a board with
// nothing to show carries (2.6, 2.7).
export function Board(): JSX.Element {
  const rows = teams.value;
  const n = note.value;
  if (!rows.length) return <p class="gone">{n.ok ? noneText : n.text}</p>;
  if (!n.ok) return <p class="gone">{n.text}</p>;
  const sel = current.value;
  return <div class="hq-board"><Attention rows={rows} /><div class="hq-tiles">{rows.map((t) => <Tile key={t.name} team={t} />)}</div>{sel ? <Detail team={sel} /> : ''}</div>;
}
