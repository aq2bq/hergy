// lib/hq/raise.js — 起きていないメンバーを起動する（r-5d1a）。
//
// 押すと POST /api/hq/refresh を呼ぶ。答えた起動（action が refresh）は、起動の
// ときと同じ欄（lib/hq/sheets.js の runStart）に渡す: 進み具合と、起動の途中の
// 問いは、起動のときと同じ形で出る。断られたら欄は開かず、その文を押した場所に
// 返す（文はサーバのもの、画面は作らない）。
import { call, problemText } from '../api.js';
import { runStart } from './sheets.js';
import type { components } from '../../../../apicontract/api-types';

type HQLaunchEnvelope = components['schemas']['HQLaunchEnvelope'];
type HQRefreshRequest = components['schemas']['HQRefreshRequest'];

// raiseMember raises one member of a running team. The answer is the sentence a
// person reads when the server says no, and '' when the raise went out.
export async function raiseMember(team: string, name: string): Promise<string> {
  try {
    const request = { team, members: [name] } satisfies HQRefreshRequest;
    const answer = await call<HQLaunchEnvelope>('POST', '/api/hq/refresh', request);
    const launch = (answer.json || {}).launch;
    if (launch) runStart(launch);
    return '';
  } catch (e) {
    return problemText(e, '起動できませんでした');
  }
}
