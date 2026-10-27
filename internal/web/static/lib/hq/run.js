// lib/hq/run.js — チーム定義の一覧の「動き」の欄（spec 5 章）。
//
// 定義が動いているチームと、そのワークスペースと 止める、動いていなければ 起動…
// （前に動いたワークスペースがあれば 前は …）。グローバルの定義は、どのプロジェクト
// でも起こせるので、動いていても 起動… を出す。
//
// チーム定義の画面は module になったので、この欄は部品（RunCell）で描く。押し込み
// は data-hq の委譲（lib/hq.js）が受ける。
import { html } from '../../vendor/preact.js';
import { rev, runningFor, lastWorkspaceOf, projectName } from './state.js';

// RunCell is one row's 動き: the running teams, or 起動… (spec 5 章). The
// headquarters holds the reading; the cell reads it and draws it.
export function RunCell({ definition, place, project }) {
  rev.value;
  const live = runningFor(definition, place, project);
  return html`${live.map((t) => {
    const label = t.starting ? '起動の途中 · 許可待ち' : (t.waiting || []).length ? '動いている · 許可待ち' : '動いている';
    const where = (place === 'global' ? `${t.project_name || projectName(t.project)} · ` : '') + (t.workspace.label || t.workspace.id);
    return html`<span class=${'hq-on' + (t.starting || (t.waiting || []).length ? ' wait' : '')}>${label}</span><span class="hq-wsn">${where}</span><button type="button" class="hq-btn sm danger" data-hq="stop" data-team=${t.team}>止める</button>`;
  })}${!live.length || place === 'global' ? html`<button type="button" class="hq-btn sm" data-hq="start" data-def=${definition} data-place=${place || 'project'} data-project=${place === 'global' ? '' : (project || '')}>起動…</button>${!live.length && lastWorkspaceOf(definition) ? html`<span class="hq-last">前は ${lastWorkspaceOf(definition)}</span>` : ''}` : ''}`;
}
