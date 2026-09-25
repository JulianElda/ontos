package main

import (
	"flag"
	"fmt"
	"slices"
	"strings"

	"github.com/JulianElda/ontos/internal/ontos"
)

func (a *app) cmdSearch(args []string) error {
	fs := a.newFlagSet("search", "ontos search [query] [--subject s] [--category c] [--tag t] [--json]")
	var f ontos.Filter
	fs.StringVar(&f.Subject, "subject", "", "only entries on this subject")
	fs.StringVar(&f.Category, "category", "", "only entries in this category")
	fs.StringVar(&f.Tag, "tag", "", "only entries with this tag")
	asJSON := fs.Bool("json", false, "print JSON")
	terms, err := parseArgs(fs, args)
	if err != nil {
		return err
	}
	s, err := a.load()
	if err != nil {
		return err
	}
	if err := s.Check(f); err != nil {
		return err
	}

	hits := s.Search(strings.Join(terms, " "), f)
	if *asJSON {
		return a.printJSON(hits)
	}
	if len(hits) == 0 {
		a.printf("No entries match.\n")
		return nil
	}
	for _, h := range hits {
		a.printf("- [%s] %s: %s (%s)\n", h.Category, h.Title, h.Trigger, h.ID)
	}
	return nil
}

func (a *app) cmdGet(args []string) error {
	fs := a.newFlagSet("get", "ontos get <id>... [--json]")
	asJSON := fs.Bool("json", false, "print JSON")
	ids, err := parseArgs(fs, args)
	if err != nil {
		return err
	}
	if len(ids) == 0 {
		fs.Usage()
		return fmt.Errorf("name at least one id (`ontos search` lists them)")
	}
	s, err := a.load()
	if err != nil {
		return err
	}
	// Look every id up before printing any, so a typo fails the whole call
	// rather than leaving half the output.
	entries := make([]*ontos.Entry, len(ids))
	for i, id := range ids {
		if entries[i], err = s.Entry(id); err != nil {
			return err
		}
	}

	if *asJSON {
		return a.printJSON(entries)
	}
	for i, e := range entries {
		if i > 0 {
			a.printf("\n")
		}
		a.printf("%s", entryMarkdown(e))
	}
	return nil
}

func entryMarkdown(e *ontos.Entry) string {
	var b strings.Builder
	fmt.Fprintf(&b, "## %s (%s)\n\n", e.Title, e.ID)
	fields := []struct{ key, value string }{
		{"category", e.Category},
		{"certainty", e.Certainty},
		{"subjects", strings.Join(e.Subjects, ", ")},
		{"trigger", e.Trigger},
		{"tags", strings.Join(e.Tags, ", ")},
		{"checked", e.Checked},
		{"sources", strings.Join(e.Sources, ", ")},
		{"related", strings.Join(e.Related, ", ")},
	}
	for _, f := range fields {
		if f.value != "" {
			fmt.Fprintf(&b, "- %s: %s\n", f.key, f.value)
		}
	}
	fmt.Fprintf(&b, "\n%s\n", strings.TrimSpace(e.Body))
	return b.String()
}

// entryFlags are the flags add and update share. They bind straight onto an
// Entry, so update can parse onto a copy of the existing one and every flag
// not given leaves its field as it was.
type entryFlags struct {
	fs       *flag.FlagSet
	bodyFile string
	lists    map[string]*listFlag
}

func bindEntryFlags(fs *flag.FlagSet, e *ontos.Entry) *entryFlags {
	ef := &entryFlags{fs: fs, lists: map[string]*listFlag{}}
	list := func(name, usage string) {
		ef.lists[name] = &listFlag{}
		fs.Var(ef.lists[name], name, usage)
	}
	fs.StringVar(&e.Title, "title", e.Title, "worded the way Claude would look it up; for a diagnosis, the symptom")
	fs.StringVar(&e.Category, "category", e.Category, "one of: "+strings.Join(ontos.Categories, ", "))
	fs.StringVar(&e.Trigger, "trigger", e.Trigger, "one line: read this when…")
	fs.StringVar(&e.Certainty, "certainty", e.Certainty, "one of: "+strings.Join(ontos.Certainties, ", "))
	fs.StringVar(&e.Checked, "checked", e.Checked, "what it was last verified against: a sha, a date, or dep@version")
	fs.StringVar(&ef.bodyFile, "body-file", "", "file holding the markdown body, or - for stdin")
	list("subject", "subject name (repeatable)")
	list("tag", "topic cutting across categories (repeatable)")
	list("source", "repo path the entry was verified against, no line numbers (repeatable)")
	list("related", "id of a related entry (repeatable)")
	return ef
}

// apply copies the list flags that were given onto e, replacing each list
// whole, and reads the body if --body-file was given. It reports whether any
// flag was given at all.
func (ef *entryFlags) apply(a *app, e *ontos.Entry) (bool, error) {
	set := given(ef.fs)
	for name, dst := range map[string]*[]string{"subject": &e.Subjects, "tag": &e.Tags, "source": &e.Sources, "related": &e.Related} {
		if set[name] {
			*dst = slices.Clone(*ef.lists[name])
		}
	}
	if set["body-file"] {
		body, err := a.readBody(ef.bodyFile)
		if err != nil {
			return true, err
		}
		e.Body = body
	}
	return len(set) > 0, nil
}

func (a *app) cmdAdd(args []string) error {
	fs := a.newFlagSet("add", "ontos add --title t --category c --subject s... --trigger t --certainty c --body-file f|- [--tag t...] [--checked c] [--source p...] [--related id...]")
	e := &ontos.Entry{}
	ef := bindEntryFlags(fs, e)
	positional, err := parseArgs(fs, args)
	if err != nil {
		return err
	}
	if len(positional) > 0 {
		fs.Usage()
		return fmt.Errorf("unexpected argument %q; add takes flags only, and the body goes through --body-file", positional[0])
	}
	set := given(fs)
	var missing []string
	for _, name := range []string{"title", "category", "subject", "trigger", "certainty", "body-file"} {
		if !set[name] {
			missing = append(missing, "--"+name)
		}
	}
	if len(missing) > 0 {
		return fmt.Errorf("missing required flag(s): %s", strings.Join(missing, ", "))
	}

	s, err := a.load()
	if err != nil {
		return err
	}
	if _, err := ef.apply(a, e); err != nil {
		return err
	}
	if err := s.PutEntry(e); err != nil {
		return err
	}
	a.printf("%s\n", e.ID)
	return nil
}

func (a *app) cmdUpdate(args []string) error {
	fs := a.newFlagSet("update", "ontos update <id> [--title t] [--category c] [--subject s...] [--trigger t] [--certainty c] [--body-file f|-] [--tag t...] [--checked c] [--source p...] [--related id...]")
	s, err := a.load()
	if err != nil {
		return err
	}
	// The id comes first on the command line but the flags bind onto the
	// entry it names, so find it before parsing.
	if len(args) == 0 || strings.HasPrefix(args[0], "-") {
		fs.Usage()
		return fmt.Errorf("name the entry to update first: ontos update <id> [flags]")
	}
	old, err := s.Entry(args[0])
	if err != nil {
		return err
	}
	e := *old
	ef := bindEntryFlags(fs, &e)
	positional, err := parseArgs(fs, args[1:])
	if err != nil {
		return err
	}
	if len(positional) > 0 {
		fs.Usage()
		return fmt.Errorf("unexpected argument %q; update takes one id", positional[0])
	}
	changed, err := ef.apply(a, &e)
	if err != nil {
		return err
	}
	if !changed {
		fs.Usage()
		return fmt.Errorf("nothing to update: give at least one flag")
	}
	if err := s.PutEntry(&e); err != nil {
		return err
	}
	a.printf("updated %s\n", e.ID)
	return nil
}

func (a *app) cmdDelete(args []string) error {
	fs := a.newFlagSet("delete", "ontos delete <id>")
	ids, err := parseArgs(fs, args)
	if err != nil {
		return err
	}
	if len(ids) != 1 {
		fs.Usage()
		return fmt.Errorf("name exactly one id")
	}
	s, err := a.load()
	if err != nil {
		return err
	}
	rewritten, err := s.DeleteEntry(ids[0])
	if len(rewritten) > 0 {
		a.printf("removed %s from the related list of: %s\n", ids[0], strings.Join(rewritten, ", "))
	}
	if err != nil {
		return err
	}
	a.printf("deleted %s\n", ids[0])
	return nil
}
