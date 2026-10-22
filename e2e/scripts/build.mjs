// The serve every walk drives is built here, once for the run: `go build
// -tags e2e`, the binary whose clock and zone come from the environment
// (HERGY_E2E_NOW, TZ; d-f978). The published binary keeps no such reading.
import { execFileSync } from 'node:child_process';
import { mkdirSync } from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';

const here = path.dirname(fileURLToPath(import.meta.url));
const repo = path.resolve(here, '..', '..');
const bin = path.join(here, '..', '.bin');

export default function globalSetup() {
  mkdirSync(bin, { recursive: true });
  execFileSync('go', ['build', '-tags', 'e2e', '-o', path.join(bin, 'hergy'), '.'], {
    cwd: repo,
    stdio: 'inherit',
  });
}
