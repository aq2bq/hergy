// The band's behaviour (spec-nav.md 1.4, 1.5). It remembers where a person last
// stood in each screen and carries the changes of the screen they are leaving
// that were not saved yet. The page says what it is through window.hergyPage;
// nothing here talks to the server, and a page whose own script did not run
// still walks the band's links.
//
// 帯そのもの（`.screens` の span / a、数、`.wait`）はサーバーが描いたままで、
// ここはその上の印と数だけを書く（要素は作り直さない。webui-preact-design 5 章）。
// 画面は setCount / setActivity / takeDraft を import で読む。
const band = document.querySelector('.screens');
const key = (band && band.dataset.key) || '';

// The page's own script runs after this one (both are deferred, and the page
// is read second), so what the page is is read when it is needed, not here.
const page = () => window.hergyPage || null;
const mine = () => { const p = page(); return p && p.screen ? p.screen : ''; };

// sessionStorage holds one place and one draft per screen, under this
// project's own key, so two projects on one machine do not mix. A browser
// that refuses to hold them still walks the links.
const at = (what, screen) => 'hergy.' + key + '.' + what + '.' + screen;
const read = (k) => { try { return window.sessionStorage.getItem(k); } catch (e) { return null; } };
const write = (k, v) => { try { window.sessionStorage.setItem(k, v); } catch (e) { /* the band still links */ } };
const drop = (k) => { try { window.sessionStorage.removeItem(k); } catch (e) { /* as above */ } };

const scr = (name) => (band ? band.querySelector('.scr[data-screen="' + name + '"]') : null);
const setDot = (el, on) => {
  if (!el) return;
  const dot = el.querySelector('.dot');
  if (on && !dot) {
    const i = document.createElement('i');
    i.className = 'dot';
    el.appendChild(i);
  }
  if (!on && dot) dot.remove();
};

// ---- 居た所 (1.4) ----------------------------------------------------
function remember() {
  const screen = mine();
  if (!screen) return;
  write(at('at', screen), location.pathname + location.search);
}

// decorate points the band's links at the place each screen was left, and
// shows the dot of a screen whose changes were carried over. The screen the
// person is on is a span, not a link, so it is not among these.
function decorate() {
  if (!band) return;
  band.querySelectorAll('a.scr[data-screen]').forEach((a) => {
    const name = a.dataset.screen;
    const was = read(at('at', name));
    if (was && was.startsWith('/' + name)) a.setAttribute('href', was);
    setDot(a, !!read(at('draft', name)));
  });
}

// ---- 画面へ渡すもの -------------------------------------------------------
// takeDraft hands one screen's unsaved changes over when the band walked
// here, once, and forgets them. It answers null when the band did not walk
// here, or when the screen held nothing. The draft is dropped either way:
// one this page did not take is one nobody may put back (R2).
export function takeDraft(screen) {
  const raw = read(at('draft', screen));
  const carried = !!read(at('carry', screen));
  drop(at('carry', screen));
  drop(at('draft', screen));
  if (!raw || !carried) return null;
  try { return JSON.parse(raw); } catch (e) { return null; }
}

// setCount is how a page keeps the band's number the same as its own when a
// profile or a role is added or removed (1.1).
export function setCount(screen, n) {
  const el = scr(screen);
  const num = el && el.querySelector('.n');
  if (num) num.textContent = String(n);
}

// setActivity writes the activity's two numbers (spec-activity 2.1): 活動 N
// while this project has a team running, and 待ち N while a member waits for
// a human or a permission, which is dropped at zero. counts is false while
// herdr cannot be read or none is running, and then 活動 stands alone (2.7).
// The screen calls this on every re-reading, so the band's two numbers are
// the reading's own and not the page's first ones.
export function setActivity(running, waiting, counts) {
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

if (band) {
  // The page moves the address bar (a filter, an opened row, a chosen file)
  // through these two calls: wrapping them is what keeps the place right
  // without the pages doing anything for it.
  const pushState = history.pushState.bind(history);
  const replaceState = history.replaceState.bind(history);
  history.pushState = function (...args) { pushState(...args); remember(); };
  history.replaceState = function (...args) { replaceState(...args); remember(); };

  // ---- 保存前の変更 (1.5) ----------------------------------------------
  // Only the band's own walk carries the changes: pressing a link here writes
  // the draft of the screen being left and marks the screen being entered, and
  // only that arrival reads it back. A reload, the back button, a typed URL and
  // hergy go without them — and a draft they left behind is dropped, so it
  // cannot come back later (R2). The mark is written even from a page that is
  // no screen (a 404), so a draft waiting for the screen it points at comes
  // back the same way from there.
  band.querySelectorAll('a.scr').forEach((a) => {
    a.addEventListener('click', () => {
      write(at('carry', a.dataset.screen), '1');
      const screen = mine();
      if (!screen) return;
      const p = page();
      if (p && p.dirty && p.dirty()) write(at('draft', screen), JSON.stringify(p.take()));
      else drop(at('draft', screen));
    });
  });

  document.addEventListener('hergy:dirty', (e) => setDot(scr(mine()), !!e.detail));

  // Both scripts have run by the time the document is parsed, so the page
  // knows what it is: remember the place, and point the links.
  if (document.readyState === 'loading') document.addEventListener('DOMContentLoaded', () => { remember(); decorate(); });
  else { remember(); decorate(); }
}
