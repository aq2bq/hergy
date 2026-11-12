package words

var referenceRules = []RuleID{"team.identity", "reference.running-against-herdr", "reference.reading-the-output", "reference.web-ui", "reference.resolution", "reference.team-definition-and-member-names", "reference.settings-files-and-their-keys", "reference.rule-documents", "reference.bootstrap-prompt-placeholders"}

func Reference(lang string) string { return Describe(referenceRules, lang) + "\n" }
