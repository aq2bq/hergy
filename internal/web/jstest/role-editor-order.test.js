import { expect, test } from 'bun:test';
import { layout, later, serve } from './page.js';

test('shared role editor keeps the newest check and retains errors during a failed read', async () => {
  layout('roles-data', {});
  const pending = [];
  serve((method, url) => {
    if (method === 'POST' && url === '/api/role/check') {
      const one = later(); pending.push(one); return one.promise;
    }
  });
  const { roleEditor } = await import('../src/lib/role-editor');
  const editor = roleEditor();
  const first = editor.check.run({ place: 'global', project: '/sample', name: 'builder' });
  const second = editor.check.run({ place: 'global', project: '/sample', name: 'builder' });
  const problem = { sev: 'err', text: 'invalid' };
  pending[1].resolve({ problems: [problem] }); await second;
  pending[0].resolve({ problems: [] }); await first;
  expect(editor.check.value.value.json.problems).toEqual([problem]);
  const failed = editor.check.run({ place: 'global' });
  pending[2].reject(new Error('offline')); await failed;
  expect(editor.check.value.value.json.problems).toEqual([problem]);
  const absent = editor.check.run({ place: 'global' });
  pending[3].resolve({}); await absent;
  expect(editor.check.value.value.json.problems).toEqual([problem]);
  const empty = editor.check.run({ place: 'global' });
  pending[4].resolve({ problems: [] }); await empty;
  expect(editor.check.value.value.json.problems).toEqual([]);
});
