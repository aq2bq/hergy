package options

import "testing"

// validValue is a value each flag that takes one accepts, so a test can ask
// "does the command take this flag" without tripping on the value.
var validValue = map[string]string{
	"as": "alpha", "dir": ".", "team": "t", "profile": "p", "purpose": "x",
	"task": "t", "from": "m", "lang": "en", "timeout": "5000",
}

// flagArgs writes a flag the way the command line does: the switch alone, or
// with a value.
func flagArgs(f *Flag) []string {
	if f.Value == nil {
		return []string{"--" + f.Name}
	}
	return []string{"--" + f.Name, validValue[f.Name]}
}

// argvOf is a command line for one command: n positional arguments, then every
// required flag of the command except skip.
func argvOf(s *Spec, n int, skip *Flag) []string {
	argv := []string{s.Name}
	for i := 0; i < n; i++ {
		argv = append(argv, "arg")
	}
	for _, u := range s.Flags {
		if u.Required && u.Flag != skip {
			argv = append(argv, flagArgs(u.Flag)...)
		}
	}
	return argv
}

func accepts(s *Spec, f *Flag) bool {
	for _, list := range [][]Use{s.Flags, common} {
		for _, u := range list {
			if u.Flag == f {
				return true
			}
		}
	}
	return false
}

// TestParserAcceptsExactlyTheTable: for every command and every flag hergy
// reads, the parser takes the flag when the command's row lists it (or it is
// --lang or --help), and refuses it otherwise.
func TestParserAcceptsExactlyTheTable(t *testing.T) {
	for i := range specs {
		s := &specs[i]
		if s.Bare {
			continue
		}
		for _, f := range allFlags {
			if f == flagHelp {
				continue // it stops the parse and prints the command's help: the help tests cover it
			}
			n := len(s.Args)
			for _, a := range s.Args {
				if a.Or == f {
					n-- // --all stands in for <member>; the two together are an error
				}
			}
			argv := append(argvOf(s, n, nil), flagArgs(f)...)
			if f == flagJSON && accepts(s, flagDry) {
				argv = append(argv, "--dry") // --json is an error without --dry
			}
			_, err := Parse(argv)
			if want := accepts(s, f); want && err != nil {
				t.Errorf("hergy %s --%s: the table lists it, but Parse refuses it: %v", s.Name, f.Name, err)
			} else if !want && err == nil {
				t.Errorf("hergy %s --%s: the table does not list it, but Parse accepts it", s.Name, f.Name)
			}
		}
	}
}

// TestPositionalsMatchTable: the parser counts positional arguments in a branch
// of its own, with an error message for each command. The count agrees with the
// Args of the table: all of them are taken, one more is refused, a required one
// cannot be left out, and an optional one can.
func TestPositionalsMatchTable(t *testing.T) {
	for i := range specs {
		s := &specs[i]
		if s.Bare {
			continue
		}
		required := 0
		var or *Arg
		for j := range s.Args {
			switch {
			case s.Args[j].Or != nil:
				or = &s.Args[j]
			case !s.Args[j].Optional:
				required++
			}
		}
		if _, err := Parse(argvOf(s, len(s.Args), nil)); err != nil {
			t.Errorf("hergy %s with all %d positional argument(s) of the table: %v", s.Name, len(s.Args), err)
		}
		if _, err := Parse(argvOf(s, len(s.Args)+1, nil)); err == nil {
			t.Errorf("hergy %s has %d positional argument(s) in the table, but Parse accepts one more", s.Name, len(s.Args))
		}
		if required > 0 {
			if _, err := Parse(argvOf(s, required-1, nil)); err == nil {
				t.Errorf("hergy %s requires %d positional argument(s) in the table, but Parse accepts %d", s.Name, required, required-1)
			}
		}
		if or == nil {
			if _, err := Parse(argvOf(s, required, nil)); err != nil {
				t.Errorf("hergy %s requires only %d positional argument(s) in the table, but Parse refuses that many: %v", s.Name, required, err)
			}
			continue
		}
		if _, err := Parse(argvOf(s, required, nil)); err == nil {
			t.Errorf("hergy %s needs <%s> or --%s in the table, but Parse accepts neither", s.Name, or.Name["en"], or.Or.Name)
		}
		if _, err := Parse(append(argvOf(s, required, nil), flagArgs(or.Or)...)); err != nil {
			t.Errorf("hergy %s --%s stands in for <%s> in the table, but Parse refuses it: %v", s.Name, or.Or.Name, or.Name["en"], err)
		}
	}
}

// TestRequiredFlagsMatchTable: a flag the table marks required is one the
// command cannot run without, and every other flag it can.
func TestRequiredFlagsMatchTable(t *testing.T) {
	for i := range specs {
		s := &specs[i]
		if s.Bare {
			continue
		}
		for _, u := range s.Flags {
			_, err := Parse(argvOf(s, len(s.Args), u.Flag))
			if u.Required && err == nil {
				t.Errorf("hergy %s: the table requires --%s, but Parse runs without it", s.Name, u.Flag.Name)
			}
			if !u.Required && err != nil {
				t.Errorf("hergy %s: the table does not require --%s, but Parse refuses the command without it: %v", s.Name, u.Flag.Name, err)
			}
		}
	}
}
