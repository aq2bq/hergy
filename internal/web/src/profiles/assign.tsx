// profiles/assign.tsx — 割り当てのタブ（spec 2 節、R6）。
//
// チーム定義ごとの札と、メンバー・ワーカーのプロファイルの選択を描く。押し込みは
// 自分の onClick / onChange から actions.ts と state.ts の関数を直接呼ぶ
// （webui-preact-design 5 章 1）。
import { assign, assignKind, pickChanged, teamDirty, teamMissing, ISLAND, setPick, tellDirty, labelOf } from './state.js';
import { pressKind, pressTeamSave } from './actions.js';
import type { AssignRow, KindBar, OptionGroup, Team } from './types.js';

// inputValue is what a select holds after a person chose in it.
function inputValue(e: Event): string {
  return e.target instanceof HTMLSelectElement ? e.target.value : '';
}

const optionKind = (row: AssignRow): string => {
  const group = (row.options || []).find((g: OptionGroup) => (g.names || []).includes(row.profile));
  return group ? group.kind : '';
};

// kindCounts is the bar's numbers: how many picks each kind holds now.
function kindCounts(): Record<string, number> {
  const counts: Record<string, number> = {};
  (assign.value.teams || []).forEach((team) => (team.rows || []).forEach((row) => {
    const kind = optionKind(row);
    if (kind) counts[kind] = (counts[kind] || 0) + 1;
  }));
  return counts;
}

export function Kindbar() {
  const kinds: KindBar[] = (ISLAND.assign && ISLAND.assign.kinds) || [];
  const counts = kindCounts();
  return (
    <div class="kindbar">
      {kinds.map((k) => {
        const count = counts[k.name] || 0;
        const grow = Math.max(1, Math.min(20, count));
        const on = assignKind.value !== null && assignKind.value === k.name;
        const dim = assignKind.value !== null && !on;
        return <button class={'kseg ' + (k.kindClass || '') + ' g' + grow + (dim ? ' dim' : '')} data-act="kind-hi" data-k={k.name} aria-pressed={on ? 'true' : 'false'} onClick={() => pressKind(k.name)}><span>{k.name}</span><b>{count}</b></button>;
      })}
    </div>
  );
}

function Pick({ row, team }: { row: AssignRow; team: string }) {
  const kind = optionKind(row);
  return (
    <div class="pick">
      {pickChanged(row) ? <s class="was">{row.saved}</s> : ''}
      <i class={'sw ' + (kind ? 'k-' + kind : row.kindCls || '')}></i>
      <span class="sel">
        <select data-k={row.key} data-act="pick" aria-label={`${row.name} のプロファイル`} onChange={(e: Event) => { setPick(team, row.key, inputValue(e)); tellDirty(); }}>
          {row.missing ? <option selected value={row.profile} data-kind="">{row.profile}</option> : ''}
          {(row.options || []).map((g) => <optgroup label={g.kind}>{(g.names || []).map((name) => <option value={name} data-kind={g.kind} selected={name === row.profile}>{name}</option>)}</optgroup>)}
        </select>
      </span>
    </div>
  );
}

function AssignRowView({ row, team }: { row: AssignRow; team: string }) {
  const dim = assignKind.value !== null && optionKind(row) !== assignKind.value;
  return (
    <div class={'arow' + (row.missing ? ' missing' : '') + (dim ? ' dim' : '') + (pickChanged(row) ? ' changed' : '')} data-k={row.key} data-saved={row.saved || ''}>
      <div class="who">{row.kind === 'm'
        ? <><code>{row.name}</code><span class="role"><code>role {row.role}</code></span></>
        : <><span class="purpose">{row.name}</span><span class="role"><code>from {row.role}</code></span></>}</div>
      <Pick row={row} team={team} />
      {row.missing ? <ul class="vm"><li><span class="sev err">問題</span><span>{row.error}</span></li></ul> : ''}
    </div>
  );
}

function TeamCard({ team }: { team: Team }) {
  const dirty = teamDirty(team);
  const missing = teamMissing(team);
  const rows = team.rows || [];
  return (
    <section class="team" data-g={team.name} data-hash={team.hash} data-place={team.place || 'project'}>
      <header class="th">
        <span class="tl">チーム定義</span><code class="tid">{team.name}</code><span class="pill on">{labelOf(team.place)}</span><span class="tn">{team.members}</span>
        <span class="tside">{team.err > 0 ? <span class="sev err">問題 {team.err}</span> : ''}{team.saved && !dirty ? <span class="ok">保存しました</span> : ''}{dirty > 0 ? <><span class="dirty">{dirty} 人</span><button class="btn primary" data-act="g-save" disabled={missing > 0} onClick={() => pressTeamSave(team.name)}>保存</button></> : ''}</span>
        <code class="path">{team.path}</code>
        {team.place === 'global' ? <span class="tnote" data-role="place-note">グローバルの定義です。直すと、すべてのプロジェクトの同じチーム定義が変わります</span> : ''}
      </header>
      {rows.filter((row) => row.kind === 'm').map((row) => <AssignRowView key={row.key} row={row} team={team.name} />)}
      {team.workers > 0 ? <><div class="sub">ワーカー</div>{rows.filter((row) => row.kind === 'w').map((row) => <AssignRowView key={row.key} row={row} team={team.name} />)}</> : ''}
      {team.refusal ? <span class="sev err refusal">{team.refusal}</span> : ''}
    </section>
  );
}

export function AssignPage() {
  return (
    <>
      <Kindbar />
      <div class="teams">
        {assign.value.empty ? <p class="none">チーム定義はありません</p> : ''}
        {(assign.value.teams || []).map((team) => <TeamCard key={team.name} team={team} />)}
      </div>
    </>
  );
}
