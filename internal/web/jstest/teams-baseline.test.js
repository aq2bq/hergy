// spec-teams X3: a save stops only on an error the written file did not hold.
// The file's own problems come from the check sent with no ops, a request of
// its own: an edit sent while that check is still out draws its answer over
// the panel, but the baseline is still the file's. Here the file's member
// names a profile that does not resolve (an error), so a lost baseline would
// shut the save. The file's answer comes last here, after every other answer
// has drawn, so nothing but that answer can open the save again. (Moved from
// the browser walk TEAMS-X3, which slowed every check by 700 ms, waited 2.6 s
// on a clock, and passed only because a later answer drew the foot.)
import { expect, test } from 'bun:test';
import { later, layout, serve, settle, type } from './page.js';

test('an edit made while the first check is out keeps the file as the save\'s baseline', async () => {
  history.replaceState({}, '', '/teams?open=docs');
  layout('teams-data', {
    dirs: { project: '.hergy/teams', global: '~/.config/hergy/teams' },
    profiles: ['opus'],
    team: {
      items: [{
        name: 'docs', effective: 'project',
        places: { project: { path: '.hergy/teams/docs/team.yaml', hash: 'h1', doc: {
          mission: '文書を書く', members: [{ name: 'writer', role: 'writer', profile: 'gemini-flash' }], workers: [], turns: [],
        } } },
      }],
      files: [],
    },
  });
  const fileError = { sev: 'err', at: 'members[0].profile', text: 'プロファイル gemini-flash がありません' };
  const fileCheck = later();
  serve((method, url, body) => {
    if (url.startsWith('/api/team/check') && body.part === 'team') {
      // The op-less check is the file's reading; it is held until the test lets
      // it go. A check with ops answers at once, with the same error.
      return body.doc.ops.length ? { problems: [fileError] } : fileCheck.promise;
    }
    if (url.startsWith('/api/team/check')) return { problems: [] };
    if (url.startsWith('/api/role')) return { items: [] };
    return undefined;
  });
  await import('../static/teams/main.js');
  await settle();

  const mission = document.querySelector('.tdef.open .in-mission');
  expect(mission).not.toBe(null);
  type(mission, '書きかけの使命');
  await settle();
  // The edit's check is debounced; its answer comes before the file's.
  await new Promise((resolve) => setTimeout(resolve, 300));
  await settle();
  fileCheck.resolve({ problems: [fileError] });
  await settle();

  expect(document.querySelector('.tdef.open .in-mission').value).toBe('書きかけの使命');
  expect(document.querySelector('.tdef.open .panel.team-p .vlist').textContent).toContain('gemini-flash');
  expect(document.querySelector('.tdef.open .panel.team-p [data-act=t-save]').disabled).toBe(false);
});
