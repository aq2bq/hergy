// profiles/types.ts — プロファイルの画面が持つ形（webui-preact-design 4 章・5 章）。
//
// サーバーの答えは API のルール（internal/apicontract/api-types）の型で受け、
// lib/api がそれを画面のモデルに読み替える。この画面がさらに足す形（面・札・
// 帯に渡す下書き）だけをここで名付ける。
import type { components } from '../../../apicontract/api-types';
import type { Place, Schemas } from '../lib/api.js';
import type { Conflict } from '../lib/conflict.js';

// ---- the API's own shapes ----------------------------------------------
// The answers the profile screen reads are the ones openapi.yaml writes: the
// types are made from that file, not written here.
export type ProfileList = Schemas['ProfileList'];
export type ProfileCheck = Schemas['ProfileCheck'];
export type Problem = Schemas['Problem'];
export type ProfileChip = Schemas['ProfileChip'];
export type EnvPair = Schemas['ProfileEnvPair'];
export type HQProfileGroup = Schemas['HQProfileGroup'];
export type HQProjects = Schemas['HQProjects'];

// ---- the page's island -------------------------------------------------
// ISLAND is the JSON the page carries (the Go side's p2Island): the profile
// list, the assignment cards, and the headquarters' groups.
export type KindBar = {
  name: string;
  count: number;
  grow: string;
  kindClass: string;
  dim: boolean;
  on: boolean;
};

export type OptionGroup = { kind: string; names: string[] };

export type IslandRow = {
  kind: string;
  id: string;
  name: string;
  role: string;
  profile: string;
  missing: boolean;
  error: string;
  options: OptionGroup[];
  kindOf: string;
  kindCls: string;
  dim: boolean;
};

export type IslandTeam = {
  name: string;
  place: string;
  path: string;
  members: number;
  workers: number;
  err: number;
  rows: IslandRow[];
  hash: string;
  saved: boolean;
};

export type IslandAssign = { kinds: KindBar[]; teams: IslandTeam[]; empty: boolean };

export type IslandHQ = {
  projects: HQProjects;
  profiles?: HQProfileGroup[];
  // Why the headquarters' lists could not be read; a screen says it (r-1db7 L1).
  error?: string;
};

export type Island = { profile?: ProfileList; assign?: IslandAssign; hq?: IslandHQ };

// ---- the screen's own state --------------------------------------------
// One profile's panel: what the file holds (saved), the draft the fields edit,
// and what the last check answered.
export type SavedProfile = { name: string; kind: string; args: string[]; env: EnvPair[] };

export type Draft = { name: string; kind: string; args: string[]; env: EnvPair[] };

export type Panel = {
  key: string;
  rowKey: string;
  file: Place;
  group: string;
  path: string;
  label: string;
  otherLabel: string;
  both: boolean;
  shadowed: boolean;
  saved: SavedProfile | null;
  hash: string;
  users: number;
  rename_targets: number;
  draft: Draft;
  problems: Problem[];
  errors: number;
  savedLine: boolean;
  refusal: string;
  // conflict stands where a save was refused because the file moved: the file as
  // the answer showed it, and the panel before 元に戻す (r-3c77).
  conflict: PanelConflict | null;
  confirming: boolean;
  rename: boolean;
  layerSwitch: boolean;
};

// PanelConflict is one refused save of a panel.
export type PanelConflict = { c: Conflict; undone?: boolean; before?: { saved: SavedProfile | null; draft: Draft; hash: string } };

export type ModelPlace = { label: string; state: string };

export type Row = {
  key: string;
  name: string;
  group: string;
  kind: string;
  kindClass: string;
  args: string;
  env: string[];
  more: number;
  users: number;
  roles: string;
  err: number;
  warn: number;
  chips: components['schemas']['ProfileChip'][];
  places: ModelPlace[];
  isNew: boolean;
  open: boolean;
  panels: Panel[];
};

export type ChangeSet = {
  name: boolean;
  kind: boolean;
  args: boolean[];
  env: { key: boolean; value: boolean }[];
  count: number;
};

export type FocusAsk = { key: string; field: string; index: number; select: boolean; scroll?: boolean; seq: number };
export type ScrollAsk = { key: string; seq: number };

// ---- the assign tab ----------------------------------------------------
export type AssignRow = {
  key: string;
  kind: string;
  name: string;
  role: string;
  profile: string;
  saved: string;
  options: OptionGroup[];
  missing: boolean;
  error: string;
  kindCls: string;
};

export type Team = {
  name: string;
  place: Place;
  hash: string;
  path: string;
  members: number;
  workers: number;
  err: number;
  saved: boolean;
  refusal: string;
  rows: AssignRow[];
};

export type Assign = { empty: boolean; teams: Team[] };

// ---- the YAML tab ------------------------------------------------------
// One profiles.yaml as the YAML tab draws it: the file's place, the words the
// tab names it by, its text, its hash and the problems of that text.
export type YamlFile = {
  file: string;
  label: string;
  path: string;
  text: string;
  hash: string;
  problems: Problem[];
};

// A file's text as this screen keeps it: what the file was read with (base),
// what a person typed (text), and a carried draft's text (carried).
export type YamlOne = { base?: string; text?: string; carried?: string | null };

// ---- what one screen hands the next through the band -------------------
// take's answer and restore's argument (lib/nav holds it between the screens).
export type Carried = {
  rows?: { file: string; saved: string | null; hash: string; values: Draft }[];
  ytexts?: { file: string; hash: string; text: string }[];
  picks?: { team: string; hash: string; key: string; profile: string }[];
};

// ---- the address -------------------------------------------------------
export type OpenTarget = { name: string; group?: string };
