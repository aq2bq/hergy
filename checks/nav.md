上の帯（画面を選ぶ帯）と、URL・404 の確かめの一覧。作り物は `testdata/profiles-v2` に `testdata/roles-v2` と `testdata/nav` を重ねたもの（動いているチームは無い）。
確かめ方（作り物の写し方、XDG の差し替え、serve の起こし方、記録の残し方）は TESTING.md に従う。

| ID | 層 | 確かめること（人がすること → 見えるべきこと） |
|---|---|---|
| NAV-W1 | E2E | 幅 2000・明で `/profiles`、`/roles?open=builder`、`/nope` を開く → 帯が 1 列で `hergy / <フォルダ名>` の右にある。居る画面の項目が濃い地 |
| NAV-W2 | E2E | 幅 2000・暗で同じ 3 つを開く → NAV-W1 と同じ並び。居る画面の項目は明るい地に暗い字 |
| NAV-W3 | E2E | 幅 1440・明で同じ 3 つを開く → NAV-W1 と同じ。画面のタブが `一覧` `割り当て 22` `YAML`（ロールの画面は `割り当て 19`） |
| NAV-W4 | E2E | 幅 1440・暗で同じ 3 つを開く → NAV-W3 と同じ |
| NAV-W5 | E2E | 幅 500・明で同じ 3 つを開く → 帯が 2 段で、4 つの項目が幅いっぱいに並ぶ。ページが横に流れない |
| NAV-W6 | E2E | 幅 500・暗で同じ 3 つを開く → NAV-W5 と同じ |
| NAV-D1 | E2E | `data-theme="light"` と `data-theme="dark"` を付けて開く → NAV-W1〜W6 の明・暗と同じに見える |
| NAV-D2 | 受け口 | 帯の CSS（nav.css）を読む → 色の直書きが無い（全部トークン） |
| NAV-S1 | E2E | どの画面でも帯を見る → 並びが `活動`（動いているチームが無いので数なし）、`チーム定義 6`、`プロファイル 11`、`ロール 6` |
| NAV-S2 | 受け口 | 帯の HTML を読む → `<nav class="screens" aria-label="画面">` の中に 4 つ。居る画面は `<span class="scr on" aria-current="page">`、ほかの 3 つは `<a class="scr" href="…">` |
| NAV-S3 | E2E | 各画面の最初のタブを見る → `一覧`（数なし）。帯とタブで同じ語と数が 2 回並ばない |
| NAV-S4 | E2E | 帯のある画面を全部開く → 「層」「layer」「担当」「役割」「版」がどこにも無い |
| NAV-N1 | E2E | プロファイルの画面で帯の `ロール` を押す → 帯の下がまるごとロールの画面に替わり、帯の濃い地がロールへ移る。URL が `/roles`（前に居た所を覚えていればそこ） |
| NAV-N2 | E2E | 帯の `活動`、`チーム定義` を押す → それぞれ `/activity`、`/teams` に着く |
| NAV-N3 | E2E | 帯の居る画面の項目を押す → 何も変わらない |
| NAV-N4 | E2E | `hergy` を押す → `/` に行き、NAV-O1 の行き先に着く（何も開いていない一覧） |
| NAV-N5 | E2E | ロールの一覧で builder を開き、帯でプロファイルへ、帯でロールへ戻る → `/roles?open=builder` で builder が開いている。ロールの `割り当て` に居て同じことをすると `/roles/assign` に戻る |
| NAV-N6 | E2E | プロファイルの割り当てで core の impl-a を opus-deep に替え、保存せずに帯でロールへ → 帯の `プロファイル` に青い丸。ロールの画面は何も変わらない |
| NAV-N7 | E2E | NAV-N6 のあと帯でプロファイルへ戻る → `/profiles/assign` で、impl-a が opus-deep の青い破線、core のカードの頭に `1 人`。帯の丸は居る画面の色で残る。保存すると丸が消える |
| NAV-N8 | E2E | NAV-N6 のあと、プロファイルの画面を開かずに team.yaml を外で書き換え、帯でプロファイルへ → 下書きは戻らない。帯の丸は消える |
| NAV-N9 | E2E | NAV-N6 のあと、プロファイルの画面を再読み込みする → 下書きは戻らない（持ち越すのは帯で移ったときだけ） |
| NAV-N10 | E2E | 帯の居ない画面の項目へ Tab で移り Enter → NAV-N1 と同じ。フォーカスの枠が見える |
| NAV-O1 | E2E | `hergy serve --no-open` で起こし、出た URL を開く → このプロジェクトのチームが動いていれば `/activity`、動いていなければ `/teams` に着く。`GET /` は `302` |
| NAV-O2 | 受け口 | 消した URL（`/definitions`、`/definitions/{name}` とその下、`/agreement`、`/agreement/new`、`/agreement/{rule}`、`/running`、`/team`、`/rules`、`/diff`、`/bump`、`/figure`、`/panel/`、`/activity/{team}`）を GET で開く → 帯のある 404 の画面に `その画面はありません。`。どれも転送されない |
| NAV-O3 | 受け口 | `/static/hergy.css`、`/static/htmx-2.0.4.min.js` を GET で開く → 404 |
| NAV-O4 | 受け口 | `/profiles?layer=project` を開く → `?layer=` を付けないのと同じ画面（転送しない） |
| NAV-O5 | E2E | 別のブラウザのタブで画面を開く → 帯の `href` が `/activity`、`/teams`、`/profiles`、`/roles`（覚えた所が無い） |
| NAV-O6 | 受け口 | 消したテンプレートと static の名前（`layout.html` `hergy.css` `htmx` `ui.js` `copy.js` `/definitions` `/agreement`）を `grep -rn` で探す（git が追わない作業用のフォルダと、CHANGELOG の過去の版の節は除く） → 0 件。`go run ./tools/check` が exit 0 |
