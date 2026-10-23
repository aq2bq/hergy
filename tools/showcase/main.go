// Command showcase runs the showcase (ショーケース): the real `hergy serve` over
// sample data of a real machine's size and a made-up herdr, so a person can look
// at the screens as they are now without building a mock up from nothing.
//
//	go run ./tools/showcase                       the standard scene
//	go run ./tools/showcase <scene>               one scene, by name
//	go run ./tools/showcase run <scene> -- <args> a hergy command in the scene
//	go run ./tools/showcase snap <scene> <path> <out>   one screen, as HTML
//	go run ./tools/showcase -h                    the scenes, and what each is for
//
// Nothing of the person's machine is read or written: the config, the state and
// the home directory of the serve are made fresh under the work directory on
// every serve, the project is a git repository of its own there, and herdr is
// the made-up one beside them. The sample data is testdata/showcase/<scene>/,
// which is written in the repository, so what a screen saves never reaches it.
//
// run and snap stand on the same work directory as the serve: a scene that is
// already laid out is used as it is, so a command or a copy never takes a
// running serve's data away.
package main

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"syscall"
	"time"
)

const (
	// dataDir is where each scene's sample data stands, from the module root.
	dataDir = "testdata/showcase"
	// marker is the file this tool leaves in the work directory it made, so a
	// later run may clear it and no other directory is ever cleared.
	marker = ".showcase"
	// stopGrace is how long the serve is given to stop before it is killed.
	stopGrace = 10 * time.Second
	// projectDir is the work directory's project: a name of its own, so the
	// band that writes the project's name reads like a person's project.
	projectDir = "shop"
)

// scene is one state of the screens: the sample data it is laid from and the
// words -h says about it.
type scene struct {
	name string
	// about is what the scene shows, in one line for -h.
	about string
	// layers are the directories under dataDir whose trees are laid one over
	// the other, in this order. A scene that shows the same data with a
	// different herdr, or with a rewritten prompt template, names the scene it
	// stands on first, so the two cannot drift apart.
	layers []string
}

// scenes is the one table the run, -h and the sample data come from.
var scenes = []scene{
	{
		name:   "standard",
		about:  "本物の規模の見本データ（チーム定義 4+3、ロール 4+3、core はメンバー 5 人・ワーカー 6 種・手番 16 本）。チームは止まっている",
		layers: []string{"standard"},
	},
	{
		name:   "running",
		about:  "standard と同じデータで、core チームが動いている（仕事中・止まっている・許可待ち・終わりが混ざり、ワーカー 2 人も動いている）",
		layers: []string{"standard", "running"},
	},
	{
		name:   "empty",
		about:  "チーム定義もロールも無い、初めて使う人の状態",
		layers: []string{"empty"},
	},
	{
		name:   "broken",
		about:  "壊れた team.yaml と、無いロールを指すメンバーを含む",
		layers: []string{"broken"},
	},
}

// options is one command line.
type options struct {
	scene string
	port  int
	work  string
	bin   string
	help  bool
	// named says a scene was named on the command line, so a second one is a
	// mistake rather than the default being replaced.
	named bool
}

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

// run is the whole command line: the word before the scene picks a road, and a
// scene with no word is the serve.
func run(args []string, stdout, stderr io.Writer) int {
	if len(args) > 0 {
		switch args[0] {
		case "run":
			return runInScene(args[1:], stdout, stderr)
		case "snap":
			return snapScreen(args[1:], stdout, stderr)
		}
	}
	return serveScene(args, stdout, stderr)
}

// usage is -h: the roads, the scenes with what each one is for, and the flags.
func usage() string {
	var b strings.Builder
	b.WriteString("ショーケース: 本物の hergy serve を、本物の規模の見本データと偽の herdr で起こす。\n\n")
	b.WriteString("使い方:\n")
	b.WriteString("  go run ./tools/showcase [<場面>] [--port N] [--work DIR] [--bin PATH]\n")
	b.WriteString("  go run ./tools/showcase run <場面> [--work DIR] [--bin PATH] -- <hergy の引数...>\n")
	b.WriteString("  go run ./tools/showcase snap <場面> <パス> <出力先> [--work DIR] [--bin PATH]\n\n")
	b.WriteString("場面（書かなければ standard）:\n")
	for _, sc := range scenes {
		fmt.Fprintf(&b, "  %-9s %s\n", sc.name, sc.about)
	}
	b.WriteString(`
道:
  serve  本物の hergy serve を起こして URL を出し、Ctrl-C で止まる。起こすたびに見本データを写し直す
  run    場面の作業用ディレクトリ・環境・bin/hergy・プロジェクトのまま hergy を走らせ、出力と終了コードをそのまま返す
         （すでに写してある場面は写し直さない。serve が動いていても壊さない）
  snap   その画面の DOM を Playwright で写し、script を除いて CSS を中に取り込み、file:// で同じ見た目になる HTML を出力先に書く

旗:
  --port N    serve が待つポート（既定 0: OS が選ぶ。127.0.0.1 だけを待つ）
  --work DIR  作業用ディレクトリ（既定 .local/showcase/<場面>。serve は起こすたびに作り直す）
  --bin PATH  この hergy を使う（既定: hergy をビルドして作業用ディレクトリに置く）
  -h          これを出す

本物の ~/.config/hergy、~/.local/state/hergy、herdr には触れない。
`)
	return b.String()
}

// serveScene is the serve: lay the scene out fresh, start it, and wait for the
// person to stop it.
func serveScene(args []string, stdout, stderr io.Writer) int {
	opts, err := parseArgs(args)
	if err != nil {
		fmt.Fprintln(stderr, "showcase:", err)
		fmt.Fprint(stderr, usage())
		return 2
	}
	if opts.help {
		fmt.Fprint(stdout, usage())
		return 0
	}
	sc, root, err := pick(opts)
	if err != nil {
		fmt.Fprintln(stderr, "showcase:", err)
		return 1
	}
	work, err := workPath(root, sc, opts)
	if err != nil {
		fmt.Fprintln(stderr, "showcase:", err)
		return 1
	}
	work, hergy, err := prepare(root, sc, opts, work, stdout, stderr)
	if err != nil {
		fmt.Fprintln(stderr, "showcase:", err)
		return 1
	}
	srv, err := startServe(work, hergy, opts.port, stderr)
	if err != nil {
		fmt.Fprintln(stderr, "showcase:", err)
		return 1
	}
	fmt.Fprintf(stdout, "ショーケース: %s — %s\n作業用: %s\nURL: %s\n（Ctrl-C で止まります）\n", sc.name, sc.about, work, srv.url)
	if err := waitStop(srv); err != nil {
		fmt.Fprintln(stderr, "showcase:", err)
		return 1
	}
	return 0
}

// runInScene runs one hergy command in the scene's own work directory and
// environment, with the same binary and the same project the serve reads, so a
// prompt it prints is the prompt the screen would show. The words and the exit
// code are hergy's own.
func runInScene(args []string, stdout, stderr io.Writer) int {
	opts, hergyArgs, help, err := parseRunArgs(args)
	if err != nil {
		fmt.Fprintln(stderr, "showcase:", err)
		fmt.Fprint(stderr, usage())
		return 2
	}
	if help {
		fmt.Fprint(stdout, usage())
		return 0
	}
	sc, root, err := pick(opts)
	if err != nil {
		fmt.Fprintln(stderr, "showcase:", err)
		return 1
	}
	work, hergy, err := layOrReuse(root, sc, opts, stdout, stderr)
	if err != nil {
		fmt.Fprintln(stderr, "showcase:", err)
		return 1
	}
	cmd := exec.Command(hergy, hergyArgs...) //nolint:gosec // G204: hergy is this tool's own binary and the arguments are the person's own
	cmd.Dir = filepath.Join(work, projectDir)
	cmd.Env = serveEnv(work)
	cmd.Stdin = os.Stdin
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	err = cmd.Run()
	var exited *exec.ExitError
	switch {
	case errors.As(err, &exited):
		return exited.ExitCode()
	case err != nil:
		fmt.Fprintln(stderr, "showcase:", err)
		return 1
	}
	return 0
}

// snapScreen writes one screen of a scene as a static HTML file: the serve is
// started over the scene, Playwright takes the drawn page, the scripts go, and
// every stylesheet is written into the page with its fonts pointing at the
// repository's own static directory, so the file reads the same from file://.
func snapScreen(args []string, stdout, stderr io.Writer) int {
	opts, rest, help, err := parseSnapArgs(args)
	if err != nil {
		fmt.Fprintln(stderr, "showcase:", err)
		fmt.Fprint(stderr, usage())
		return 2
	}
	if help {
		fmt.Fprint(stdout, usage())
		return 0
	}
	sc, root, err := pick(opts)
	if err != nil {
		fmt.Fprintln(stderr, "showcase:", err)
		return 1
	}
	work, hergy, err := layOrReuse(root, sc, opts, stdout, stderr)
	if err != nil {
		fmt.Fprintln(stderr, "showcase:", err)
		return 1
	}
	srv, err := startServe(work, hergy, 0, stderr)
	if err != nil {
		fmt.Fprintln(stderr, "showcase:", err)
		return 1
	}
	defer srv.stop()
	out, err := filepath.Abs(rest[1])
	if err != nil {
		fmt.Fprintln(stderr, "showcase:", err)
		return 1
	}
	if err := runSnap(root, srv.url, rest[0], out, stderr); err != nil {
		fmt.Fprintln(stderr, "showcase:", err)
		return 1
	}
	fmt.Fprintf(stdout, "写しました: %s\n（%s%s の画面）\n", out, srv.url, rest[0])
	return 0
}

// pick is the scene and the module root, which every road starts from.
func pick(opts options) (scene, string, error) {
	sc, err := findScene(opts.scene)
	if err != nil {
		return scene{}, "", err
	}
	root, err := moduleRoot()
	if err != nil {
		return scene{}, "", err
	}
	return sc, root, nil
}

// parseArgs reads the serve's command line, without a flag package: every value
// has to say what it is for in the words the table above uses.
func parseArgs(args []string) (options, error) {
	out := options{scene: scenes[0].name}
	for i := 0; i < len(args); i++ {
		name, value, hasValue := strings.Cut(args[i], "=")
		switch name {
		case "-h", "--help", "help":
			out.help = true
		case "--port", "-port":
			if !hasValue {
				i++
				if i == len(args) {
					return out, errors.New("--port の後ろにポートの数を書いてください")
				}
				value = args[i]
			}
			port, err := strconv.Atoi(value)
			if err != nil || port < 0 || port > 65535 {
				return out, fmt.Errorf("--port の %q は 0 から 65535 の数ではありません", value)
			}
			out.port = port
		case "--work", "-work":
			value, i, hasValue = takeValue(args, i, value, hasValue)
			if !hasValue {
				return out, errors.New("--work の後ろにディレクトリを書いてください")
			}
			out.work = value
		case "--bin", "-bin":
			value, i, hasValue = takeValue(args, i, value, hasValue)
			if !hasValue {
				return out, errors.New("--bin の後ろに hergy のパスを書いてください")
			}
			out.bin = value
		default:
			if strings.HasPrefix(args[i], "-") {
				return out, fmt.Errorf("%q は知らない旗です", args[i])
			}
			if out.named {
				return out, fmt.Errorf("場面は 1 つだけです（%q と %q）", out.scene, args[i])
			}
			out.scene, out.named = args[i], true
		}
	}
	return out, nil
}

// parseRunArgs reads `run <場面> [flags] -- <hergy の引数...>`: the tool's own
// words stand before the --, and hergy's own after it, untouched.
func parseRunArgs(args []string) (options, []string, bool, error) {
	cut := slices.Index(args, "--")
	head := args
	var hergyArgs []string
	if cut >= 0 {
		head, hergyArgs = args[:cut], args[cut+1:]
	}
	opts, err := parseArgs(head)
	if err != nil {
		return opts, nil, false, err
	}
	if opts.help {
		return opts, nil, true, nil
	}
	if cut < 0 {
		return opts, nil, false, errors.New("hergy の引数の前に -- を書いてください（例: run standard -- up core --dry --json）")
	}
	if len(hergyArgs) == 0 {
		return opts, nil, false, errors.New("-- の後ろに hergy の引数を書いてください")
	}
	return opts, hergyArgs, false, nil
}

// parseSnapArgs reads `snap <場面> <パス> <出力先> [flags]`, and returns the
// path and the output after the scene.
func parseSnapArgs(args []string) (options, []string, bool, error) {
	out := options{scene: scenes[0].name}
	var pos []string
	for i := 0; i < len(args); i++ {
		name, value, hasValue := strings.Cut(args[i], "=")
		switch name {
		case "-h", "--help", "help":
			out.help = true
		case "--work", "-work":
			value, i, hasValue = takeValue(args, i, value, hasValue)
			if !hasValue {
				return out, nil, false, errors.New("--work の後ろにディレクトリを書いてください")
			}
			out.work = value
		case "--bin", "-bin":
			value, i, hasValue = takeValue(args, i, value, hasValue)
			if !hasValue {
				return out, nil, false, errors.New("--bin の後ろに hergy のパスを書いてください")
			}
			out.bin = value
		default:
			if strings.HasPrefix(args[i], "-") {
				return out, nil, false, fmt.Errorf("%q は知らない旗です", args[i])
			}
			pos = append(pos, args[i])
		}
	}
	if out.help {
		return out, nil, true, nil
	}
	if len(pos) != 3 {
		return out, nil, false, errors.New("snap には <場面> <パス> <出力先> の 3 つを書いてください（例: snap running /activity out.html）")
	}
	out.scene, out.named = pos[0], true
	return out, pos[1:], false, nil
}

// takeValue is one flag's value, from --name=value or from the argument after
// it. i and the returned index are the same when the value came with the flag.
func takeValue(args []string, i int, value string, hasValue bool) (string, int, bool) {
	if hasValue {
		return value, i, true
	}
	if i+1 == len(args) {
		return "", i, false
	}
	return args[i+1], i + 1, true
}

// findScene is the scene a person named. A name the table does not hold is a
// mistake, and the words say which names there are.
func findScene(name string) (scene, error) {
	names := make([]string, 0, len(scenes))
	for _, sc := range scenes {
		if sc.name == name {
			return sc, nil
		}
		names = append(names, sc.name)
	}
	return scene{}, fmt.Errorf("場面 %q はありません（%s のどれか）", name, strings.Join(names, "、"))
}

// moduleRoot walks up from the working directory to the nearest go.mod.
func moduleRoot() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", errors.New("go.mod が上のディレクトリにありません（hergy のリポジトリの中で打ってください）")
		}
		dir = parent
	}
}

// workPath is the work directory of one scene: the one a person named, or the
// scene's own under .local/showcase.
func workPath(root string, sc scene, opts options) (string, error) {
	work := opts.work
	if work == "" {
		work = filepath.Join(root, ".local", "showcase", sc.name)
	}
	return filepath.Abs(work)
}

// layOrReuse is the work directory and the binary of a road that must not lay a
// scene out again: the scene is prepared when it is not there, and used as it is
// when it is, so a running serve keeps its data.
func layOrReuse(root string, sc scene, opts options, stdout, stderr io.Writer) (string, string, error) {
	work, err := workPath(root, sc, opts)
	if err != nil {
		return "", "", err
	}
	have, err := isLaid(work)
	if err != nil {
		return "", "", err
	}
	if !have {
		return prepare(root, sc, opts, work, stdout, stderr)
	}
	hergy := opts.bin
	if hergy == "" {
		hergy = filepath.Join(work, "bin", "hergy")
		if _, err := os.Stat(hergy); errors.Is(err, fs.ErrNotExist) {
			if err := execIn(root, stdout, stderr, "go", "build", "-o", hergy, "."); err != nil {
				return "", "", err
			}
		}
	}
	if _, err := os.Stat(hergy); err != nil {
		return "", "", fmt.Errorf("--bin の %s が読めません: %w", hergy, err)
	}
	return work, hergy, nil
}

// isLaid says the work directory is this tool's own, laid out before: a serve, a
// command or a copy may stand on it.
func isLaid(work string) (bool, error) {
	info, err := os.Stat(work)
	switch {
	case errors.Is(err, fs.ErrNotExist):
		return false, nil
	case err != nil:
		return false, err
	case !info.IsDir():
		return false, fmt.Errorf("%s はディレクトリではありません", work)
	}
	if _, err := os.Stat(filepath.Join(work, marker)); err != nil {
		return false, nil
	}
	return true, nil
}

// prepare lays one scene's work directory out and returns it with the hergy the
// roads run: the project, the config, the state, the home directory, the
// made-up herdr, and the binary.
func prepare(root string, sc scene, opts options, work string, stdout, stderr io.Writer) (string, string, error) {
	if err := resetWork(work); err != nil {
		return "", "", err
	}
	for _, dir := range []string{projectDir, "cfg", "state", "home", "bin", "herdr"} {
		if err := os.MkdirAll(filepath.Join(work, dir), 0o755); err != nil {
			return "", "", err
		}
	}
	if err := os.WriteFile(filepath.Join(work, marker), []byte("hergy showcase\n"), 0o644); err != nil {
		return "", "", err
	}
	for _, layer := range sc.layers {
		from := filepath.Join(root, dataDir, layer)
		for _, part := range []struct{ from, to string }{
			{"project", projectDir},
			{"config", "cfg"},
			{"state", "state"},
			{"herdr", "herdr"},
		} {
			src := filepath.Join(from, part.from)
			if err := copyTree(src, filepath.Join(work, part.to)); err != nil {
				return "", "", err
			}
		}
	}
	if err := fillTimes(work, time.Now()); err != nil {
		return "", "", err
	}
	if err := writeFakeHerdr(filepath.Join(work, "bin", "fake-herdr")); err != nil {
		return "", "", err
	}
	// The project is a git repository of its own: hergy finds the project root
	// through git, and this keeps it from being the repository hergy lives in.
	if err := execIn(filepath.Join(work, projectDir), stdout, stderr, "git", "init", "-q"); err != nil {
		return "", "", err
	}
	hergy := opts.bin
	if hergy == "" {
		hergy = filepath.Join(work, "bin", "hergy")
		if err := execIn(root, stdout, stderr, "go", "build", "-o", hergy, "."); err != nil {
			return "", "", err
		}
	}
	if _, err := os.Stat(hergy); err != nil {
		return "", "", fmt.Errorf("--bin の %s が読めません: %w", hergy, err)
	}
	return work, hergy, nil
}

// resetWork makes the work directory empty. A directory without this tool's mark
// is only cleared when it is already empty: a path a person typed must never
// take their files away.
func resetWork(work string) error {
	info, err := os.Stat(work)
	switch {
	case errors.Is(err, fs.ErrNotExist):
		return os.MkdirAll(work, 0o755)
	case err != nil:
		return err
	case !info.IsDir():
		return fmt.Errorf("%s はディレクトリではありません", work)
	}
	if _, err := os.Stat(filepath.Join(work, marker)); err != nil {
		entries, readErr := os.ReadDir(work)
		if readErr != nil {
			return readErr
		}
		if len(entries) > 0 {
			return fmt.Errorf("%s は以前にこの道具が作ったディレクトリではありません。空のディレクトリを --work に渡してください", work)
		}
	}
	if err := os.RemoveAll(work); err != nil {
		return err
	}
	return os.MkdirAll(work, 0o755)
}

// copyTree lays one tree over another: every file keeps its own permission
// bits, and a tree that is not there is nothing to lay.
func copyTree(from, to string) error {
	if _, err := os.Stat(from); errors.Is(err, fs.ErrNotExist) {
		return nil
	} else if err != nil {
		return err
	}
	return filepath.WalkDir(from, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(from, path)
		if err != nil {
			return err
		}
		target := filepath.Join(to, rel)
		if d.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		raw, err := os.ReadFile(path) //nolint:gosec // G304: the path is under the repository's own testdata
		if err != nil {
			return err
		}
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}
		return os.WriteFile(target, raw, info.Mode().Perm())
	})
}

// minutePlace is a moment the sample data keeps for the run: @T-<分>@ is that
// many minutes before the run, written as RFC 3339 - the shape hergy's own
// state files write. It lets a scene say a state began 47 minutes ago instead of
// always "a few seconds ago", which is what a person looking at the screens
// wants to read.
var minutePlace = regexp.MustCompile(`@T-([0-9]+)@`)

// fillTimes fills every place in the work directory, so the sample data is laid
// down as it is and only the moments are the run's own.
func fillTimes(work string, now time.Time) error {
	return filepath.WalkDir(work, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		raw, err := os.ReadFile(path) //nolint:gosec // G304: the path is under the work directory this tool made
		if err != nil {
			return err
		}
		if !bytes.Contains(raw, []byte("@T-")) {
			return nil
		}
		filled := minutePlace.ReplaceAllFunc(raw, func(place []byte) []byte {
			minutes, err := strconv.Atoi(string(place[3 : len(place)-1]))
			if err != nil {
				return place
			}
			return []byte(now.Add(-time.Duration(minutes) * time.Minute).Format(time.RFC3339))
		})
		return os.WriteFile(path, filled, 0o644) //nolint:gosec // G122: path is a file of the work directory this tool just laid down
	})
}

// fakeHerdrBody is the made-up herdr: the answers are the files of the scene's
// herdr directory, and it writes nothing. It answers the calls hergy's screens
// make and nothing else, so a screen that starts asking for another call says so
// here rather than reaching a real herdr.
const fakeHerdrBody = `#!/bin/sh
# The made-up herdr of the showcase (tools/showcase). Answers come from the
# scene's herdr directory; nothing is written.
d="$SHOWCASE_HERDR_DIR"
if [ -z "$2" ]; then
  echo "herdr $1"
  for x in get prompt start list read focus send-keys current split rename close create process-info; do echo "  herdr $1 $x"; done
  exit 0
fi
case "$1 $2" in
  "tab list") cat "$d/tabs.json" ;;
  "agent list") cat "$d/agents.json" ;;
  "agent read")
    if [ -f "$d/screens/$3.txt" ]; then cat "$d/screens/$3.txt"; else echo '{"error":{"code":"agent_not_found"}}'; exit 1; fi ;;
  "pane current") echo '{"result":{"pane":{"workspace_id":"w1"}}}' ;;
  *) echo '{"result":{}}' ;;
esac
`

// writeFakeHerdr writes the made-up herdr where the serve is told to find it.
func writeFakeHerdr(path string) error {
	return os.WriteFile(path, []byte(fakeHerdrBody), 0o755) //nolint:gosec // G302: this is the program the serve runs
}

// serveEnv is the environment hergy runs with: every place it reads or writes is
// under the work directory, and nothing points at the person's own config,
// state, home or herdr. The serve, run and snap all start hergy with it.
func serveEnv(work string) []string {
	own := map[string]bool{
		"XDG_CONFIG_HOME":    true,
		"XDG_STATE_HOME":     true,
		"HOME":               true,
		"HERDR_ENV":          true,
		"HERDR_PANE_ID":      true,
		"HERGY_HERDR":        true,
		"HERGY_GY":           true,
		"SHOWCASE_HERDR_DIR": true,
	}
	env := make([]string, 0, len(os.Environ())+8)
	for _, one := range os.Environ() {
		name, _, _ := strings.Cut(one, "=")
		if !own[name] {
			env = append(env, one)
		}
	}
	return append(env,
		"XDG_CONFIG_HOME="+filepath.Join(work, "cfg"),
		"XDG_STATE_HOME="+filepath.Join(work, "state"),
		"HOME="+filepath.Join(work, "home"),
		"HERDR_ENV=",
		"HERDR_PANE_ID=",
		"HERGY_HERDR="+filepath.Join(work, "bin", "fake-herdr"),
		// No gy: the activity screen reads no questions and says so, rather
		// than reaching the person's ledger.
		"HERGY_GY="+filepath.Join(work, "bin", "no-gy"),
		"SHOWCASE_HERDR_DIR="+filepath.Join(work, "herdr"),
	)
}

// urlPattern is the URL hergy serve prints once its socket is open.
var urlPattern = regexp.MustCompile(`http://127\.0\.0\.1:[0-9]+`)

// served is one hergy serve this tool started.
type served struct {
	cmd  *exec.Cmd
	done chan error
	url  string
}

// startServe starts `hergy serve --no-open` over the work directory's project and
// returns it once its URL is printed.
func startServe(work, hergy string, port int, stderr io.Writer) (*served, error) {
	cmd := exec.Command(hergy, "serve", "--no-open", "--port", strconv.Itoa(port)) //nolint:gosec // G204: hergy is the binary this tool built or was given, and the flags are fixed
	cmd.Dir = filepath.Join(work, projectDir)
	cmd.Env = serveEnv(work)
	cmd.Stderr = stderr
	out, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("hergy serve を起こせません: %w", err)
	}
	url, err := readURL(out)
	if err != nil {
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
		return nil, err
	}
	go func() { _, _ = io.Copy(io.Discard, out) }()
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	return &served{cmd: cmd, done: done, url: url}, nil
}

// stop ends the serve and waits for it to go.
func (s *served) stop() {
	_ = s.cmd.Process.Signal(syscall.SIGTERM)
	select {
	case <-s.done:
	case <-time.After(stopGrace):
		_ = s.cmd.Process.Kill()
		<-s.done
	}
}

// waitStop waits until Ctrl-C (or SIGTERM) and stops the serve.
func waitStop(srv *served) error {
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	defer signal.Stop(stop)
	select {
	case err := <-srv.done:
		// A serve that stopped by itself: Ctrl-C reaches it directly when it
		// runs in this terminal, so a clean stop is the person's own.
		if err != nil {
			return fmt.Errorf("serve が止まりました: %w", err)
		}
		return nil
	case <-stop:
	}
	srv.stop()
	return nil
}

// readURL reads what the serve prints until the URL is on it. A serve that stops
// first is the failure, with everything it said.
func readURL(out io.Reader) (string, error) {
	reader := bufio.NewReader(out)
	var said strings.Builder
	for {
		line, err := reader.ReadString('\n')
		said.WriteString(line)
		if found := urlPattern.FindString(said.String()); found != "" {
			return found, nil
		}
		if err != nil {
			if errors.Is(err, io.EOF) {
				return "", fmt.Errorf("serve が URL を出す前に止まりました: %s", strings.TrimSpace(said.String()))
			}
			return "", err
		}
	}
}

// snapScript is the copy itself, written for Playwright: the page is read as a
// person reads it, its stylesheets are written into it (their fonts point at the
// repository's own static directory), its scripts go, and the HTML is saved.
// The page is opened at 1440 wide in the light colours, which is the width the
// screenshots of the scenes stand at.
const snapScript = `import { createRequire } from 'node:module';
import { writeFileSync } from 'node:fs';
import { pathToFileURL } from 'node:url';

const require = createRequire(process.env.SHOWCASE_REPO + '/e2e/package.json');
const { chromium } = require('playwright');

const staticBase = pathToFileURL(process.env.SHOWCASE_STATIC + '/').href;
const browser = await chromium.launch();
try {
  const page = await browser.newPage({
    viewport: { width: 1440, height: 900 },
    colorScheme: 'light',
    deviceScaleFactor: 1,
  });
  await page.goto(process.env.SHOWCASE_URL + process.env.SHOWCASE_PATH);
  await page.waitForLoadState('networkidle', { timeout: 5000 }).catch(() => {});
  await page.waitForTimeout(1200);
  const html = await page.evaluate(async (base) => {
    for (const link of [...document.querySelectorAll('link[rel="stylesheet"]')]) {
      const href = link.getAttribute('href') || '';
      const text = await fetch(href).then((r) => r.text());
      const style = document.createElement('style');
      style.textContent = text.split('/static/').join(base);
      link.replaceWith(style);
    }
    for (const script of [...document.querySelectorAll('script')]) script.remove();
    // The band's height is the width's own measure, not the width the copy was
    // taken at: the app sets --stick from .bar's height (teams-v2.js setStick),
    // and a copy that kept the 1440 number would stand an open row's head under
    // a taller band at a narrow width. The copy carries that one behaviour
    // again and nothing else; every other script is gone.
    const stick = document.createElement('script');
    stick.textContent =
      "(function(){var s=function(){var b=document.querySelector('.bar');" +
      "document.documentElement.style.setProperty('--stick',(b?b.offsetHeight:48)+'px')};" +
      "s();addEventListener('resize',s);addEventListener('load',s);" +
      "if(document.fonts&&document.fonts.ready)document.fonts.ready.then(s)})()";
    document.body.appendChild(stick);
    // The number the page carried is the width the copy was taken at, so the
    // copy carries none: the script above sets it at the width it is opened at.
    document.documentElement.removeAttribute('style');
    // outerHTML writes the emptied style attribute back on <html>; the copy has
    // no use for it, and a person reading the file should not see one.
    const outer = document.documentElement.outerHTML.replace(/^<html([^>]*?) style=""/, '<html$1');
    return ('<!doctype html>\n' + outer).split('/static/').join(base);
  }, staticBase);
  writeFileSync(process.env.SHOWCASE_OUT, html);
} finally {
  await browser.close();
}
`

// runSnap writes one screen to out, through Playwright from e2e/node_modules.
func runSnap(root, base, page, out string, stderr io.Writer) error {
	runtime, err := jsRuntime(root)
	if err != nil {
		return err
	}
	dir, err := os.MkdirTemp("", "hergy-showcase-snap-")
	if err != nil {
		return err
	}
	defer func() { _ = os.RemoveAll(dir) }()
	script := filepath.Join(dir, "snap.mjs")
	if err := os.WriteFile(script, []byte(snapScript), 0o600); err != nil {
		return err
	}
	env := append(os.Environ(),
		"SHOWCASE_REPO="+root,
		"SHOWCASE_URL="+base,
		"SHOWCASE_PATH="+page,
		"SHOWCASE_OUT="+out,
		"SHOWCASE_STATIC="+filepath.Join(root, "internal", "web", "static"),
	)
	if browsers := filepath.Join(root, "e2e", ".bin", "browsers"); isDir(browsers) {
		env = append(env, "PLAYWRIGHT_BROWSERS_PATH="+browsers)
	}
	cmd := exec.Command(runtime, script) //nolint:gosec // G204: runtime is bun or node, and the script is this tool's own
	cmd.Dir = root
	cmd.Env = env
	cmd.Stdout = stderr // Playwright's own lines are not the answer
	cmd.Stderr = stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("画面 %s を写せません: %w", page, err)
	}
	if _, err := os.Stat(out); err != nil {
		return fmt.Errorf("出力先 %s が書けていません: %w", out, err)
	}
	return nil
}

// jsRuntime is the JavaScript the copy runs on: the bun tools/check pins, or the
// bun or node the person has.
func jsRuntime(root string) (string, error) {
	for _, cand := range []string{filepath.Join(root, ".bin", "bun"), "bun", "node"} {
		if path, err := exec.LookPath(cand); err == nil {
			return path, nil
		}
	}
	return "", errors.New("bun も node も見つかりません（go run ./tools/check で .bin/bun を入れるか、node を入れてください）")
}

// isDir says path is a directory.
func isDir(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}

// execIn runs one program at dir, with its words going where this tool's do.
func execIn(dir string, stdout, stderr io.Writer, name string, args ...string) error {
	cmd := exec.Command(name, args...) //nolint:gosec // G204: name is go or git and the arguments are this tool's own
	cmd.Dir = dir
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("%s %s: %w", name, strings.Join(args, " "), err)
	}
	return nil
}
