// The band reads HQ's server readings. A screen may request a refresh, but it
// cannot supply a replacement count or write the DOM primitives. There is no
// second cache of counts; listedrows.go decides rows on the server.
import { st } from './hq/state.js';
const scr = (name: string): HTMLElement | null => document.querySelector<HTMLElement>('.screens .scr[data-screen="' + name + '"]');

// setCount is how a page keeps the band's number the same as its own when a
// profile or a role is added or removed (1.1).
function setCount(screen: string, n: number): void {
  const el = scr(screen);
  const num = el?.querySelector('.n');
  if (num) num.textContent = String(n);
}

// setActivity writes the activity's two numbers (spec-activity 2.1): アクティビティ N
// while this project has a team running, and 待ち N while a member waits for
// a human or a permission, which is dropped at zero. counts is false while
// herdr cannot be read or none is running, and then アクティビティ stands alone (2.7).
// The screen calls this on every re-reading, so the band's two numbers are
// the reading's own and not the page's first ones.
function setActivity(running: number, waiting: number, counts: boolean): void {
  const el = scr('activity');
  if (!el) return;
  let num = el.querySelector('.n');
  if (counts && !num) {
    num = document.createElement('span');
    num.className = 'n';
    el.appendChild(num);
  }
  if (!counts && num) {
    num.remove();
    num = null;
  }
  if (num) num.textContent = String(running);
  let wait = el.querySelector('.wait');
  if (!wait && waiting > 0) {
    wait = document.createElement('span');
    wait.className = 'wait';
    if (num) num.after(wait);
    else el.appendChild(wait);
  }
  if (!wait) return;
  if (waiting > 0) wait.textContent = '待ち ' + waiting;
  else wait.remove();
}

// ---- 帯 ---------------------------------------------------------------
// bandActivity keeps the band's two numbers the reading's own: アクティビティ N（動いている
// チーム）と 待ち N（人間を待っているメンバー）(spec 2 章, spec-activity 2.1).
export function bandActivity() {
  const teams = st.running.value.teams || [];
  const waiting = teams.reduce((n, t) => n + (t.waiting || []).length, 0);
  setActivity(teams.length, waiting, !!st.running.value.herdr.ok && teams.length > 0);
}

// bandCounts writes the projects reading's totals into the band, so the four
// numbers are the listed projects' and the global layer's (spec 2 章). The server
// writes them too; this keeps them right after a project is listed or removed
// without a reload.
export function bandCounts() {
  const p = st.projects.value;
  if (!p) return;
  const totals = { teams: 0, roles: 0, profiles: 0 };
  const add = (one: { definitions?: number; roles?: number; profiles?: number } | null | undefined) => {
    if (!one) return;
    totals.teams += one.definitions || 0;
    totals.roles += one.roles || 0;
    totals.profiles += one.profiles || 0;
  };
  if (p.mode === 'listed') {
    ((p.projects || []).filter((x) => x.listed)).forEach(add);
    add(p.global);
  } else {
    // One project and the global layer: the same rows the server's own band
    // counts (r-ea17).
    add(p.current);
    add(p.global);
  }
  setCount('teams', totals.teams);
  setCount('roles', totals.roles);
  setCount('profiles', totals.profiles);
}

