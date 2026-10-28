// spec-teams B7, d-a575: a point the figure laid out stays where it stands. The
// places are decided when the draft changes (state.js's seedPlaces), not while
// the figure is drawn, so adding a member does not move the members already
// there — the walk TEAMS-C2 reads the same rule as "no two points over each
// other", and a person reads it as "the figure does not jump".
import { expect, test } from 'bun:test';
import { layout, serve, settle } from './page.js';
import { click, island } from './teams-page.js';

const member = (name) => ({ name, role: name, profile: 'coder' });

test('a member added to the canvas leaves the members that stood where they stood', async () => {
  history.replaceState({}, '', '/teams?open=core');
  layout('teams-data', island({ members: [member('lead'), member('builder'), member('reviewer')], workers: [] }));
  serve((method, url) => {
    if (url.startsWith('/api/team/check')) return { problems: [] };
    if (url.startsWith('/api/role')) return { items: [] };
    return undefined;
  });
  const S = await import('../src/teams/state');
  await import('../src/teams/main');
  await settle();

  const at = (name) => {
    const g = document.querySelector(`.tdef[data-name="core"] .cv-n.member[data-cv-n="${name}"]`);
    return g ? g.getAttribute('transform') : null;
  };
  const r = S.byName('core');
  expect(Object.keys(r.layout.value).sort()).toEqual(['builder', 'human', 'lead', 'reviewer']);
  const before = { lead: at('lead'), builder: at('builder'), reviewer: at('reviewer') };
  expect(before.lead).not.toBe(null);

  click(document.querySelector('.tdef[data-name="core"] [data-cv-act=m-add]'));
  await settle();

  // The added member stands somewhere of its own, and the three that were there
  // did not move.
  const name = Object.keys(r.layout.value).find((id) => id.startsWith('member-'));
  expect(name).toBeDefined();
  expect(at(name)).not.toBe(null);
  expect({ lead: at('lead'), builder: at('builder'), reviewer: at('reviewer') }).toEqual(before);
});
