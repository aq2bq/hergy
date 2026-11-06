import { expect, test } from 'bun:test';
import { PAGE_POSITION, movePage } from '../src/lib/page-position';

// calls swaps window.scrollBy and Element.scrollIntoView for recorders, so a
// test sees whether a page (or an element) was asked to move.
function calls() {
  const scrollBy = window.scrollBy;
  const intoView = Element.prototype.scrollIntoView;
  const made = [];
  window.scrollBy = ((arg) => { made.push({ kind: 'scrollBy', arg }); });
  Element.prototype.scrollIntoView = function (arg) { made.push({ kind: 'scrollIntoView', arg }); };
  const restore = () => { window.scrollBy = scrollBy; Element.prototype.scrollIntoView = intoView; };
  return { made, restore };
}

test('every act says where the page goes', () => {
  const acts = Object.keys(PAGE_POSITION);
  expect(acts.length).toBeGreaterThan(0);
  for (const act of acts) {
    const row = PAGE_POSITION[act];
    expect(['band', 'nearest', 'center', 'stay']).toContain(row.at);
    expect(row.why.length).toBeGreaterThan(0);
    expect(row.d).toMatch(/^(d-|ac-)/);
  }
});

test("a 'stay' act moves neither the page nor its element", () => {
  const el = document.createElement('div');
  document.body.appendChild(el);
  for (const [act, row] of Object.entries(PAGE_POSITION)) {
    if (row.at !== 'stay') continue;
    const seen = calls();
    try {
      movePage(act, el);
      expect(seen.made).toEqual([]);
    } finally {
      seen.restore();
    }
  }
  el.remove();
});

test("'band' puts the row's top just under the band", () => {
  const el = document.createElement('div');
  el.getBoundingClientRect = () => ({ top: 200, bottom: 260, height: 60 });
  document.body.appendChild(el);
  const seen = calls();
  try {
    movePage('copyRow', el);
    expect(seen.made).toEqual([{ kind: 'scrollBy', arg: { top: 200 - 62, behavior: 'smooth' } }]);
  } finally {
    seen.restore();
    el.remove();
  }
});

test("'band' reads the band's height and the extra room it is given", () => {
  const el = document.createElement('div');
  el.getBoundingClientRect = () => ({ top: 200, bottom: 260, height: 60 });
  document.body.appendChild(el);
  document.documentElement.style.setProperty('--stick', '50px');
  const seen = calls();
  try {
    movePage('editSelection', el, { extra: 30 });
    expect(seen.made).toEqual([{ kind: 'scrollBy', arg: { top: 200 - 50 - 30, behavior: 'smooth' } }]);
  } finally {
    seen.restore();
    el.remove();
    document.documentElement.style.removeProperty('--stick');
  }
});

test("'band' leaves a row that is already under the band alone", () => {
  const el = document.createElement('div');
  el.getBoundingClientRect = () => ({ top: 63, bottom: 90, height: 27 });
  document.body.appendChild(el);
  const seen = calls();
  try {
    movePage('dupRow', el);
    expect(seen.made).toEqual([]);
  } finally {
    seen.restore();
    el.remove();
  }
});

test("'nearest' and 'center' put the element into view by that block", () => {
  const el = document.createElement('div');
  document.body.appendChild(el);
  for (const act of ['waitingRow', 'assignPerson', 'personFromURL']) {
    const seen = calls();
    try {
      movePage(act, el);
      expect(seen.made).toEqual([{ kind: 'scrollIntoView', arg: { block: PAGE_POSITION[act].at, behavior: 'smooth' } }]);
    } finally {
      seen.restore();
    }
  }
  el.remove();
});
