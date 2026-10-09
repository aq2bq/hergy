// Package prompt builds the bootstrap prompts. A template names its parts with
// %{...}; each part is built only when the template asks for it. A bare % stays
// as it is.
package prompt

import (
	"fmt"
	"strings"
)

// MemberTemplate is the default bootstrap prompt for a team member.
const MemberTemplate = `あなたは %{team} チームの%{role}担当、%{name} です。

## あなたの役割
%{role_doc}
%{docs}
## チームの名簿
%{roster}

## Herdr での連携
%{herdr}
%{spawn}
ここまでを読んだら、次の指示まで待機してください。
`

// WorkerTemplate is the bootstrap prompt for a hired worker.
const WorkerTemplate = `あなたは %{team} チームの%{lead_role}担当 %{lead} が払い出した%{purpose}ワーカー、%{name} です。

## 依頼
%{task}

## Herdr での連携
%{herdr}

依頼が終わったら、%{lead} へ ` + "`herdr agent prompt`" + ` で結果を報告し、待機してください。
`

// Expand replaces every %{name} with the value its builder returns. An unknown
// placeholder is an error; %{ with no closing } and %{} stay literal.
func Expand(template string, parts map[string]func() string) (string, error) {
	var b strings.Builder
	for i := 0; i < len(template); {
		if template[i] == '%' && i+1 < len(template) && template[i+1] == '{' {
			rest := template[i+2:]
			end := strings.IndexByte(rest, '}')
			if end > 0 {
				key := rest[:end]
				build, ok := parts[key]
				if !ok {
					return "", fmt.Errorf("unknown prompt placeholder: %%{%s}", key)
				}
				b.WriteString(build())
				i += 2 + end + 1
				continue
			}
		}
		b.WriteByte(template[i])
		i++
	}
	return b.String(), nil
}

// RosterEntry is one line of the roster.
type RosterEntry struct {
	Name string
	Role string
}

// Roster lists the team, marking the reader.
func Roster(members []RosterEntry, me string) string {
	lines := make([]string, 0, len(members))
	for _, member := range members {
		mark := ""
		if member.Name == me {
			mark = "（あなた）"
		}
		lines = append(lines, fmt.Sprintf("- %s担当: %s%s", member.Role, member.Name, mark))
	}
	return strings.Join(lines, "\n")
}

// Herdr states how to reach the others with Herdr as it is; the launcher adds
// no messaging of its own.
func Herdr(intro string) string {
	return `担当どうしの連絡には Herdr の CLI を使う。相手には名簿の名前で届く。自分のペインに書くことは人間への報告であり、他の担当には届かない。
- 依頼・報告・手番を渡す: ` + "`herdr agent prompt <名前> \"<本文>\"`" + `。本文は「` + intro + `」で始める
- 相手が受け取ったか、終えたかを待つ: ` + "`herdr agent wait <名前> --until working`" + `、または ` + "`--until idle --until done`" + `
- 相手の画面を読む: ` + "`herdr agent read <名前> --source recent --lines 80`" + `
- 人間に判断や確認を求める: 自分のペインに書いたうえで ` + "`herdr notification show \"<件名>\" --body \"<要点>\" --sound request`" + `
- 細かい使い方は ` + "`herdr agent`" + ` と ` + "`herdr notification`" + ` のヘルプで確かめる`
}

// SpawnNote states what a member may spawn. It is written into the prompt and
// never enforced: a hire outside it only prints a note. `command` is the hire
// command line shown to the member, with the placeholders it should fill in.
func SpawnNote(memberName string, purposes, profileNames []string, profileKinds map[string]string, command string) string {
	allowed := make([]string, 0, len(profileNames))
	for _, name := range profileNames {
		allowed = append(allowed, fmt.Sprintf("%s（%s）", name, profileKinds[name]))
	}
	return fmt.Sprintf(`
## ワーカーの払い出し
必要なときは、次の目的でワーカーを払い出してよい: %s
使ってよいプロファイル: %s
払い出し方: `+"`%s`"+`
ワーカーは別のタブで起動し、終わったらあなた（%s）へ報告してくる。ワーカー名は名簿と重ならない英小文字にする。
`, strings.Join(purposes, "、"), strings.Join(allowed, "、"), command, memberName)
}
