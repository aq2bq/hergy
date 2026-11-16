import { launchWaitingLabel, stopTeamLabel } from '../lib/screen-words.js';
import { term } from '../lib/terms.generated.js';
import { call, problemText } from '../lib/api';
import { useState, useEffect, useRef } from '../lib/preact';
// The activity board draws each team's figure and the members waiting for a human.
import type { JSX } from 'preact';
import * as A from '../lib/hq.js';
import { openSheet, teamCount } from '../lib/apply/state.js';
import { teams, empty, fetchError, note, current, noneText, ST, NS, lastMoment, pick, openedTeam } from './state.js';
import type { ActivityTeam, CrewMember } from './state.js';
import { Figure } from './figure.js';
import { movePage } from '../lib/page-position.js';

// A workspace herdr did not name is left out whole, so no separator stands
// without a word (r-5b96 audit 11).
const whereText = (t: ActivityTeam, node: Pick<CrewMember, 'herdr' | 'name'> | null): string => [
  t.workspace ? `${term('workspace')} ${t.workspace}` : '',
  `タブ ${t.name}`,
  `ペイン ${(node && node.herdr) || t.name}`,
].filter(Boolean).join(' · ');
// A waiting member's question and the action to answer it live in its team card.
function Waiting({ team: t }: { team: ActivityTeam }): JSX.Element | null {
  const [error, setError] = useState('');
  const [pending, setPending] = useState('');
  const [moved, setMoved] = useState('');
  async function focus(m: CrewMember) {
    if (pending) return;
    setPending(m.herdr); setError(''); setMoved('');
    try { await call('POST', '/api/activity/focus', { team: t.name, member: m.herdr }); setMoved(m.herdr); }
    catch (e) { setError(problemText(e, 'herdr でペインへ移動できませんでした')); }
    finally { setPending(''); }
  }
  const waiting = t.crew.filter((m) => m.state === 'blocked' || m.state === 'handed');
  if (!waiting.length) return null;
  return <div class="hq-waiting">{waiting.map((m) => {
    const q = m.question || t.question || (t.starting ? `${term('start')}の途中で、このフォルダを信頼するかを聞いています` : '');
    return <div class="hq-wait" data-member={m.herdr} key={m.herdr}>
      <code>{m.herdr || (t.name + '-' + m.name)}</code>
      <span class="hq-question">{q}</span>
      <span class="hq-where">{whereText(t, m)}</span>
      <button type="button" class="hq-btn primary" disabled={!!pending} aria-busy={pending === m.herdr} onClick={() => focus(m)}>{pending === m.herdr ? '移動中…' : moved === m.herdr ? 'herdr で移動しました' : 'herdr でペインへ移動'}</button>
    </div>;
  })}{error ? <p role="alert" class="hq-err">{error}</p> : null}</div>;
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
    ? <p class="hq-why">{`${term('handed')}の${term('member')}はいません。最後に動いたのは `}{lastMoment(t) || (t.since && t.since.at) || ''}。lead に続きを渡すか、<code>hergy nudge</code> でうながす</p>
    : '';
  // 定義を組み立てられないチームは、理由も直し方もこの行に出す: 本部の動きの欄と
  // 同じ文で、hergy status が書くもの（r-fc16）。
  const broken = t.error ? <p class="hq-why" data-role="definition-error">{t.error}</p> : '';
  const defHref = `/teams?open=${encodeURIComponent(A.token(t.definition, t.place === 'global' ? 'global' : t.project))}`;
  const changed = teamCount(t.name);
  return <article class={'hq-tile ' + t.state + (sel ? ' sel' : '')} data-team={t.name}>
    <button type="button" class="hq-th" data-pick={t.name} aria-pressed={sel ? 'true' : 'false'} onClick={() => pick(t.name)}>
      <span class="hq-tn">{t.name}</span>{projectWorkspace ? <span class="hq-pw">{projectWorkspace}</span> : ''}
      <span class={'st ' + t.state}>{t.starting ? launchWaitingLabel() : (ST[t.state] || '')}</span>
      <span class="hq-tl">{t.since ? <><b>{t.since.len}</b> {t.since.at}</> : ''}</span>
    </button>
    <Figure team={t} />
    {turnLine(t)}
    {broken}
    {stall}
    <Waiting team={t} />
    {countsLine(t)}
    <div class="hq-tactions" aria-label={`${t.name} の操作`}>
      <a class="hq-btn sm" href={defHref}>{`${term('team-definition')}を開く`}</a>
      {changed > 0 ? <button type="button" class="hq-btn sm" data-hq="apply" data-team={t.name} onClick={() => openSheet(t.name)}>{`${term('apply')}していない変更 `}<b>{changed}</b></button> : ''}
      <button type="button" class="hq-btn sm danger" data-hq="stop" data-team={t.name} onClick={() => A.stopSheet(t.name)}>{stopTeamLabel()}</button>
    </div>
  </article>;
}

// Board is the team cards, or the one line a board with nothing to show carries.
export function Board(): JSX.Element {
  // The confirmation every screen shares (r-d480): /activity stops a team
  // through it too, so the shape is the same on all four screens.
  return <>{fetchError.value ? <p class="hq-err" data-role="hq-error">{fetchError.value}</p> : null}<BoardRows /></>;
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
  if (!rows.length) return empty.value ? <p class="gone">{n.ok ? noneText : n.text}</p> : <></>;
  if (!n.ok) return <p class="gone">{n.text}</p>;
  return <div class="hq-board"><div class="hq-tiles">{rows.map((t) => <Tile key={t.name} team={t} />)}</div>{rows.some(t => t.workers.some(w => !w.from)) ? <p class="hq-worker-guide">{`雇い主不明：${term('launch-record')}には、${term('worker')}を雇った${term('member')}の情報がありません。${term('worker')}のペインで確認してください。`}</p> : null}</div>;
}
