// A problem about a team agreement keeps its draft rule target when the rule
// has no ID, so the finding row opens the right rule and its switch still works.
import { expect, test } from 'bun:test';
import { layout, serve, settle } from './page.js';
import { click } from './teams-page.js';

const island = {
  dirs: { project: '.hergy/teams', global: '~/.config/hergy/teams' },
  profiles: ['coder'],
  team: { items: [{
    name: 'core', effective: 'project',
    places: { project: { path: '.hergy/teams/core/team.yaml', hash: 'h1', doc: {
      description: '注文 API を作り直す',
      members: [{ name: 'lead', role: 'lead', profile: 'coder' }],
      workers: [], turns: [['human', 'lead'], ['lead', 'human']],
      agreement: {
        path: '.hergy/teams/core/agreement.yaml', hash: 'a1', has: true,
        doc: { name: '', title: 'core チームのルール', revision: 1, sections: [{
          heading: '仕事', level: 0, intro: '', rules: [{ text: '保存前の規則。', in_prompt: true, note: '' }],
        }] },
      },
    } } },
  }], files: [] },
};

test('a finding row and the switch find an agreement rule without an ID', async () => {
  history.replaceState({}, '', '/teams');
  layout('teams-data', island);
  serve((method, url, body) => {
    if (url.startsWith('/api/team/check') && body?.part === 'team') {
      return {
        problems: [{
          sev: 'warn', at: 'rule:.hergy/teams/core/agreement.yaml', field: 'text',
          section: 0, rule: 0, text: '保存前の規則を確認してください',
        }],
        members: [], preview: { lines: [], diff: { added: [], removedBefore: [], removedAtEnd: false } },
      };
    }
    if (url.startsWith('/api/team/check')) return { problems: [] };
    if (url.startsWith('/api/role')) return { items: [{ name: 'lead' }] };
    return undefined;
  });
  await import('../src/teams/main');
  const state = await import('../src/teams/state');
  await settle();

  const row = document.querySelector('.tdef[data-name="core"]');
  click(row.querySelector('.sum'));
  await settle();
  const finding = row.querySelector('.panel.team-p .vlist .vitem[data-act="vrule"]');
  expect(finding, 'the agreement finding is in the team rules list').toBeTruthy();
  const rec = state.allRecs()[0];
  const key = rec.agreeDraft.value.sections[0].rules[0]._k;
  expect(finding.dataset.k).toBe(String(key));
  click(finding);
  await settle();

  const rule = row.querySelector(`.panel.agree-p .rule[data-k="${key}"]`);
  expect(rule.classList.contains('open')).toBe(true);
  expect(rule.querySelector('.rid b').textContent.trim()).toBe('保存で付く');
  expect(rule.querySelector('.redit .ridv').textContent.trim()).toBe('保存で付く');
  click(rule.querySelector('.onsw'));
  await settle();
  expect(rule.querySelector('.onsw').getAttribute('aria-checked')).toBe('false');
});
