// The work fixture the browser walks stand on, laid out once per test.
//
// internal/app's e2e harness laid a project out with newFixture and the screen's
// own copy*Fixture (a project with the screen's testdata over it, a config
// directory outside the home directory, its own state directory, a made-up
// herdr, and no hergy config anywhere else). Each test now lays its own out, so
// two tests can run at the same time: nothing outside its own root is read or
// written — not the person's ~/.config/hergy, not ~/.local/state/hergy.
//
// The activity screen's fixture keeps places in its files (@T-<分>@ and
// @PROJECT@), which are filled in the way internal/app/activity_scale_test.go
// fills them: a moment that many minutes before the run's own clock, and the
// project the teams were launched from.
import {
  chmodSync,
  mkdirSync,
  mkdtempSync,
  readFileSync,
  readdirSync,
  statSync,
  writeFileSync,
} from 'node:fs';
import { execFileSync } from 'node:child_process';
import { fileURLToPath } from 'node:url';
import path from 'node:path';
import os from 'node:os';

const here = path.dirname(fileURLToPath(import.meta.url));

// repo is the hergy module's root: the testdata, and the go.mod the serve is
// built from.
export const repo = path.resolve(here, '..', '..');
// hergyBin is the serve a run drives: the binary built with -tags e2e (下の
// build.mjs), so its clock and zone can be placed (d-f978).
export const hergyBin = process.env.HERGY_BIN || path.join(here, '..', '.bin', 'hergy');

// clockStart is the moment the run's screens are read at (r-f647): the serve
// reads it as its own beginning (HERGY_E2E_NOW, d-f978), and the fixture's
// moments are counted back from it. It is a fixed moment, so what the rows say
// does not depend on when or where the run happens.
export const clockStart = process.env.HERGY_E2E_NOW || '2026-10-21T00:30:00+09:00';
// zone is the zone the run reads its clocks in, and the one the serve is given
// through TZ.
export const zone = 'Asia/Tokyo';

// The pieces of testdata each screen's fixture is made of, overlaid in order.
// A screen's fixture is its project's .hergy and its machine's hergy directory;
// the navigation screen's is the role screen's fixture with the navigation's
// shared teams over it (spec-nav.md 5.1).
const screens = {
  nav: ['roles-v2', 'nav'],
  profiles: ['profiles-v2'],
  roles: ['roles-v2'],
  teams: ['teams-v2'],
  activity: ['teams-v2'],
};

// The fake herdr a screen with no team running answers with: the calls a status
// or a down needs, no tabs and no agents (internal/app/app_test.go runningHerdr
// with nothing, twice).
const fakeHerdr = `#!/bin/sh
g="$1"; s="$2"
if [ -z "$s" ]; then
  echo "herdr $g"
  for x in get prompt start list read focus send-keys current split rename close create process-info; do echo "  herdr $g $x"; done
  exit 0
fi
case "$g $s" in
  "tab list")     echo '{"result":{"tabs":[]}}' ;;
  "agent list")   echo '{"result":{"agents":[]}}' ;;
  "pane current") echo '{"result":{"pane":{"workspace_id":"w1"}}}' ;;
  "tab create")   echo '{"result":{"tab":{"tab_id":"t1"},"root_pane":{"pane_id":"p1"}}}' ;;
  "pane split")   echo '{"result":{"pane":{"pane_id":"p2"}}}' ;;
  *) echo '{"result":{}}' ;;
esac
`;

// lay lays one screen's fixture out in a work root of its own and returns where
// its parts stand and the environment the serve reads.
export function lay(screen) {
  const root = mkdtempSync(path.join(os.tmpdir(), 'hergy-e2e-'));
  const proj = path.join(root, 'proj');
  const cfg = path.join(root, 'cfg');
  const state = path.join(root, 'state');
  const home = path.join(root, 'home');
  for (const dir of [proj, cfg, state, home]) mkdirSync(dir, { recursive: true });

  for (const one of screens[screen] ?? []) {
    for (const [from, to] of [
      [path.join('project', '.hergy'), path.join(proj, '.hergy')],
      [path.join('config', 'hergy'), path.join(cfg, 'hergy')],
    ]) {
      const src = path.join(repo, 'testdata', one, from);
      if (!isDir(src)) continue;
      copy(src, to);
    }
  }
  mkdirSync(path.join(cfg, 'hergy'), { recursive: true });
  writeFileSync(path.join(cfg, 'hergy', 'config.yaml'), 'version: 1\nlang: ja\n');

  // The activity screen's own places: a made-up herdr and gy, the project's
  // ledger, and the launch record, with the fixture's moments filled in.
  const work = path.join(cfg, 'activity');
  const herdr = path.join(cfg, 'herdr');
  let gy = path.join(cfg, 'no-gy');
  if (screen === 'activity') {
    const at = new Date(clockStart);
    for (const part of ['bin', 'herdr', 'gy']) {
      copy(path.join(repo, 'testdata', 'activity', part), path.join(work, part), at, proj);
    }
    copy(path.join(repo, 'testdata', 'activity', 'project'), proj, at, proj);
    copy(path.join(repo, 'testdata', 'activity', 'state'), state, at, proj);
    gy = path.join(work, 'bin', 'gy');
  } else {
    writeFileSync(herdr, fakeHerdr);
    chmodSync(herdr, 0o755);
  }

  // The project is a git repository of its own: hergy finds the project root
  // through git, as the Go fixture's temporary project is.
  execFileSync('git', ['init', '-q'], { cwd: proj });

  const env = {
    XDG_CONFIG_HOME: cfg,
    XDG_STATE_HOME: state,
    HOME: home,
    HERDR_ENV: '',
    HERDR_PANE_ID: '',
    HERGY_HERDR: screen === 'activity' ? path.join(work, 'bin', 'herdr') : herdr,
    HERGY_GY: gy,
    TZ: zone,
    // The serve's clock (d-f978) starts here and goes on; the fixture's own
    // moments are counted back from the same moment.
    HERGY_E2E_NOW: clockStart,
  };
  return { root, proj, cfg, state, home, work, env };
}

// copy copies one directory tree over another, filling the activity fixture's
// places when a moment is given, and keeping every file's own permission bits
// (the fake herdr and gy are scripts that must stay runnable). A name ending in
// .tmpl is written without it: the fixture marks the files whose places are
// filled in, and hergy's own state files carry no such suffix.
function copy(from, to, at, project) {
  for (const name of readdirSync(from)) {
    const src = path.join(from, name);
    const target = path.join(to, name.replace(/\.tmpl$/, ''));
    const info = statSync(src);
    if (info.isDirectory()) {
      mkdirSync(target, { recursive: true });
      copy(src, target, at, project);
      continue;
    }
    mkdirSync(path.dirname(target), { recursive: true });
    const raw = readFileSync(src);
    if (at) {
      writeFileSync(target, places(raw.toString('utf8'), at, project));
    } else {
      writeFileSync(target, raw);
    }
    chmodSync(target, info.mode & 0o777);
  }
}

// places fills one file's places: @PROJECT@ is the project the teams were
// launched from, and @T-<分>@ is that many minutes before the run's own moment,
// as RFC 3339 — the shape hergy's own state files write.
function places(text, at, project) {
  return text
    .replaceAll('@PROJECT@', project)
    .replace(/@T-([0-9]+)@/g, (_, minutes) =>
      rfc3339(new Date(at.getTime() - Number(minutes) * 60_000)),
    );
}

// rfc3339 writes one moment the way Go's time.RFC3339 does, in the run's zone.
function rfc3339(at) {
  const p = parts(at);
  return `${p.year}-${p.month}-${p.day}T${p.hour}:${p.minute}:${p.second}+09:00`;
}

// parts is one moment's own numbers in the run's zone (Asia/Tokyo), which is the
// zone the serve writes its clocks in (TZ).
export function parts(at) {
  const f = new Intl.DateTimeFormat('en-CA', {
    timeZone: zone,
    year: 'numeric',
    month: '2-digit',
    day: '2-digit',
    hour: '2-digit',
    minute: '2-digit',
    second: '2-digit',
    hour12: false,
  });
  const out = {};
  for (const { type, value } of f.formatToParts(at)) out[type] = value;
  return out;
}

function isDir(p) {
  try {
    return statSync(p).isDirectory();
  } catch {
    return false;
  }
}
