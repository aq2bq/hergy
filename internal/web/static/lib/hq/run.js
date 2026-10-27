// lib/hq/run.js — チーム定義の一覧の「動き」の欄（spec 5 章）。
//
// 定義が動いているチームと、そのワークスペースと 止める、動いていなければ 起動…
// （前に動いたワークスペースがあれば 前は …）。グローバルの定義は、どのプロジェクト
// でも起こせるので、動いていても 起動… を出す。
//
// チーム定義の画面はまだ classic script なので、この欄は文字列で返す
// （`runCellHTML`）。その画面が module になったら、同じ見た目の部品に置き換える。
import { esc, runningFor, lastWorkspaceOf, projectName } from './state.js';

export function runCellHTML(definition, place, project) {
  const live = runningFor(definition, place, project);
  const parts = live.map((t) => {
    const label = t.starting ? '起動の途中 · 許可待ち' : (t.waiting || []).length ? '動いている · 許可待ち' : '動いている';
    const where = (place === 'global' ? `${t.project_name || projectName(t.project)} · ` : '') + (t.workspace.label || t.workspace.id);
    return `<span class="hq-on${t.starting || (t.waiting || []).length ? ' wait' : ''}">${esc(label)}</span><span class="hq-wsn">${esc(where)}</span><button type="button" class="hq-btn sm danger" data-hq="stop" data-team="${esc(t.team)}">止める</button>`;
  });
  if (!live.length || place === 'global') {
    const last = !live.length ? lastWorkspaceOf(definition) : '';
    // A global definition chooses its project in the start sheet, so its button
    // names none (designer's note).
    const startProject = place === 'global' ? '' : (project || '');
    parts.push(`<button type="button" class="hq-btn sm" data-hq="start" data-def="${esc(definition)}" data-place="${esc(place || 'project')}" data-project="${esc(startProject)}">起動…</button>${last ? `<span class="hq-last">前は ${esc(last)}</span>` : ''}`);
  }
  return parts.join('');
}
