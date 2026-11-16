import { term } from '../lib/terms.generated.js';
import type { JSX } from 'preact';
import { TeamFigure } from '../lib/hq/figure.js';
import type { ActivityTeam } from './state.js';
export function Figure({ team }: { team: ActivityTeam }): JSX.Element {
  const unknown = team.workers.some((w) => !team.crew.some((m) => m.name === w.from));
  return <div class="hq-fig">
    <TeamFigure team={team} turns={team.turns} W={440} layout="packed" />
    {unknown ? <p class="hq-worker-note">{`雇った${term('member')}を確認できない${term('worker')}がいます。`}</p> : null}
  </div>;
}
