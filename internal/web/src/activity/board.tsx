import { term } from '../lib/terms.generated.js';
import { call, problemText } from '../lib/api';
import { useState, useEffect, useRef } from '../lib/preact';
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
import { teams, note, current, noneText, ST, NS, lastMoment, pick, openedTeam } from './state.js';
import type { ActivityTeam, CrewMember } from './state.js';
import { Figure } from './figure.js';
import { movePage } from '../lib/page-position.js';

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
  if (tile) movePage('waitingRow', tile);
}

// attention is the first tier: every member waiting for a human, with where to
// answer, and every team none of whose members holds the turn (spec 6 章).
function Attention({ rows }: { rows: ActivityTeam[] }): JSX.Element {
  const [error, setError] = useState('');
  const [pending, setPending] = useState('');
  async function focus(t: ActivityTeam, m: CrewMember) {
    if (pending) return;
    setPending(m.herdr); setError('');
    try { await call('POST', '/api/activity/focus', { team: t.name, member: m.herdr }); setError(''); }
    catch (e) { setError(problemText(e, '端末へ移動できませんでした')); }
    finally { setPending(''); }
  }
  const waiting: JSX.Element[] = [];
  for (const t of rows) {
    for (const m of t.crew) {
      if (m.state !== 'blocked' && m.state !== 'handed') continue;
      const q = m.question || t.question || (t.starting ? `${term('start')}の途中で、このフォルダを信頼するかを聞いています` : '');
      waiting.push(<div class="hq-at" data-pick={t.name} onClick={() => pickTile(t.name)}><span class={'st ' + (m.state === 'blocked' ? 'answer' : 'turn')}>{m.state === 'blocked' ? term('waiting-for-permission') : ST.turn}</span>
        <span class="hq-aw"><code>{m.herdr || (t.name + '-' + m.name)}</code><span class="hq-pw">{t.projectName}</span></span>
        <span class="hq-aq">{q}</span>
        <span class="hq-where">{whereText(t, m)}</span>
        <span class="hq-tl">{m.since ? m.since : ''}</span><button type="button" class="hq-btn" onClick={() => pickTile(t.name)}>{`${term('team')}を見る`}</button><button type="button" class="hq-btn primary" disabled={!!pending} aria-busy={pending === m.herdr} onClick={(e) => { e.stopPropagation(); focus(t, m); }}>{pending === m.herdr ? '端末へ移動中…' : '端末で返答する'}</button></div>);
    }
  }
  const stalled: JSX.Element[] = rows.filter((t) => t.state === 'stalled').map((t) => <button type="button" class="hq-at stall" data-pick={t.name} onClick={() => pickTile(t.name)}><span class="st stalled">止まっている</span>
    <span class="hq-aw"><code>{t.name}</code><span class="hq-pw">{t.projectName}</span></span>
    <span class="hq-aq">{`誰も${term('turn')}を持っていない`}</span>
    <span class="hq-where">{tabWhere(t)}</span>
    <span class="hq-tl">{(t.since && t.since.len) || ''}</span></button>);
  if (!waiting.length && !stalled.length) return <section class="hq-att calm"><p>{`${term('waiting-for-the-human')}${term('team')}も、止まっている${term('team')}もありません。`}</p></section>;
  return <section class="hq-att"><div class="hq-ah"><b>{term('waiting-for-the-human')}</b><span class="n">{waiting.length}</span>{stalled.length ? <><b class="sp">止まっている</b><span class="n">{stalled.length}</span></> : ''}</div><div class="hq-al">{waiting}{stalled}</div>{error ? <p role="alert" class="hq-err">{error}</p> : null}<p class="hq-sub">{`止まっている：エージェントは${term('start')}していますが、今は作業していません。動いていない：${term('start')}中のエージェントがいません。`}</p></section>;
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
  return <div class="hq-tc">{group(term('member'), t.crew)}{group(term('worker'), t.workers)}</div>;
}

// turnLine は、hergy の見張りが記録したターンの持ち主（r-bc26）。文はサーバーが
// 作り、ここは出すだけ。見張りが見ていないチームには出さない。
function turnLine(t: ActivityTeam): JSX.Element | '' {
  return t.turn ? <p class="hq-turn" data-role="turn">{t.turn.text}</p> : '';
}

function Tile({ team: t }: { team: ActivityTeam }): JSX.Element {
  const sel = t.name === current.value?.name;
  const projectWorkspace = [t.projectName, t.workspace].filter(Boolean).join(' · ');
  const stall = t.state === 'stalled'
    ? <p class="hq-why">{`誰も${term('turn')}を持っていません。${term('handed')}の${term('member')}もいません。最後に動いたのは `}{lastMoment(t) || (t.since && t.since.at) || ''}。lead に続きを渡すか、<code>hergy nudge</code> でうながす</p>
    : '';
  // 定義を組み立てられないチームは、理由も直し方もこの行に出す: 本部の動きの欄と
  // 同じ文で、hergy status が書くもの（r-fc16）。
  const broken = t.error ? <p class="hq-why" data-role="definition-error">{t.error}</p> : '';
  const defHref = `/teams?open=${encodeURIComponent(A.token(t.definition, t.place === 'global' ? 'global' : t.project))}`;
  const changed = teamCount(t.name);
  return <article class={'hq-tile ' + t.state + (sel ? ' sel' : '')} data-team={t.name}>
    <button type="button" class="hq-th" data-pick={t.name} aria-pressed={sel ? 'true' : 'false'} onClick={() => pick(t.name)}>
      <span class="hq-tn">{t.name}</span>{projectWorkspace ? <span class="hq-pw">{projectWorkspace}</span> : ''}
      <span class={'st ' + t.state}>{t.starting ? `${term('start')}の途中 · ${term('waiting-for-permission')}` : (ST[t.state] || '')}</span>
      <span class="hq-tl">{t.since ? <><b>{t.since.len}</b> {t.since.at}</> : ''}</span>
    </button>
    <Figure team={t} />
    {turnLine(t)}
    {broken}
    {stall}
    {countsLine(t)}
    <div class="hq-tactions" aria-label={`${t.name} の操作`}>
      <a class="hq-btn sm" href={defHref}>{`${term('team-definition')}を開く`}</a>
      {changed > 0 ? <button type="button" class="hq-btn sm" data-hq="apply" data-team={t.name} onClick={() => openSheet(t.name)}>{`${term('apply')}していない変更 `}<b>{changed}</b></button> : ''}
      <button type="button" class="hq-btn sm danger" data-hq="stop" data-team={t.name} onClick={() => A.stopSheet(t.name)}>{`${term('team')}を${term('down')}…`}</button>
    </div>
  </article>;
}

// Board is the whole screen: attention rows and team tiles, or the one line a board with
// nothing to show carries (2.6, 2.7).
export function Board(): JSX.Element {
  // The confirmation every screen shares (r-d480): /activity stops a team
  // through it too, so the shape is the same on all four screens.
  return <><BoardRows /></>;
}

function BoardRows(): JSX.Element {
  const rows = teams.value;
  const n = note.value;
  const fromAddress = useRef(openedTeam().name);
  useEffect(() => {
    if (!fromAddress.current) return;
    const tile = Array.from(document.querySelectorAll<HTMLElement>('.hq-tile[data-team]')).find((el) => el.dataset.team === fromAddress.current);
    if (!tile) return;
    fromAddress.current = '';
    movePage('openByURL', tile, { instant: true });
  }, [rows]);
  if (!rows.length) return <p class="gone">{n.ok ? noneText : n.text}</p>;
  if (!n.ok) return <p class="gone">{n.text}</p>;
  return <div class="hq-board"><Attention rows={rows} /><div class="hq-tiles">{rows.map((t) => <Tile key={t.name} team={t} />)}</div>{rows.some(t => t.workers.some(w => !w.from)) ? <p class="hq-worker-guide">{`雇い主不明：${term('launch-record')}には、${term('worker')}を雇った${term('member')}の情報がありません。${term('worker')}のペインで確認してください。`}</p> : null}</div>;
}
