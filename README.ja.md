# hergy

[English](README.md) | 日本語

> [!NOTE]
>
> hergyは実験中のプロジェクトです。当面の間は利用方法が大きく変わる恐れがあります。互換性はまだ約束できません。

hergyは[herdr](https://github.com/herdrdev/herdr) + [gy](https://github.com/aq2bq/gy)をベースにした、不確実性の高い活動をAIエージェントチームで乗り切るための小さなCLIです。

hergy は、AI で楽してチームを組み、ものづくりをしたい人のための道具です。誰を、どの役割で、どのモデルで動かすかを YAML に書けば、そのチームを herdr の上に一度に立ち上げます。

## hergy は何をして、何をしないのか

hergy が持つのは、チームの編成と規律を管理、運用するだけで、Claude Code・Codex・OpenCode など各ベンダーが提供するAIエージェントハーネスはそのまま利用します。チーム内のエージェントの連携、状態確認、起動はそのままherdrを利用します。
セッション中のニーズ、論点、要求、受け入れ条件、決定などは全てgyへ記録され、利用されます。

hergy では、ユーザー自身が設定したチームの役割、規律をファイルで管理、運用し、チームの編成、組成を楽にすることを今は目指しています。


## インストール

```sh
go install github.com/aq2bq/hergy@latest
```

[herdr](https://github.com/herdrdev/herdr) と、使うエージェント CLI（`claude`、`codex`、`opencode` など）も必要です。

## はじめかた

lead と worker の2人チームのひな形を書き出します。

```sh
hergy new myteam        # ~/.config/hergy/teams/myteam/ に書き出す（--local でプロジェクトの .hergy/ に）
```

`team.yaml`、`protocol.md`（チームの約束）、`roles/lead.md`、`roles/worker.md` ができます。決まりは1件1行で、すぐ下の HTML コメントに「なぜ」を書いています。HTML コメントはエージェントには送られません。

herdr のシェルペインで実行します。

```sh
hergy up myteam --dry   # 起動せず、担当ごとのプロンプトと読み込むファイルを確認
hergy up myteam         # 起動
```

担当は herdr では `myteam-lead`、`myteam-worker` として動きます。担当名の前にチーム名が自動で付くためです（下記）。

## コマンド

| コマンド | 内容 |
|---|---|
| `hergy new <チーム>` | lead と worker の2人チームのひな形を書き出す |
| `hergy up [チーム]` | チームを起動する。`--dry` で herdr に触れず計画（担当ごとのプロンプトをそのままの文）を出し、`--json` では計画を JSON で出す |
| `hergy down [チーム]` | 生きている担当のペインを閉じる。`--dry` で閉じる予定を出す。チームのタブごとは閉じない |
| `hergy refresh <担当名>` / `--all` | 待機中の担当に初期プロンプトを送り直す |
| `hergy hire <名前> --profile P --purpose X --task "…"` | ワーカーを別タブに起動して仕事を頼む（主に担当が使う） |
| `hergy ls` | 使えるチームと、その層を一覧する |
| `hergy show <チーム>` | 最終的な設定と、各値がどのファイルから来たかを出す |
| `hergy validate` | すべての設定ファイルを検査し、壊れているものを示す |
| `hergy version` | 入っている hergy のコミットと日時を出す |

共通のオプション: `--global` / `--local`（層の指定）、`-C <ディレクトリ>`（プロジェクトの指定）、`--lang ja`（日本語で表示。既定は英語）、`--json`（`--dry` と併用し、計画を文ではなく JSON で出す）。`refresh` と `hire` は `--team` でチームを指定します。`up` / `down` / `refresh` / `hire` は `--as <チーム名>` で、同じ設定を違うチーム名で起動できます（下記）。

## 設定の置き場

| 層 | 置き場 |
|---|---|
| グローバル | `$XDG_CONFIG_HOME/hergy/`（未設定なら `~/.config/hergy/`） |
| ローカル | `<プロジェクト>/.hergy/` |

- チームは `teams/<チーム名>/team.yaml` に置きます。同名ならローカルが優先です
- `profiles.yaml` はどちらの層にも置けます。ローカルのチームはグローバルの上にローカルを重ねて読み（同名ならローカルが優先）、グローバルのチームはグローバルだけを読みます
- グローバルの `config.yaml` に `version: 1` と `lang: ja` を書くと、`--lang` なしでも日本語になります。無ければ `hergy new` が（その実行の表示の言語で）作り、あればそのままにします
- `hergy up <チーム> --dry` は、担当ごとのプロンプトと読み込むファイルを表示します

## team.yaml

```yaml
version: 1              # 必須
team: myteam            # 任意。書くなら設定名（teams/<設定名>/）と一致させる
cwd: .                  # 作業場所（プロジェクトの直下が基準）
docs: [protocol.md]     # 全員に渡す文書。書いた順に連結。ディレクトリなら *.md を名前順に
roles: [roles]          # 役割ファイルを探す場所（省略時は roles/）
prompt: "…"             # 初期プロンプトのテンプレート（任意）
members:
  - name: lead            # 担当名（短い名前）。herdr では「<チーム名>-lead」になる
    role: lead          # roles/lead.md を読む
    profile: opus       # または kind / args / env を直接書く
    may_spawn:          # 払い出してよい目的とプロファイル（プロンプトに書くだけで、強制しない）
      purposes: [調査]
      profiles: [codex]
```

### 1つの設定から複数のチームを並べて起動する

`--as <チーム名>` を付けると、同じ設定を違うチーム名で起動できます。1つの `team.yaml` から2つのチームを並べて動かせます。`--as` を付けなければ、チーム名は設定名（`teams/` の下のディレクトリ名）です。

担当名（`members[].name`）は短い名前（`lead`、`worker`）で書きます。hergy が起動するときにチーム名を付けるので、herdr での名前は `<チーム名>-<担当名>` になります。`demo` の設定なら `demo-lead`、`demo-worker`、`--as dev2` なら `dev2-lead`、`dev2-worker` です。チーム名は自動で付くので、両方に `lead` がいても herdr で衝突しません。担当名に `%{team}` が残っていると止めます（この置換は名前では使わなくなりました）。

チーム名は `[a-z][a-z0-9_-]{0,24}` に、付けた後の `<チーム名>-<担当名>` は `[a-z][a-z0-9_-]{0,31}` に一致させてください。hergy は起動中のチームを記録しないので、`down` / `refresh` / `hire` も同じように指します。コマンドで担当を指すとき（`refresh <担当名>`、`hire --from <担当名>`）は、書いた短い名前を使います:

```sh
hergy up demo --as alpha            # demo の設定を alpha というチーム名で起動
hergy up demo --as beta             # 同じ設定をもう一度 beta として起動
hergy refresh lead --team demo --as alpha   # alpha の lead に送り直す
hergy down demo --as alpha          # alpha の担当だけを閉じる
```

## profiles.yaml

```yaml
version: 1
profiles:
  opus:
    kind: claude                          # herdr のエージェントの種類
    args: ["--model", "claude-opus-5-5"]  # エージェント CLI にそのまま渡す引数
  dev:
    kind: opencode
    env:                                  # ペインに渡す環境変数
      OPENCODE_CONFIG_CONTENT: '{"model":"…"}'
```

## 初期プロンプトの置換

| 置換 | 中身 |
|---|---|
| `%{team}` / `%{role}` / `%{name}` | チーム名、役割、担当の名前（herdr での名前 `<チーム名>-<担当名>`） |
| `%{role_doc}` | 役割ファイル |
| `%{docs}` | `docs` の文書 |
| `%{roster}` | 名簿 |
| `%{herdr}` | herdr での連絡の仕方 |
| `%{spawn}` | 払い出しの許可とコマンド（`may_spawn` がある担当のみ） |

既定のテンプレートは仮のものです。

## これから

- 手番を持っている担当の表示（`hergy turn`）
- 担当どうしのやりとりの定型化と記録（`hergy pass`、`hergy log`）
- 決まりの一覧と、外して比べる仕組み（`hergy rules`）
- 人間への質問の受け取り、役割名での指示（`hergy ask`、`hergy tell`）
- 設定の検査（`hergy validate`）
- 設定の昇格・移行（`hergy promote`、`migrate`）
- チームの入れ子、TUI

## ライセンス

[MIT](LICENSE)
