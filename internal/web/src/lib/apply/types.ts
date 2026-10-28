// lib/apply/types.ts — 反映の答えの形（n-4b22、設計は apply-design.md）。
//
// 形は API のルール（internal/apicontract/openapi.yaml の ApplyList）にある。ここは
// その写しで、pico が /api/apply を main に入れたら、画面は lib/api.js の
// ApiAnswer から取る（このファイルは消える）。
export type ApplyNote = { ok?: boolean; text?: string };

export type ApplyChange = {
  kind: string;
  document?: string;
  rule?: string;
  text?: string;
};

// How is the way hergy would put one member's definition change into the
// running team: 送り直す、起動する、起動し直す（会話が消える）、反映できない。
export type ApplyHow = 'resend' | 'start' | 'restart' | 'cannot';

export type ApplyMember = {
  name: string;
  short?: string;
  role?: string;
  state: string;
  how: ApplyHow;
  changes?: ApplyChange[];
  can_apply?: boolean;
  why_not?: string;
  conversation_lost?: boolean;
};

export type ApplyTeam = {
  team: string;
  definition?: string;
  project?: string;
  project_name?: string;
  error?: string;
  notes?: string[];
  members?: ApplyMember[];
};

export type ApplyList = {
  herdr?: ApplyNote;
  count?: number;
  teams?: ApplyTeam[];
};

export type ApplyCount = { count?: number };

export type ApplyEnvelope = { launch?: import('../../../../apicontract/api-types').components['schemas']['HQLaunch'] };
