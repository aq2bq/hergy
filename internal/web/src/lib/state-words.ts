// lib/state-words.ts — the one table from a state's API name to the word a
// person reads. A member's state and a team's state are different concepts
// with different words: a member at rest is idle, a team nobody moves is stalled.
import { term } from './terms.generated.js';

export const memberStateWord: Readonly<Record<string, string>> = {
  active: term('working'), idle: term('idle'), blocked: term('waiting-for-permission'),
  handed: term('handed'), absent: term('not-running'),
};

export const teamStateWord: Readonly<Record<string, string>> = {
  answer: term('waiting-for-permission'), turn: term('handed'), stalled: term('stalled'), working: term('working'),
};
