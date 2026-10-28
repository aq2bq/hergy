// look.mjs — look at the screens over one of the e2e samples, without the
// walk's browser.
//
//   bun e2e/scripts/look.mjs <sample> [--port N]
//
// The sample is laid out in a work directory of its own (the e2e fixture's own
// `lay`, so the walks and this command never drift apart), the serve is started
// with --no-open, and the URL it prints is the one a person opens. Ctrl-C stops
// the serve and takes the work directory away.
//
// Nothing of the person's machine is read or written: XDG_CONFIG_HOME,
// XDG_STATE_HOME and HOME are the work directory's own (d-a861). To look at a
// prototype branch, run this command inside that branch's worktree; to put two
// versions side by side, give them different --port.
//
// The e2e binary is the one the walks drive (built with -tags e2e, so the
// sample's clock is the fixture's, d-f978). It is built here when it is not
// there yet, from the tree this command stands in.
import { execFileSync, spawn } from 'node:child_process';
import { existsSync, mkdirSync, rmSync } from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { hergyBin, lay, repo } from './fixture.mjs';

const samples = ['activity', 'hq', 'nav', 'profiles', 'roles', 'teams'];

function usage() {
  return `使い方: bun e2e/scripts/look.mjs <見本> [--port N]\n\n見本: ${samples.join('、')}\n`;
}

function argumentsOf(argv) {
  let sample = '';
  let port = 0;
  for (let i = 0; i < argv.length; i++) {
    const one = argv[i];
    if (one === '--port') {
      port = Number(argv[++i] ?? '');
      if (!Number.isInteger(port) || port < 0) throw new Error('--port のあとに番号が要ります');
      continue;
    }
    if (one === '-h' || one === '--help') return { help: true };
    if (one.startsWith('-')) throw new Error(`知らない旗です: ${one}`);
    if (sample) throw new Error(`見本は 1 つだけです: ${sample}、${one}`);
    sample = one;
  }
  if (!samples.includes(sample)) throw new Error(sample ? `知らない見本です: ${sample}` : '見本の名前が要ります');
  return { sample, port };
}

// build makes the e2e serve the walks drive, from this tree.
function build() {
  mkdirSync(path.dirname(hergyBin), { recursive: true });
  execFileSync('go', ['build', '-tags', 'e2e', '-o', hergyBin, '.'], { cwd: repo, stdio: 'inherit' });
}

let main;
try {
  main = argumentsOf(process.argv.slice(2));
} catch (e) {
  process.stderr.write(`${e.message}\n\n${usage()}`);
  process.exit(2);
}

if (main.help) {
  process.stdout.write(usage());
  process.exit(0);
}

if (!existsSync(hergyBin)) build();

// fixture is this run's own work directory: the sample laid out, and the
// settings the serve reads.
const fixture = lay(main.sample);
let child = null;
let stopping = false;

function clean() {
  if (child && child.exitCode === null) child.kill('SIGTERM');
  rmSync(fixture.root, { recursive: true, force: true });
}

function stop(code) {
  if (stopping) return;
  stopping = true;
  clean();
  process.exit(code);
}

process.on('SIGINT', () => stop(0));
process.on('SIGTERM', () => stop(0));
process.on('exit', () => { if (!stopping) rmSync(fixture.root, { recursive: true, force: true }); });

child = spawn(hergyBin, ['serve', '--no-open', '--port', String(main.port)], {
  cwd: fixture.proj,
  env: { ...process.env, ...fixture.env },
  stdio: ['ignore', 'pipe', 'pipe'],
});

let said = '';
let printed = false;
const look = (chunk) => {
  said += chunk.toString();
  const found = said.match(/http:\/\/127\.0\.0\.1:[0-9]+/);
  if (found && !printed) {
    printed = true;
    process.stdout.write(`${found[0]}\n`);
    process.stdout.write('止めるときは Ctrl-C。\n');
  }
};
child.stdout.on('data', look);
child.stderr.on('data', (chunk) => {
  process.stderr.write(chunk);
  look(chunk);
});
child.on('exit', (code) => {
  if (!printed) process.stderr.write(`serve が URL を出さずに終わりました（${code}）:\n${said}`);
  stop(code === null ? 1 : code);
});
