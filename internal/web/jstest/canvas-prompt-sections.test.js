// r-04fc: a topology section stands as one band from its heading to the end of
// its lines. When the person's own template holds the heading, the band takes
// it in and the heading keeps no origin of its own; blank lines between the
// heading and the section are part of the band (d-57c6).
import { expect, test } from 'bun:test';
import { layout, serve, settle } from './page.js';
import { click, island } from './teams-page.js';

test('a template heading, with a blank line under it, stands inside its section band', async () => {
  history.replaceState({}, '', '/teams?open=core');
  layout('teams-data', island());
  const template = 'あなたは core-lead です。\n## メンバー\n\n';
  const roster = '- core-builder（ロール builder）\n';
  const contactHeading = '## 連絡\n';
  const contact = '- herdr で送る。\n';
  const turns = '## ターンを渡す相手\n- core-builder\n';
  const current = template + roster + contactHeading + contact + turns;
  const segments = [
    { kind: 'template', text: template, origin: { kind: 'hergy' } },
    { kind: 'roster', text: roster, origin: { kind: 'roster' } },
    { kind: 'template', text: contactHeading, origin: { kind: 'hergy' } },
    { kind: 'herdr', text: contact, origin: { kind: 'hergy' } },
    { kind: 'turns', text: turns, origin: { kind: 'turns' } },
  ];
  const answer = { runs: [], selected: null, status: 'draft', reason: '', recorded: null, current, segments,
    problems: [], diff: [], project: '/example', executable: 'hergy', language: 'ja', team: 'core', member: 'lead', draft: true };
  serve((method, url, body) => {
    if (url.startsWith('/api/team/prompt-comparison')) return answer;
    if (url.startsWith('/api/team/check') && body && body.part === 'team') return { problems: [], members: [{ name: 'lead', segments }] };
    if (url.startsWith('/api/team/check')) return { problems: [] };
    if (url.startsWith('/api/role')) return { items: [] };
    return undefined;
  });
  await import('../src/teams/main');
 const { PROMPT_READ } = await import('../src/teams/prompt-source.js'); PROMPT_READ.wait = 0; await settle();
  click(document.querySelector('.tdef[data-name="core"] .cv-n.member[data-cv-n="lead"]')); await settle();
  click(document.querySelector('.tdef[data-name="core"] [data-cv-side="prompt"]')); await settle();
  click(document.querySelector('.cv-prompt button')); await settle(); await settle();

  const rows = [...document.querySelectorAll('.prompt-current-line')];
  const row = (text) => rows.find((one) => one.querySelector('.prompt-line-text')?.textContent === text);
  const heading = row('## メンバー\n');
  const blank = row('\n');
  const body = row('- core-builder（ロール builder）\n');
  // The heading and the blank line under it stand in the roster band, and the
  // band starts at the heading.
  expect(heading.dataset.section).toBe('roster');
  expect(heading.hasAttribute('data-section-start')).toBe(true);
  expect(heading.querySelectorAll('.prompt-origin').length).toBe(0);
  expect(blank.dataset.section).toBe('roster');
  expect(body.dataset.section).toBe('roster');
  expect(body.hasAttribute('data-section-end')).toBe(true);

  // A heading the section itself carries is in its band as well, and is marked
  // the way it always was.
  const turnsHeading = row('## ターンを渡す相手\n');
  expect(turnsHeading.dataset.section).toBe('turns');
  expect(turnsHeading.hasAttribute('data-section-start')).toBe(true);
  expect(turnsHeading.querySelectorAll('.prompt-origin').length).toBe(1);

  // The 連絡 heading starts its own band, not the roster's.
  const contactHeadingRow = row('## 連絡\n');
  expect(contactHeadingRow.dataset.section).toBe('herdr');
  expect(contactHeadingRow.hasAttribute('data-section-start')).toBe(true);
});
