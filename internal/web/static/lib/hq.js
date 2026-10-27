// lib/hq.js — 本部の帯と欄（n-7f3c、モデルは d-73fc、画面は d-8a45）。module の入口。
//
// この 1 本が持つもの:
//   - 帯の「プロジェクト N」と、その欄（並べる・外す・候補・パスで並べる）
//   - 帯の「探す」（/ キー）。並べたプロジェクトとグローバルのチーム・
//     ワークスペース・メンバー・ロール・規則・プロファイルを一度に探す
//   - チーム定義の一覧の「動き」の欄（起動…・動いている・止める）と、
//     起動・起動している・止めるの欄
//   - 動いているチームの読み。一覧と活動は、この読みから同じ状態を出す
//
// データはすべて本部の API（/api/hq/…）から読む。文はこの画面が持ち、状態の名
// （answer、blocked、…）はサーバのものをそのまま使う。
//
// 中身は 4 つに分かれている: lib/hq/state.js（読みと状態）、lib/hq/figure.js
// （チームの図の部品）、lib/hq/run.js（一覧の「動き」の欄）、lib/hq/sheets.js
// （欄と検索）。classic の画面は今までどおり window.hergyHQ を読む（4 章）。
import {
  st, POLL, rev, esc, token, parseOpen, projectOfGroup, groupKey, groupLabel, groupDisplay,
  projectName, runningFor, runningTeam, lastWorkspaceOf, definitionOf, groupOf, itemGroup,
  readAll, readCore, readGroups, readRunning, startReads,
} from './hq/state.js';
import { RunCell } from './hq/run.js';
import {
  closeSheet, projectsSheet, startSheet, stopSheet, mountSearch, renderResults, moveSel,
  resetSearch, searchHook, sheetOpen, searchBoxOpen, closeBox, searchQuery,
} from './hq/sheets.js';
import { TeamFigure } from './hq/figure.js';

// ---- 押す（画面から出るもの） -------------------------------------------
// The list's 起動… and 止める, and the band's プロジェクト N, are the screens'
// own markup; the sheets' own presses are the sheets' (each one draws its own
// buttons). The parts that stand inside a sheet are not answered here.
document.addEventListener('click', async (e) => {
  const t = e.target.closest('[data-hq]');
  if (!t) return;
  const act = t.dataset.hq;
  if (act === 'projects') { e.preventDefault(); projectsSheet(); return; }
  if (act === 'start') { e.preventDefault(); startSheet(t.dataset.def, t.dataset.place, t.dataset.project); return; }
  if (act === 'stop') { e.preventDefault(); stopSheet(t.dataset.team); return; }
  if (act === 'close') { e.preventDefault(); closeSheet(); }
});

// The sheet closes when a person presses outside it; the search's box closes the
// same way (spec 3 章).
document.addEventListener('click', (e) => {
  if (sheetOpen() && !e.target.closest('.hq-sheet') && !e.target.closest('[data-hq]')) closeSheet();
  if (searchBoxOpen() && !e.target.closest('.hq-sr, .hq-search')) closeBox();
});

// The search's keys (spec 3 章): "/" from anywhere outside a field, Esc closes,
// the arrows walk the rows, Enter goes to the chosen one.
document.addEventListener('keydown', (e) => {
  if (e.key === 'Escape' && sheetOpen()) { closeSheet(); return; }
  if (e.key === 'Escape' && searchBoxOpen()) { resetSearch(); return; }
  if (e.key === '/' && !e.target.closest('input, textarea, select, [contenteditable]')) {
    const q = document.getElementById('hq-q');
    if (q) { e.preventDefault(); q.focus(); }
    return;
  }
  if (!searchBoxOpen() || !e.target.closest('.hq-search')) return;
  if (e.key === 'ArrowDown' || e.key === 'ArrowUp') { e.preventDefault(); moveSel(e.key === 'ArrowDown' ? 1 : -1); }
  if (e.key === 'Enter') {
    const pick = searchHook().shown[searchHook().sel];
    if (pick) location.href = pick;
  }
});

// A test hook, the way the other screens carry one: what the search has on show,
// so a walk of the screens can tell a key that did not land from one that did.
window.hergyHQSearch = searchHook;

// ---- classic の画面への橋（4 章） ---------------------------------------
window.hergyHQ = {
  state: st,
  POLL,
  token,
  parseOpen,
  projectOfGroup,
  groupKey,
  groupLabel,
  groupDisplay,
  projectName,
  runningFor,
  runningTeam,
  lastWorkspaceOf,
  definitionOf,
  groupOf,
  itemGroup,
  readAll,
  readCore,
  readGroups,
  readRunning,
  closeSheet,
  startSheet,
  stopSheet,
  esc,
};

// ---- 立ち上げ -----------------------------------------------------------
// The page's band is written by the server; the reading keeps its numbers right
// (spec 2 章). The first reading runs now, the groups when a search is opened
// (the activity screen needs them for the figures' turns), and the ten-second one
// while the page is looked at.
readCore().then(() => {
  // A search the person had already typed is drawn again once the groups are
  // read: the text is the part's state, not the field's (5 章 2).
  const typed = searchQuery();
  if (typed) readGroups().then(() => renderResults(typed));
  if (location.pathname.indexOf('/activity') === 0) readGroups();
});
startReads();

document.addEventListener('DOMContentLoaded', () => {
  mountSearch();
  stickBar();
});
window.addEventListener('resize', stickBar);
function stickBar() {
  const bar = document.querySelector('.bar');
  if (bar) document.documentElement.style.setProperty('--stick', bar.offsetHeight + 'px');
}

// ---- module の画面への口（4 章） ----------------------------------------
// The screens that are modules import what they read instead of going through
// window.hergyHQ.
export {
  st as state, rev, token, parseOpen, projectName, runningTeam, definitionOf, readGroups,
  readRunning, readCore, readAll, stopSheet, startSheet, closeSheet, RunCell, TeamFigure,
};
