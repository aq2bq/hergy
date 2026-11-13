// Package words owns hergy's public operation rules and vocabulary. Renderers
// select records; they never restate an operation's facts.
package words

// TermID identifies a canonical concept in both languages.
type TermID string

// Term names one concept. Meaning is for maintainers, not display text.
type Term struct {
	ID                         TermID
	English, Japanese, Meaning string
}

var terms = []Term{
	{ID: "team-name", English: "team name", Japanese: "チーム名", Meaning: "the name a running team goes by; --as changes it, and it is the first half of the herdr name"},
	{ID: "team-definition-name", English: "team definition name", Japanese: "チーム定義名", Meaning: "the directory name under teams/ that holds team.yaml; the default team name"},
	{ID: "team-definition", English: "team definition", Japanese: "チーム定義", Meaning: "how one team is built: the folder under teams/ that holds team.yaml; of two definitions of a name, the project's own is the one used"},
	{ID: "team", English: "team", Japanese: "チーム", Meaning: "one group started from a team definition; it goes by the team name, and it is what runs on herdr"},
	{ID: "description", English: "description", Japanese: "説明", Meaning: "what team.yaml's description: says: which case the team is meant for. It is one line and required. The team list, a definition's screen, the start sheet and hergy show read it; a team command's header, a running team's tiles, a member's prompt and hergy apply's drift do not"},
	{ID: "member", English: "member", Japanese: "メンバー", Meaning: "one entry of team.yaml's members; its short name is the member name"},
	{ID: "member-name", English: "member name", Japanese: "メンバー名", Meaning: "the short name written in members[].name (lead); hergy puts the team name in front of it"},
	{ID: "herdr-name", English: "herdr name", Japanese: "herdr 名", Meaning: "the name herdr and the agents see: <team>-<member>"},
	{ID: "worker", English: "worker", Japanese: "ワーカー", Meaning: "an agent a member hired with hergy hire, outside the roster"},
	{ID: "layer", English: "layer", Japanese: "層", Meaning: "one of the two settings roots: the project (<project>/.hergy) and outside it ($XDG_CONFIG_HOME/hergy). It stays inside hergy: it is not a word a person reads (I11)"},
	{ID: "project", English: "project", Japanese: "プロジェクト", Meaning: "the directory a team works in, known by its path; settings of its own, when it has any, stand in its .hergy; hergy serve lists projects by their paths and reads each together with the global settings"},
	{ID: "global", English: "global", Japanese: "グローバル", Meaning: "one of the two places a settings file stands in: プロジェクト (<project>/.hergy) or グローバル (~/.config/hergy). A screen writes it beside プロジェクト"},
	{ID: "place", English: "place", Japanese: "置き場所", Meaning: "the two settings roots a person names together: プロジェクト (<project>/.hergy) and グローバル (~/.config/hergy). It is the word a person reads for what layer says inside hergy; a screen's button names the place itself (the project's name, or グローバル), and a list draws a head for each place (d-e77c)"},
	{ID: "preferred", English: "preferred", Japanese: "優先", Meaning: "the mark on the project row when both the project and global places hold the same name; the project's profile, role document or team definition is used"},
	{ID: "inactive", English: "inactive", Japanese: "無効", Meaning: "the mark on the global row when both the project and global places hold the same name; the global profile, role document or team definition is not used because the project's is used"},
	{ID: "profile", English: "profile", Japanese: "プロファイル", Meaning: "a named engine (kind, args, env) in profiles.yaml"},
	{ID: "profile-name", English: "profile name", Japanese: "プロファイル名", Meaning: "the name under profiles: in profiles.yaml; what a member's or worker's profile: names"},
	{ID: "kind", English: "kind", Japanese: "kind", Meaning: "the kind of agent (エージェントの種類) a profile starts, for example claude, codex or opencode (hergy keeps no closed list); written in a profile, not on a member, and the Japanese text is kind too"},
	{ID: "role", English: "role", Japanese: "ロール", Meaning: "what a member acts as; the member reads that role's document"},
	{ID: "role-document", English: "role document", Japanese: "ロールの文書", Meaning: "one role's rule document, roles/<role>.md; every member of that role reads it"},
	{ID: "role-name", English: "role name", Japanese: "ロール名", Meaning: "the file name under roles/ without .md; what team.yaml's role: names"},
	{ID: "team-agreement", English: "team agreement", Japanese: "チームの規則", Meaning: "the rules a team definition gives every member of that team, written in the definition's own folder"},
	{ID: "shared-agreement", English: "shared agreement", Japanese: "全チームの規則", Meaning: "the rules hergy used to read for every team of one place, kept in agreement.yaml beside the teams; it is not read any more (d-31ad), and what is left stops the commands that build a prompt"},
	{ID: "human", English: "human", Japanese: "人間", Meaning: "the one person outside every team; a team definition's turns write it human, and a Japanese rule text writes it 人間"},
	{ID: "turn", English: "turn", Japanese: "ターン", Meaning: "one node's chance to work before handing the turn to another node; the human, a member, or a worker can hold it"},
	{ID: "hands-the-turn-to", English: "hands the turn to", Japanese: "ターンを渡す", Meaning: "the directed relation between two nodes that says which one passes the turn on; a team definition's turns write it <a> -> <b>, and a rule text still writes <a> → <b>"},
	{ID: "node", English: "node", Japanese: "ノード", Meaning: "who receives and hands on the turn; today the human, a member, or a worker, and more kinds can be added"},
	{ID: "edge", English: "edge", Japanese: "エッジ", Meaning: "one node handing the turn to another; a member -> worker edge also carries the purpose and the profile"},
	{ID: "purpose", English: "purpose", Japanese: "目的", Meaning: "what a worker is hired for; workers: pairs each purpose with a member and a profile"},
	{ID: "bootstrap-prompt", English: "bootstrap prompt", Japanese: "初期プロンプト", Meaning: "the first prompt hergy hands a member or a worker"},
	{ID: "apply", English: "apply", Japanese: "反映", Meaning: "put a saved definition's drift into the teams running from it: hergy apply, the screens' 反映, POST /api/apply. hergy decides the way for each member: 送り直し (reprompt), 起動 (start), 起動し直し (refresh), or 反映できない (cannot), where the reason and down・up are named"},
	{ID: "tell", English: "tell", Japanese: "伝える", Meaning: "send one member a message from a person or another member: hergy tell. It is not the word for 反映"},
	{ID: "start", English: "start", Japanese: "起動", Meaning: "begin a team, or a member of one: hergy up starts a team, and hergy apply starts a member the definition has and no agent is live for; 起動し直し (refresh) is starting a member again"},
	{ID: "down", English: "down", Japanese: "終了する", Meaning: "finish a running team and close its panes with hergy down; do not call this チームを止める"},
	{ID: "stop", English: "stop", Japanese: "止める", Meaning: "end one member's current turn with hergy stop; use this for a member's turn, not a team"},
	{ID: "refresh", English: "refresh", Japanese: "起動し直し", Meaning: "end a member's agent and start it again in the same pane, so its conversation starts empty; adding a member to a running team is apply"},
	{ID: "reprompt", English: "reprompt", Japanese: "送り直し", Meaning: "send the bootstrap prompt to a member again, keeping its conversation, with a preamble saying what changed when the definition moved"},
	{ID: "status", English: "status", Japanese: "状態", Meaning: "hergy status: one line per member and worker with its live agent state, then who is waiting on the human"},
	{ID: "waiting-for-permission", English: "waiting for permission", Japanese: "許可待ち", Meaning: "herdr's blocked: an agent is asking the human a question (a permission, or a question while it starts, such as whether to trust a folder) and waits for the answer"},
	{ID: "handed", English: "waiting for a reply", Japanese: "人間の返事待ち", Meaning: "a member that handed the turn to the human and waits for the answer: hergy's handed, not one of herdr's states. It stands beside 許可待ち under 人間を待っている, and the apply sheet, the activity screen and its figure say it the same way"},
	{ID: "working", English: "working", Japanese: "仕事中", Meaning: "a member's or worker's state: its agent is working (herdr's working); a team is working while at least one of its agents is"},
	{ID: "idle", English: "idle", Japanese: "待機中", Meaning: "a member's or worker's state: its agent is live and not working (herdr's idle or done). It is the normal rest between turns, and apply, reprompt and nudge act on it; do not call it 止まっている, which is a team's state"},
	{ID: "not-running", English: "not running", Japanese: "動いていない", Meaning: "a member's state: the definition has it and no agent is live for it"},
	{ID: "stalled", English: "stalled", Japanese: "止まっている", Meaning: "a running team's state: every agent is idle and the one that stopped last handed the turn to no one, so nothing moves until the human acts. It is not a member's state; a member at rest is 待機中"},
	{ID: "workspace", English: "workspace", Japanese: "ワークスペース", Meaning: "herdr's workspace: the set of tabs a team's tab is opened in; one workspace holds one team"},
	{ID: "agent", English: "agent", Japanese: "エージェント", Meaning: "the harness process (claude, codex, opencode) running in a pane for a member or a worker; herdr reports its state"},
	{ID: "prompt", English: "prompt", Japanese: "プロンプト", Meaning: "the text hergy composes from a team definition and hands a member or a worker; the first one handed is the 初期プロンプト"},
	{ID: "topology", English: "topology", Japanese: "トポロジー", Meaning: "the part of a team definition that says who is in the team and who hands the turn to whom (roster, contact, turns, workers), as against its rules; the team definition screen's tab that draws it"},
	{ID: "cheatsheet", English: "cheatsheet", Japanese: "チートシート", Meaning: "hergy cheat's text; --ai prints the agent's one"},
	{ID: "flag", English: "flag", Japanese: "オプション", Meaning: "a --word on the command line"},
	{ID: "rule", English: "rule", Japanese: "規則", Meaning: "the contents of a definition; there are no individual units with IDs"},
	{ID: "rule-note", English: "note", Japanese: "メモ", Meaning: "a legacy YAML note retained in the migration backup; current documents have no separate note field"},
	{ID: "origin", English: "origin", Japanese: "由来", Meaning: "where a rule came from: an incident, an instruction, or another rule it was derived from"},
	{ID: "reason", English: "reason", Japanese: "理由", Meaning: "why a rule is there; written as why, and sent unless send_why is false"},
	{ID: "in-prompt", English: "in the prompt", Japanese: "プロンプトに含める", Meaning: "whether a rule goes into a member's prompt; written in the file as in_prompt: false, and left unsaid when the rule goes in. A rule that is not in the prompt is a rule still: it keeps its id, its origin and its checks, and it is not counted in the amount a member is handed"},
	{ID: "check", English: "check", Japanese: "検証方法", Meaning: "how a rule is shown to have been kept: a report field or a test"},
	{ID: "report-shape", English: "report shape", Japanese: "報告の形", Meaning: "the fields of one report, written under reports"},
	{ID: "incident", English: "incident", Japanese: "インシデント", Meaning: "something that went wrong and put a rule there; one kind of origin"},
	{ID: "activity", English: "activity", Japanese: "アクティビティ", Meaning: "how a running team stands now: who is waiting on the human, what needs deciding, and why it is stopped"},
	{ID: "launch-record", English: "launch record", Japanese: "起動記録", Meaning: "what a running team was started from: the definition and the hash of its content at that time"},
	{ID: "undo", English: "undo", Japanese: "取り消す", Meaning: "take back the one operation just made (the notice's 取り消す and ⌘Z); a change already saved is not brought back by it"},
	{ID: "revert", English: "revert", Japanese: "元に戻す", Meaning: "throw away every change not saved in this panel and write the file's own text again (the save band's 元に戻す)"},
	{ID: "settings-check", English: "settings check", Japanese: "設定の検査", Meaning: "the result of checking the settings; the CLI remains hergy validate"},
	{ID: "revision-number", English: "revision", Japanese: "改訂番号", Meaning: "a document management number not sent to the agent, separate from the format version"},
	{ID: "size-guide", English: "provisional size guide", Japanese: "暫定の目安", Meaning: "the character meter line; exceeding it does not prohibit saving or starting"},
	{ID: "running-column", English: "Running", Japanese: "状態と操作", Meaning: "the column of the team definitions list that says whether a team of that definition is running, and starts or stops it"},
	{ID: "find", English: "Find", Japanese: "探す", Meaning: "the field in the band that finds running teams, team definitions, workspaces, members, roles, rules and profiles across the listed projects and the global settings"},
	{ID: "waiting-for-the-human", English: "waiting for the human", Japanese: "人間を待っている", Meaning: "the group at the top of the activity screen: members waiting for permission and members that handed the turn to the human"},
	{ID: "headquarters", English: "headquarters", Japanese: "本部", Meaning: "hergy serve reading every project listed under projects: in the global config.yaml (d-73fc). The screens use it in the sheet's title 本部に並べるプロジェクト / Projects in the headquarters and in the notice about an unlisted project, and nowhere else"},
}

// Terms returns the vocabulary records in their defined order.
func Terms() []Term { return append([]Term(nil), terms...) }

// Text returns a term in the requested language, falling back to English.
func (id TermID) Text(lang string) string {
	for _, term := range terms {
		if term.ID == id {
			if lang == "ja" {
				return term.Japanese
			}
			return term.English
		}
	}
	panic("unknown term: " + string(id))
}
