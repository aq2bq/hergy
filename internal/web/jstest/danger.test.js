// The routing table (r-ccf8): confirm asks, undoable runs and offers undo,
// none just runs.
import { describe, expect, test } from 'bun:test';
import { DANGER, runDanger } from '../src/lib/danger';
import { confirmAsk, closeConfirm } from '../src/lib/confirm';
import { undo } from '../src/lib/undo';

describe('runDanger', () => {
  test('a confirm action asks and does not run', () => {
    let ran = false;
    runDanger('deleteTeam', { run: () => { ran = true; }, confirm: { title: 't', body: 'b', label: 'x', run: () => {} } });
    expect(ran).toBe(false);
    expect(confirmAsk.value?.title).toBe('t');
    closeConfirm();
  });
  test('an undoable action runs and offers its undo', () => {
    let ran = false, back = false;
    runDanger('unlistProject', { run: () => { ran = true; }, said: '外しました', restore: () => { back = true; } });
    expect(ran).toBe(true);
    expect(undo.value?.said).toBe('外しました');
    undo.value?.restore();
    expect(back).toBe(true);
  });
  test('a none action just runs', () => {
    let ran = false;
    runDanger('abortStart', { run: () => { ran = true; } });
    expect(ran).toBe(true);
    expect(confirmAsk.value).toBeNull();
  });
  test('the table has every action', () => {
    expect(DANGER.abortStart).toBe('none');
    expect(DANGER.deleteProfile).toBe('confirm');
    expect(DANGER.conflictRevert).toBe('undoable');
  });
});

describe('lost', () => {
  test('a discard names what it loses and says it cannot be taken back', async () => {
    const { render, h } = await import('../src/lib/preact.js');
    runDanger('discardRow', { run: () => {}, lost: ['本文'], confirm: { title: '捨てますか', label: '破棄' } });
    const host = document.createElement('div');
    render(h('div', {}, confirmAsk.value.body), host);
    expect(host.textContent).toContain('本文');
    expect(host.textContent).toContain('取り消せません');
    closeConfirm();
  });
});

describe('undo', () => {
  test('a restore the server refuses keeps the notice and says why', async () => {
    const { offerUndo, takeUndo, dropUndo } = await import('../src/lib/undo.js');
    const host = document.createElement('div');
    document.body.appendChild(host);
    offerUndo({ said: '並びから外しました', restore: () => Promise.resolve('元に戻せませんでした') }, host);
    expect(host.textContent).toContain('並びから外しました');
    takeUndo();
    await new Promise((done) => setTimeout(done, 0));
    expect(host.querySelector('.undo[role="alert"]')?.textContent).toContain('元に戻せませんでした');
    expect(host.querySelector('button')).toBeNull();
    dropUndo();
    host.remove();
  });
  test('a restore that goes through closes the notice', async () => {
    const { offerUndo, takeUndo } = await import('../src/lib/undo.js');
    let back = false;
    const host = document.createElement('div');
    document.body.appendChild(host);
    offerUndo({ said: '外しました', restore: () => { back = true; } }, host);
    takeUndo();
    expect(back).toBe(true);
    expect(host.querySelector('.undo')).toBeNull();
    host.remove();
  });
  test('a notice whose row is drawn away moves below the band and still takes it back', async () => {
    const { offerUndo } = await import('../src/lib/undo.js');
    let back = false;
    document.body.innerHTML = '<div class="bar"></div>';
    const row = document.createElement('div');
    document.querySelector('.bar').appendChild(row);
    offerUndo({ said: '並びから外しました', restore: () => { back = true; } }, row);
    expect(row.textContent).toContain('並びから外しました');
    // The sheet is drawn again: the row the notice stood in is a new element,
    // and the notice moves to the shared place instead of going with it (d-2cb2).
    row.remove();
    await new Promise((done) => setTimeout(done, 0));
    const fallback = document.querySelector('.undo-fallback');
    expect(fallback?.textContent).toContain('並びから外しました');
    fallback.querySelector('button').click();
    expect(back).toBe(true);
    expect(document.querySelector('.undo')).toBeNull();
  });
});
