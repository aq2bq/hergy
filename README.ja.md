# hergy

[English](README.md) | 日本語

> [!NOTE]
>
> hergyは実験中のプロジェクトです。当面の間は利用方法が大きく変わる恐れがあります。互換性はまだ保証できません。

hergyは[herdr](https://github.com/herdrdev/herdr) + [gy](https://github.com/aq2bq/gy)をベースにした、不確実性の高いものづくりをAIエージェントチームで乗り切るための小さなCLIです。

hergy は、AI で楽してチームを組み、ものづくりをしたい人のための道具です。誰を、どのロールで、どのモデルで動かすかを YAML に書けば、そのチームを herdr の上に一度に立ち上げます。

## hergy は何をして、何をしないのか

hergy が持つのは、チームの編成と規律を管理、運用するだけで、Claude Code・Codex・OpenCode など各ベンダーが提供するAIエージェントハーネスはそのまま利用します。チーム内のエージェントの連携、状態確認、起動はそのままherdrを利用します。
セッション中のニーズ、論点、要求、受け入れ条件、決定などは全てgyへ記録され、利用されます。

hergy では、ユーザー自身が設定したチームのロール、規律をファイルで管理、運用し、チームの編成、組成を楽にすることを今は目指しています。

## インストール

```sh
go install github.com/aq2bq/hergy@latest

# 任意: AIエージェントが読む hergy スキルを入れる
npx skills add aq2bq/hergy
```

[herdr](https://github.com/herdrdev/herdr) と、使うエージェント CLI（`claude`、`codex`、`opencode` など）も必要です。

### hergy について伝える

以下のような文をプロンプトや AGENTS.md / CLAUDE.md へ書くなりして伝わるようにしておくと、AIエージェントは hergy のチームを起動・片付けるとき、チーム定義を書くとき、hergy が起動したチームの中で働くときに hergy スキルを読みます。

```
hergy のチームを起動・片付けるとき、チーム定義を書くとき、hergy が起動したチームの中で働くときは hergy スキルを読む。
```

## はじめかた

まずチーム定義のひな形を作ります。

```sh
hergy new myteam
```

herdr のシェルペインで実行します。

```sh
hergy up myteam --dry
hergy up myteam
```

迷ったら `hergy cheat` か `hergy <コマンド> --help` を読んでください。

## チーム定義（team.yaml）

```yaml
version: 1
description: "このチームが想定するケース"
team: myteam
cwd: .

prompt: "…"
members:
  - name: lead
    role: lead
    profile: opus
workers:
  - from: lead
    purpose: 調査
    profile: opus
turns:
  - human -> lead
  - lead -> human
```

## profiles.yaml

```yaml
version: 1
profiles:
  opus:
    kind: claude
    args: ["--model", "claude-opus-5-5"]
  dev:
    kind: opencode
    env:
      OPENCODE_CONFIG_CONTENT: '{"model":"…"}'
```

## これから

- ターンを持っているメンバーの表示（`hergy turn`）
- メンバーどうしのやりとりの定型化と記録（`hergy pass`、`hergy log`）
- 決まりの一覧と、外して比べる仕組み（`hergy rules`）
- 人間への質問の受け取り、ロール名での指示（`hergy ask`、`hergy tell`）
- 設定の検査（`hergy validate`）
- 設定の昇格・移行（`hergy promote`、`migrate`）
- チームの入れ子、TUI

## 変更を送るとき

変更を送る前に `go run ./tools/check diff` を一度実行してください。手元では upstream からの
変更に応じた速い検査を 1 分ほどで回し、変更が選ぶブラウザの歩き（spec）を名指しします
（`go run ./tools/check e2e tests/<名前>.spec.ts` でその歩きだけを実行できます）。CI は push
ごとに、ブラウザの歩きと race 検出を含む全品質ゲートを実行します。失敗した段を修正した後は、
その段だけを実行します。修正範囲が別の段に広がったときは diff を実行し直します。
検査の一覧は `go run ./tools/check -h` で確認できます。

## ライセンス

[MIT](LICENSE)
