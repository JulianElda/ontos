// Command ontos is the only writer of the ontos store: a knowledge base about
// repos and the things they depend on, read by Claude at session start through
// a SessionStart hook running `ontos context`, and written by Claude during
// the session through the other verbs.
//
// Every read verb prints markdown by default and JSON with --json. Errors go
// to stderr with exit status 1 and name the command that fixes them, because
// an agent reads them, and a bare failure gets improvised around.
package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/JulianElda/ontos/internal/ontos"
)

const usage = `ontos: repo knowledge for Claude Code

reading (markdown by default, --json for JSON):
  ontos context [path]              the session-start context for path (default: cwd)
  ontos search [query] [filters]    id, title, category and trigger of matching entries
      --subject <name>  --category <category>  --tag <tag>   exact, combined with AND
  ontos get <id>...                 entries in full

writing:
  ontos add --title <t> --category <c> --subject <s>... --trigger <t>
            --certainty <c> --body-file <file|->
            [--tag <t>...] [--checked <sha|date|dep@version>] [--source <path>...]
            [--related <id>...]
      prints the new id. --body-file - reads the markdown body from stdin.
  ontos update <id> [any add flag]  a flag not given leaves its field alone; a list
                                    flag replaces the whole list (--tag '' clears it)
  ontos delete <id>                 removes the entry and every related pointer to it

subjects:
  ontos subject add [--name <n>] [--url <u>] [--path <p>] [--uses <s>...]
      defaults come from cwd: the repo's origin url, the path in the repo, and
      the repo's directory name (plus /<path> in a package). --url '' makes a
      subject with no repo, such as a protocol or an environment.
  ontos subject update <name> [--url <u>] [--path <p>] [--uses <s>...]
  ontos subject list
  ontos subject which [path]        how path resolves to a subject, step by step

categories: map mechanism invariant gotcha procedure diagnosis gap reference
certainty:  confirmed (ran it or read it in code), inferred, suspected
`

func main() {
	os.Exit(run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr))
}

// app carries the streams, so tests run the whole CLI in-process.
type app struct {
	stdin          io.Reader
	stdout, stderr io.Writer
}

// run is main without the exit, returning the status instead.
func run(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	a := &app{stdin: stdin, stdout: stdout, stderr: stderr}
	if len(args) == 0 {
		a.warnf("%s", usage)
		return 2
	}

	cmd, rest := args[0], args[1:]
	var err error
	switch cmd {
	case "context":
		// The SessionStart hook runs this on every session; a non-zero exit
		// would get in the way of all of them. It has no error path.
		a.cmdContext(rest)
		return 0
	case "search":
		err = a.cmdSearch(rest)
	case "get":
		err = a.cmdGet(rest)
	case "add":
		err = a.cmdAdd(rest)
	case "update":
		err = a.cmdUpdate(rest)
	case "delete":
		err = a.cmdDelete(rest)
	case "subject":
		err = a.cmdSubject(rest)
	case "-h", "--help", "help":
		a.printf("%s", usage)
		return 0
	default:
		a.warnf("unknown command %q\n\n%s", cmd, usage)
		return 2
	}

	if errors.Is(err, flag.ErrHelp) {
		return 0 // the flag set has already printed its usage
	}
	if err != nil {
		a.warnf("ontos %s: %v\n", cmd, err)
		return 1
	}
	return 0
}

// load reads the config and the store for every verb but context. Anything
// wrong with the store goes to stderr, so it is seen without spoiling output
// that a caller may parse.
func (a *app) load() (*ontos.Store, error) {
	cfg, err := ontos.LoadConfig()
	if err != nil {
		return nil, err
	}
	s, err := ontos.Load(cfg.Store)
	if err != nil {
		return nil, err
	}
	for _, line := range s.Report() {
		a.warnf("warning: %s\n", line)
	}
	return s, nil
}

// printf and warnf write to stdout and stderr. A failed write there has
// nowhere left to be reported, so the error is dropped on purpose.
func (a *app) printf(format string, args ...any) { _, _ = fmt.Fprintf(a.stdout, format, args...) }
func (a *app) warnf(format string, args ...any)  { _, _ = fmt.Fprintf(a.stderr, format, args...) }

func (a *app) printJSON(v any) error {
	enc := json.NewEncoder(a.stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}

func (a *app) newFlagSet(name, synopsis string) *flag.FlagSet {
	fs := flag.NewFlagSet(name, flag.ContinueOnError)
	fs.SetOutput(a.stderr)
	fs.Usage = func() {
		a.warnf("usage: %s\n", synopsis)
		fs.PrintDefaults()
	}
	return fs
}

// parseArgs parses flags wherever they appear. Go's flag package stops at the
// first positional argument, so `ontos update abc --tag x` would otherwise
// silently ignore --tag. It resumes after each positional instead.
func parseArgs(fs *flag.FlagSet, args []string) ([]string, error) {
	var positional []string
	for rest := args; ; {
		if err := fs.Parse(rest); err != nil {
			return nil, err
		}
		if fs.NArg() == 0 {
			return positional, nil
		}
		positional = append(positional, fs.Arg(0))
		rest = fs.Args()[1:]
	}
}

// given reports which flags were set on the command line, as opposed to left
// at their defaults. update depends on it: a flag not given is a field left
// alone.
func given(fs *flag.FlagSet) map[string]bool {
	set := map[string]bool{}
	fs.Visit(func(f *flag.Flag) { set[f.Name] = true })
	return set
}

// listFlag collects a repeatable flag. An empty value adds nothing but still
// counts as given, which is how `update --tag ”` clears a list.
type listFlag []string

func (l *listFlag) String() string { return strings.Join(*l, ",") }

func (l *listFlag) Set(v string) error {
	if v = strings.TrimSpace(v); v != "" {
		*l = append(*l, v)
	}
	return nil
}

func (a *app) readBody(name string) (string, error) {
	var raw []byte
	var err error
	if name == "-" {
		raw, err = io.ReadAll(a.stdin)
	} else {
		raw, err = os.ReadFile(name)
	}
	if err != nil {
		return "", fmt.Errorf("reading body: %w", err)
	}
	return strings.TrimSpace(string(raw)), nil
}
