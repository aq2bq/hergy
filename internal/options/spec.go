package options

import "sort"

// Use says one command accepts one flag, and what the flag means there. The
// same flag reads differently from command to command (up --dry prints the boot
// plan, down --dry prints what would close), so the description belongs to the
// command, and there is no description to fall back on: a flag added to a
// command without one fails the tests.
type Use struct {
	Flag     *Flag
	Desc     Text
	Required bool // the command refuses to run without it (hire --purpose, --task)
}

// Arg is one positional argument.
type Arg struct {
	Name     Text // without the brackets: "definition"
	Desc     Text
	Optional bool  // written [name] rather than <name>
	Or       *Flag // a flag that stands in for it: refresh <member>|--all
}

// Spec is one command: the one place its words, its arguments and the flags it
// accepts are written. The parser, hergy <command> --help and the cheatsheet's
// command lines all read it.
type Spec struct {
	Kind    Kind
	Name    string // the command word
	Summary Text   // one line; the same sentence as the command's row in hergy --help
	Detail  Text   // optional: a line or two under the usage line of its --help
	Args    []Arg
	Flags   []Use // the flags of this command, in the order --help shows them; never --lang or --help
	Bare    bool  // version: answers before any flag is read, has no help page of its own
}

// common is what every command but version accepts. It is written once, so a
// command's Flags leaves it out.
var common = []Use{
	{Flag: flagLang, Desc: text("Language of the output (default en)", "表示の言語（既定は en）")},
	{Flag: flagHelp, Desc: text("Show this text", "この説明を表示する")},
}

func use(f *Flag, desc Text) Use { return Use{Flag: f, Desc: desc} }

func required(f *Flag, desc Text) Use { return Use{Flag: f, Desc: desc, Required: true} }

var (
	descDir = text(
		"The project root to resolve against\n(default: the git root, else the current directory)",
		"プロジェクトの場所を指定する\n（既定は git のルート。なければ今のディレクトリ）")
	descJSON    = text("With --dry, print the plan as JSON (--dry is required)", "--dry の計画を JSON で表示する（--dry が必要）")
	descTimeout = text("Startup timeout in milliseconds, 3001..300000 (default 30000)", "起動を待つ時間（ミリ秒。3001〜300000、既定 30000）")
	descTeam    = text(
		"The team to act on, by team name\n(left out: the one running team)",
		"対象のチーム名\n（動いているチームが1つだけなら省ける）")
)

// specs is every command, in the order the cheatsheet lists them.
var specs = []Spec{
	{
		Kind: Up, Name: "up",
		Summary: text("Start a team", "チームを起動する"),
		Detail: text(
			"Boot a team definition in a new herdr tab: one pane per member, each handed its bootstrap prompt.",
			"チーム定義を新しいタブに起動する。メンバーごとにペインを作り、初期プロンプトを渡す。"),
		Args: []Arg{{
			Name: text("definition", "チーム定義"), Optional: true,
			Desc: text(
				"The team definition to start\n(left out: the lone project one)",
				"起動するチーム定義の名前\n（省くと、プロジェクトのチーム定義が1つならそれ）"),
		}},
		Flags: []Use{
			use(flagAs, text(
				"Start the team under this team name (default: the team definition name)\nIt is the first half of the herdr name",
				"このチーム名で起動する（既定はチーム定義名）\nherdr 名の前半になる")),
			use(flagDry, text(
				"Print the plan (each member's prompt as text) and start nothing\nIt never reads herdr",
				"起動せず、計画（メンバーごとのプロンプト）だけを表示する\nherdr は読まない")),
			use(flagJSON, descJSON),
			use(flagTimeout, descTimeout),
			use(flagDir, descDir),
		},
	},
	{
		Kind: Down, Name: "down",
		Summary: text("Stop a team, and close the workers it hired", "チームを終了する。hire したワーカーも閉じる"),
		Detail: text(
			"Close the panes of the running team's live members and of the workers it hired.",
			"動いているチームのメンバーと、hire したワーカーのペインを閉じる。"),
		Args: []Arg{{
			Name: text("team", "チーム"), Optional: true,
			Desc: text("The team name (left out: the one running team)", "チーム名（動いているチームが1つだけなら省ける）"),
		}},
		Flags: []Use{
			use(flagDry, text(
				"Print what would close and close nothing\nIt reads herdr's live agents and tabs",
				"閉じる予定だけを表示し、何も閉じない\nherdr の今のエージェントとタブを読む")),
			use(flagDir, descDir),
		},
	},
	{
		Kind: Status, Name: "status",
		Summary: text("Show how the members and workers are doing", "メンバーとワーカーの状態を見る"),
		Detail: text(
			"One line per member and worker with its live herdr state, then who is waiting for a human.",
			"メンバーとワーカーごとに herdr の今の状態を1行ずつ出し、\n最後に、人の返事を待っている者がいるかを判定する。"),
		Args: []Arg{{
			Name: text("team", "チーム"), Optional: true,
			Desc: text("The team name (left out: every running team)", "チーム名（省くと、動いているチームをすべて一覧する）"),
		}},
		Flags: []Use{
			use(flagDir, descDir),
		},
	},
	{
		Kind: Refresh, Name: "refresh",
		Summary: text("Restart a member; its conversation starts empty", "メンバーを起動し直す。会話は消える"),
		Detail: text(
			"End each selected member's agent and start it again in the same pane. Give a member or --all.",
			"選んだメンバーのエージェントを終わらせ、同じペインで起動し直す。\nメンバーか --all のどちらかを指定する。"),
		Args: []Arg{{
			Name: text("member", "メンバー"), Or: flagAll,
			Desc: text(
				"The member to restart, by short member name or herdr name",
				"起動し直すメンバー（短いメンバー名か herdr 名）"),
		}},
		Flags: []Use{
			use(flagAll, text("Restart every member (not with a member)", "全メンバーを起動し直す（メンバーとは同時に指定できない）")),
			use(flagTeam, descTeam),
			use(flagDry, text(
				"Print what would restart and change nothing\nIt reads herdr's live agents and tabs, and refuses\nyour own pane as a real run does",
				"起動し直す予定だけを表示し、何も変えない\nherdr の今の状態を読み、自分のペインは本番と同じく断る")),
			use(flagJSON, descJSON),
			use(flagTimeout, descTimeout),
			use(flagDir, descDir),
		},
	},
	{
		Kind: Reprompt, Name: "reprompt",
		Summary: text("Send the bootstrap prompt again; the conversation stays", "初期プロンプトを送り直す。会話は残る"),
		Detail: text(
			"Re-send the bootstrap prompt to members that are idle or done. It starts nothing.\nGive a member or --all.",
			"待機中（idle か done）のメンバーに初期プロンプトを送り直す。何も起動しない。\nメンバーか --all のどちらかを指定する。"),
		Args: []Arg{{
			Name: text("member", "メンバー"), Or: flagAll,
			Desc: text(
				"The member to send it to, by short member name or herdr name",
				"送り直すメンバー（短いメンバー名か herdr 名）"),
		}},
		Flags: []Use{
			use(flagAll, text("Send it to every member (not with a member)", "全メンバーに送り直す（メンバーとは同時に指定できない）")),
			use(flagTeam, descTeam),
			use(flagDry, text(
				"Print what would be sent and send nothing\nIt never reads herdr",
				"送る予定だけを表示し、何も送らない\nherdr は読まない")),
			use(flagJSON, descJSON),
			use(flagDir, descDir),
		},
	},
	{
		Kind: Hire, Name: "hire",
		Summary: text("Add a worker", "ワーカーを追加する"),
		Detail: text(
			"A member starts a worker of its own in a tab of its own and hands it a task.",
			"メンバーが、自分のワーカーを別タブに起動して依頼を渡す。"),
		Args: []Arg{{
			Name: text("name", "名前"),
			Desc: text(
				"The worker's name (not a member name or a herdr name already in use)",
				"ワーカーの名前（メンバー名とも、使用中の herdr 名とも重ならないこと）"),
		}},
		Flags: []Use{
			required(flagPurpose, text(
				"What the worker is for (required)\nA purpose the hiring member's workers: names;\nthe profile comes from that entry",
				"ワーカーの目的（必須）\n呼ぶメンバーの workers: が名指す目的であること。\nプロファイルはその組から決まる")),
			required(flagTask, text("The task text handed to the worker (required)", "ワーカーに渡す依頼の文（必須）")),
			use(flagFrom, text(
				"The member that hires the worker (short member name or herdr name)\nLeft out: required unless exactly one member hires for this purpose",
				"ワーカーを追加するメンバー（短いメンバー名か herdr 名）\nこの目的のワーカーを持つメンバーが1人だけなら省ける")),
			use(flagTeam, descTeam),
			use(flagTimeout, descTimeout),
			use(flagDry, text(
				"Print the plan and start nothing\nIt never reads herdr",
				"起動せず、計画だけを表示する\nherdr は読まない")),
			use(flagJSON, descJSON),
			use(flagDir, descDir),
		},
	},
	{
		Kind: Ls, Name: "ls",
		Summary: text("List the team definitions", "チーム定義の一覧を見る"),
		Detail: text(
			"List the visible team definitions and the layer each came from,\nand the running team names each one booted.",
			"見えるチーム定義と、それがどの層のものか、そこから動いているチーム名を一覧する。"),
		Flags: []Use{
			use(flagDir, descDir),
		},
	},
	{
		Kind: Show, Name: "show",
		Summary: text("Show the final team definition and where each value came from", "最終的なチーム定義と、その出どころを見る"),
		Detail: text(
			"Print the effective team definition and where each value and document came from.",
			"効いているチーム定義と、各値と文書の出どころを表示する。"),
		Args: []Arg{{
			Name: text("definition", "チーム定義"),
			Desc: text("The team definition to show", "表示するチーム定義の名前"),
		}},
		Flags: []Use{
			use(flagDir, descDir),
		},
	},
	{
		Kind: New, Name: "new",
		Summary: text("Write a starter team definition", "チーム定義のひな形を作る"),
		Detail: text(
			"Write team.yaml and agreement.yaml, and the layer's roles/. An existing team is never overwritten.\n"+
				"It writes into the project's .hergy/ inside a git repository, and into the global layer otherwise.\n"+
				"If the global config.yaml (version and lang) is missing, it is written too.",
			"team.yaml・agreement.yaml と層の roles/ を書き出す。既にあるチーム定義は上書きしない。\n"+
				"書き先は、git リポジトリの中ならプロジェクトの .hergy/、外ならグローバル。\n"+
				"グローバルの config.yaml（version と lang）は、無いときだけ作る。"),
		Args: []Arg{{
			Name: text("definition", "チーム定義"),
			Desc: text(
				"The name of the team definition to write ([a-z][a-z0-9_-]{0,24})",
				"作るチーム定義の名前（[a-z][a-z0-9_-]{0,24}）"),
		}},
		Flags: []Use{
			use(flagDir, descDir),
		},
	},
	{
		Kind: Validate, Name: "validate",
		Summary: text("Check the settings files", "設定ファイルを検査する"),
		Detail: text(
			"Check the global config.yaml, both layers' profiles.yaml and every team definition.\nExit 1 if any is broken.",
			"グローバルの config.yaml、両方の層の profiles.yaml、\nすべてのチーム定義を検査する。壊れたものがあれば終了コード 1。"),
		Flags: []Use{
			use(flagDir, descDir),
		},
	},
	{
		Kind: Serve, Name: "serve",
		Summary: text("Open the web UI in a browser", "ブラウザで WebUI を開く"),
		Detail: text(
			"Serve the screens on 127.0.0.1, in the foreground, and print the URL. The URL is then\nopened in this machine's default browser. Nothing but this machine can reach it, and no\nwrite leaves the settings of the layers it shows.",
			"127.0.0.1 で画面を配り、URL を 1 行表示して、前景で動き続ける。表示した URL は、\nこの機械の既定のブラウザで開く。この機械の外からは届かず、書き込みは表示する層の設定の外へ出ない。"),
		Flags: []Use{
			use(flagPort, text(
				"The port to listen on (default 8787; 0 lets the operating system pick a free one)",
				"待ち受けるポート（既定は 8787。0 は空きポートを OS が選ぶ）")),
			use(flagNoOpen, text(
				"Print the URL and open no browser",
				"URL を出すだけで、ブラウザを開かない")),
			use(flagDir, descDir),
		},
	},
	{
		Kind: Version, Name: "version", Bare: true,
		Summary: text("Show the version", "バージョンを見る"),
	},
	{
		Kind: Cheat, Name: "cheat",
		Summary: text("Show the cheatsheet", "チートシートを見る"),
		Detail: text(
			"Print the short cheatsheet. It reads no team settings, and prints even when they are broken.",
			"短いチートシートを表示する。チーム定義などの設定は読まず、壊れていても表示する。"),
		Flags: []Use{
			use(flagAI, text(
				"Print the whole of hergy for an agent instead (always English)",
				"代わりに、AI 向けの全量（英語）を表示する")),
		},
	},
}

// allowed is the flags each command accepts, read from specs and kept as a
// lookup. Anything else is an error, so a flag that belongs to another command
// never gets silently ignored.
var allowed = buildAllowed()

func buildAllowed() map[Kind]map[string]bool {
	out := map[Kind]map[string]bool{}
	for _, s := range specs {
		if s.Bare {
			continue
		}
		names := map[string]bool{}
		for _, u := range s.Flags {
			names[u.Flag.Name] = true
		}
		for _, u := range common {
			names[u.Flag.Name] = true
		}
		out[s.Kind] = names
	}
	return out
}

// Kinds lists every command kind, Help aside, in the order the cheatsheet shows
// them. It comes from specs, so a command added there is picked up by the
// cheatsheet tests.
var Kinds = kindsOf(specs)

func kindsOf(list []Spec) []Kind {
	kinds := make([]Kind, len(list))
	for i, s := range list {
		kinds[i] = s.Kind
	}
	return kinds
}

// specOf is the table row of a command kind, or nil for Help.
func specOf(kind Kind) *Spec {
	for i := range specs {
		if specs[i].Kind == kind {
			return &specs[i]
		}
	}
	return nil
}

// specNamed is the table row for a command word, or nil.
func specNamed(name string) *Spec {
	for i := range specs {
		if specs[i].Name == name {
			return &specs[i]
		}
	}
	return nil
}

// Commands returns every command word, in the order Kinds lists them.
func Commands() []string {
	names := make([]string, len(specs))
	for i, s := range specs {
		names[i] = s.Name
	}
	return names
}

// Flags returns the flags one command accepts, --lang and --help included: the
// names the parser lets through. A cheatsheet test checks it names every flag.
// The names are the long spellings, without the leading dashes, sorted.
func Flags(kind Kind) []string {
	out := make([]string, 0, len(allowed[kind]))
	for name := range allowed[kind] {
		out = append(out, name)
	}
	sort.Strings(out)
	return out
}

// OwnFlags returns the flags that belong to one command, without the --lang
// and --help every command has. It is what the command's own line in a
// cheatsheet has to name.
func OwnFlags(kind Kind) []string {
	s := specOf(kind)
	if s == nil {
		return nil
	}
	out := make([]string, 0, len(s.Flags))
	for _, u := range s.Flags {
		out = append(out, u.Flag.Name)
	}
	sort.Strings(out)
	return out
}

func commandName(kind Kind) string {
	if s := specOf(kind); s != nil {
		return s.Name
	}
	return "help"
}
