// lib/resource.ts — サーバーへの読み込みの共有部品（webui-preact-design 5 章 3）。
//
// 画面は、読み込みを自分で並べない。resource に「どう読むか」を渡し、
// `value` / `loading` / `error` を signal として受け取る。要求は後から出した
// ものが勝ち、前の要求は AbortController で取り消す。だから画面は、どの答えが
// 今の画面のものかを番号で合わせない（要求に番号を振る必要が無い）。
//
//   const words = resource((signal) => getJSON<Words>('/api/words', { signal }));
//   words.run();                       // 読み込みを始める
//   words.value.value                  // 最後に届いた答え（まだなら null）
//   words.loading.value                // 読み込み中か
//   words.error.value                  // 最後の読み込みの失敗（または null）
//   words.abort();                     // 読みかけを取り消す（画面を離れるとき）
import { signal } from './preact.js';

// Load is how one reading reads: the request's AbortSignal first, then what run
// was given, and an answer or a thrown refusal.
export type Load<T, A extends unknown[]> = (signal: AbortSignal, ...args: A) => Promise<T>;

// Reading is one reading as the screen draws it: the newest answer, whether one
// is out, its failure, and the two things a screen does with it.
export type Reading<T, A extends unknown[]> = {
  value: { value: T | null };
  loading: { value: boolean };
  error: { value: unknown };
  run: (...args: A) => Promise<T | undefined>;
  abort: () => void;
};

// resource is one reading whose answer is state. load is called with the
// request's AbortSignal and may take arguments (run passes them on), so a screen
// reads one team through the same shape as the whole board. The reading itself
// goes through lib/api.js, which hands back the answer or throws a refusal
// (lib/api.js); this file only holds it as state.
//
// Only the newest request's answer is kept: run aborts the one before it, and an
// answer that came after its own abort is dropped. loading is true while the
// newest request is out, and error holds that request's failure.
export function resource<T, A extends unknown[] = []>(load: Load<T, A>): Reading<T, A> {
  const value = signal<T | null>(null);
  const loading = signal(false);
  const error = signal<unknown>(null);
  let inflight: { stop: AbortController } | null = null;

  async function run(...args: A): Promise<T | undefined> {
    if (inflight) inflight.stop.abort();
    const mine = { stop: new AbortController() };
    inflight = mine;
    loading.value = true;
    error.value = null;
    try {
      const next = await load(mine.stop.signal, ...args);
      if (mine.stop.signal.aborted) return undefined;
      value.value = next;
      return next;
    } catch (e) {
      if (mine.stop.signal.aborted) return undefined;
      error.value = e;
      return undefined;
    } finally {
      if (inflight === mine) {
        inflight = null;
        loading.value = false;
      }
    }
  }

  // abort drops the request that is out: its answer is not the screen's any
  // more, so it is neither kept nor drawn (a screen that is left calls this).
  function abort(): void {
    if (!inflight) return;
    inflight.stop.abort();
    inflight = null;
    loading.value = false;
    error.value = null;
  }

  return { value, loading, error, run, abort };
}
