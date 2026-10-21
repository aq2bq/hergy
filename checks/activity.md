活動の画面（`/activity`）の確かめの一覧。作り物は `testdata/teams-v2` の上に `testdata/activity`（作り物の herdr と gy。動いているチーム 4、開いている問い 55）を重ねたもの。
確かめ方（作り物の写し方と時刻の置き換え、XDG・`HERGY_HERDR`・`HERGY_GY` の差し替え、serve の起こし方、記録の残し方）は TESTING.md に従う。

| ID | 層 | 確かめること（人がすること → 見えるべきこと） |
|---|---|---|
| ACTIVITY-S1 | E2E | どの画面でも帯を見て `活動` を押す → 帯が `活動 4` と `待ち 2`（`--warn` の地）。押すと `/activity` に着き、居ると濃い地 |
| ACTIVITY-S2 | E2E | 画面を開く → 画面のタブが `一覧` だけ |
| ACTIVITY-S3 | E2E | 一覧を見る → 行が 4 つ、並びが core、migrate、core-hotfix、research（人間を待つチームが先で長く待っている順、次に止まっているチーム、最後に仕事中） |
| ACTIVITY-S4 | E2E | 行の名前を見る → core-hotfix の名前の下に `定義 core`。ほかの行には無い |
| ACTIVITY-S5 | E2E | 開いたばかりの画面を見る → core、migrate、core-hotfix が開いていて、research は閉じている |
| ACTIVITY-S6 | E2E | 開いた行の中を見る → 左が「頭 → 問い → 画面の最後」、右が「メンバー → ワーカー」 |
| ACTIVITY-S7 | E2E | 設計の試作と並べる → HTML のクラス名が同じ |
| ACTIVITY-T1 | E2E | 一覧を見る → `許可待ち` `人間の番` `止まっている` `仕事中` `最後のターン` `人間` `から` `より前` `以上` が出る |
| ACTIVITY-T2 | E2E | 開いた行を見る → `ロール <role>` `画面の最後` `▸ 上に N 行` `ワーカー` `人間に渡した` `動いていない` `定義 <名前>` が出る |
| ACTIVITY-T3 | E2E | 画面を全部見る → 「層」「layer」「担当」「役割」「版」「台帳」がどこにも無い |
| ACTIVITY-C1 | E2E | 行を開かずに見る → 行の `いま` と `誰` だけで、core の lead（`人間の番`、`最後のターン lead → 人間`）と migrate の tester（`許可待ち`、`tester`）が人間を待っているとわかる |
| ACTIVITY-C2 | E2E | core を開く → q-7c2e の題と選択肢 2 つが最初に見える。q-51b9 と q-0d4a は 1 行ずつ（`1 日前`、`2 日前`）。ほかの 51 件の問いは画面のどこにも無い（画面に出る問いは全部で 4 件） |
| ACTIVITY-C3 | E2E | migrate を開く → 問いの段が無く、画面の最後に `Would you like to run the following command?` と psql の行が見える。1 行目が空行でも 6 行に見える |
| ACTIVITY-C4 | E2E | core と migrate のメンバーの欄を見る → core は lead だけが `人間に渡した` で、ほかは `止まっている`。migrate は impl が `仕事中` |
| ACTIVITY-C5 | E2E | core と core-hotfix の行を見る → core に `最後のターン lead → 人間` と `2 時間 40 分`。core-hotfix に `最後のターン impl-a`、`4 時間 5 分以上`、`<時刻> より前` |
| ACTIVITY-C6 | E2E | core のメンバーの欄を見る → 止まった時刻の新しい順（lead、impl-a、impl-b、reviewer、analyst） |
| ACTIVITY-P1 | E2E | 行を押す → その行の真下だけが開く・閉じる。URL の `open` が替わる。仕事中の research を押すと、その場でメンバーとワーカーの欄が出る（左の欄は無い） |
| ACTIVITY-P2 | E2E | migrate の `▸ 上に N 行` を押す → その画面の最後の箱だけが上へ伸び、`▾ 上の行をしまう` に替わる。押すと戻る |
| ACTIVITY-P3 | E2E | 行の主の問いが 5 件以上ある作り物で `ほか N 件` を押す → その問いの段の下に残りの問いが 1 行ずつ出る |
| ACTIVITY-P4 | E2E | 開いたまま、作り物の agents.json の core-lead を working に替えて 10〜15 秒待つ → core の行が `仕事中` になり並びが動く。帯が `待ち 1` になる。フォーカス、スクロール、開いている行、伸ばした箱は動かない |
| ACTIVITY-E1 | E2E | `HERGY_HERDR` を無いパスにして開く → 一覧に herdr の文が 1 行（`動いているチームはありません` ではない）、帯の `活動` に数が無い |
| ACTIVITY-E2 | E2E | `HERGY_GY` を無いパスにして core を開く → 問いの段が無いだけで、文は出ない。画面の最後とメンバーはそのまま |
| ACTIVITY-E3 | E2E | 作り物の gy を exit 1 にして core を開く → 問いの所に `警告` と gy の文が 1 行。画面の最後とメンバーはそのまま |
| ACTIVITY-E4 | E2E | 作り物の tabs.json を空にして開く → `動いているチームはありません`。帯の `活動` に数が無い。`GET /` は `/teams` へ 302 |
| ACTIVITY-E5 | 受け口 | チームが動いているときに `GET /` → `/activity` へ 302 |
| ACTIVITY-W1 | E2E | 幅 2000・1440・500 で開く → 2000・1440 は行が 1 段（名前・いま・誰・いつから）でいつからが右寄せ、開いた中は左（62rem まで）と右（17〜21rem）が横に並んで左に寄る。900 以下は右の欄が左の下。500 は行が 2 段（名前といま / 誰といつから）で時刻を隠し長さだけ |
| ACTIVITY-W2 | E2E | どの幅でも → ページが横に流れない。500 で画面の最後の psql の行が折り返す |
| ACTIVITY-D1 | E2E | 明、暗、`data-theme="light"`、`data-theme="dark"` で開く → どれも文字が読める |
| ACTIVITY-D2 | 受け口 | 画面の CSS を読む → 色の直書きが無い |
| ACTIVITY-V1 | 受け口 | 画面の JS と受け口の答えを読む → 長さ・時刻・何分前の文は全部 `/api/activity` の答えの文で、JS が作っていない |
| ACTIVITY-V2 | 受け口 | 呼ばれたことを書く作り物の gy で `GET /api/activity` を呼ぶ → gy が 1 度も呼ばれない |
| ACTIVITY-V3 | 受け口 | 同じ作り物の gy で `GET /api/activity/core` を呼ぶ → `gy show` は q-7c2e、q-51b9、q-0d4a の 3 回だけ。`POST /api/activity/check`・`PUT`・`DELETE /api/activity/core` は 405、知らないチームは 404 |
