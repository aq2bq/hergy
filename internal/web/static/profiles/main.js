// profiles/main.js — プロファイルの画面の入口（webui-preact-design 4 章）。
//
// mount と、押し込み・住所・帯への約束だけを持つ。描くのは view.js / assign.js /
// yaml.js、状態は state.js、押し込みの中身は actions.js。ここも DOM を書かない
// （5 章 1）。
import { html, render } from '../vendor/preact.js';
import { insertTab } from '../lib/yamltab.js';
import { takeDraft } from '../lib/nav.js';
import {
  view, TAB, rows, assignKind, dirty, take, tellDirty, restore,
  syncAssignURL, initAssign, initYaml, setPick, panelByKey, saveTeam, patchPanel, pathFor,
} from './state.js';
import { initPanel, scheduleCheck } from './actions.js';
import * as actions from './actions.js';
import { List } from './view.js';
import { AssignPage } from './assign.js';
import { YamlPage } from './yaml.js';

function App() {
  if (TAB === 'assign') return html`<${AssignPage} />`;
  if (TAB === 'yaml') return html`<${YamlPage} />`;
  return html`<${List} />`;
}

// The page says what it is through window.hergyPage (nav.js reads it, 5 章 7).
// settled is true once the first reading is drawn: this screen's reading comes
// with the page, so it is true at once and stays true.
const page = { screen: 'profiles', dirty, take, tellDirty, settled: true };
window.hergyPage = page;

// The presses on the page land here, once: the same delegation the screen always
// had, with the work handed to actions.js (so no part writes another part's DOM).
view.addEventListener('click', (event) => {
  const button = event.target.closest('[data-act]');
  if (!button) return;
  const act = button.dataset.act;
  if (act === 'goto') return; // the link itself moves the page
  const panel = button.closest('.panel');
  const key = panel ? panel.dataset.key : '';
  const row = button.closest('.row');
  const team = button.closest('.team');
  const section = button.closest('.yed');
  event.preventDefault();

  if (act === 'toggle' && row && button.classList.contains('sum')) { actions.pressToggle(row.dataset.key); return; }
  if (act === 'kind-filter') { actions.pressFilter(button.closest('.fchip').dataset.k || ''); return; }
  if (act === 'new') { actions.pressNew(); return; }
  if (act === 'set-kind' && panel) { actions.pressSetKind(key, button.dataset.k); return; }
  if (act === 'arg-add' && panel) { actions.pressArgAdd(key); return; }
  if (act === 'arg-del' && panel) { actions.pressArgDel(key, Number(button.dataset.i)); return; }
  if (act === 'env-add' && panel) { actions.pressEnvAdd(key); return; }
  if (act === 'env-del' && panel) { actions.pressEnvDel(key, Number(button.dataset.i)); return; }
  if (act === 'save' && panel) { actions.pressSave(key); return; }
  if (act === 'dup' && panel) { actions.pressDup(key); return; }
  if (act === 'copy' && panel) { actions.pressCopy(key); return; }
  if (act === 'discard' && panel) { actions.pressDiscard(key); return; }
  if (act === 'del' && panel) { actions.pressDel(key); return; }
  if (act === 'del-no' && panel) { actions.pressDelNo(key); return; }
  if (act === 'del-yes' && panel) { actions.pressDelYes(key); return; }
  if (act === 'pick-layer' && panel) { pressLayer(key, button.dataset.x); return; }
  if (act === 'kind-hi') { pressKind(button.dataset.k); return; }
  if (act === 'g-save' && team) { pressTeamSave(team.dataset.g); return; }
  if (act === 'yaml-jump' && section) { window.hergyYaml.jump(section, Number(button.dataset.line)); return; }
  if (act === 'yaml-revert' && section) { window.hergyYaml.revert(section); return; }
  if (act === 'yaml-save' && section) { window.hergyYaml.save(section); return; }
});

// pressLayer moves a row being added between the project's file and the
// global one: the path, the segment and the pending pill follow.
function pressLayer(key, file) {
  const panel = panelByKey(key);
  if (!panel || panel.file === file) return;
  patchPanel(key, { file, path: pathFor(file, panel.group) });
  scheduleCheck(key);
  tellDirty();
}

// pressKind is the assign tab's kind: the other kinds and people go thin, the
// chosen one stays plain, and the address keeps it (R6).
function pressKind(kind) {
  assignKind.value = assignKind.value === kind ? null : kind;
  syncAssignURL();
}

function pressTeamSave(name) {
  return saveTeam(name);
}

view.addEventListener('input', (event) => {
  const target = event.target;
  if (target.id === 'q') { actions.pressQuery(target.value); return; }
  if (!target.dataset.f) return;
  const panel = target.closest('.panel');
  if (!panel) return;
  if (target.dataset.f === 'rename') return;
  actions.inputField(panel.dataset.key, target.dataset.f, Number(target.dataset.i || 0), target.value);
});

view.addEventListener('change', (event) => {
  const target = event.target;
  if (target.dataset.f === 'kind-other') {
    const panel = target.closest('.panel');
    if (panel) actions.pressKindOther(panel.dataset.key, target.value);
    return;
  }
  if (target.dataset.f === 'rename') {
    const panel = target.closest('.panel');
    if (panel) actions.pressRename(panel.dataset.key, target.checked);
    return;
  }
  if (target.matches('.arow select')) {
    const row = target.closest('.arow');
    const team = target.closest('.team');
    if (row && team) setPick(team.dataset.g, row.dataset.k, target.value);
  }
});

view.addEventListener('keydown', (event) => {
  const target = event.target;
  if (target.classList && target.classList.contains('sum') && (event.key === 'Enter' || event.key === ' ')) {
    event.preventDefault();
    actions.pressToggle(target.closest('.row').dataset.key);
    return;
  }
  if (target.classList && target.classList.contains('ytext') && event.key === 'Tab' && !event.shiftKey) {
    event.preventDefault();
    insertTab(target);
  }
});

// A dragged argument's grip moves the whole chip; the chips' order is the order
// the save writes (spec 4 章).
view.addEventListener('pointerdown', (event) => {
  const grip = event.target.closest('.grip');
  if (grip) grip.closest('.arg').setAttribute('draggable', 'true');
});
view.addEventListener('dragstart', (event) => {
  const arg = event.target.closest('.arg');
  if (arg) arg.classList.add('dragging');
});
view.addEventListener('dragover', (event) => {
  if (!event.target.closest('.arg')) return;
  event.preventDefault();
});
view.addEventListener('drop', (event) => {
  const target = event.target.closest('.arg');
  const source = view.querySelector('.dragging');
  if (!target || !source || target === source) return;
  event.preventDefault();
  const rect = target.getBoundingClientRect();
  const list = source.closest('.args');
  if (event.clientX < rect.left + rect.width / 2) list.insertBefore(source, target); else list.insertBefore(source, target.nextSibling);
  source.classList.remove('dragging');
  source.removeAttribute('draggable');
  const panel = source.closest('.panel');
  if (!panel) return;
  const order = Array.from(list.querySelectorAll('.arg')).map((el) => el.querySelector('input').value);
  actions.reorderArgs(panel.dataset.key, order);
});
view.addEventListener('dragend', (event) => {
  const arg = event.target.closest('.arg');
  if (arg) { arg.classList.remove('dragging'); arg.removeAttribute('draggable'); }
});

view.addEventListener('scroll', (event) => {
  if (event.target.classList && event.target.classList.contains('ytext')) {
    const section = event.target.closest('.yed');
    const gutter = section && section.querySelector('.gutter');
    if (gutter) gutter.scrollTop = event.target.scrollTop;
  }
}, true);

// ---- mounting -----------------------------------------------------------
// The server draws the page as it always did; the screen draws it again from
// its own state, so the words and the shapes stand and the state is the one the
// presses move (4 章).
view.replaceChildren();
render(html`<${App} />`, view);

if (TAB === 'assign') {
  initAssign();
  syncAssignURL();
  if (location.hash) flashFromHash();
} else if (TAB === 'yaml') {
  initYaml();
} else {
  // Every row the address opens asks what is wrong with its draft, as it did.
  rows.value.filter((row) => row.open).forEach((row) => row.panels.forEach((panel) => initPanel(panel.key)));
}

// flashFromHash puts the cursor on the person a chip named (P11).
function flashFromHash() {
  const id = location.hash.slice(1);
  const dot = id.lastIndexOf('.');
  if (dot < 0) return;
  const team = id.slice(0, dot);
  const key = id.slice(dot + 1);
  requestAnimationFrame(() => {
    const row = view.querySelector(`.team[data-g="${CSS.escape(team)}"] .arow[data-k="${CSS.escape(key)}"]`);
    if (!row) return;
    row.scrollIntoView({ block: 'center' });
    row.classList.add('flash');
    const select = row.querySelector('select');
    if (select) select.focus({ preventScroll: true });
  });
}

// The band's promise: what one screen left unsaved is handed to this one, and
// dropped whether or not it could be put back (spec-nav 1.5). It is put back
// after the tab is set up, so what was typed is not read as the file's own text.
const carried = takeDraft('profiles');
if (carried) restore(carried);
tellDirty();

// A change a person makes, and a save's answer, both move the dot.
document.addEventListener('input', tellDirty);
document.addEventListener('change', tellDirty);
document.addEventListener('click', tellDirty);
