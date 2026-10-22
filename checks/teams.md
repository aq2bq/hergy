チーム定義の画面（`/teams`、`/teams/yaml`）の確かめの一覧。作り物は `testdata/teams-v2`（チーム定義はプロジェクト 4・共通 3、core のルールは 53 規則で CORE-8 を含めない）。
大きさ・守るもの・監査の決めは TESTING.md に従う。
大きさ: 小 2・中 10・大 4・監査 3（全体 19。もとは 63、d-3e43 の前は 21。中の 10 件は下の「まだテストの無い項目」）。
まとめた: TEAMS-M2 ← M1 M3 ／ TEAMS-M4 ← M5 ／ TEAMS-F1 ← F2 F3 ／ TEAMS-F4 ← F5 F6 ／ TEAMS-N3 ← N4 ／ TEAMS-N5 ← N6 ／ TEAMS-N7 ← N8 ／ TEAMS-V2 ← V3 ／ TEAMS-P5 ← P1 P2 P3 P4 P6 P7 P8 P9 P10 P13 Y1 Y4 Y5 ／ TEAMS-Y2 ← Y3 Y7 ／ TEAMS-S2 ← S1 S4 S5 S6 S7 S8 ／ TEAMS-T1 ← T2 T3 T4 T5 ／ TEAMS-W1 ← W2 W3 D1 D2 P12 Y8。消したものは無い
消した（d-3e43、2026-10-22。もっと小さいテストと重なる）: TEAMS-F4（ROLES-F1・F2・F5 と同じ書き手）・TEAMS-Y6（変えていない定義は TEAMS-Y2 の比べ、Op のあとは TestTeamsFigureCheckAndEffect と TestTeamsFigureSaveWritesTheOneLine）。大の 4 本の文を、残す比べだけに書き直した

| ID | 大きさ | 守るもの | 確かめること（人がすること → 見えるべきこと） |
|---|---|---|---|
| TEAMS-M2 | 小 | 場面 T9・T10 | core の impl-b → reviewer を消す op の効果を求める → 消える行が `core-impl-b · ターンを渡す相手` の `- core-reviewer（ロール reviewer）` で、`hergy up core --dry --lang ja` のその行と 1 字違わず、増える問題に `規則 CORE-12 が impl-b → reviewer と書いていますが、…`。analyst から出る最後の 1 本では、消える行と `メンバー analyst から出るターンがありません…` の両方 |
| TEAMS-M4 | 小 | 場面 T7 | ルールの Markdown のプレビューの行を、`hergy up <定義> --dry --lang ja` の初期プロンプトのそのチームのルールの部分と比べる → 1 字違わない。CORE-8 は行番号の無い打ち消しの行で、`N 行 · N 文字` に入らない |
| TEAMS-P5 | 大 | 場面 T5・T8・T9・書き込み（打った使命・消したターンが保存で書かれる、目的を打つとワーカーが 1 人のまま）・不具合: 打つと欄が描き直されてカーソルが外れる・札に `undefined`・消える行が 2 回出る（R6）・押した規則が窓の外に開く（R12） | core の lead を選んで `＋ ワーカー` を押し、目的を 1 字ずつ打つ → 打つ間カーソルは欄のまま、札に `undefined` が無く、足の数は `1 か所`。research の scout → 人間を消して検査の列の RES-6 を押す → ルールのタブでその規則が窓の中に開き、本文にカーソル。core の使命を替えて保存する → team.yaml で替わったのは `mission:` の 1 行だけ。impl-b → reviewer を `消す` を押すと札の消える行が 1 回だけ出て、保存すると team.yaml から `- impl-b -> reviewer` の 1 行だけが消える |
| TEAMS-N1 | 大 | 場面 T18・書き込み（共通に書く物・書かない物、取り消しは何も書かない）・不具合: ルールのタブから保存できない（監査 R9）・profiles.yaml が丸ごと書き直される（R3）・帯の数が保存と削除に追いつかない（R7） | `＋ チーム定義を追加` を押し、名前 `ops` を打ち、ルールのタブに替えてそこから保存する → 一番上の行が `ops` になり、profiles.yaml には claude-default の 2 行だけが足され、帯が 7 になる。もう一度 `＋` を押して共通を選び `ops` を保存する → 共通に lead.yaml と builder.yaml が書かれ、共通の profiles.yaml には claude-default が無い。プロジェクトにだけある定義を 1 つ削除すると帯が 1 減る。`＋` を押して取り消すと、プロジェクトの teams の下は変わらない |
| TEAMS-Y2 | 大 | 書き込み（見せた文と書く文が同じ）・不具合: 足した行が足の帯に隠れる（R2） | `/teams?open=core` を開く → YAML の列が core の team.yaml と 1 行ずつ同じ（コメントを含めて）。analyst から reviewer へ線を足す → 足した行が箱の中に見える所まで箱が送られる |
| TEAMS-V1 | 大 | 不具合: 読めない定義を描いて落ちる・絶対パス（TEAMS-V4 の近く）・問題が別の点に付く（R10）・何も変えていないのに `元に戻す` が出る（R5） | core を開く → 図の枠の頭に team.yaml のパスが出る。ルールのタブに替える → `元に戻す` は出ていない。docs（名前の解けないプロファイル）を開く → 例外が無く図と保存が描かれ、問題は翻訳のワーカーの点だけに付く |
| TEAMS-S2 | 監査 | 見た目 | 帯・タブ・一覧の 6 列・開いた行・トポロジーとルールのパネル・YAML のファイルの列の並びと、クラス名を試作と並べて見る |
| TEAMS-T1 | 監査 | 見た目 | 画面の語（`トポロジー` `ルール` `チームのルール` ほか、札の語）と、使わない語（「層」「担当」「役割」「版」「約束」「チームの姿」）が無いことを見る |
| TEAMS-W1 | 監査 | 見た目 | 幅 2000・1440・500 と明・暗・`data-theme` で並びと色を見る → 横に流れない、500 で図が収まり定義の操作がタブの下、札が点を隠さない、色はトークンだけ |


## まだテストの無い項目（2026-10-21、n- の need。テストを書いたら上の表に戻す）

2026-10-22（d-3e43）: 次の 7 件は、ID の付いていない中のテストが internal/web/teams_v2_test.go に既にある。新しく書かずに、そのテストの名前に ID を入れて上の表に戻す: TEAMS-S3 ← TestTeamsListCarriesTheTable ／ TEAMS-F1 ← TestTeamsFigureSaveWritesTheOneLine ／ TEAMS-F7 ← TestTeamsFigureSaveRefusesANewProblem ／ TEAMS-N2 ← TestTeamsDraftSaveRefusesANameProblem ／ TEAMS-N3 ← TestTeamsDraftSaveWritesTheStarterFolder ／ TEAMS-N5・TEAMS-N7 ← TestTeamsDraftDuplicateWritesTheSameFiles・TestTeamsCopyAndDeleteTakeOnePlace ／ TEAMS-P11 ← TestTeamsYamlSaveRefusesANewError。TEAMS-V2 と TEAMS-V4 はまだ合うテストが無い（TEAMS-V4 は lead の合意で残した。いまは TEAMS-V1 の歩きの比べ 1 つ（図の枠のパス）だけが近い所を見ている）

- TEAMS-S3 ／ 中 ／ 場面 T3・T4 ／ `GET /api/team` → 名前の順に 6 行。docs はプロジェクトが効き、共通は隠れている。共通の review と draft も行になる
- TEAMS-F1 ／ 中 ／ 場面 T5・T8・書き込み ／ ターンを 1 本消す、使命だけを変える、ワーカーを足す、をそれぞれ保存に送る → team.yaml の差分が、その `- a -> b` の 1 行、`mission:` の 1 行、workers: の最後の 3 行だけ
- TEAMS-F7 ／ 中 ／ 場面 T10・書き込み ／ docs（翻訳の gemini-flash の問題）で使命だけを直す保存と、ターンを消して渡し先の無いメンバーを作る保存を送る → 前者は書かれ、後者は断られる
- TEAMS-N2 ／ 中 ／ 不具合: 空のまま・既存の名前を読む ／ 新しい定義の検査を、名前を空・`Bad!`・`core` にして送る → それぞれ `チーム定義名を書いてください`、`チーム定義名は [a-z][a-z0-9_-]{0,24} の形にしてください: "Bad!"`、`チーム定義 "core" はすでにあります…` が名前に付く。`core` のときも既にある core のルールは読まない。答えのパスは打った名前のもの
- TEAMS-N3 ／ 中 ／ 場面 T18・書き込み ／ 名前 `ops` の新しい定義の保存を送る（プロジェクトと共通の 2 通り） → `<置き場所>/teams/ops/team.yaml` と `agreement.yaml` が `hergy new ops` と 1 字違わない。プロジェクトでは profiles.yaml の差分が claude-default の 2 行だけ、共通では無いロールの文書（lead.yaml・builder.yaml）も書く。答えの `writes` に書いた全部が並ぶ
- TEAMS-N5 ／ 中 ／ 書き込み ／ core の複製（`core-2`）と、review（共通）のプロジェクトへのコピーの保存を送る → 書いた 2 つのファイルが元と 1 字違わない。コピーのあと review は 1 行で、プロジェクトが効く
- TEAMS-N7 ／ 中 ／ 書き込み ／ review（プロジェクトと共通にある）の削除を送る → プロジェクトの 2 つのファイルとフォルダだけが消え、共通が効く。保存していない新しい定義は何も書かない
- TEAMS-P11 ／ 中 ／ 不具合: 共有部品の保存 ／ YAML のタブの保存（team.yaml と agreement.yaml の文まるごと）を送る → 打った文がそのまま書かれる。前からある問題では止めず、増えた問題で断る
- TEAMS-V2 ／ 中 ／ 場面 T18（ファイルを開かずに同じ検査） ／ 作り物の検査の答えと `hergy validate --lang ja` を比べる → 文が同じで、画面だけの検査が無い（BUILD-4 の問題、docs の gemini-flash、DRAFT-3・REV-4・WRITE-7 の警告、共通の docs の隠れている警告、読む量）
- TEAMS-V4 ／ 中 ／ 不具合: 絶対パス ／ 共通をホームの下（`~/.config/hergy`）に置いた作り物で、一覧・1 件・検査・YAML の答えを読む → 作業用ディレクトリの絶対パスが無く、ファイルは `.hergy/…` か `~/.config/hergy/…` の形
