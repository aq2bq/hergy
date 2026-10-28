// teams-page.js lays out the teams page for the attribute column's tests: the
// island the server writes, the answer the check gives, and the presses a
// person makes. A test file imports the screen's module once (its state stands
// for its page), so the three column tests stand in three files; this holds
// what they share.

// island is the teams page's data: one definition with a lead and a builder,
// one worker, and the roles the column offers.
export function island(over) {
  return {
    dirs: { project: '.hergy/teams', global: '~/.config/hergy/teams' },
    profiles: ['coder'],
    team: {
      items: [{
        name: 'core', effective: 'project',
        places: { project: { path: '.hergy/teams/core/team.yaml', hash: 'h1', doc: {
          mission: '注文 API を作り直す',
          members: [
            { name: 'lead', role: 'lead', profile: 'coder' },
            { name: 'builder', role: 'builder', profile: 'coder' },
          ],
          workers: [{ from: 'lead', purpose: '実装', profile: 'coder' }],
          turns: [['human', 'lead'], ['lead', 'human']],
          ...(over || {}),
        } } },
      }],
      files: [],
    },
  };
}

export const roleDoc = [
  '# lead',
  '',
  '## すること',
  '最初に動く。',
  '',
  '## しないこと',
  '勝手に保存しない。',
].join('\n');

// memberSegments is the check's answer for core-lead: the template, the roster,
// the folded role document, the folded team rules and hergy's contact guide.
export const memberSegments = [
  { kind: 'template', text: 'あなたは core-lead です。\n## 名簿\n', origin: { kind: 'hergy' } },
  { kind: 'roster', text: '- core-builder（ロール builder）\n\n', origin: { kind: 'roster' } },
  { kind: 'role_doc', text: roleDoc + '\n\n', origin: { kind: 'role', name: 'lead', place: 'project', path: '.hergy/roles/lead.yaml' } },
  { kind: 'team_rules', text: '## チームのルール\n\n- TEAM-1 このチームの約束。\n\n', origin: { kind: 'team_rules', path: '.hergy/teams/core/agreement.yaml' } },
  { kind: 'herdr', text: '## 連絡のしかた\n\nherdr で送る。\n', origin: { kind: 'hergy' } },
];

// checkAnswer is the answer `/api/team/check` gives for the saved definition.
export function checkAnswer(segments) {
  return { problems: [], members: [{ name: 'lead', segments }] };
}

// real is a line as the walk read it: the column writes a space for an empty
// line of Markdown, and the prompt holds nothing there.
export const real = (text) => (text === ' ' ? '' : text);

// linesOf reads the column's shown lines by the prompt's own line numbers.
export function linesOf(box) {
  const out = new Map();
  box.querySelectorAll('.yl[data-ln]').forEach((el) => {
    out.set(Number(el.dataset.ln), el.querySelector('.yt').textContent);
  });
  return out;
}

// foldsOf reads the folded documents the column draws.
export function foldsOf(box) {
  return [...box.querySelectorAll('.cv-fold')].map((el) => ({
    a: Number(el.dataset.a), b: Number(el.dataset.b),
    doc: el.dataset.doc || '', role: el.dataset.role || '', place: el.dataset.place || '',
    box: el,
  }));
}

// docText is a prompt put back together from its segments.
export const docText = (segments) => segments.map((segment) => segment.text).join('');

export function click(el) {
  el.dispatchEvent(new Event('click', { bubbles: true }));
}

// choose sets a select the way a person does.
export function choose(el, value) {
  el.value = value;
  el.dispatchEvent(new Event('change', { bubbles: true }));
}
