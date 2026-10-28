// page.js lays out what a screen's page holds before its module is imported —
// the mount points, the embedded data — and answers the screen's requests from
// the test, so a test says when each answer comes and nothing waits on a clock.

// later is a promise the test settles by hand.
export function later() {
  let settle;
  const promise = new Promise((resolve, reject) => { settle = { resolve, reject }; });
  return { promise, ...settle };
}

// layout writes the page's body: the screen's mount points and its data, the
// way the server's template writes them.
export function layout(dataID, data) {
  document.body.innerHTML = '<div class="wrap"><div class="bar"></div><div id="tabs"></div><div id="view"></div></div>';
  const script = document.createElement('script');
  script.type = 'application/json';
  script.id = dataID;
  script.textContent = JSON.stringify(data);
  document.body.appendChild(script);
}

// band is what every page's band reads from the headquarters when the test
// does not say otherwise: no project listed, herdr answering, nothing running.
const band = {
  '/api/hq/projects': { mode: 'none', projects: [] },
  '/api/hq/running': { herdr: { ok: true }, teams: [] },
  '/api/hq/workspaces': { workspaces: [] },
};

// serve answers fetch from answer(method, url, body), which returns the JSON
// (or a promise of it); what it leaves undefined, the band's reads answer.
// Every request is kept in the list it returns.
export function serve(answer) {
  const asked = [];
  globalThis.fetch = async (url, opts = {}) => {
    const body = opts.body ? JSON.parse(opts.body) : undefined;
    asked.push({ method: opts.method || 'GET', url: String(url), body });
    let json = await answer(opts.method || 'GET', String(url), body);
    if (json === undefined) json = band[String(url).split('?')[0]];
    const text = JSON.stringify(json === undefined ? {} : json);
    return { ok: true, status: 200, text: async () => text, headers: { get: () => 'application/json' } };
  };
  return asked;
}

// settle lets the answers already given reach the screen: the module's own
// awaits run before the test reads the page.
export async function settle() {
  for (let i = 0; i < 10; i++) await Promise.resolve();
  await new Promise((resolve) => setTimeout(resolve, 0));
}

// type puts a value in a field the way a person's typing does.
export function type(el, value) {
  el.value = value;
  el.dispatchEvent(new Event('input', { bubbles: true }));
}
