# hergy

Herdr の一段上（チーム・役割・手番）だけを扱う CLI です。チームの編成と
起動を YAML で書き、チーム設定とプロファイルを層に分けて解決します。
決まりの中身（共通文書や役割ファイル）は持たず、使う人が自分の設定に
置きます。

## 使う

```bash
go build -o hergy .

# 起動前に、どの層のどのファイルを使うかを確認（Herdr に触れない）
hergy up [チーム] --dry

# チームを起動（Herdr 管理下のシェルペインで）
hergy up [チーム]

# idle な担当へ初期プロンプトを再投入
hergy refresh <担当>          # 1人
hergy refresh --all           # 全員

# ワーカーを1人払い出す
hergy hire <名前> --profile P --purpose X --task "…" [--from 担当]

# 見えるチームと、層を重ねた最終形
hergy ls [--global|--local]
hergy show <チーム>

# プロジェクトの直下（git ルート）を明示したいときは -C を付ける
hergy show <チーム> -C /path/to/project
```

`up` は起動のたびに、どの層のどのファイルを使ったかを担当ごとに表示します
（実際に読んだ役割ファイルと文書だけ。使っていない `profiles.yaml` は「使った」と
出しません）。`show` は層を重ねた最終形と、各値・各文書の出どころを出します。
`ls --global` はグローバルのパスを出し、ローカルにもあるチームには印を付けます。
`up --dry` は出どころを標準エラーへ、展開済みの計画（JSON）だけを標準出力へ出します。

`HERGY_HERDR` に実行ファイルを指定すると、`herdr` を差し替えられます
（偽の herdr で確かめるため）。本物の Herdr に触れるのは、Herdr 管理下の
シェルペイン（`HERDR_ENV=1` と `HERDR_PANE_ID`）で `--dry` を付けずに
実行したときだけです。

## 層

グローバル → ローカル。後の層が勝ちます。hergy は文書や役割を内蔵しません。

| 層 | 置き場 | 中身 |
|---|---|---|
| グローバル | `$XDG_CONFIG_HOME/hergy/`（未設定なら `~/.config/hergy/`） | `profiles.yaml`、`teams/<名前>/team.yaml`、任意で `teams/<名前>/*.md` と `teams/<名前>/roles/` |
| ローカル | `<プロジェクト>/.hergy/` | 同上（`profiles.yaml` は置きません） |

チームは名前だけで呼びます。ローカル（`<プロジェクト>/.hergy/teams/<名前>/team.yaml`）が先、
グローバルが後。同名はローカルが勝ちます。引数なしは、ローカルにチームが
1つだけならそれを使い、それ以外は候補を示して止まります。プロジェクトの
直下は git のルート（無ければカレント）で、`-C <dir>` でも指定できます。

プロファイルはグローバルにだけ置きます。役割ファイルは、チームの `roles` に
並べたディレクトリだけを探し、先に見つかったものを丸ごと使います（混ぜません）。
見つからなければ、探した場所を示して止まります。`roles` を書かないときは、
チームの `roles/` を見ます。

文書は、`docs` に書いたものだけが入ります。書かなければ文書は入りません。
hergy は内蔵の文書を持たないので、`builtin:` という指し方はありません
（書かれていたら理由を示して止まります）。

## team.yaml

```yaml
version: 1            # 必須。無い・違う版は読まずに止まる（migrate は未実装）
team: myteam          # 省略時はディレクトリ名
cwd: .                # プロジェクトの直下（git ルート）基準
docs: [protocol.md]   # このファイルの場所基準
roles: [roles]        # 役割ファイルを探す場所。先に見つかったものを丸ごと使う
members:
  - name: lead
    role: 進行管理
    profile: opus     # か、kind/args/env を直接書く
    may_spawn:        # 明示するだけ。外れても拒まない（注意を出して続ける）
      purposes: [UI試作, 調査]
      profiles: [sonnet, seeker]
```

`docs` を書かない場合は、文書は入りません。`prompt` を空文字にするとエラーです
（書かない、とは区別します）。`args` と `env` の値は文字列だけを受け付けます
（数値・真偽値・null はエラー。数値は引用符で囲んでください）。

## profiles.yaml

```yaml
version: 1
profiles:
  opus:   { kind: claude, args: ["--model", "claude-opus-5-5"] }
  seeker: { kind: codex,  args: ["-m", "gpt-6-luna"] }
  dev:
    kind: opencode
    args: ["--standalone"]
    env:
      OPENCODE_CONFIG_CONTENT: '{"model":"..."}'   # その担当のペインに --env で渡す
```

`args` は argv の要素として1つずつ渡し、シェルは通しません。`env` は
`herdr agent start` に口が無いため、ペインの作成（`tab create` / `pane split`）の
`--env` で渡します。

## 確かめ方

```bash
go build ./... && go vet ./...
```

実機を伴う確認は、`HERGY_HERDR` に偽の herdr を指定し、一時的な
`XDG_CONFIG_HOME` とプロジェクトを作って行います。本物の Herdr での起動は
人間の合図があってからです。