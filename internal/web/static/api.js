// api.js — the one way a screen talks to the server (spec-boundary 7, r-2cc2).
// Every kind of thing a screen reads and writes answers under /api/ in one shape
// (spec-boundary 5): a list, one document, a check, a save, a delete. This file
// holds the call, the token of this run, the 300ms wait a check rides on, and the
// place a refusal is shown. A screen keeps its own look and sides with it.
window.hergyAPI = (() => {
  'use strict';

  const view = document.getElementById('view');
  const token = (view && view.dataset.token) || '';
  const $ = (sel, root = document) => root.querySelector(sel);
  const $$ = (sel, root = document) => Array.from(root.querySelectorAll(sel));
  const enc = (s) => encodeURIComponent(String(s == null ? '' : s));

  // call sends one request with the token of this run. A write carries its body
  // as JSON, and the token rides in the header, so POST, PUT and DELETE are one
  // shape (a DELETE carries no body at all).
  async function call(method, url, payload) {
    const opts = { method, credentials: 'same-origin', headers: { 'X-Hergy-Token': token } };
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

  return {
    token,
    $,
    $$,
    call,
    // list is GET /api/{kind}: every item and the hashes behind them.
    list: (kind) => call('GET', `/api/${kind}`),
    // get is GET /api/{kind}/{name}?place=: one document.
    get: (kind, name, place) => call('GET', `/api/${kind}/${enc(name)}${place ? `?place=${enc(place)}` : ''}`),
    // check is POST /api/{kind}/check: what is wrong with a draft or a text.
    check: (kind, payload) => call('POST', `/api/${kind}/check`, payload),
    // save is PUT /api/{kind}/{name}: write one document.
    save: (kind, name, payload) => call('PUT', `/api/${kind}/${enc(name)}`, payload),
    // del is DELETE /api/{kind}/{name}?place=&hash=: take one document away.
    del: (kind, name, place, hash) => call('DELETE', `/api/${kind}/${enc(name)}?place=${enc(place)}&hash=${enc(hash)}`),
    // after waits ms and calls fn, forgetting an earlier wait on the same box.
    after: (box, ms, fn) => {
      clearTimeout(box._apiTimer);
      box._apiTimer = setTimeout(fn, ms);
    },
    // refusal puts the server's own sentence beside a box's acts, the way every
    // screen shows one. The screen builds no sentence of its own (spec-boundary
    // 3).
    refusal: (box, text) => {
      $$('.refusal', box).forEach((el) => el.remove());
      const span = document.createElement('span');
      span.className = 'sev err refusal';
      span.textContent = text;
      const acts = $('.acts', box);
      if (acts) acts.parentNode.insertBefore(span, acts); else box.appendChild(span);
    },
    // problemText is the sentence one refusal carries, or the fallback word.
    problemText: (answer, fallback) => (answer.json && answer.json.text) || fallback,
  };
})();