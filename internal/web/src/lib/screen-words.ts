// Shared sentences stay together so every screen uses the same vocabulary.
import { term } from './terms.generated.js';

export function globalDefinitionNotice(): string {
  return `${term('global')}の${term('team-definition')}です。直すと、すべての${term('project')}の同じ${term('team-definition')}が変わります`;
}

export function globalPlaceNotice(): string {
  return `どの${term('project')}からも使える。同じ名前があれば、その${term('project')}のほうが使われる`;
}

export function noTeamDefinitions(): string {
  return `${term('team-definition')}はありません`;
}

export function launchLabel(place: string): string {
  return place === 'global' ? `${term('project')}を選んで${term('start')}する` : `この${term('project')}で${term('start')}する`;
}

export function launchWaitingLabel(): string {
  return `${term('start')}の途中 · ${term('waiting-for-permission')}`;
}

export function stopTeamLabel(): string {
  return `${term('team')}を${term('down')}…`;
}

export function turnRecipientLabel(): string {
  return `${term('hands-the-turn-to')}相手`;
}
