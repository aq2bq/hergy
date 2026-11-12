// Temporary F1 compatibility for the unmigrated role/team editors. Remove this
// module when both editors use editing. No new operations belong here.
import { sendLegacy, type Kind, type Request, type Answer, type RawAnswer, type CallOptions } from './api.js';
type LegacyKind = Exclude<Kind, 'profile'>;
declare const legacyBrand: unique symbol;
export type LegacyEditedRequest<K extends Kind, A extends 'check' | 'save'> = Readonly<{
  kind: K; action: A; name: string; body: Request<K, A>; [legacyBrand]: true;
}>;
function request<K extends Kind, A extends 'check' | 'save'>(kind: K, action: A, name: string, body: Request<K, A>): LegacyEditedRequest<K, A> {
  return { kind, action, name, body } as LegacyEditedRequest<K, A>;
}
export const check = <K extends LegacyKind>(kind: K, payload: Request<K, 'check'>, opts?: CallOptions): Promise<RawAnswer<Answer<K, 'check'>>> =>
  sendLegacy<K, 'check'>(request(kind, 'check', '', payload), opts);
export const save = <K extends LegacyKind>(kind: K, name: string, payload: Request<K, 'save'>, opts?: CallOptions): Promise<RawAnswer<Answer<K, 'save'>>> =>
  sendLegacy<K, 'save'>(request(kind, 'save', name, payload), opts);
