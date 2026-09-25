package main

import (
	"fmt"
	"slices"
	"strings"

	"github.com/JulianElda/ontos/internal/ontos"
)

func (a *app) cmdSubject(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: ontos subject add|update|list|which (see `ontos help`)")
	}
	switch args[0] {
	case "add":
		return a.cmdSubjectAdd(args[1:])
	case "update":
		return a.cmdSubjectUpdate(args[1:])
	case "list":
		return a.cmdSubjectList(args[1:])
	case "which":
		return a.cmdSubjectWhich(args[1:])
	default:
		return fmt.Errorf("unknown subject verb %q; use add, update, list or which", args[0])
	}
}

func (a *app) cmdSubjectAdd(args []string) error {
	fs := a.newFlagSet("subject add", "ontos subject add [--name n] [--url u] [--path p] [--uses s...]")
	sub := &ontos.Subject{}
	var uses listFlag
	fs.StringVar(&sub.Name, "name", "", "subject name (default: the repo's directory name, plus /<path> in a package)")
	fs.StringVar(&sub.URL, "url", "", "git remote (default: the repo's origin; '' for a subject with no repo)")
	fs.StringVar(&sub.Path, "path", "", "package path in the repo (default: cwd's path in the repo, empty at the root)")
	fs.Var(&uses, "uses", "another subject whose entries this one loads (repeatable)")
	positional, err := parseArgs(fs, args)
	if err != nil {
		return err
	}
	if len(positional) > 0 {
		fs.Usage()
		return fmt.Errorf("unexpected argument %q; subject add takes flags only", positional[0])
	}
	set := given(fs)
	sub.Uses = uses

	// Every default comes from where the command runs, and a flag given
	// overrides just that one: --name nix inside a repo still takes its url,
	// which is what --url '' is for. The path is relative to the repo the url
	// names, so it only defaults from cwd when the url does too.
	loc, err := ontos.Locate(".")
	if err != nil {
		return err
	}
	if loc.Toplevel == "" {
		if !set["name"] {
			return fmt.Errorf("not inside a git repository, so there is no default name; pass --name (and --url and --path if it has a repo)")
		}
	} else {
		if !set["url"] {
			sub.URL = loc.Remote
			if !set["path"] {
				sub.Path = loc.Rel
			}
		}
		if !set["name"] {
			p, err := ontos.CleanSubjectPath(sub.Path)
			if err != nil {
				return err
			}
			sub.Name = loc.RepoName()
			if p != "" {
				sub.Name += "/" + p
			}
		}
	}

	s, err := a.load()
	if err != nil {
		return err
	}
	if _, ok := s.Subjects[strings.TrimSpace(sub.Name)]; ok {
		return fmt.Errorf("subject %q already exists (fix: `ontos subject update %s`)", sub.Name, sub.Name)
	}
	if err := s.PutSubject(sub); err != nil {
		return err
	}
	a.printf("added subject %s\n", describeSubject(sub))
	a.warnUses(s, sub)
	return nil
}

func (a *app) cmdSubjectUpdate(args []string) error {
	fs := a.newFlagSet("subject update", "ontos subject update <name> [--url u] [--path p] [--uses s...]")
	var url, path string
	var uses listFlag
	fs.StringVar(&url, "url", "", "git remote; '' for no repo")
	fs.StringVar(&path, "path", "", "package path in the repo; '' for the root")
	fs.Var(&uses, "uses", "another subject whose entries this one loads (repeatable; --uses '' clears)")
	positional, err := parseArgs(fs, args)
	if err != nil {
		return err
	}
	if len(positional) != 1 {
		fs.Usage()
		return fmt.Errorf("name exactly one subject; there is no rename, since entries refer to subjects by name")
	}
	set := given(fs)
	if len(set) == 0 {
		fs.Usage()
		return fmt.Errorf("nothing to update: give at least one flag")
	}

	s, err := a.load()
	if err != nil {
		return err
	}
	old, err := s.Subject(positional[0])
	if err != nil {
		return err
	}
	sub := *old
	if set["url"] {
		sub.URL = url
	}
	if set["path"] {
		sub.Path = path
	}
	if set["uses"] {
		sub.Uses = slices.Clone(uses)
	}
	if err := s.PutSubject(&sub); err != nil {
		return err
	}
	a.printf("updated subject %s\n", describeSubject(&sub))
	a.warnUses(s, &sub)
	return nil
}

// warnUses reports names in uses that are not subjects yet. Not an error: a
// subject may be added before the ones it uses.
func (a *app) warnUses(s *ontos.Store, sub *ontos.Subject) {
	for _, name := range s.UnknownUses(sub) {
		a.warnf("warning: %q is not a subject yet (fix: `ontos subject add --name %s`)\n", name, name)
	}
}

func (a *app) cmdSubjectList(args []string) error {
	fs := a.newFlagSet("subject list", "ontos subject list [--json]")
	asJSON := fs.Bool("json", false, "print JSON")
	positional, err := parseArgs(fs, args)
	if err != nil {
		return err
	}
	if len(positional) > 0 {
		fs.Usage()
		return fmt.Errorf("unexpected argument %q", positional[0])
	}
	s, err := a.load()
	if err != nil {
		return err
	}
	subs := s.SortedSubjects()
	if *asJSON {
		return a.printJSON(subs)
	}
	if len(subs) == 0 {
		a.printf("No subjects yet (fix: `ontos subject add` in a repo).\n")
		return nil
	}
	for _, sub := range subs {
		a.printf("- %s\n", describeSubject(sub))
	}
	return nil
}

// describeSubject is one line: the name, where it lives, what it uses.
func describeSubject(sub *ontos.Subject) string {
	where := "no repo"
	if sub.URL != "" {
		where = sub.URL
		if sub.Path != "" {
			where += ", path " + sub.Path
		}
	}
	line := fmt.Sprintf("%s: %s", sub.Name, where)
	if len(sub.Uses) > 0 {
		line += "; uses " + strings.Join(sub.Uses, ", ")
	}
	return line
}

func (a *app) cmdSubjectWhich(args []string) error {
	fs := a.newFlagSet("subject which", "ontos subject which [path] [--json]")
	asJSON := fs.Bool("json", false, "print JSON")
	positional, err := parseArgs(fs, args)
	if err != nil {
		return err
	}
	if len(positional) > 1 {
		fs.Usage()
		return fmt.Errorf("name at most one path")
	}
	dir := "."
	if len(positional) == 1 {
		dir = positional[0]
	}
	s, err := a.load()
	if err != nil {
		return err
	}
	r, err := s.Resolve(dir)
	if err != nil {
		return err
	}
	if *asJSON {
		return a.printJSON(r)
	}
	for i, step := range r.Steps {
		a.printf("%d. %s\n", i+1, step)
	}
	if r.Subject == nil {
		a.printf("\nNo subject resolves here (fix: `ontos subject add` in this directory).\n")
	} else {
		a.printf("\nSubject: %s\n", describeSubject(r.Subject))
	}
	return nil
}
