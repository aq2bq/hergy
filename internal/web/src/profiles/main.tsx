// profiles/main.tsx — プロファイルの画面の入口（webui-preact-design 4 章）。
//
// mount と、住所・帯への受け渡しだけを持つ。描くのは view.tsx / assign.tsx /
// yaml.tsx、状態は state.ts、押し込みは部品が自分の onClick / onInput から
// actions.ts の関数を直接呼ぶ。ここも DOM を書かない（5 章 1）。
import { render } from '../lib/preact.js';
import { takeDraft } from '../lib/nav.js';
import {
  view, TAB, rows, dirty, take, tellDirty, restore, syncAssignURL, initAssign, initYaml,
} from './state.js';
import { initPanel } from './actions.js';
import { List } from './view.js';
import { AssignPage } from './assign.js';
import { YamlPage } from './yaml.js';

function App() {
  if (TAB === 'assign') return <AssignPage />;
  if (TAB === 'yaml') return <YamlPage />;
  return <List />;
}

// The page says what it is through window.hergyPage (nav.js reads it, 5 章 7).
// settled is true once the first reading is drawn: this screen's reading comes
// with the page, so it is true at once and stays true.
const page = { screen: 'profiles', dirty, take, tellDirty, settled: true };
Object.assign(window, { hergyPage: page });

// ---- mounting -----------------------------------------------------------
// The server draws the page as it always did; the screen draws it again from
// its own state, so the words and the shapes stand and the state is the one the
// presses move (4 章).
view.replaceChildren();
render(<App />, view);

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
function flashFromHash(): void {
  const id = location.hash.slice(1);
  const dot = id.lastIndexOf('.');
  if (dot < 0) return;
  const team = id.slice(0, dot);
  const key = id.slice(dot + 1);
  requestAnimationFrame(() => {
    const row = view.querySelector(`.team[data-g="${CSS.escape(team)}"] .arow[data-k="${CSS.escape(key)}"]`);
    if (!(row instanceof HTMLElement)) return;
    row.scrollIntoView({ block: 'center' });
    row.classList.add('flash');
    const select = row.querySelector('select');
    if (select instanceof HTMLElement) select.focus({ preventScroll: true });
  });
}

// The band's promise: what one screen left unsaved is handed to this one, and
// dropped whether or not it could be put back (spec-nav 1.5). It is put back
// after the tab is set up, so what was typed is not read as the file's own text.
const carried = takeDraft('profiles');
if (carried) restore(carried);
tellDirty();
