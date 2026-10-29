// r-1db7 L6: a project already in the list is said so, and the page is not read
// again as though it had been added.
import { expect, test } from 'bun:test';
import { serve, settle, type } from './page.js';

test('a project already listed is said so, and nothing is posted', async () => {
  history.replaceState({}, '', '/activity');
  document.body.innerHTML = '<div class="wrap"><div class="bar"></div><div id="tabs"></div><div id="view"></div></div>';
  const asked = serve((method, url) => {
    if (url === '/api/hq/projects') {
      return {
        mode: 'listed',
        current: { path: '/work/alpha', name: 'alpha' },
        projects: [{ path: '/work/shop', name: 'shop', display: '~/src/shop', listed: true }],
      };
    }
    return undefined;
  });
  const hq = await import('../src/lib/hq');
  await hq.readCore();
  await settle();
  const { projectsSheet } = await import('../src/lib/hq/sheets.js');
  projectsSheet();
  await settle();

  type(document.getElementById('hq-path'), '~/src/shop');
  await settle();
  document.querySelector('[data-hq=add]').click();
  await settle();

  expect(document.querySelector('.hq-err').textContent).toBe('shop はもう並んでいます');
  expect(asked.filter((one) => one.method === 'POST')).toEqual([]);
});
