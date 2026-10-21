package web

import (
	"regexp"
	"strings"
	"testing"
)

// The activity list's items the gate reads at the 受け口 layer: D2 (the sheet's
// colours), V1 (the words come with the answer) and V2 (the list never reaches
// the ledger). The screen's own walks of the same IDs live in internal/app
// (e2e_activity_items_test.go); these hold the two promises that are the
// boundary's, not the page's.

// TestACTIVITY_D2_TheScreenWritesNoColours: the activity screen's sheet names no
// colour of its own. The one place it may name one is where it defines the
// design's tokens — the mock's :root and its two dark blocks, which spec-activity
// 4 章 says to copy as they stand — so every declaration that sets a token is
// taken out first, and what is left may read var(...) and nothing else
// (spec-activity 7.5 D2).
func TestACTIVITY_D2_TheScreenWritesNoColours(t *testing.T) {
	raw, err := files.ReadFile("static/activity-v2.css")
	if err != nil {
		t.Fatal(err)
	}
	text := regexp.MustCompile(`(?s)/\*.*?\*/`).ReplaceAllString(string(raw), "")
	text = regexp.MustCompile(`--[A-Za-z0-9-]+\s*:[^;]*;`).ReplaceAllString(text, "")
	for _, find := range []*regexp.Regexp{
		regexp.MustCompile(`#[0-9a-fA-F]{3,8}\b`),
		regexp.MustCompile(`\brgba?\(`),
		regexp.MustCompile(`\bhsla?\(`),
		regexp.MustCompile(`\boklch\(`),
		regexp.MustCompile(`\bcolor\(`),
	} {
		if found := find.FindAllString(text, -1); len(found) != 0 {
			t.Errorf("activity-v2.css names the colour %v of its own", found)
		}
	}
	colours := map[string]bool{
		"color": true, "background": true, "background-color": true,
		"border": true, "border-color": true, "border-top": true, "border-right": true,
		"border-bottom": true, "border-left": true, "border-top-color": true,
		"border-right-color": true, "border-bottom-color": true, "border-left-color": true,
		"outline": true, "outline-color": true, "fill": true, "stroke": true,
		"box-shadow": true, "text-decoration-color": true, "caret-color": true,
	}
	// The words a colour property may carry without a token: the page's own
	// tokens (var), and the four values that name no colour at all.
	plain := map[string]bool{"none": true, "0": true, "inherit": true, "unset": true, "initial": true, "transparent": true, "currentColor": true}
	flat := strings.NewReplacer("{", ";", "}", ";").Replace(text)
	for decl := range strings.SplitSeq(flat, ";") {
		name, value, ok := strings.Cut(decl, ":")
		if !ok {
			continue
		}
		prop := strings.TrimSpace(name)
		if !colours[prop] {
			continue
		}
		v := strings.TrimSpace(value)
		if strings.Contains(v, "var(") || plain[v] {
			continue
		}
		t.Errorf("activity-v2.css sets %s to %q, which is not one of the page's tokens", prop, v)
	}
}

// TestACTIVITY_V1_TheWordsComeWithTheAnswer: the screen's script writes no length,
// no moment and no 何分前 — the reading behind the answer writes every one of
// them, and the answer carries them through as they stand (spec-activity 7.6 V1,
// 6 A4).
func TestACTIVITY_V1_TheWordsComeWithTheAnswer(t *testing.T) {
	// The script the screen runs makes none of them.
	raw, err := files.ReadFile("static/activity-v2.js")
	if err != nil {
		t.Fatal(err)
	}
	code := activityTestCode(string(raw))
	for _, word := range []string{"時間", "分", "秒", "以上", "より前", "から", "日", "時"} {
		if strings.Contains(code, word) {
			t.Errorf("the screen's own script says %q, which the answer is the one to say", word)
		}
	}

	// And the answers carry the reader's words untouched, both for the list's
	// いつから and for the opened row's 何分前.
	f := newFixture(t)
	since := &ActivitySince{Len: "2 時間 40 分", At: "13:34 から"}
	server, err := New(f.catalog(), fakeActivity{
		list: ActivityList{
			Items: []ActivityItem{{Name: "demo", Summary: ActivitySummary{
				Definition: "demo", State: "turn", Last: "lead", Since: since,
			}}},
			Running: 1, Waiting: 1, Herdr: ActivityNote{OK: true},
		},
		docs: map[string]ActivityDoc{"demo": {
			Name: "demo", Herdr: "demo-lead", Role: "lead", State: "turn", Last: "lead", Since: since,
			Questions: []ActivityQuestion{{ID: "q-1", Title: "このままでよいか", Ago: "1 日前", Options: []string{"はい", "いいえ"}}},
			Tail:      ActivityTail{Lines: []string{"$ make", "done"}, Shown: 2},
			Gy:        ActivityNote{OK: true},
		}},
	})
	if err != nil {
		t.Fatal(err)
	}

	_, list := f.get(t, server, "/api/activity")
	for _, want := range []string{`"len":"2 時間 40 分"`, `"at":"13:34 から"`} {
		if !strings.Contains(list, want) {
			t.Errorf("GET /api/activity does not carry the reader's own %s:\n%s", want, list)
		}
	}
	_, doc := f.get(t, server, "/api/activity/demo")
	for _, want := range []string{`"ago":"1 日前"`, `"len":"2 時間 40 分"`} {
		if !strings.Contains(doc, want) {
			t.Errorf("GET /api/activity/demo does not carry the reader's own %s:\n%s", want, doc)
		}
	}

	// The page shell writes none of them either: it is the 一覧 tab and the one
	// sentence for an empty list, and every other word comes with the answer.
	_, page := f.get(t, server, "/activity")
	for _, word := range []string{"時間", "分", "秒", "以上", "より前", "から", "日", "時"} {
		if strings.Contains(page, word) {
			t.Errorf("the /activity page writes %q of its own, which the answer is the one to say", word)
		}
	}
}

// countingActivity counts how a server reads it: how many times the list was
// read and how many times one team was. One is the only path of the reading that
// reaches gy (spec-activity 3, 6 A1), so the list calling it never means the list
// read the ledger.
type countingActivity struct {
	list        ActivityList
	docs        map[string]ActivityDoc
	lists, ones int
}

func (c *countingActivity) List() ActivityList { c.lists++; return c.list }

func (c *countingActivity) One(name string) (ActivityDoc, bool) {
	c.ones++
	doc, ok := c.docs[name]
	return doc, ok
}

// TestACTIVITY_V2_TheListNeverReadsTheLedger: GET /api/activity reads the list
// alone and never one team, so the one path that reaches gy is not walked when a
// person only wants to see what is running (spec-activity 7.6 V2).
func TestACTIVITY_V2_TheListNeverReadsTheLedger(t *testing.T) {
	f := newFixture(t)
	reading := &countingActivity{
		list: ActivityList{Items: []ActivityItem{{Name: "demo"}}, Running: 1, Herdr: ActivityNote{OK: true}},
		docs: map[string]ActivityDoc{"demo": {Name: "demo"}},
	}
	server, err := New(f.catalog(), reading)
	if err != nil {
		t.Fatal(err)
	}

	resp, body := f.get(t, server, "/api/activity")
	if resp.StatusCode != 200 {
		t.Fatalf("GET /api/activity answers %d, want 200", resp.StatusCode)
	}
	if reading.lists != 1 {
		t.Errorf("GET /api/activity read the list %d times, want once", reading.lists)
	}
	if reading.ones != 0 {
		t.Errorf("GET /api/activity read one team %d times, want none: the list never reaches the ledger", reading.ones)
	}
	// The list's answer is the rows and the band's numbers alone: a question or a
	// pane belongs to the opened row (V1・V3).
	if strings.Contains(body, "questions") || strings.Contains(body, "tail") {
		t.Errorf("the list's answer carries a question or a pane, which only the opened row has:\n%s", body)
	}

	// The counting is real: the opened row is the one that reads One, which is
	// the reading that walks into gy.
	resp, _ = f.get(t, server, "/api/activity/demo")
	if resp.StatusCode != 200 {
		t.Fatalf("GET /api/activity/demo answers %d, want 200", resp.StatusCode)
	}
	if reading.ones != 1 {
		t.Errorf("GET /api/activity/demo read one team %d times, want once", reading.ones)
	}
}

// activityTestCode is one script with its comments taken out: what the screen's
// own code says, rather than what its notes say about it (V1).
func activityTestCode(script string) string {
	code := regexp.MustCompile(`(?s)/\*.*?\*/`).ReplaceAllString(script, "")
	var out []string
	for line := range strings.SplitSeq(code, "\n") {
		if at := strings.Index(line, "//"); at >= 0 {
			line = line[:at]
		}
		out = append(out, line)
	}
	return strings.Join(out, "\n")
}
