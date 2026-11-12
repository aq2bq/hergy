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
  const { editRole } = await import('../src/lib/editing');
  const { resourceIdentity } = await import('../src/lib/resource-identity');
  const editor = editRole(resourceIdentity('role', { place: 'global' }), { saved: null, draft: { name: 'builder', title: '', text: '' }, hash: '', text: '' });
  const first = editor.check();
  const second = editor.check();
  const problem = { sev: 'err', text: 'invalid' };
  pending[1].resolve({ problems: [problem] }); await second;
  pending[0].resolve({ problems: [] }); await first;
  expect(editor.read.value.problems).toEqual([problem]);
  const failed = editor.check();
  pending[2].reject(new Error('offline')); await failed;
  expect(editor.read.value.problems).toEqual([problem]);
  const absent = editor.check();
  pending[3].resolve({}); await absent;
  expect(editor.read.value.problems).toEqual([problem]);
  const empty = editor.check();
  pending[4].resolve({ problems: [] }); await empty;
  expect(editor.read.value.problems).toEqual([]);
  editor.dispose();
});
