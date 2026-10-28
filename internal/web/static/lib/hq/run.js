// lib/hq/run.js — チーム定義の一覧の「動き」の欄（spec 5 章、r-5d1a）。
//
// 定義が動いているチームと、そのワークスペースと 止める、動いていなければ 起動…
// （前に動いたワークスペースがあれば 前は …）。グローバルの定義は、どのプロジェクト
// でも起動できるので、動いていても 起動… を出す。
//
// 動いているチームが定義に持っていて、まだ起きていないメンバーは、名前と 起動 で
// 出す（r-5d1a）。押すと起動する（lib/hq/raise.js）: 起動の途中かどうかは起動の欄が
// 出すので、動いている間は 起動 を出さない。
//
// 定義を組み立てられないチームは、まだ起きていないメンバーを名指しできない（本部の
// 答えは動いているエージェントだけを並べる）。黙って 起動 が消えないよう、答えが
// 持っている理由（t.error、hergy status と同じ文で直し方も書いてある）を出す
// （r-fc16）。
//
// チーム定義の画面は module になったので、この欄は部品（RunCell）で描く。押し込み
// はこの部品が受ける（webui-components 2 章）: 起動… は起動の欄、止める は止める欄を
// 開く。data-hq は e2e と CSS が読むので残す。
import { html, useState } from '../../vendor/preact.js';
import { runningFor, lastWorkspaceOf, projectName } from './state.js';
import { startSheet, stopSheet } from './sheets.js';
import { raiseMember } from './raise.js';

// RunCell is one row's 動き: the running teams, the members of theirs that are
// not up, or 起動… (spec 5 章). The headquarters holds the reading; the cell reads
// it and draws it.
export function RunCell({ definition, place, project }) {
  const live = runningFor(definition, place, project);
  const [error, setError] = useState('');
  const press = async (team, name) => { setError(await raiseMember(team, name)); };
  return html`${live.map((t) => {
    const label = t.starting ? '起動の途中 · 許可待ち' : (t.waiting || []).length ? '動いている · 許可待ち' : '動いている';
    const where = (place === 'global' ? `${t.project_name || projectName(t.project)} · ` : '') + (t.workspace.label || t.workspace.id);
    const notUp = t.not_up || [];
    return html`<span class=${'hq-on' + (t.starting || (t.waiting || []).length ? ' wait' : '')}>${label}</span><span class="hq-wsn">${where}</span><button type="button" class="hq-btn sm danger" data-hq="stop" data-team=${t.team} onClick=${() => stopSheet(t.team)}>止める</button>${t.error ? html`<span class="hq-why" data-role="running-error">${t.error}</span>` : ''}${notUp.length ? html`<span class="hq-up" data-role="not-up">${notUp.map((n) => html`<span class="hq-upm"><code>${n}</code>${t.starting ? '' : html`<button type="button" class="hq-btn sm" data-hq="raise" data-team=${t.team} data-member=${n} aria-label=${`${n} を起動`} onClick=${() => press(t.team, n)}>起動</button>`}</span>`)}</span>` : ''}`;
  })}${error ? html`<span class="hq-err" data-role="raise-error">${error}</span>` : ''}${!live.length || place === 'global' ? html`<button type="button" class="hq-btn sm" data-hq="start" data-def=${definition} data-place=${place || 'project'} data-project=${place === 'global' ? '' : (project || '')} onClick=${() => startSheet(definition, place || 'project', place === 'global' ? '' : (project || ''))}>起動…</button>${!live.length && lastWorkspaceOf(definition) ? html`<span class="hq-last">前は ${lastWorkspaceOf(definition)}</span>` : ''}` : ''}`;
}
