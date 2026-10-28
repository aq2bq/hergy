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
// （欄と探す欄）。画面はここから import して読む。
import {
  st, setRunning, token, parseOpen, projectName, runningTeam, definitionOf,
  readAll, readCore, readGroups, readRunning, startReads,
} from './hq/state.js';
import { RunCell } from './hq/run.js';
import {
  closeSheet, projectsSheet, startSheet, stopSheet, mountSearch, moveSel,
  resetSearch, searchHook, sheetOpen, searchBoxOpen, closeBox,
} from './hq/sheets.js';
import { TeamFigure } from './hq/figure.js';
import { raiseMember } from './hq/raise.js';

// ---- 押す（画面から出るもの） -------------------------------------------
// The lists' 起動… and 止める are the parts' own presses now (lib/hq/run.js,
// activity/board.js). The band's プロジェクト N is the server's markup, written
// once, so this part answers that one button where it stands.
document.querySelectorAll('[data-hq="projects"]').forEach((b) => {
  b.addEventListener('click', (e) => { e.preventDefault(); projectsSheet(); });
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

// ---- 立ち上げ -----------------------------------------------------------
// The page's band is written by the server; the reading keeps its numbers right
// (spec 2 章). The first reading runs now, the groups when a search is opened
// (the activity screen needs them for the figures' turns), and the ten-second one
// while the page is looked at.
readCore().then(() => {
  // The groups a search reads are read when a search is opened (sheets.js's
  // pressSearch); the activity screen needs them for the figures' turns.
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

// ---- the screens' way in ------------------------------------------------
// A screen imports what it reads: nothing of the headquarters goes through a
// global any more.
export {
  st as state, setRunning, token, parseOpen, projectName, runningTeam, definitionOf, readGroups,
  readRunning, readCore, readAll, stopSheet, startSheet, closeSheet, RunCell, TeamFigure, raiseMember,
};
