package main

// The reading gate refuses a walk that compares what the page said once (r-291a).
//
// helpers.ts's read waits until the page can give an expression — an element that
// is not drawn throws in the page — but not until the expression says what the
// walk expects. The app adds some of its marks after the answer to a press: a
// member's 問題 comes with the role documents read behind it, 保存しました with
// the save. A walk that reads once and compares what it got therefore asks the
// page a question it may not be ready to answer, and fails on the run where the
// machine is a little slower — TEAMS-V1 did on main 4f0ec6d, after four green CI
// runs of the branch and two green runs of the gate by hand.
//
// The comparison goes through a reading that is read again until it matches:
// Playwright's expect.poll (helpers.ts's readTo wraps it with the walks' own
// message). So the gate refuses the two shapes that compare one reading:
//
//   - expect( given a value it awaits itself: expect(await read<T>(...)).toBe(x)
//   - expect( given a name one await of read filled: const n = await read(...);
//     expect(n).toBe(x)
//
// expect.poll( is the shape the gate asks for, so it is the one expect whose
// argument may hold an await: the poll reads again until the matcher holds.
//
// The second shape is read as far as a name assigned in the file, not through
// the value: a walk that splits a reading and compares the pieces is out of
// reach of a reading of the text (it was written nowhere after r-291a).

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// browserGlob is where the browser walks stand: Playwright's spec files, run by bun.
const browserGlob = "e2e/tests/*.spec.ts"

// readingGate reads every walk's spec file and refuses the comparisons that take
// one reading.
func (r *runner) readingGate() error {
	paths, err := filepath.Glob(filepath.Join(r.root, browserGlob))
	if err != nil {
		return err
	}
	if len(paths) == 0 {
		return errNoWalk
	}
	sort.Strings(paths)
	var findings []string
	for _, path := range paths {
		raw, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(r.root, path)
		if err != nil {
			return err
		}
		findings = append(findings, specFindings(filepath.ToSlash(rel), string(raw))...)
	}
	if len(findings) == 0 {
		return nil
	}
	for _, finding := range findings {
		fmt.Println(finding)
	}
	return fmt.Errorf("1 回だけ読んだ値を比べる expect が %d 件", len(findings))
}

// specFindings is what one spec file compares once: a line for every expect(
// that takes a reading the walk made once.
func specFindings(name, text string) []string {
	code := maskSpec(text)
	read := namesReadOnce(code)
	var out []string
	for _, call := range expectCalls(code) {
		arg := strings.TrimSpace(code[call.from:call.to])
		where := fmt.Sprintf("%s:%d", name, lineOf(text, call.at))
		switch {
		case containsWord(arg, "await"):
			out = append(out, fmt.Sprintf(
				"%s: expect( が await した値を 1 回だけ読んで比べています: %s。helpers.ts の readTo（または expect.poll）で、期待に合うまで読み直してください",
				where, oneLine(arg)))
		case isIdentifier(arg) && read[arg]:
			out = append(out, fmt.Sprintf(
				"%s: expect( が 1 回だけ読んだ値 %s を比べています。helpers.ts の readTo（または expect.poll）で、期待に合うまで読み直してください",
				where, arg))
		}
	}
	return out
}

// maskSpec returns the spec's text with its comments and the bodies of its
// strings and templates replaced by spaces, one byte for one byte, so that a
// place found in the mask is the same place in the file. What is left is code:
// expect(, await, and the names a walk compares.
//
// A template's ${...} is masked with it, which is what the walks want: the
// scripts they read the page with stand inside templates, and a read named in
// one of those is the page's reading, not the walk's.
func maskSpec(text string) string {
	out := []byte(text)
	blank := func(from, to int) {
		for i := from; i < to && i < len(out); i++ {
			if out[i] != '\n' {
				out[i] = ' '
			}
		}
	}
	for i := 0; i < len(text); {
		switch {
		case strings.HasPrefix(text[i:], "//"):
			end := strings.IndexByte(text[i:], '\n')
			if end < 0 {
				end = len(text) - i
			}
			blank(i, i+end)
			i += end
		case strings.HasPrefix(text[i:], "/*"):
			end := strings.Index(text[i+2:], "*/")
			if end < 0 {
				blank(i, len(text))
				return string(out)
			}
			blank(i, i+2+end+2)
			i += 2 + end + 2
		case text[i] == '\'' || text[i] == '"' || text[i] == '`':
			quote := text[i]
			j := i + 1
			for j < len(text) {
				if text[j] == '\\' {
					j += 2
					continue
				}
				if text[j] == quote {
					j++
					break
				}
				j++
			}
			blank(i, j)
			i = j
		default:
			i++
		}
	}
	return string(out)
}

// expectCall is one expect( that is not a poll: where it stands, and the bytes
// of its first argument.
type expectCall struct {
	at       int
	from, to int
}

// expectCalls finds every expect( of the code, a poll left out: the calls whose
// first argument the gate holds to being a reading that matches.
func expectCalls(code string) []expectCall {
	var out []expectCall
	for i := 0; i < len(code); i++ {
		if !strings.HasPrefix(code[i:], "expect") {
			continue
		}
		if i > 0 && (isIdentByte(code[i-1]) || code[i-1] == '.') {
			continue
		}
		j := i + len("expect")
		poll := false
		for j < len(code) && code[j] == '.' {
			k := j + 1
			for k < len(code) && isIdentByte(code[k]) {
				k++
			}
			if code[j+1:k] == "poll" {
				poll = true
			}
			j = k
		}
		if j >= len(code) || code[j] != '(' {
			continue
		}
		end := matching(code, j, '(', ')')
		if end < 0 {
			continue
		}
		if !poll {
			out = append(out, expectCall{at: i, from: j + 1, to: firstArgument(code, j+1, end)})
		}
		i = end
	}
	return out
}

// matching is the place one bracket closes at, from the bracket at open, with
// brackets of the same kind counted and the other kinds left to their own.
func matching(code string, open int, left, right byte) int {
	depth := 0
	for i := open; i < len(code); i++ {
		switch code[i] {
		case left:
			depth++
		case right:
			depth--
			if depth == 0 {
				return i
			}
		}
	}
	return -1
}

// firstArgument is where the first argument of a call ends: the comma at the
// call's own depth, or the call's closing bracket.
func firstArgument(code string, from, end int) int {
	depth := 0
	for i := from; i < end; i++ {
		switch code[i] {
		case '(', '[', '{':
			depth++
		case ')', ']', '}':
			depth--
		case ',':
			if depth == 0 {
				return i
			}
		}
	}
	return end
}

// namesReadOnce is every name one statement filled with an await of read: the
// const, let or var whose value is a reading, which a walk may hold but not
// compare once.
func namesReadOnce(code string) map[string]bool {
	out := map[string]bool{}
	for i := 0; i < len(code); i++ {
		keyword := ""
		for _, word := range []string{"const", "let", "var"} {
			if strings.HasPrefix(code[i:], word) {
				keyword = word
				break
			}
		}
		if keyword == "" {
			continue
		}
		if i > 0 && isIdentByte(code[i-1]) {
			continue
		}
		j := i + len(keyword)
		if j < len(code) && isIdentByte(code[j]) {
			continue
		}
		for j < len(code) && (code[j] == ' ' || code[j] == '\t') {
			j++
		}
		k := j
		for k < len(code) && isIdentByte(code[k]) {
			k++
		}
		if k == j {
			continue
		}
		name := code[j:k]
		for k < len(code) && (code[k] == ' ' || code[k] == '\t') {
			k++
		}
		if k >= len(code) || code[k] != '=' || strings.HasPrefix(code[k:], "==") {
			continue
		}
		end := statementEnd(code, k+1)
		value := code[k+1 : end]
		if containsWord(value, "await") && containsWord(value, "read") {
			out[name] = true
		}
		i = end
	}
	return out
}

// statementEnd is where the statement that starts at from ends: its semicolon,
// or the line it stands on when the statement carries none.
func statementEnd(code string, from int) int {
	depth := 0
	for i := from; i < len(code); i++ {
		switch code[i] {
		case '(', '[', '{':
			depth++
		case ')', ']', '}':
			depth--
		case ';':
			if depth == 0 {
				return i
			}
		case '\n':
			if depth == 0 {
				return i
			}
		}
	}
	return len(code)
}

// containsWord is whether one text holds one word with its own ends: read is a
// word, readTo is not.
func containsWord(text, word string) bool {
	for i := 0; ; {
		at := strings.Index(text[i:], word)
		if at < 0 {
			return false
		}
		start := i + at
		end := start + len(word)
		if (start == 0 || !isIdentByte(text[start-1])) && (end >= len(text) || !isIdentByte(text[end])) {
			return true
		}
		i = start + 1
	}
}

// isIdentifier is whether one trimmed text is a name and nothing else.
func isIdentifier(text string) bool {
	if text == "" || !isIdentStart(text[0]) {
		return false
	}
	for i := 1; i < len(text); i++ {
		if !isIdentByte(text[i]) {
			return false
		}
	}
	return true
}

func isIdentStart(c byte) bool {
	return c == '_' || c == '$' || (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z')
}

func isIdentByte(c byte) bool {
	return isIdentStart(c) || (c >= '0' && c <= '9')
}

// lineOf is the 1-based line one byte stands on.
func lineOf(text string, at int) int {
	if at > len(text) {
		at = len(text)
	}
	return strings.Count(text[:at], "\n") + 1
}

// oneLine squashes whitespace, so that a reading of a call stays on the finding's
// own line.
func oneLine(text string) string {
	return strings.Join(strings.Fields(text), " ")
}

// errNoWalk is what the gate says when the walks it must read are not there: a
// green run over no spec file would say nothing about the walks.
var errNoWalk = errors.New("no e2e/tests/*.spec.ts to read")
