// lib/hq/run.tsx — チーム定義の一覧の「動き」の欄（spec 5 章、r-5d1a、n-4b22）。
//
// 定義が動いているチームと、その名前・ワークスペースと 終了する。動いていなければ、
// プロジェクトのチーム定義は「このプロジェクトで起動する」、グローバルのチーム定義は
// 「プロジェクトを選んで起動する」を出す（前に動いたワークスペースがあれば 前は …）。
// グローバルの定義は、どのプロジェクトでも起動できるので、動いていてもこのボタンを出す
// （r-df61、d-b023）。
//
// 動いているチームに反映していない変更があるときは、反映していない変更の数（
// lib/apply の teamCount）を出し、押すとそのチームに絞った反映の欄を開く（n-4b22）。
// まだ起きていないメンバーを 1 人ずつ起動する道は無くなった: 足したメンバーは
// 反映（起動する）の 1 つで扱う（d-36ef）。
//
// 定義を組み立てられないチームは、まだ起きていないメンバーを名指しできない（本部の
// 答えは動いているエージェントだけを並べる）。黙って何も出さないことにならないよう、
// 答えが持っている理由（t.error、hergy status と同じ文で直し方も書いてある）を出す
// （r-fc16）。
//
// チーム定義の画面は module になったので、この欄は部品（RunCell）で描く。押し込み
// はこの部品が受ける（webui-components 2 章）: 起動のボタンは起動の欄、終了する は
// 終了する欄、反映していない変更 N は反映の欄を開く。data-hq は e2e と CSS が読むので
// 残す。
import type { JSX } from 'preact';
import { runningFor, lastWorkspaceOf, projectName } from './state.js';
import type { RunningTeam } from './state.js';
import { startSheet, stopSheet } from './sheets.js';
import { openSheet, teamCount } from '../apply/state.js';

// startLabel is what the row's button says it will do: a global definition asks
// for the project first, and a project's starts here (r-df61, d-b023).
const startLabel = (place: string): string => (place === 'global' ? 'プロジェクトを選んで起動する' : 'このプロジェクトで起動する');

// RunCell is one row's 動き: the running teams, and the start button (spec 5 章).
// The headquarters holds the reading; the cell reads it and draws it. A team
// started under a name of its own shows that name beside its state (r-df61).
export function RunCell({ definition, place, project }: { definition: string; place: string; project?: string }): JSX.Element {
  const live: RunningTeam[] = runningFor(definition, place, project || '');
  return <>{live.map((t) => {
    const label = t.starting ? '起動の途中 · 許可待ち' : (t.waiting || []).length ? '動いている · 許可待ち' : '動いている';
    const where = [place === 'global' ? (t.project_name || projectName(t.project)) : '', t.workspace.label || t.workspace.id || ''].filter(Boolean).join(' · ');
    const notUp = t.not_up || [];
    const changed = teamCount(t.team);
      return <><span class={'hq-on' + (t.starting || (t.waiting || []).length ? ' wait' : '')}>{label}</span>{t.team !== definition ? <code class="hq-tn" data-role="team-name">{t.team}</code> : ''}{where ? <span class="hq-wsn">{where}</span> : ''}<button type="button" class="hq-btn sm danger" data-hq="stop" data-team={t.team} onClick={() => stopSheet(t.team)}>終了する</button>{t.error ? <span class="hq-why" data-role="running-error">{t.error}</span> : ''}{changed ? <button type="button" class="hq-btn sm" data-hq="apply" data-team={t.team} onClick={() => openSheet(t.team)}>反映していない変更 <b>{changed}</b></button> : ''}{notUp.length ? <span class="hq-up" data-role="not-up">{notUp.map((n) => <span class="hq-upm"><code>{n}</code></span>)}</span> : ''}</>;
  })}{!live.length || place === 'global' ? <><button type="button" class="hq-btn sm" data-hq="start" data-def={definition} data-place={place || 'project'} data-project={place === 'global' ? '' : (project || '')} onClick={() => startSheet(definition, place || 'project', place === 'global' ? '' : (project || ''))}>{startLabel(place || 'project')}</button>{!live.length && lastWorkspaceOf(definition) ? <span class="hq-last">前は {lastWorkspaceOf(definition)}</span> : ''}</> : ''}</>;
}

