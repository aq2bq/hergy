活動の画面（`/activity`）の確かめの一覧。作り物は `testdata/teams-v2` の上に `testdata/activity`（作り物の herdr と gy。動いているチーム 4、開いている問い 55）を重ねたもの。時刻は固定した「いま」から作る。
大きさ・守るもの・監査の決めは TESTING.md に従う。
大きさ: 小 2・中 5・大 2・監査 3（全体 12。もとは 32。d-3e43 で項目は消さず、大の比べを削った）。
まとめた: ACTIVITY-C4 ← C6 ／ ACTIVITY-C5 ← V1 ／ ACTIVITY-S3 ← S4 C1（答え） ／ ACTIVITY-C2 ← V2 V3 P3 ／ ACTIVITY-E1 ← E4 ／ ACTIVITY-E3 ← E2 ／ ACTIVITY-C1 ← S1 S5 S6 P1 P2 ／ ACTIVITY-S7 ← S2 ／ ACTIVITY-T1 ← T2 T3 ／ ACTIVITY-W1 ← W2 D1 D2。NAV-O1 へ移した: ACTIVITY-E5。消したものは無い
消した（d-3e43、2026-10-22）: 項目は無い。大の 2 本の文を、残す比べだけに書き直した

| ID | 大きさ | 守るもの | 確かめること（人がすること → 見えるべきこと） |
|---|---|---|---|
| ACTIVITY-C4 | 小 | 場面 S3 | 作り物の agent の状態と seq から 1 チームを判定する → core は lead が `handed`、ほかが止まっている。メンバーの並びは人間を待つ・仕事中・止まっている（止まった時刻の新しい順: lead、impl-a、impl-b、reviewer、analyst）・動いていない |
| ACTIVITY-C5 | 小 | 場面 S4・不具合: 壁の時計 | 固定した「いま」と止まった時刻から言い方を作る → `2 時間 40 分` と `<時刻> から`、serve の前からなら `4 時間 5 分以上` と `<時刻> より前`、問いは `2 時間 45 分前`・`1 日前`・`2 日前`。壁の時計を読まない |
| ACTIVITY-S3 | 中 | 場面 S1・S7 | `GET /api/activity` → 行が core、migrate、core-hotfix、research の順。core は `turn`（最後のターン lead）、migrate は `answer`（tester）、core-hotfix は `stalled`（impl-a、定義 core）、research は `working`。`running 4`・`waiting 2`。人の作業用のタブは入らない |
| ACTIVITY-C2 | 中 | 場面 S2 | 呼ばれたことを書く作り物の gy で、`GET /api/activity` と `GET /api/activity/core` を呼ぶ → 一覧は gy を呼ばない。core の問いは q-7c2e（題と選択肢 2 つ）、q-51b9、q-0d4a の 3 件だけで、`gy show` も 3 回だけ。ほかの 52 件は返らない |
| ACTIVITY-C3 | 中 | 場面 S2 | `GET /api/activity/migrate` → 問いは無く、画面の最後の 6 行（1 行目は空行）に `Would you like to run the following command?` と psql の行。40 行まで返る |
| ACTIVITY-E1 | 中 | 不具合: 読めないときに落ちる | `HERGY_HERDR` を無いパスにして、またタブを空にして `GET /api/activity` → 前者は herdr の文と数なし、後者は `items` が空で `running 0` |
| ACTIVITY-E3 | 中 | 不具合: 読めないときに落ちる | 作り物の gy を exit 1 にして、また無いパスにして `GET /api/activity/core` → 前者は gy の文、後者は文なしで問いが空。画面の最後とメンバーはどちらも返る |
| ACTIVITY-C1 | 大 | 場面 S1 | `/activity` を開く → 行を開かずに、core の行の `いま` と `誰` が `人間の番`・`最後のターンlead→人間`、migrate の行が `許可待ち`・`tester` と読める |
| ACTIVITY-P4 | 大 | 不具合: 読み直しで帯が追いつかない・フォーカスが動く | migrate の画面の最後を伸ばしてそこにフォーカスを置き、少し下へ送ったまま、作り物の agents.json の core-lead を working に替えて読み直しを待つ → フォーカス・スクロール・URL は動かず、伸ばした箱は 8 行のまま。帯が `活動 4` `待ち 1` になる |
| ACTIVITY-S7 | 監査 | 見た目 | タブ（`一覧` だけ）・行・開いた中の左右の並びと、クラス名を試作と並べて見る |
| ACTIVITY-T1 | 監査 | 見た目 | 画面の語（`許可待ち` `人間の番` `最後のターン` ほか）と、使わない語（「層」「担当」「役割」「版」「台帳」）が無いことを見る |
| ACTIVITY-W1 | 監査 | 見た目 | 幅 2000・1440・500 と明・暗・`data-theme` で並びと色を見る → 横に流れない、500 で 2 段・時刻なし、psql の行が折り返す、色はトークンだけ |
