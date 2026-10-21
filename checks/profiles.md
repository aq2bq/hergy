プロファイルの画面（`/profiles`、`/profiles/assign`、`/profiles/yaml`）の確かめの一覧。作り物は `testdata/profiles-v2`（プロファイル 11 行、チーム定義 4、メンバー 15、ワーカー 3）。
大きさ・守るもの・監査の決めは TESTING.md に従う。
大きさ: 小 0・中 11・大 2・監査 3（全体 16。もとは 41）。
まとめた: PROFILES-P1 ← P5 P6（画面の道） ／ PROFILES-P7 ← P5 P6（差分） ／ PROFILES-P8 ← P3 P9 ／ PROFILES-F1 ← P10 ／ PROFILES-P12 ← V2 ／ PROFILES-P11 ← P13 ／ PROFILES-V4 ← P14 ／ PROFILES-V5 ← V1 V6 ／ PROFILES-S1 ← S2 S4 S5 S6 S7 ／ PROFILES-T1 ← T2 T3 T4 ／ PROFILES-W1 ← W2 W3 W4 D1 D2 D3。消した（守るものが言えない）: PROFILES-P2

| ID | 大きさ | 守るもの | 確かめること（人がすること → 見えるべきこと） |
|---|---|---|---|
| PROFILES-S3 | 中 | 場面 P6・P7 | `GET /api/profile` → sonnet-fast は 1 行で、プロジェクトが効き、共通は隠れている |
| PROFILES-T5 | 中 | 場面 P3 | `GET /api/profile/opencode-deepseek` → env の値が伏せられず、2022 文字の JSON がそのまま入っている |
| PROFILES-P7 | 中 | 書き込み | sonnet-fast の引数を 1 つと env の値を 1 つ替えた保存を受け口に送る → profiles.yaml の差分がその 2 行だけ |
| PROFILES-P4 | 中 | 書き込み | 使われているプロファイルの名前を替え、使っている人も書き換える保存を送る → profiles.yaml の名前の行と、使っている team.yaml の `profile:` の行だけが替わる |
| PROFILES-P8 | 中 | 書き込み | 追加・複製・もう一方へのコピーの保存を送る → 追加と複製はその置き場所の profiles.yaml に、コピーはもう一方の profiles.yaml に、その項目だけが足される。ほかの行は 1 字も変わらない |
| PROFILES-F1 | 中 | 書き込み | 使われている fable-review の削除を送る → profiles.yaml からその項目だけが消え、割り当ての答えで core のその人に `members[4].profile: fable-review というプロファイルはありません`、core に `問題 1` |
| PROFILES-P12 | 中 | 場面 P4・書き込み | 割り当てで migrate の impl を別のプロファイルに替える保存を送る → team.yaml の差分がその `profile:` の 1 行だけ。docs（翻訳の gemini-flash の問題）は、増えた問題があれば断る |
| PROFILES-V4 | 中 | 不具合: 共有部品の保存 | YAML のタブの保存（文まるごと）を送る → 打った文がそのまま書かれる。文を崩した文は問題の行番号付きで断り、何も書かない |
| PROFILES-V3 | 中 | 不具合: 空のまま・形の違う名前 | 名前を空・`sonnet-5.5` にした検査と保存を送る → 名前の欄に問題（`profiles.sonnet-5.5: プロファイル名は [a-z][a-z0-9_-]{0,31} の形にしてください` ほか）、保存は断り何も書かない |
| PROFILES-V5 | 中 | 場面 P2（ファイルを開かずに同じ検査） | 作り物の検査の答えと `hergy validate --lang ja` を比べる → 文が同じ（頭のファイルパスを除く）で、画面だけの検査が無い。共通の codex の警告は YAML の 9 行目 |
| PROFILES-V7 | 中 | 不具合: 絶対パス | 共通をホームの下（`~/.config/hergy`）に置いた作り物で、一覧・1 件・検査の答えを読む → 作業用ディレクトリの絶対パスが無く、ファイルは `.hergy/…` か `~/.config/hergy/…` の形 |
| PROFILES-P1 | 大 | 場面 P1・P2 | 一覧で sonnet-fast の行を押して開き、引数と env を直して保存する → その行の真下で開き、足に `保存しました`、行の要約が替わる |
| PROFILES-P11 | 大 | 場面 P4・P5 | opus-plan の使っている人の札を押し、割り当てのタブのその人の行でプロファイルを名前で選んで保存する → その行が画面の中央で光り選択欄にフォーカス、選ぶとそのカードの頭と帯が替わり、保存で戻る |
| PROFILES-S1 | 監査 | 見た目 | タブ・一覧の列・パネル・割り当てのカード・YAML の並びと、クラス名を試作と並べて見る |
| PROFILES-T1 | 監査 | 見た目 | 画面の語（`＋ プロファイルを追加` ほか）と、使わない語が無いことを見る |
| PROFILES-W1 | 監査 | 見た目 | 幅 2000・1440・500 と明・暗・`data-theme` で並びと色を見る → 横に流れない、500 で長い JSON と 8 個の引数が折り返す、色はトークンだけ |
