// teams-copy-place.js lays out the teams screen for the copy's own tests
// (r-a028, ac-f07b, ac-456e): a headquarters with one definition under beta's
// head and nothing in the global layer, the check's answer for the copy, and the
// presses a person makes.
//
// A test file imports the screen's module once — its state stands for its page —
// so each state of the copy's save stands in a file of its own, and this holds
// what they share.
import { layout, settle } from './page.js';

const BETA = '/work/beta';

// projects is the headquarters the island carries: this serve's project, and two
// more under their own heads.
const projects = {
  mode: 'listed',
  current: { path: '/work/alpha', name: 'alpha' },
  projects: [
    { path: '/work/alpha', name: 'alpha' },
    { path: BETA, name: 'beta' },
    { path: '/work/gamma', name: 'gamma' },
  ],
};

// definition is beta's own definition: the row a person copies into the global
// layer.
const definition = {
  name: 'beta-dev', effective: 'project',
  places: { project: { path: '.hergy/teams/beta-dev/team.yaml', hash: 'h1', doc: {
    description: '見積もりの計算をそろえる',
    members: [{ name: 'lead', role: 'lead', profile: 'alpha-fast' }],
    workers: [],
    turns: [['human', 'lead'], ['lead', 'human']],
  } } },
};

// page lays the teams screen out as the headquarters above.
export function page() {
  history.replaceState({}, '', '/teams');
  layout('teams-data', {
    dirs: { project: '.hergy/teams', global: '~/.config/hergy/teams' },
    profiles: ['alpha-fast'],
    hq: {
      projects,
      teams: [
        { project: projects.projects[0], list: { items: [] } },
        { project: projects.projects[1], list: { items: [definition] } },
        { project: projects.projects[2], list: { items: [] } },
        { global: true, list: { items: [] } },
      ],
      roles: [], profiles: [],
    },
    team: { items: [], files: [] },
  });
}

// seed is the check's answer for the copy: the draft its save would write, and
// the document it stands on.
export function seed(over) {
  return {
    problems: [],
    doc: {
      description: '見積もりの計算をそろえる',
      members: [{ name: 'lead', role: 'lead', profile: 'alpha-fast' }],
      workers: [],
      turns: [['human', 'lead'], ['lead', 'human']],
      agreement: { path: '~/.config/hergy/teams/beta-dev/agreement.yaml', hash: '', has: false, rules: 0 },
    },
    draft: {
      name: 'beta-dev', place: 'global', starter: false,
      path: '~/.config/hergy/teams/beta-dev/team.yaml', hash: '',
      writes: ['~/.config/hergy/teams/beta-dev/team.yaml', '~/.config/hergy/teams/beta-dev/agreement.yaml'],
      has_roles: true, has_profiles: true,
    },
    preview: { lines: [], diff: { added: [], removedBefore: [], removedAtEnd: false } },
    ...(over || {}),
  };
}

// refused makes the checks of a row that is not there yet answer the server's
// refusal, the way a base that cannot be read does.
export function refused(text) {
  const was = globalThis.fetch;
  globalThis.fetch = async (url, opts = {}) => {
    const body = opts.body ? JSON.parse(opts.body) : {};
    if (String(url).startsWith('/api/team/check') && body.base) {
      const said = JSON.stringify({ text });
      return { ok: false, status: 404, text: async () => said, headers: { get: () => 'application/json' } };
    }
    return was(url, opts);
  };
}

// copyOf presses beta's copy and answers the row it made: the one row that is not
// there yet, with its name field.
export async function copyOf() {
  const row = () => document.querySelector('.tdef[data-name="beta-dev|' + BETA + '"]');
  row().querySelector('.sum').dispatchEvent(new Event('click', { bubbles: true }));
  await settle();
  row().querySelector('[data-act=copy]').dispatchEvent(new Event('click', { bubbles: true }));
  await settle();
  return draftRow();
}

// draftRow is the row that is not there yet: the one with a name field.
export const draftRow = () => [...document.querySelectorAll('.tdef')].find((el) => el.querySelector('.in-name'));

// save is the 保存 of one row, and why is what its foot says about a save it does
// not offer.
export const save = (row) => row.querySelector('.pf [data-act=t-save]');
export const why = (row) => (row.querySelector('.pf .refusal') || {}).textContent || '';

// typing puts a value in a field the way a person's typing does, and lets the
// screen's debounce ask its check.
export async function typing(field, value) {
  field.value = value;
  field.dispatchEvent(new Event('input', { bubbles: true }));
  await new Promise((resolve) => setTimeout(resolve, 300));
  await settle();
}
