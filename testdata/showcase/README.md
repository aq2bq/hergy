# ショーケースの見本データ

tools/showcase が起こすショーケースの見本データ。場面ごとに 1 つのディレクトリを持ち、
その中の project/、config/、state/、herdr/ を、場面の作業用ディレクトリへ写す。

## 場面

- standard: チーム定義 4+3、ロール 4+3。core はメンバー 5 人・ワーカー 6 種・手番 16 本で、
  どのチームも動いていない。
- running: standard に重ねる層。herdr/ と state/ だけを持ち、偽の herdr が core を
  動かしている扱いにする。state/ の起動記録は core の 5 人分で、impl-a は
  builder.yaml の規則 1 つ、lead はターン 1 本、reviewer は reviewer.yaml の文書が
  古い（伝えていない人 3 人）。analyst と impl-b は今のプロンプトで起動した扱い。
- empty: チーム定義もロールも無い、初めて使う人の状態。
- broken: 壊れた team.yaml と、無いロール（auditor）を指すメンバーを含む。

## ファイルの置き場所

- project/.hergy/: プロジェクトのチーム定義とロール
- config/hergy/: XDG_CONFIG_HOME/hergy（共通のチーム定義・ロール・プロファイル、config.yaml）
- state/hergy/: XDG_STATE_HOME/hergy（活動の「いつから」を覚える activity.json）
- herdr/: 偽の herdr の答え。tabs.json、agents.json、`screens/<エージェント名>.txt`
  - startup-blocked.json: 名前を書いたメンバーは、書いた秒数だけ起動の途中で許可待ちになる
  - startup-failed.json: 名前を書いたメンバーは、起動が書いた文で失敗する

## --mock（試作をショーケースの中で見る）

`go run ./tools/showcase <場面> --mock <ディレクトリ>` は、serve の前に 127.0.0.1 の受け口を
立て、HTML の答えにだけそのディレクトリの CSS と JS を差し込む。試作のファイルは同じ origin の
`/mock/<ファイル名>` で返すので、画面の CSP（`default-src 'self'`）のままで動く。style 属性は
CSP に止められるので、試作は class か CSSOM（`element.style.setProperty`）で書く。

差し込む画面とファイルは、そのディレクトリの `mock.yaml` で決める。置かなければ、その
ディレクトリの `.css` を全部（名前順）そのあと `.js` を全部（名前順）、すべての画面に入れる。

```yaml
version: 1
paths: [/teams]        # 前方一致。/teams は /teams と /teams/… に入る
inject: [a.css, b.js]  # 差し込むファイルと順
```

見本は `mock/sample/mock.js`（要素を 1 つ足すだけ。e2e の SHOWCASE-M1 が見る）。

## 気をつけること

- ここのファイルはショーケースが写す元である。画面から保存したものは、ここへ戻らない。
- standard のデータは testdata/teams-v2 を元に、core の workers を 6 本にし（lead・reviewer に
  2 本ずつ、impl-a・impl-b に 1 本ずつ）、roles/builder.yaml の BUILD-4 の derived を BUILD-1 に
  直したもの。testdata/teams-v2 は変えない。
- 写すときに `@T-<分>@` は、その分前の時刻（RFC 3339）に置き換わる。活動の「いつから」を
  見本らしい長さにするために使う。
- 非公開のプロジェクト名や人の設定を持ち込まない。数と形だけを写す。
