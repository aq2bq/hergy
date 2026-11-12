// Temporary F1 compatibility for the unmigrated role/team editors. Remove this
// module when both editors use editing. No new operations belong here.
import { call, type Kind, type Request, type Answer, type RawAnswer, type CallOptions } from './api.js';
export const check = <K extends Kind>(kind: K, payload: Request<K, 'check'>, opts?: CallOptions): Promise<RawAnswer<Answer<K, 'check'>>> =>
  call('POST', `/api/${kind}/check`, payload, opts);
export const save = <K extends Kind>(kind: K, name: string, payload: Request<K, 'save'>, opts?: CallOptions): Promise<RawAnswer<Answer<K, 'save'>>> =>
  call('PUT', `/api/${kind}/${encodeURIComponent(name)}`, payload, opts);
