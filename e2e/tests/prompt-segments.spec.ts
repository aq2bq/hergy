// PROMPT-SEGMENTS — a prompt is the texts of its segments joined, character
// for character. The walk runs hergy's own plan roads over the showcase's
// standard and running scenes: core (the sample's largest team), research and
// docs (the two teams whose templates the sample rewrites) and one hired worker.
// It reads only the JSON plans, so it needs no browser and touches no screen.
import { test, expect } from '@playwright/test';
import { execFileSync } from 'node:child_process';
import { mkdtempSync, rmSync } from 'node:fs';
import path from 'node:path';
import os from 'node:os';
import { hergyBin, repo } from '../scripts/fixture.mjs';

type Origin = { kind: string; name?: string; path?: string; place?: string };
type Segment = { kind: string; text: string; origin: Origin };
type Member = { name: string; prompt: string; segments: Segment[] };
type Plan = { members: Member[] };

// plan runs one hergy command in one scene over the walk's own work directory
// and returns its JSON answer.
function plan(scene: 'standard' | 'running', work: string, args: string[]): Plan {
  const said = execFileSync(
    'go',
    ['run', './tools/showcase', 'run', scene, '--work', work, '--bin', hergyBin, '--', ...args],
    { cwd: repo, encoding: 'utf8', stdio: ['ignore', 'pipe', 'pipe'] },
  );
  return JSON.parse(said) as Plan;
}

// joined is a member's prompt rebuilt from its own segments.
const joined = (member: Member) => member.segments.map((segment) => segment.text).join('');

// segmentOf finds one segment of a kind, or says which kinds are there.
function segmentOf(member: Member, kind: string): Segment | undefined {
  return member.segments.find((segment) => segment.kind === kind);
}

for (const scene of ['standard', 'running'] as const) {
  test(`PROMPT-SEGMENTS-${scene}: every line of a prompt is one of its segments`, () => {
    const work = mkdtempSync(path.join(os.tmpdir(), `hergy-prompt-${scene}-`));
    try {
      for (const [team, args] of [
        ['core', ['up', 'core', '--dry', '--json']],
        ['research', ['up', 'research', '--dry', '--json']],
        ['docs', ['up', 'docs', '--dry', '--json']],
      ] as const) {
        const answer = plan(scene, work, args as unknown as string[]);
        expect(answer.members.length, `${team} has members`).toBeGreaterThan(0);
        for (const member of answer.members) {
          expect(joined(member), `${member.name}'s segments join to its prompt`).toBe(member.prompt);
          expect(segmentOf(member, 'role_doc'), `${member.name}'s role document is a segment`).toBeTruthy();
          expect(segmentOf(member, 'team_rules'), `${member.name}'s team rules are a segment`).toBeTruthy();
        }
        // The sample's custom templates: research rewrites the team's template
        // for every member; docs rewrites one member's own.
        if (team === 'research') {
          for (const member of answer.members) {
            const template = segmentOf(member, 'template');
            expect(template?.origin.kind, `${member.name}'s template is the team's`).toBe('team');
            expect(member.prompt, `${member.name}'s team template reaches it`).toContain(
              '調べ始める前に、問いを 1 文で書き',
            );
          }
        }
        if (team === 'docs') {
          const writer = answer.members.find((member) => member.name === 'docs-writer');
          expect(writer, 'the docs team has its writer').toBeTruthy();
          const template = writer ? segmentOf(writer, 'template') : undefined;
          expect(template?.origin.kind, "docs-writer's template is its own").toBe('member');
          expect(writer?.prompt, "docs-writer's member template reaches it").toContain(
            '書き始める前に、読み手を 1 人決め',
          );
        }
      }

      // The worker's prompt is the same shape: its task is its own segment.
      const hire = plan(scene, work, [
        'hire', 'helper', '--purpose', '調べ物', '--task', 'read the thing', '--team', 'core', '--dry', '--json',
      ]);
      expect(hire.members.length, 'the hire plan holds the worker').toBe(1);
      const worker = hire.members[0];
      expect(joined(worker), "the worker's segments join to its prompt").toBe(worker.prompt);
      const task = segmentOf(worker, 'task');
      expect(task?.text, "the worker's task is its own segment").toBe('read the thing');
      expect(task?.origin.kind, "the task's origin is the hire").toBe('hire');
      expect(segmentOf(worker, 'herdr')?.origin.kind, "the worker's contact guide is hergy's").toBe('hergy');
    } finally {
      rmSync(work, { recursive: true, force: true });
    }
  });
}
