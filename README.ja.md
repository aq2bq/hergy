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

lead と worker の2人チームのひな形を書き出します。

```sh
hergy new myteam        # ~/.config/hergy/teams/myteam/ に書き出す（--local でプロジェクトの .hergy/ に）
```

`team.yaml`、`protocol.md`（チームの約束）、`roles/lead.md`、`roles/builder.md` ができます。決まりは1件1行で、すぐ下の HTML コメントに「なぜ」を書いています。HTML コメントはエージェントには送られません。

herdr のシェルペインで実行します。

```sh
hergy up myteam --dry   # 起動せず、メンバーごとのプロンプトと読み込むファイルを確認
hergy up myteam         # 起動
```

メンバーは herdr では `myteam-lead`、`myteam-builder` として動きます。メンバー名の前にチーム名が自動で付くためです（下記）。

迷ったら `hergy cheat` が短いチートシートを出します（`hergy cheat --ai` はエージェント向けの全量を英語で出す）。

## コマンド

| コマンド | 内容 |
|---|---|
| `hergy new <チーム定義>` | lead と worker の2人チームのひな形を書き出す |
| `hergy up [チーム定義]` | チーム定義からチームを起動する。チーム名は `--as` で付け、既定はチーム定義名。`--dry` で herdr に触れず計画（メンバーごとのプロンプトをそのままの文）を出し、`--json` では計画を JSON で出す |
| `hergy down [チーム]` | 動いているチームのメンバーと、そのチームが追加したワーカーのペインを閉じる。`--dry` で閉じる予定を出す。タブごとは閉じない |
| `hergy status [チーム]` | チーム名を付けると、メンバーとワーカーを1行ずつ、生きている `agent_status`（working/idle/done/blocked）、生きていなければ `動いていない`、ワーカーには `（ワーカー）` を付けて出す。最後に人間の返事待ち（`blocked`）のメンバーか、誰も動いていないかを出す。チーム名を省くと、動いているチームをすべて、チーム定義名と人数、同じ判定を付けて出す。herdr を読むだけ |
| `hergy refresh <メンバー名>` / `--all` | 動いているチームのメンバーを起動し直す。生きているメンバーはエージェントを終わらせて同じペインで起動し直すので、会話は空から始まる。生きていないメンバーは、hergy が名付けたペインがシェルのままであればそこで起動し直す（ペインが無いか別のものが動いていればエラーにし、`down` のあと `up` を促す）。チーム定義を読み直すので、書き換えた profile・kind・args・prompt が効く（`env` は効かない。下記）。メンバーは自分自身を起動し直せない。別のペインか、人間が実行する |
| `hergy reprompt <メンバー名>` / `--all` | 待機中のメンバーに初期プロンプトを送り直す。会話はそのまま |
| `hergy hire <名前> --profile P --purpose X --task "…"` | ワーカーを別タブ（`worker:<チーム名>/<名前>`）に起動して仕事を頼む（主にメンバーが使う） |
| `hergy ls` | 使えるチーム定義とその層、チーム定義ごとに起動中のチーム名を一覧する |
| `hergy show <チーム定義>` | 最終的なチーム定義と、各値がどのファイルから来たかを出す |
| `hergy validate` | すべての設定ファイルを検査し、壊れているものを示す |
| `hergy version` | 入っている hergy のコミットと日時を出す |
| `hergy cheat` | 短いチートシートを出す。`--ai` はエージェント向けの全量を、設定を読まずに英語で出す |

共通のオプション: `--global` / `--local`（層の指定）、`-C <ディレクトリ>`（プロジェクトの指定）、`--lang ja`（日本語で表示。既定は英語）、`--json`（`--dry` と併用し、計画を文ではなく JSON で出す）。`refresh` / `reprompt` / `hire` は `--team` で対象のチームをチーム名で指定し、`refresh` は `--timeout` も取ります。`up` は `--as <チーム名>` で、同じチーム定義を違うチーム名で起動できます（下記）。

メンバーに初期プロンプトを渡し直す方法は2つあります。`reprompt` は会話をそのままに送り直すので、メンバーが読む文書を直したあとはこちらです。`refresh` はエージェントを終わらせて起動し直すので、会話は空から始まり、書き換えた `kind` / `args` / `profile` / prompt が反映されます。すでに動いていないメンバーは、hergy が名付けたペインがシェルのままであればそこで起動し直します（ペインが無いか別のものが動いていれば何も変えず、`down` のあと `up` を促します）。`env` はペインを作ったときに決まるので、`env` を変えたあとは `hergy down` のあと `hergy up` です。`refresh` はペインと `env` をそのまま保ちます。また `refresh` は、hergy 自身が動いているペイン（`HERDR_PANE_ID`）のメンバーを断ります。止めると hergy が先に終わり、エージェントを起動し直せないためです。別のペインか、人間が実行してください。`refresh --dry` は同じ判定を出すため生きているメンバーを読みます。`reprompt --dry` は何も読みません。

## チーム定義と設定の置き場

| 層 | 置き場 |
|---|---|
| グローバル | `$XDG_CONFIG_HOME/hergy/`（未設定なら `~/.config/hergy/`） |
| ローカル | `<プロジェクト>/.hergy/` |

- チーム定義は `teams/<チーム定義名>/team.yaml` に置きます。同名ならローカルが優先です
- `profiles.yaml` とグローバルの `config.yaml` は設定ファイルです。`profiles.yaml` はどちらの層にも置けます。ローカルのチーム定義はグローバルの上にローカルを重ねて読み（同名ならローカルが優先）、グローバルのチーム定義はグローバルだけを読みます
- グローバルの `config.yaml` に `version: 1` と `lang: ja` を書くと、`--lang` なしでも日本語になります。無ければ `hergy new` が（その実行の表示の言語で）作り、あればそのままにします
- `hergy up <チーム定義> --dry` は、メンバーごとのプロンプトと読み込むファイルを表示します

## チーム定義（team.yaml）

```yaml
version: 1              # 必須
team: myteam            # 任意。書くならチーム定義名（teams/<チーム定義名>/）と一致させる
cwd: .                  # 作業場所（プロジェクトの直下が基準）
docs: [protocol.md]     # 全員に渡す文書。書いた順に連結。ディレクトリなら *.md を名前順に
roles: [roles]          # 役割ファイルを探す場所（省略時は roles/）
prompt: "…"             # 初期プロンプトのテンプレート（任意）
members:
  - name: lead            # メンバー名（短い名前）。herdr では「<チーム名>-lead」になる
    role: lead          # roles/lead.md を読む
    profile: opus       # または kind / args / env を直接書く
    may_spawn:          # 追加してよい目的とプロファイル（プロンプトに書くだけで、強制しない）
      purposes: [調査]
      profiles: [codex]
```

### 1つのチーム定義から複数のチームを並べて起動する

`--as <チーム名>` を付けると、同じチーム定義を違うチーム名で起動できます。1つの `team.yaml` から2つのチームを並べて動かせます。`--as` を付けなければ、チーム名はチーム定義名（`teams/` の下のディレクトリ名）です。

メンバー名（`members[].name`）は短い名前（`lead`、`builder`）で書きます。hergy が起動するときにチーム名を付けるので、herdr での名前は `<チーム名>-<メンバー名>` になります。`demo` のチーム定義なら `demo-lead`、`demo-builder`、`--as dev2` なら `dev2-lead`、`dev2-builder` です。チーム名は自動で付くので、両方に `lead` がいても herdr で衝突しません。メンバー名に `%{team}` が残っていると止めます（この置換は名前では使わなくなりました）。

チーム名は `[a-z][a-z0-9_-]{0,24}` に、付けた後の `<チーム名>-<メンバー名>` は `[a-z][a-z0-9_-]{0,31}` に一致させてください。hergy は起動中のチームを記録しません。チーム名で指すコマンドは、チームの herdr タブ（`team:<チーム名>@<チーム定義名>`）からチーム定義を見つけるので、`down` / `refresh` / `reprompt` / `hire` はどれも動いているチームをチーム名で指します。コマンドでメンバーを指すとき（`refresh <メンバー名>`、`reprompt <メンバー名>`、`hire --from <メンバー名>`）は、書いた短い名前を使います:

```sh
hergy up demo --as alpha            # demo のチーム定義を alpha というチーム名で起動
hergy up demo --as beta             # 同じチーム定義をもう一度 beta として起動
hergy reprompt lead --team alpha    # alpha の lead に、会話を保ったまま送り直す
hergy down alpha                    # alpha のメンバーとワーカーだけを閉じる
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
| `%{team}` / `%{role}` / `%{name}` | チーム名、役割、メンバーの名前（herdr での名前 `<チーム名>-<メンバー名>`） |
| `%{role_doc}` | 役割ファイル |
| `%{docs}` | `docs` の文書 |
| `%{roster}` | 名簿 |
| `%{herdr}` | herdr での連絡の仕方と、仲間の増やし方（`herdr agent start` ではなく `hergy hire`） |
| `%{spawn}` | ワーカーを追加してよい目的と、そのコマンド（`may_spawn` があるメンバーのみ） |

既定のテンプレートは仮のものです。

## これから

- 手番を持っているメンバーの表示（`hergy turn`）
- メンバーどうしのやりとりの定型化と記録（`hergy pass`、`hergy log`）
- 決まりの一覧と、外して比べる仕組み（`hergy rules`）
- 人間への質問の受け取り、役割名での指示（`hergy ask`、`hergy tell`）
- 設定の検査（`hergy validate`）
- 設定の昇格・移行（`hergy promote`、`migrate`）
- チームの入れ子、TUI

## ライセンス

[MIT](LICENSE)
