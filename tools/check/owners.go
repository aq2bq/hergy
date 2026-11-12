package main

// The owners gate holds one concept to one owner (r-ea17, ac-f62f, d-4d88).
//
// Concept ② "一覧の行と件数" has two owners:
//
//   - the server's internal/catalog is the only place that decides which rows a
//     list draws and what the band's numbers are; a list answer and the band ask
//     it (ListedRows / ShownRows / Counts) and never count rows themselves.
//   - the WebUI's lib/band-counts.ts is the only module that writes the band's
//     numbers (setCount / setActivity); a screen hands in what a reading said.
//
// The gate reads the source, not the behaviour: a name that writes the band or
// asks the owner for rows may stand only in the files the rule below names. A
// file that needs the name and is not there is either moved to the owner or
// added to tools/check/owners.yml's allowlist, with why and the decision that
// allowed it (gy's d-…).
//
// It starts as a report and becomes a gate: with enforce: false the findings are
// printed and the gate passes, so the copies can be merged before the gate
// fails. done is the gate failing, which is what enforce: true makes it.

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"go.yaml.in/yaml/v3"
)

// ownersPath is the gate's own settings, from the module root.
const ownersPath = "tools/check/owners.yml"

// ownerRule is one name and the files that may hold it: the owner first, then
// the places that ask it.
type ownerRule struct {
	name  string
	where string // "go" or "ts", which files the name is looked for in
	files []string
	// scope, when set, reads only files under these directory prefixes: a rule
	// about one layer does not list every caller in the other layers.
	scope []string
}

// ownerRules is the table the gate reads. The lists are paths or directory
// prefixes from the module root.
var ownerRules = []ownerRule{
	// The band's numbers are written by band-counts.ts, through the DOM
	// primitives nav.ts holds, and nowhere else.
	{name: "setCount", where: "ts", files: []string{"internal/web/src/lib/nav.ts", "internal/web/src/lib/band-counts.ts"}},
	{name: "setActivity", where: "ts", files: []string{"internal/web/src/lib/nav.ts", "internal/web/src/lib/band-counts.ts"}},
	{name: "applyCounts", where: "ts", files: []string{"internal/web/src/lib/band-counts.ts", "internal/web/src/lib/hq/state.ts"}},
	{name: "applyActivity", where: "ts", files: []string{"internal/web/src/lib/band-counts.ts", "internal/web/src/lib/hq/state.ts"}},
	// The rows and the counts are decided in internal/catalog; the lists and the
	// band's first value ask it. The row rules of r-929e are gone: a name that
	// decided rows anywhere else fails here.
	{name: "Counts", where: "go", files: []string{"internal/catalog/", "internal/ops/hq.go", "internal/web/nav.go"}},
	{name: "ListedRows", where: "go", files: []string{"internal/catalog/", "internal/web/"}},
	{name: "ShownRows", where: "go", files: []string{"internal/catalog/", "internal/web/"}},
	{name: "ReadableOf", where: "go", files: []string{"internal/catalog/", "internal/web/"}},
	{name: "ListedRoles", where: "go", files: []string{"internal/catalog/"}},
	{name: "navRoles", where: "go", files: []string{"internal/catalog/"}},
}

// The danger section: the one table of what a dangerous action does, and the one
// place the confirmation and the undo notice stand (r-ccf8, d-2cce, d-aeeb). A
// screen calls runDanger and marks its button; it keeps no confirmation state of
// its own and builds no notice of its own.
const (
	dangerOwner   = "internal/web/src/lib/danger.ts"
	dangerDialog  = "internal/web/src/lib/confirm.tsx" // askConfirm's definition
	dangerNotice  = "internal/web/src/lib/undo.tsx"    // offerUndo's definition
	dangerMounted = "internal/web/src/lib/dialogs.tsx" // the one <ConfirmDialog
)

// dangerNameRules are the old confirmation names and the two calls only the
// owner makes. confirm is the browser's window.confirm when it stands in a
// screen, which is not the state this section replaced: owners.yml allows those
// two places, with why.
var dangerNameRules = []ownerRule{
	{name: "confirm", where: "ts", files: []string{dangerOwner}},
	{name: "setConfirm", where: "ts", files: []string{dangerOwner}},
	{name: "pressDel", where: "ts", files: []string{dangerOwner}},
	{name: "pressDelNo", where: "ts", files: []string{dangerOwner}},
	{name: "confirming", where: "ts", files: []string{dangerOwner}},
	{name: "askConfirm", where: "ts", files: []string{dangerOwner, dangerDialog}},
	{name: "offerUndo", where: "ts", files: []string{dangerOwner, dangerNotice}},
	// The browser's own confirmation is the same concept: it asks too, and it
	// is asked for by the owner alone (d-021e).
	{name: "window.confirm", where: "ts", files: []string{dangerOwner}},
}

// The prompt section (⑤ r-ba9b): the text handed to a member or a worker is
// built by promptsFor (internal/web/api_prompt.go) and read by prompt-source.ts,
// and nowhere else. The check and the two panels ask the owners; they build no
// text and keep no second reading of it.
const (
	promptBuilder = "internal/web/api_prompt.go"
	promptReader  = "internal/web/src/teams/prompt-source.ts"
)

// promptGoRules are the builder's calls, read only in the WebUI's server: the
// command's own senders (up, hire, apply) are outside this layer and keep their
// calls.
var promptGoRules = []ownerRule{
	{name: "prompt.Member", where: "go", scope: []string{"internal/web/"}, files: []string{promptBuilder}},
	{name: "prompt.Worker", where: "go", scope: []string{"internal/web/"}, files: []string{promptBuilder}},
	{name: "t2PromptOf", where: "go", scope: []string{"internal/web/"}, files: []string{promptBuilder}},
	{name: "prompt.TurnsOf", where: "go", scope: []string{"internal/web/"}, files: []string{promptBuilder}},
	{name: "prompt.WorkersOf", where: "go", scope: []string{"internal/web/"}, files: []string{promptBuilder}},
}

// promptWords are the names the copies used. They stand only in their owner, and
// the ones with no owner at all (an old cache, an old reading) stand nowhere.
var promptWords = []struct {
	name  string
	files []string
}{
	{"wprompts", nil},
	{"wpromptReq", nil},
	{"workerPrompt", nil},
	{"promptsOf", nil},
	{"promptsReady", nil},
	{"memberPrompts", nil},
	{"keepRun", nil},
	{"promptRun", []string{promptReader}},
	{"runOf", []string{promptReader}},
	{"setRun", []string{promptReader}},
}

// promptStrings are the addresses only the reader may name: a screen that calls
// them itself is the second reading the section exists for. The worker check
// body is constructed only by editing; prompt-source asks its semantic operation.
var promptStrings = []struct {
	text  string
	files []string
}{
	{"/api/team/prompt-comparison", []string{promptReader}},
	{"part: 'worker'", []string{"internal/web/src/lib/editing.ts"}},
	{`part: "worker"`, []string{"internal/web/src/lib/editing.ts"}},
}

// ownerAllow is one exception: a name in a file, with why and the decision that
// allowed it.
type ownerAllow struct {
	Path     string `yaml:"path"`
	Name     string `yaml:"name"`
	Why      string `yaml:"why"`
	Decision string `yaml:"decision"`
}

// ownersFile is tools/check/owners.yml.
type ownersFile struct {
	Enforce       bool         `yaml:"enforce"`
	DangerEnforce bool         `yaml:"danger_enforce"`
	PromptEnforce bool         `yaml:"prompt_enforce"`
	Allow         []ownerAllow `yaml:"allow"`
}

// ownersGate reports (and, once enforced, refuses) a name outside its owner.
func (r *runner) ownersGate() error {
	conf, err := r.ownersConf()
	if err != nil {
		return err
	}
	for _, one := range conf.Allow {
		if one.Path == "" || one.Name == "" || one.Why == "" || one.Decision == "" {
			return fmt.Errorf("%s: 許可リストの %q（%s）に、道・名前・理由・決定（d-…）のすべてを書いてください", ownersPath, one.Name, one.Path)
		}
		if !strings.HasPrefix(one.Decision, "d-") {
			return fmt.Errorf("%s: 許可リストの %q の決定は d-… で書いてください: %q", ownersPath, one.Name, one.Decision)
		}
	}
	findings, err := r.ownerFindings(conf)
	if err != nil {
		return err
	}
	danger, err := r.dangerFindings(conf)
	if err != nil {
		return err
	}
	prompt, err := r.promptFindings(conf)
	if err != nil {
		return err
	}

	return ownersVerdict(conf, findings, danger, prompt)
}

// ownersVerdict prints what the sections found and fails the enforced ones.
// Each section turns Hard on its own: a section still reporting prints its
// findings and passes, and an enforced one fails (owners.yml).
func ownersVerdict(conf ownersFile, findings, danger, prompt []string) error {
	sections := []struct {
		findings []string
		enforce  bool
		name     string
	}{
		{findings, conf.Enforce, "enforce"},
		{danger, conf.DangerEnforce, "danger_enforce"},
		{prompt, conf.PromptEnforce, "prompt_enforce"},
	}
	var all []string
	var which strings.Builder
	failed := false
	for _, one := range sections {
		if len(one.findings) == 0 {
			continue
		}
		all = append(all, one.findings...)
		if one.enforce {
			failed = true
		} else {
			which.WriteString(" " + one.name)
		}
	}
	if len(all) == 0 {
		return nil
	}
	for _, finding := range all {
		fmt.Println(finding)
	}
	if failed {
		return fmt.Errorf("持ち主の外で帯の件数を書く・一覧の行を決める・危ない操作の確認や取り消しを書く・渡る文を組み立てる・選んだものと右の列を書く・開いている行の住所を書く・ページを動かす・確認つきの送信を持つ場所が %d 件", len(all))
	}
	fmt.Printf("owners: %d 件（報告のみ。tools/check/owners.yml の%s を true にする前に、持ち主へ寄せるか許可リストに理由と決定を添えてください）\n", len(all), which.String())
	return nil
}

func (r *runner) ownersConf() (ownersFile, error) {
	var out ownersFile
	raw, err := os.ReadFile(filepath.Join(r.root, ownersPath))
	if err != nil {
		return out, err
	}
	if err := yaml.Unmarshal(raw, &out); err != nil {
		return out, fmt.Errorf("%s: %w", ownersPath, err)
	}
	return out, nil
}

// ownerFindings is one line for every place a watched name stands outside its
// owner: the file and line, the name, and where it belongs.
func (r *runner) ownerFindings(conf ownersFile) ([]string, error) {
	files, err := r.sourceFiles()
	if err != nil {
		return nil, err
	}
	var out []string
	for _, path := range files {
		rel, err := filepath.Rel(r.root, path)
		if err != nil {
			return nil, err
		}
		rel = filepath.ToSlash(rel)
		raw, err := os.ReadFile(path)
		if err != nil {
			return nil, err
		}
		// The gate reads code: comments and the bodies of strings say the names
		// without calling them, and the mask keeps the places (reading.go's own
		// reading of a spec).
		code := maskSpec(string(raw))
		goFile := strings.HasSuffix(rel, ".go")
		for _, rule := range ownerRules {
			if rule.where == "go" != goFile {
				continue
			}
			if len(rule.scope) > 0 && !allowed(rel, rule.scope) {
				continue
			}
			if goFile && strings.HasSuffix(rel, "_test.go") {
				continue
			}
			if allowed(rel, rule.files) || allowedBy(rel, conf.Allow, rule.name) {
				continue
			}
			for _, line := range callLines(code, rule.name) {
				out = append(out, fmt.Sprintf("%s:%d: %s はここではなく持ち主（%s）に置いてください。寄せられない場合は %s の allow に理由と決定を添えてください",
					rel, line, rule.name, strings.Join(rule.files, "、"), ownersPath))
			}
		}
	}
	sort.Strings(out)
	return out, nil
}

// sourceFiles is the code the gate reads: the server's Go and the WebUI's
// TypeScript, from the module root.
func (r *runner) sourceFiles() ([]string, error) {
	var out []string
	err := filepath.Walk(filepath.Join(r.root, "internal"), func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			if info.Name() == "node_modules" {
				return filepath.SkipDir
			}
			return nil
		}
		switch {
		case strings.HasSuffix(path, ".go"), strings.HasSuffix(path, ".ts"), strings.HasSuffix(path, ".tsx"):
			out = append(out, path)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Strings(out)
	return out, nil
}

// allowed is whether a path stands in one list of paths or directory prefixes.
func allowed(path string, files []string) bool {
	for _, one := range files {
		if path == one || strings.HasSuffix(one, "/") && strings.HasPrefix(path, one) {
			return true
		}
	}
	return false
}

// allowedBy is whether one allowlist entry covers this name and file.
func allowedBy(path string, entries []ownerAllow, name string) bool {
	for _, one := range entries {
		if one.Path == path && one.Name == name {
			return true
		}
	}
	return false
}

// callLines is the 1-based lines where a name is called or defined: the name at
// a word boundary, followed by ( or =, so a longer name is not it.
func callLines(code, name string) []int {
	var out []int
	for i := 0; i < len(code); {
		j := strings.Index(code[i:], name)
		if j < 0 {
			break
		}
		at := i + j
		before := byte(' ')
		if at > 0 {
			before = code[at-1]
		}
		after := byte(' ')
		end := at + len(name)
		if end < len(code) {
			after = code[end]
		}
		if !isIdentByte(before) && (after == '(' || after == '=') {
			out = append(out, lineOf(code, at))
		}
		i = at + len(name)
	}
	return out
}

// promptFindings is the prompt section: the builder's calls (only in the
// WebUI's server), the names the copies used, and the two addresses only the
// reader names. It reads the WebUI's Go and TypeScript, not the command's.
func (r *runner) promptFindings(conf ownersFile) ([]string, error) {
	files, err := r.sourceFiles()
	if err != nil {
		return nil, err
	}
	var out []string
	for _, path := range files {
		rel, err := filepath.Rel(r.root, path)
		if err != nil {
			return nil, err
		}
		rel = filepath.ToSlash(rel)
		goFile := strings.HasSuffix(rel, ".go")
		if goFile && strings.HasSuffix(rel, "_test.go") {
			continue
		}
		if !goFile && !allowed(rel, []string{"internal/web/"}) {
			continue // the WebUI's screens, not a generated contract or another kind
		}
		raw, err := os.ReadFile(path)
		if err != nil {
			return nil, err
		}
		out = append(out, promptFileFindings(rel, string(raw), goFile, conf)...)
	}
	return out, nil
}

// promptFileFindings reads one file for the prompt section's four kinds of
// copy: a builder call, an old name, and the two addresses a screen may not
// call itself.
func promptFileFindings(rel, raw string, goFile bool, conf ownersFile) []string {
	var out []string
	code := maskSpec(raw)
	if goFile {
		out = append(out, promptBuilderFindings(rel, code, conf)...)
	}
	out = append(out, promptNameFindings(rel, code, conf)...)
	if !goFile {
		out = append(out, promptAddressFindings(rel, raw)...)
	}
	return out
}

// promptBuilderFindings is the builder's calls outside api_prompt.go.
func promptBuilderFindings(rel, code string, conf ownersFile) []string {
	var out []string
	for _, rule := range promptGoRules {
		if len(rule.scope) > 0 && !allowed(rel, rule.scope) {
			continue
		}
		if allowed(rel, rule.files) || allowedBy(rel, conf.Allow, rule.name) {
			continue
		}
		for _, line := range wordLines(code, rule.name) {
			out = append(out, fmt.Sprintf("%s:%d: %s はここではなく %s に置いてください", rel, line, rule.name, promptBuilder))
		}
	}
	return out
}

// promptNameFindings is the names the copies used, back again.
func promptNameFindings(rel, code string, conf ownersFile) []string {
	var out []string
	for _, one := range promptWords {
		if allowed(rel, one.files) || allowedBy(rel, conf.Allow, one.name) {
			continue
		}
		for _, line := range wordLines(code, one.name) {
			out = append(out, fmt.Sprintf("%s:%d: %s は %s に置いてください", rel, line, one.name, promptReader))
		}
	}
	return out
}

// promptAddressFindings is the two addresses only the reader may name.
func promptAddressFindings(rel, raw string) []string {
	var out []string
	for _, one := range promptStrings {
		if allowed(rel, one.files) {
			continue
		}
		for _, line := range stringLines(raw, one.text) {
			out = append(out, fmt.Sprintf("%s:%d: %s を直に呼ばず %s を通してください", rel, line, one.text, promptReader))
		}
	}
	return out
}

// wordLines is the 1-based lines where a whole word stands: the name with no
// identifier character before or after it, so setRun is not setRunning.
func wordLines(code, name string) []int {
	pattern := regexp.MustCompile(`(^|[^A-Za-z0-9_$])` + regexp.QuoteMeta(name) + `([^A-Za-z0-9_$]|$)`)
	var out []int
	for _, at := range pattern.FindAllStringIndex(code, -1) {
		out = append(out, lineOf(code, at[0]))
	}
	return out
}

// stringLines is the 1-based lines where a text stands in the source as written,
// strings included: an address a screen calls itself is a string.
func stringLines(text, want string) []int {
	var out []int
	for i := 0; ; {
		j := strings.Index(text[i:], want)
		if j < 0 {
			return out
		}
		out = append(out, lineOf(text, i+j))
		i += j + len(want)
	}
}

// dangerFindings is the danger section's four rules: the old confirmation names
// and the owner's two calls outside their files, a confirmation dialog mounted
// anywhere but the one place, and a danger button without an act the table
// knows.
func (r *runner) dangerFindings(conf ownersFile) ([]string, error) {
	files, err := r.sourceFiles()
	if err != nil {
		return nil, err
	}
	acts, err := r.dangerActs()
	if err != nil {
		return nil, err
	}
	var out []string
	for _, path := range files {
		rel, err := filepath.Rel(r.root, path)
		if err != nil {
			return nil, err
		}
		rel = filepath.ToSlash(rel)
		if !strings.HasSuffix(rel, ".ts") && !strings.HasSuffix(rel, ".tsx") {
			continue
		}
		raw, err := os.ReadFile(path)
		if err != nil {
			return nil, err
		}
		code := maskSpec(string(raw))
		out = append(out, dangerFileFindings(rel, code, conf)...)
		out = append(out, dangerClosingWarning(rel, string(raw), conf)...)
		out = append(out, dangerAttrFindings(rel, string(raw), acts)...)
	}
	sort.Strings(out)
	return out, nil
}

// dangerFileFindings is one file's names and dialog mounts: the old confirmation
// names, the owner's two calls, and <ConfirmDialog standing only in its one
// place.
func dangerFileFindings(rel, code string, conf ownersFile) []string {
	var out []string
	for _, rule := range dangerNameRules {
		if allowed(rel, rule.files) || allowedBy(rel, conf.Allow, rule.name) {
			continue
		}
		for _, line := range plainCallLines(code, rule.name) {
			out = append(out, fmt.Sprintf("%s:%d: %s は持ち主（%s）に置いてください。寄せられない場合は %s の allow に理由と決定を添えてください",
				rel, line, rule.name, dangerOwner, ownersPath))
		}
	}
	if rel == dangerMounted {
		return out
	}
	for i, line := range strings.Split(code, "\n") {
		if strings.Contains(line, "<ConfirmDialog") {
			out = append(out, fmt.Sprintf("%s:%d: <ConfirmDialog は %s の 1 か所だけに置いてください", rel, i+1, dangerMounted))
		}
	}
	return out
}

// dangerClosingWarning is the browser's beforeunload. It stands in a string,
// which maskSpec blanks, so it is read from the text; only the owner or an
// allowlist entry may hold it (d-3a42).
func dangerClosingWarning(rel, raw string, conf ownersFile) []string {
	if allowed(rel, []string{dangerOwner}) || allowedBy(rel, conf.Allow, "beforeunload") {
		return nil
	}
	var out []string
	for i, line := range strings.Split(raw, "\n") {
		if strings.Contains(line, "beforeunload") && strings.Contains(line, "EventListener(") {
			out = append(out, fmt.Sprintf("%s:%d: ブラウザの確認（beforeunload）は持ち主（%s）の外に置かず、%s の allow に理由と決定を添えてください", rel, i+1, dangerOwner, ownersPath))
		}
	}
	return out
}

// dangerActs is the actions lib/danger.ts's DANGER table names: the only values
// a danger button may carry.
func (r *runner) dangerActs() (map[string]bool, error) {
	raw, err := os.ReadFile(filepath.Join(r.root, dangerOwner))
	if os.IsNotExist(err) {
		// A tree without the owner has no table to hold a button to.
		return map[string]bool{}, nil
	}
	if err != nil {
		return nil, err
	}
	s := string(raw)
	at := strings.Index(s, "const DANGER")
	if at < 0 {
		return nil, fmt.Errorf("%s: DANGER の表がありません", dangerOwner)
	}
	open := strings.IndexByte(s[at:], '{')
	if open < 0 {
		return nil, fmt.Errorf("%s: DANGER の表がありません", dangerOwner)
	}
	open += at
	depth := 0
	for i := open; i < len(s); i++ {
		switch s[i] {
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				out := map[string]bool{}
				for _, one := range regexp.MustCompile(`([A-Za-z_][A-Za-z0-9_]*)\s*:`).FindAllStringSubmatch(s[open+1:i], -1) {
					out[one[1]] = true
				}
				if len(out) == 0 {
					return nil, fmt.Errorf("%s: DANGER の表に操作がありません", dangerOwner)
				}
				return out, nil
			}
		}
	}
	return nil, fmt.Errorf("%s: DANGER の表が閉じていません", dangerOwner)
}

// dangerAttrFindings is the danger buttons' own rule: a button carrying
// data-danger says which action it runs (data-danger-act), and the name is one
// the table knows. The attribute is read from the tag it stands in, braces and
// strings skipped, so a button's onClick does not end the tag early.
func dangerAttrFindings(rel, raw string, acts map[string]bool) []string {
	var out []string
	for i := 0; ; {
		j := strings.Index(raw[i:], "data-danger")
		if j < 0 {
			break
		}
		at := i + j
		end := at + len("data-danger")
		// data-danger-act is the act itself, not a button without one.
		if end < len(raw) && raw[end] == '-' {
			i = end
			continue
		}
		start := strings.LastIndexByte(raw[:at], '<')
		stop := jsxTagEnd(raw, at)
		if start < 0 || stop < 0 {
			i = end
			continue
		}
		line := lineOf(raw, at)
		if value, ok := jsxAttr(raw[start:stop], "data-danger-act"); !ok {
			out = append(out, fmt.Sprintf("%s:%d: data-danger のボタンに data-danger-act がありません。%s の DANGER にある操作の名前を書いてください", rel, line, dangerOwner))
		} else if !acts[value] {
			out = append(out, fmt.Sprintf("%s:%d: data-danger-act の %q は %s の DANGER にありません", rel, line, value, dangerOwner))
		}
		i = stop
	}
	return out
}

// jsxTagEnd is where the tag holding at ends: the > outside braces and strings.
func jsxTagEnd(raw string, at int) int {
	depth := 0
	for i := at; i < len(raw); i++ {
		switch raw[i] {
		case '{':
			depth++
		case '}':
			if depth > 0 {
				depth--
			}
		case '\'', '"', '`':
			quote := raw[i]
			for i++; i < len(raw); i++ {
				if raw[i] == '\\' {
					i++
					continue
				}
				if raw[i] == quote {
					break
				}
			}
		case '>':
			if depth == 0 {
				return i + 1
			}
		}
	}
	return -1
}

// jsxAttr is one attribute's string value in a tag, as data-danger-act writes it.
func jsxAttr(tag, name string) (string, bool) {
	_, after, ok := strings.Cut(tag, name)
	if !ok {
		return "", false
	}
	rest := after
	eq := strings.IndexByte(rest, '=')
	if eq < 0 {
		return "", false
	}
	rest = strings.TrimLeft(rest[eq+1:], " \t")
	if rest == "" || rest[0] != '"' && rest[0] != '\'' {
		return "", false
	}
	quote := rest[0]
	if end := strings.IndexByte(rest[1:], quote); end >= 0 {
		return rest[1 : 1+end], true
	}
	return "", false
}

// plainCallLines is callLines for a name a screen owns itself: a member of
// something else (window.confirm) is not that name (d-021e).
func plainCallLines(code, name string) []int {
	var out []int
	for i := 0; i < len(code); {
		j := strings.Index(code[i:], name)
		if j < 0 {
			break
		}
		at := i + j
		before := byte(' ')
		if at > 0 {
			before = code[at-1]
		}
		after := byte(' ')
		end := at + len(name)
		if end < len(code) {
			after = code[end]
		}
		if !isIdentByte(before) && before != '.' && (after == '(' || after == '=') {
			out = append(out, lineOf(code, at))
		}
		i = at + len(name)
	}
	return out
}
