// lib/api.js — the one way a screen talks to the server (spec-boundary 7, r-2cc2).
//
// Every kind of thing a screen reads and writes answers under /api/ in one shape
// (spec-boundary 5): a list, one document, a check, a save, a delete. This file
// holds the call, the token of this run, the shape each answer is read as, and
// the refusal a write can meet. A screen keeps its own look and draws a refusal
// as its own state — nothing here writes the page (webui-preact-design 5 章 1).
//
// The classic screens still read this through window.hergyAPI, which the file
// also writes while they are being moved (4 章). Their `call` answers
// {ok, status, json, text}; the module's own `call` throws Refusal instead, so a
// screen can hold the refusal as a resource's error and draw it (5 章 2・3).
// The page carries this run's token wherever it writes; the activity screen
// holds it on its list, the others on #view (r-a8fa).
export const token = (document.querySelector('[data-token]') || {}).dataset?.token || '';
export const $ = (sel, root = document) => root.querySelector(sel);
export const $$ = (sel, root = document) => Array.from(root.querySelectorAll(sel));
const enc = (s) => encodeURIComponent(String(s == null ? '' : s));

// Refusal is the server saying it will not take this: the status it answered,
// the sentence it wrote (its text body, or json.text), and the answer behind it.
// A screen draws the sentence; it builds none of its own (spec-boundary 3).
export class Refusal extends Error {
  constructor(status, text, json) {
    super(text || `HTTP ${status}`);
    this.name = 'Refusal';
    this.status = status;
    this.text = text;
    this.json = json;
  }
}

// raw sends one request with the token of this run and answers what came back,
// whatever the status. A write carries its body as JSON, and the token rides in
// the header, so POST, PUT and DELETE are one shape (a DELETE carries no body at
// all). Nothing here decides what a screen does with a non-OK answer.
export async function raw(method, url, payload, { signal: stop } = {}) {
  const opts = { method, credentials: 'same-origin', headers: { Accept: 'application/json' }, signal: stop };
  if (token) opts.headers['X-Hergy-Token'] = token;
  if (payload !== undefined) {
    opts.headers['Content-Type'] = 'application/json';
    opts.body = JSON.stringify(payload);
  }
  const response = await fetch(url, opts);
  const text = await response.text();
  let json = null;
  if ((response.headers.get('content-type') || '').includes('application/json')) {
    try { json = JSON.parse(text); } catch (e) { json = null; }
  }
  return { ok: response.ok, status: response.status, json, text };
}

// call is one request whose refusal is an error: {status, json, text} when the
// server took it, a Refusal (with the sentence in it) when it did not.
export async function call(method, url, payload, opts) {
  const a = await raw(method, url, payload, opts);
  if (!a.ok) throw new Refusal(a.status, (a.json && a.json.text) || a.text, a.json);
  return a;
}

// ---- 書き込みと読み込みの形（spec-boundary 5）------------------------------
// One place writes the addresses, so a screen asks for what it wants and not for
// an address. Each takes the request's AbortSignal last, so a resource can drop
// a read that is no longer the screen's (webui-preact-design 5 章 3).
const urls = {
  list: (kind) => `/api/${kind}`,
  // get is GET /api/{kind}/{name}?place=&project=: one document. project names
  // the listed project it stands in (d-73fc), empty for this one.
  get: (kind, name, place, project) => `/api/${kind}/${enc(name)}?${place ? `place=${enc(place)}&` : ''}${project ? `project=${enc(project)}` : ''}`,
  // del is DELETE /api/{kind}/{name}?place=&hash=&agreement_hash=&project=: take
  // one document away. The team kind takes a whole folder, so it carries the
  // second file's hash too (spec-teams B6).
  del: (kind, name, place, hash, agreementHash, project) => `/api/${kind}/${enc(name)}?place=${enc(place)}&hash=${enc(hash)}${agreementHash ? `&agreement_hash=${enc(agreementHash)}` : ''}${project ? `&project=${enc(project)}` : ''}`,
};

// read is a GET whose json is the answer: a list or one document.
export async function read(url, opts) {
  const a = await call('GET', url, undefined, opts);
  return a.json;
}
export const list = (kind, opts) => read(urls.list(kind), opts);
export const get = (kind, name, place, project, opts) => read(urls.get(kind, name, place, project), opts);
// check is POST /api/{kind}/check: what is wrong with a draft or a text.
export const check = (kind, payload, opts) => call('POST', `/api/${kind}/check`, payload, opts);
// save is PUT /api/{kind}/{name}: write one document.
export const save = (kind, name, payload, opts) => call('PUT', `/api/${kind}/${enc(name)}`, payload, opts);
export const del = (kind, name, place, hash, agreementHash, project, opts) => call('DELETE', urls.del(kind, name, place, hash, agreementHash, project), undefined, opts);

// ---- 変更を伝える (r-a8fa) -------------------------------------------------
// delivery is GET /api/delivery: the running members whose prompt is not the one
// hergy last handed them, by running team.
export const delivery = (opts) => read('/api/delivery', opts);
// deliveryCount is GET /api/delivery/count: the band's number alone.
export const deliveryCount = (opts) => read('/api/delivery/count', opts);
// deliver is POST /api/delivery: hand the current prompt to the named herdr
// members. A refusal comes back 422 with the one sentence in text.
export const deliver = (members, opts) => call('POST', '/api/delivery', { members }, opts);
// saved says one save wrote. Every screen's successful save says it, and the
// band reads its number again (deliver.js).
export const saved = () => document.dispatchEvent(new CustomEvent('hergy:saved'));

// problemText is the sentence one refusal carries, or the fallback word. The
// refusal is what call threw, and a screen holds it as a resource's error.
export const problemText = (refusal, fallback) => (refusal && (refusal.text || (refusal.json && refusal.json.text))) || fallback;

// ---- reading an answer's shape (spec-boundary 5) ---------------------
// One place reads what the server answers; a screen asks for its kind's model
// and draws that. No screen digs into the fields of an answer itself.

const PLACES = ['project', 'global'];
const PLACE_LABEL = { project: 'プロジェクト', global: 'グローバル' };
const countOf = (c) => ({ err: (c && c.err) || 0, warn: (c && c.warn) || 0 });
const jsonOf = (v) => JSON.stringify(v || []);

// profileModel is GET /api/profile read as the profile screen's model: the rows
// of the list, the two files the YAML tab draws, and the files' hashes.
export function profileModel(list, openNames) {
  const open = openNames || [];
  return {
    rows: ((list && list.items) || []).map((item) => profileRow(item, open.indexOf(item.name) >= 0)),
    files: ((list && list.files) || []).map((f) => ({
      file: f.place, label: PLACE_LABEL[f.place] || f.label, path: f.path, text: f.text, hash: f.hash,
      problems: f.problems || [],
    })),
    hashes: (list && list.hashes) || {},
  };
}

// profileRow is one item read as the row the list draws: each place becomes a
// panel, and the item's summary its columns.
function profileRow(item, isOpen) {
  const places = item.places || {};
  const hidden = item.hidden || [];
  const s = item.summary || {};
  const names = PLACES.filter((f) => places[f]);
  const both = names.length > 1;
  const panels = names.map((file) => {
    const place = places[file] || {}, doc = place.doc || {};
    const other = file === 'project' ? 'global' : 'project';
    const c = countOf(place.problems);
    return {
      file, label: PLACE_LABEL[file], path: place.path || '', saved: true,
      shadowed: hidden.indexOf(file) >= 0, both, other, otherLabel: PLACE_LABEL[other],
      name: doc.name || '', kind: doc.kind || '',
      args: doc.args || [], env: doc.env || [],
      argsJSON: jsonOf(doc.args), envJSON: jsonOf(doc.env),
      savedName: doc.name || '', savedKind: doc.kind || '',
      savedArgs: jsonOf(doc.args), savedEnv: jsonOf(doc.env),
      hash: place.hash || '', users: both ? 0 : (s.users || 0),
      renameUsers: place.readers || 0, err: c.err, warn: c.warn,
      savedLine: !!place.savedLine,
    };
  });
  const c = countOf(item.problems);
  return {
    name: item.name || '', kind: s.kind || '', kindClass: s.kindClass || '',
    new: false, open: !!isOpen,
    places: names.map((file) => ({ label: PLACE_LABEL[file], state: hidden.indexOf(file) >= 0 ? 'shadow' : 'on' })),
    args: s.args || '', env: s.env || [], more: s.more || 0,
    users: s.users || 0, roles: s.roles || '', err: c.err, warn: c.warn,
    chips: s.chips || [], panels,
  };
}

// roleModel is GET /api/role read as the role screen's model: the rows, each
// place holding the document behind it.
export function roleModel(list) {
  return { roles: ((list && list.items) || []).map(roleRow) };
}

function roleRow(item, i) {
  const places = item.places || {}, s = item.summary || {};
  const row = { uid: i + 1 };
  if (s.markdown) { row.markdown = s.markdown; row.name = item.name || ''; return row; }
  PLACES.forEach((l) => {
    const place = places[l];
    row[l] = (place && place.doc)
      ? Object.assign({}, place.doc, { hash: place.hash || '', text: place.text || '', path: place.path || '' })
      : null;
  });
  if (s.noDoc) row.noDoc = true;
  if (s.isNew) row.isNew = true;
  return row;
}

// teamModel is GET /api/team read as the role screen's 割り当て model: one team
// per definition, with the members' roles and the place it stands in. The hash
// a save carries is the project file's: the assign tab writes there.
export function teamModel(list) {
  return {
    teams: ((list && list.items) || []).map((item) => {
      const places = item.places || {}, s = item.summary || {};
      const eff = item.effective || (places.project ? 'project' : 'global');
      const place = places[eff] || {}, proj = places.project || {};
      return {
        id: item.name || '', hash: proj.hash || '', place: eff,
        turns: ((place.doc || {}).turns || []).map((t) => [t.from, t.to]),
        members: (s.members || []).map((m) => ({ name: m.name, saved: m.role || '', draft: m.role || '' })),
      };
    }),
  };
}

// ---- the classic screens' way in (4 章, until the last screen is moved) ----
// A screen still written as a classic script reads window.hergyAPI. It gets the
// old shape: a call that answers {ok, status, json, text} instead of throwing,
// and the helpers that write the page for it (refusal), which no module uses.
window.hergyAPI = {
  token,
  $,
  $$,
  call: raw,
  profileModel,
  roleModel,
  teamModel,
  list: (kind) => raw('GET', urls.list(kind)),
  get: (kind, name, place, project) => raw('GET', urls.get(kind, name, place, project)),
  check: (kind, payload) => raw('POST', `/api/${kind}/check`, payload),
  save: (kind, name, payload) => raw('PUT', `/api/${kind}/${enc(name)}`, payload),
  del: (kind, name, place, hash, agreementHash, project) => raw('DELETE', urls.del(kind, name, place, hash, agreementHash, project)),
  delivery: () => raw('GET', '/api/delivery'),
  deliveryCount: () => raw('GET', '/api/delivery/count'),
  deliver: (members) => raw('POST', '/api/delivery', { members }),
  saved,
  // after waits ms and calls fn, forgetting an earlier wait on the same box.
  after: (box, ms, fn) => {
    clearTimeout(box._apiTimer);
    box._apiTimer = setTimeout(fn, ms);
  },
  // refusal puts the server's own sentence beside a box's acts, the way every
  // screen shows one. The screen builds no sentence of its own (spec-boundary 3).
  refusal: (box, text) => {
    $$('.refusal', box).forEach((el) => el.remove());
    const span = document.createElement('span');
    span.className = 'sev err refusal';
    span.textContent = text;
    const acts = $('.acts', box);
    if (acts) acts.parentNode.insertBefore(span, acts); else box.appendChild(span);
  },
  problemText,
  // ---- reading an answer (spec-boundary 5) ----------------------------
  // A save, a check and a refusal answer under /api/, and the fields they carry
  // are read here, once, so no screen digs into an answer itself.
  answer: (a) => (a && a.json) || {},
  problems: (a) => ((a && a.json && a.json.problems) || []),
  // checkAnswer is a check's whole answer: the problems, the Markdown a person
  // reads, how the change moved it, and the people it named.
  checkAnswer: (a) => {
    const json = (a && a.json) || {}, effect = json.effect || {};
    return {
      problems: json.problems || [], preview: json.preview || null, prompt: json.prompt || null,
      diff: effect.diff || null, turns: effect.turns || null, readers: json.readers || [],
      size: json.size || 0, limit: json.limit || 0,
    };
  },
};
