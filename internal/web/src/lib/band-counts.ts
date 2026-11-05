import { setCount, setActivity } from './nav.js';

// The band's numbers are written here and nowhere else (r-ea17, d-4d88). A
// screen hands in the numbers a reading gave it; it never counts rows or teams
// itself.
export type BandCounts = { teams: number; roles: number; profiles: number };

export function applyCounts(counts: BandCounts): void {
  setCount('teams', counts.teams);
  setCount('roles', counts.roles);
  setCount('profiles', counts.profiles);
}

export function applyActivity(running: number, waiting: number, counts: boolean): void {
  setActivity(running, waiting, counts);
}
