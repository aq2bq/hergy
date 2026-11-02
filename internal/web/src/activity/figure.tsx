import type { JSX } from 'preact';
import { useState } from '../lib/preact';
import { TeamFigure } from '../lib/hq/figure.js';
import type { ActivityTeam } from './state.js';
export function Figure({ team }: { team: ActivityTeam }): JSX.Element {
  const [member, setMember] = useState('human');
  const turns = team.turns.filter(([from, to]) => from === member || to === member);
  const unknown = team.workers.some((w) => !team.crew.some((m) => m.name === w.from));
  return <div class="hq-fig">
    <label class="hq-worker-note">表示するターン <select aria-label={`${team.name} の表示するターン`} value={member} onChange={(e) => setMember(e.currentTarget.value)}><option value="human">人間</option>{team.crew.map((m) => <option value={m.name}>{m.name}</option>)}</select></label>
    <TeamFigure team={team} turns={turns} W={440} layout="packed" />
    {unknown ? <p class="hq-worker-note">ワーカー（雇ったメンバーを確認できない）：起動記録では、ワーカーを雇ったメンバーを確認できません。ワーカーのペインで確認してください。</p> : null}
  </div>;
}
