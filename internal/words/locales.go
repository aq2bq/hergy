package words

// localeMessages owns translated output that is not an operation rule or a help
// label. It uses the same Text templates and vocabulary as the rule catalog.
var localeMessages = map[string]Text{
	"status.blocked_suffix":       bilingual(" ({{waiting-for-permission}})", "（{{waiting-for-permission}}）"),
	"status.unchanged":            bilingual("{{working}} (unchanged for %d min)", "{{working}}（%d 分変化なし）"),
	"status.can_down":             bilingual("hergy down %s still closes the panes hergy started for this {{team}}", "hergy down %s は、この{{team}}のために hergy が{{start}}したペインを閉じられます"),
	"status.not_live":             bilingual("{{not-running}}", "{{not-running}}"),
	"status.worker_suffix":        bilingual(" ({{worker}})", "（{{worker}}）"),
	"status.human_waiting":        bilingual("waiting for a {{human}}: %s", "{{handed}}: %s"),
	"status.nobody_working":       bilingual("nobody is {{working}}", "誰も動いていません"),
	"status.definition_elsewhere": bilingual("the {{team-definition}} is not in this {{project}}; run hergy status from the {{project}} that booted it", "この{{project}}にない{{team-definition}}です。{{start}}した{{project}}で hergy status を打ってください"),
	"status.none":                 bilingual("no {{team}}s are running", "動いている{{team}}はありません"),
	"status.turn":                 bilingual("{{turn}}", "{{turn}}"),
	"status.turn_holder":          bilingual("%s (handed by %s)", "%s（%s から受けた）"),
	"status.turn_human":           bilingual("the {{human}} (from %s)%s", "{{human}}（%s から）%s"),
	"status.turn_why_silent":      bilingual(": it stopped without writing a line", "。1 行も書かずに止まったため"),
	"status.turn_why_undelivered": bilingual(": the recipient never started", "。渡し先に届かなかったため"),
	"status.turn_why_nobody":      bilingual(": it answered nobody", "。本人が「誰にも」と答えたため"),
	"status.turn_why_unclear":     bilingual(": its answer names no one party", "。本人の答えから渡し先が決まらないため"),
	"status.turn_why_unanswered":  bilingual(": it could not be asked", "。本人に聞けなかったため"),
	"status.turn_holder_asked":    bilingual("%s (handed by %s); asked who it is for", "%s（%s から受けた）。渡し先を聞いています"),
	"status.turn_waits":           bilingual("awaiting", "答え待ち"),
	"status.no_watcher":           bilingual("no watcher is running; hergy turns %s {{start}}s it\n", "見張りが居ません。hergy turns %s で{{start}}できます\n"),
}

// LocaleMessages projects the catalog-owned locale values through the same
// vocabulary expansion as rules and labels. Runtime format slots stay intact.
func LocaleMessages(lang string) map[string]string {
	messages := make(map[string]string, len(localeMessages))
	for key, text := range localeMessages {
		messages[key] = text.In(lang)
	}
	for _, rule := range rules {
		if rule.ErrorKey != "" {
			if _, exists := messages[rule.ErrorKey]; exists {
				panic("two owners of locale key: " + rule.ErrorKey)
			}
			messages[rule.ErrorKey] = rule.Pattern(lang)
		}
	}
	return messages
}
