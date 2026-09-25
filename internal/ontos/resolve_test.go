package ontos

import (
	"path/filepath"
	"testing"
)

func TestResolveByURL(t *testing.T) {
	s := newStore(t, "")
	mustSubject(t, s, &Subject{Name: "repo", URL: "github.com/Org/Repo"})
	mustSubject(t, s, &Subject{Name: "repo/packages/logger", URL: "github.com/Org/Repo", Path: "packages/logger"})
	// A sibling whose path is a string prefix but not a directory prefix.
	mustSubject(t, s, &Subject{Name: "repo/packages/log", URL: "github.com/Org/Repo", Path: "packages/log"})
	dir := gitRepo(t, "git@github.com:Org/Repo.git")

	for rel, want := range map[string]string{
		".":                   "repo",
		"packages":            "repo",
		"packages/logger":     "repo/packages/logger",
		"packages/logger/src": "repo/packages/logger",
	} {
		r, err := s.Resolve(filepath.Join(dir, rel))
		if err != nil {
			t.Fatal(err)
		}
		if r.Subject == nil || r.Subject.Name != want {
			t.Errorf("%s resolved to %v, want %s\nsteps: %q", rel, r.Subject, want, r.Steps)
		}
	}
}

func TestResolveFallsBackToName(t *testing.T) {
	s := newStore(t, "")
	mustSubject(t, s, &Subject{Name: "repo"})
	dir := gitRepo(t, "")

	r, err := s.Resolve(filepath.Join(dir, "packages", "logger"))
	if err != nil {
		t.Fatal(err)
	}
	if r.Subject == nil || r.Subject.Name != "repo" {
		t.Fatalf("resolved to %v, want the name fallback to repo\nsteps: %q", r.Subject, r.Steps)
	}
	if last := r.Steps[len(r.Steps)-1]; last != `fallback: subject "repo" is named after the toplevel and has no path` {
		t.Errorf("last step = %q, want the fallback explained", last)
	}
}

func TestResolveOutsideGit(t *testing.T) {
	s := newStore(t, "")
	mustSubject(t, s, &Subject{Name: "x"})
	r, err := s.Resolve(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if r.Subject != nil {
		t.Errorf("resolved %v outside a git repo", r.Subject)
	}
}
