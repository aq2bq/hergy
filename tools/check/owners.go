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
	{name: "Counts", where: "go", files: []string{"internal/catalog/", "internal/app/hq.go", "internal/web/nav.go"}},
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
// them itself is the second reading the section exists for.
var promptStrings = []struct {
	text  string
	files []string
}{
	{"/api/team/prompt-comparison", []string{promptReader}},
	{"part: 'worker'", []string{promptReader}},
	{`part: "worker"`, []string{promptReader}},
}

// The selection section (⑥ r-3d54): the pick and the right column are written
// by teams/selection.ts alone, and the table that decides where the column goes
// is selection-table.ts. A screen reads the pick through pickOf and the column
// through sideOf; the old writer (selectPick) and the old reads (livePick,
// selOf) stand nowhere, and sideOf is defined only in the owner.
const (
	selectionOwner = "internal/web/src/teams/selection.ts"
	selectionTable = "internal/web/src/teams/selection-table.ts"
)

// selectionRemoved are the names the owner replaced: a screen that brings one
// back is the copy this section exists for.
var selectionRemoved = []string{"selectPick", "livePick", "selOf"}

// selectionWrite matches a write of the two fields on any record: r.sel.value =
// and r.side.value =. A comparison (===) is a read, not a write.
var selectionWrite = regexp.MustCompile(`\.(sel|side)\.value\s*=(?:[^=]|$)`)

// sideOfDefinition matches where sideOf is defined, not where it is called: the
// screens may call it, the definition stands in the owner.
var sideOfDefinition = regexp.MustCompile(`\b(function\s+sideOf|const\s+sideOf\s*[=:])`)

// The address section (⑥-2, d-a143): the rows a screen has open are written and
// read by lib/address.ts alone. A screen asks it for the open rows; it does not
// spell the address itself.
const addressOwner = "internal/web/src/lib/address.ts"

// addressKey is the two spellings a screen would write by itself: the open value
// set and read. Both quote kinds; the receiver's name does not matter.
var addressKey = regexp.MustCompile(`\.(set|get)\(\s*['"]open['"]`)

// changedOwner is the one module that decides 「変えたか」 (r-1e9a).
const changedOwner = "internal/web/src/lib/changed.ts"

// changedNames are the comparators the owner replaced. A definition or a call
// outside changed.ts fails once changed_enforce is true.
var changedNames = []string{
	"diffCount", "changeCount", "agreeDiff", "bornOf", "fileDirty",
	"roleDraftsDirty", "rowDirty", "changedOf", "isDirtyPanel", "sameBorn", "pickChanged",
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
	Enforce             bool         `yaml:"enforce"`
	ChangedEnforce      bool         `yaml:"changed_enforce"`
	DangerEnforce       bool         `yaml:"danger_enforce"`
	PromptEnforce       bool         `yaml:"prompt_enforce"`
	SelectionEnforce    bool         `yaml:"selection_enforce"`
	AddressEnforce      bool         `yaml:"address_enforce"`
	PagePositionEnforce bool         `yaml:"page_position_enforce"`
	Allow               []ownerAllow `yaml:"allow"`
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
	changed, err := r.changedFindings(conf)
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
	selection, err := r.selectionFindings(conf)
	if err != nil {
		return err
	}
	address, err := r.addressFindings(conf)
	if err != nil {
		return err
	}
	page, err := r.pagePositionFindings(conf)
	if err != nil {
		return err
	}
	return ownersVerdict(conf, append(findings, changed...), danger, prompt, selection, address, page)
}

// ownersVerdict prints what the sections found and fails the enforced ones.
// Each section turns Hard on its own: a section still reporting prints its
// findings and passes, and an enforced one fails (owners.yml).
func ownersVerdict(conf ownersFile, findings, danger, prompt, selection, address, page []string) error {
	all := append(append(append(append(append(append([]string{}, findings...), danger...), prompt...), selection...), address...), page...)
	if len(all) == 0 {
		return nil
	}
	for _, finding := range all {
		fmt.Println(finding)
	}
	failed := len(findings) > 0 && conf.Enforce || len(danger) > 0 && conf.DangerEnforce ||
		len(prompt) > 0 && conf.PromptEnforce || len(selection) > 0 && conf.SelectionEnforce ||
		len(address) > 0 && conf.AddressEnforce || len(page) > 0 && conf.PagePositionEnforce
	if !failed {
		which := ""
		if len(findings) > 0 && !conf.Enforce {
			which += " enforce"
		}
		if len(danger) > 0 && !conf.DangerEnforce {
			which += " danger_enforce"
		}
		if len(prompt) > 0 && !conf.PromptEnforce {
			which += " prompt_enforce"
		}
		if len(selection) > 0 && !conf.SelectionEnforce {
			which += " selection_enforce"
		}
		if len(address) > 0 && !conf.AddressEnforce {
			which += " address_enforce"
		}
		if len(page) > 0 && !conf.PagePositionEnforce {
			which += " page_position_enforce"
		}
		fmt.Printf("owners: %d 件（報告のみ。tools/check/owners.yml の%s を true にする前に、持ち主へ寄せるか許可リストに理由と決定を添えてください）\n", len(all), which)
		return nil
	}
	return fmt.Errorf("持ち主の外で帯の件数を書く・一覧の行を決める・危ない操作の確認や取り消しを書く・渡る文を組み立てる・選んだものと右の列を書く・開いている行の住所を書く・ページを動かす場所が %d 件", len(all))
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

// selectionFindings is the selection section: a write of the pick or the right
// column outside the owner, an old writer or reader back again, and sideOf
// defined anywhere but the owner.
func (r *runner) selectionFindings(conf ownersFile) ([]string, error) {
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
		if !strings.HasSuffix(rel, ".ts") && !strings.HasSuffix(rel, ".tsx") {
			continue
		}
		if rel == selectionOwner || rel == selectionTable {
			continue
		}
		raw, err := os.ReadFile(path)
		if err != nil {
			return nil, err
		}
		code := maskSpec(string(raw))
		out = append(out, selectionFileFindings(rel, code, conf)...)
	}
	sort.Strings(out)
	return out, nil
}

// selectionFileFindings reads one file outside the owner: the two writes, the
// names the owner replaced, and a second definition of sideOf.
func selectionFileFindings(rel, code string, conf ownersFile) []string {
	var out []string
	for i, line := range strings.Split(code, "\n") {
		field := selectionWrite.FindStringSubmatch(line)
		if field == nil || allowedBy(rel, conf.Allow, "r."+field[1]+".value") {
			continue
		}
		out = append(out, fmt.Sprintf("%s:%d: r.%s を書くのは持ち主（%s）だけです。何が起きたかを select に渡してください。寄せられない場合は %s の allow に理由と決定を添えてください",
			rel, i+1, field[1], selectionOwner, ownersPath))
	}
	for _, name := range selectionRemoved {
		if allowedBy(rel, conf.Allow, name) {
			continue
		}
		for _, line := range wordLines(code, name) {
			out = append(out, fmt.Sprintf("%s:%d: %s は持ち主（%s）に置いてください。寄せられない場合は %s の allow に理由と決定を添えてください",
				rel, line, name, selectionOwner, ownersPath))
		}
	}
	if allowedBy(rel, conf.Allow, "sideOf") {
		return out
	}
	for i, line := range strings.Split(code, "\n") {
		if sideOfDefinition.MatchString(line) {
			out = append(out, fmt.Sprintf("%s:%d: sideOf の定義は持ち主（%s）に 1 つだけ置いてください（呼ぶのは自由です）", rel, i+1, selectionOwner))
		}
	}
	return out
}

// addressFindings is the address section: a screen that spells the open value
// itself instead of asking lib/address.ts.
func (r *runner) addressFindings(conf ownersFile) ([]string, error) {
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
		if rel == addressOwner {
			continue
		}
		if !strings.HasSuffix(rel, ".ts") && !strings.HasSuffix(rel, ".tsx") {
			continue
		}
		raw, err := os.ReadFile(path)
		if err != nil {
			return nil, err
		}
		// The open value is a string, which maskSpec blanks: the rule reads the
		// text, the way the prompt section reads its addresses.
		for i, line := range strings.Split(string(raw), "\n") {
			if !addressKey.MatchString(line) || allowedBy(rel, conf.Allow, "open") {
				continue
			}
			out = append(out, fmt.Sprintf("%s:%d: open を自分で書いたり読んだりせず、持ち主（%s）の writeOpen・readOpen を通してください。寄せられない場合は %s の allow に理由と決定を添えてください",
				rel, i+1, addressOwner, ownersPath))
		}
	}
	sort.Strings(out)
	return out, nil
}

// The page-position section (⑦ r-6ff8 / d-c5ce / d-a504): lib/page-position.ts
// moves the page (window) and sends the wheel, and nowhere else does. A panel
// may scroll its own content only inside itself, and only the panels owners.yml
// names may do it; a focus never scrolls the page to the field it lands in.
const pagePositionOwner = "internal/web/src/lib/page-position.ts"

// pagePositionMethods are the browser calls that move a page or an element.
var pagePositionMethods = []string{"scrollBy", "scrollTo", "scrollIntoView"}

// pagePositionScrolls are the fields a panel writes to move its own content.
var pagePositionScrolls = []string{"scrollTop", "scrollLeft"}

// pagePositionFindings reads the WebUI's TypeScript for the page's position: a
// moving call, a scroll field's write, a focus without preventScroll, or the old
// name bringUnderBand, outside their owner.
func (r *runner) pagePositionFindings(conf ownersFile) ([]string, error) {
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
		if !strings.HasSuffix(rel, ".ts") && !strings.HasSuffix(rel, ".tsx") {
			continue
		}
		if !allowed(rel, []string{"internal/web/src/"}) {
			continue // the screens, not the generated contract or another kind
		}
		raw, err := os.ReadFile(path)
		if err != nil {
			return nil, err
		}
		out = append(out, pagePositionFileFindings(rel, maskSpec(string(raw)), conf)...)
	}
	sort.Strings(out)
	return out, nil
}

// pagePositionFileFindings is one file's findings. The owner moves the page;
// another file asks it. The old name stands nowhere, the owner included.
func pagePositionFileFindings(rel, code string, conf ownersFile) []string {
	var out []string
	if rel != pagePositionOwner {
		for _, name := range pagePositionMethods {
			if allowedBy(rel, conf.Allow, name) {
				continue
			}
			for _, line := range methodLines(code, name) {
				out = append(out, fmt.Sprintf("%s:%d: .%s( はページを動かします。持ち主（%s）の movePage を通すか、欄の中なら %s の allow に理由と決定を添えてください",
					rel, line, name, pagePositionOwner, ownersPath))
			}
		}
		for _, name := range pagePositionScrolls {
			if allowedBy(rel, conf.Allow, name) {
				continue
			}
			for _, line := range scrollWriteLines(code, name) {
				out = append(out, fmt.Sprintf("%s:%d: .%s の代入は欄の中だけにしてください。欄の持ち主の中なら %s の allow に理由と決定を添えてください",
					rel, line, name, ownersPath))
			}
		}
		for _, line := range focusLines(code) {
			out = append(out, fmt.Sprintf("%s:%d: focus() に preventScroll を付けてください（ブラウザがページを動かします）", rel, line))
		}
	}
	for _, line := range wordLines(code, "bringUnderBand") {
		out = append(out, fmt.Sprintf("%s:%d: bringUnderBand は持ち主（%s）の movePage に置き換えてください", rel, line, pagePositionOwner))
	}
	return out
}

// methodLines is the 1-based lines where name is called as a method: a dot
// before it and an open parenthesis after, so .scrollTo( is not .scrollTop.
func methodLines(code, name string) []int {
	var out []int
	needle := "." + name + "("
	for i := 0; ; {
		j := strings.Index(code[i:], needle)
		if j < 0 {
			return out
		}
		out = append(out, lineOf(code, i+j))
		i += j + len(needle)
	}
}

// scrollWriteLines is the 1-based lines where a scroll field is written: =, +=
// or -=, but not the === of a comparison.
func scrollWriteLines(code, name string) []int {
	pattern := regexp.MustCompile(`\.` + regexp.QuoteMeta(name) + `\s*(\+=|-=|=)`)
	var out []int
	for _, at := range pattern.FindAllStringSubmatchIndex(code, -1) {
		opEnd := at[3] // the operator's end
		if code[at[2]:at[3]] == "=" && opEnd < len(code) && code[opEnd] == '=' {
			continue // == or ===
		}
		out = append(out, lineOf(code, at[0]))
	}
	return out
}

// focusLines is the 1-based lines where a focus call stands without
// preventScroll in the same call: the browser would scroll the page to it.
func focusLines(code string) []int {
	var out []int
	for i := 0; ; {
		j := strings.Index(code[i:], ".focus(")
		if j < 0 {
			return out
		}
		at := i + j
		open := at + len(".focus")
		close := closingParen(code, open)
		if close < 0 {
			i = open + 1
			continue
		}
		if !strings.Contains(code[open:close+1], "preventScroll") {
			out = append(out, lineOf(code, at))
		}
		i = close + 1
	}
}

// closingParen is the index of the parenthesis that closes the one at open.
func closingParen(code string, open int) int {
	depth := 0
	for i := open; i < len(code); i++ {
		switch code[i] {
		case '(':
			depth++
		case ')':
			depth--
			if depth == 0 {
				return i
			}
		}
	}
	return -1
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

// changedFindings is the names and the comparisons the changed owner replaced.
func (r *runner) changedFindings(conf ownersFile) ([]string, error) {
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
		if !strings.HasSuffix(rel, ".ts") && !strings.HasSuffix(rel, ".tsx") {
			continue
		}
		raw, err := os.ReadFile(path)
		if err != nil {
			return nil, err
		}
		code := maskSpec(string(raw))
		if rel != changedOwner {
			for _, name := range changedNames {
				if allowedBy(rel, conf.Allow, name) {
					continue
				}
				for _, line := range callLines(code, name) {
					out = append(out, fmt.Sprintf("%s:%d: %s は持ち主（%s）に置いてください", rel, line, name, changedOwner))
				}
			}
		}
		if rel == changedOwner {
			continue
		}
		for i, line := range strings.Split(code, "\n") {
			if strings.Contains(line, "JSON.stringify(") && (strings.Contains(line, "!==") || strings.Contains(line, "===")) && !allowedBy(rel, conf.Allow, "JSON.stringify") {
				out = append(out, fmt.Sprintf("%s:%d: 比べ方を自分で書かず、持ち主（%s）に聞いてください（JSON.stringify の比較）", rel, i+1, changedOwner))
			}
		}
	}
	sort.Strings(out)
	return out, nil
}
