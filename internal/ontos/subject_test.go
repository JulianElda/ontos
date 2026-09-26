package ontos

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestNormalizeURL(t *testing.T) {
	const want = "github.com/Org/Repo"
	for _, raw := range []string{
		"git@github.com:Org/Repo.git",            // ssh, scp-like
		"https://user@github.com/Org/Repo.git",   // https with a user and .git
		"https://GitHub.COM/Org/Repo",            // uppercase host
		"ssh://git@github.com:22/Org/Repo.git/",  // ssh URL with a port
		"  git@GITHUB.com:Org/Repo.git\n",        // whitespace from git config
		"https://user:token@github.com/Org/Repo", // credentials never survive
	} {
		if got := NormalizeURL(raw); got != want {
			t.Errorf("NormalizeURL(%q) = %q, want %q", raw, got, want)
		}
	}
	// The path keeps its case: only the host is case-insensitive.
	if got := NormalizeURL("git@github.com:org/repo.git"); got == want {
		t.Errorf("path case was folded: %q", got)
	}
	if got := NormalizeURL(""); got != "" {
		t.Errorf("empty remote gave %q", got)
	}
}

func TestSubjectFileEscapesSlashes(t *testing.T) {
	if got, want := subjectFile("repo/packages/logger"), "repo%2Fpackages%2Flogger.json"; got != want {
		t.Errorf("subjectFile = %q, want %q", got, want)
	}
}

func TestPutSubjectRejectsDuplicateLocation(t *testing.T) {
	s := newStore(t, "")
	mustSubject(t, s, &Subject{Name: "repo", URL: "github.com/Org/Repo"})
	if err := s.PutSubject(&Subject{Name: "other", URL: "git@github.com:Org/Repo.git"}); err == nil {
		t.Error("a second subject with the same url and path was accepted")
	}
	// Subjects with no repo share the empty location freely.
	mustSubject(t, s, &Subject{Name: "nix"})
	mustSubject(t, s, &Subject{Name: "dev-vm"})
}

func TestCleanSubjectPath(t *testing.T) {
	for in, want := range map[string]string{"": "", ".": "", "packages/logger/": "packages/logger", "./a/../b": "b"} {
		if got, err := CleanSubjectPath(in); err != nil || got != want {
			t.Errorf("CleanSubjectPath(%q) = %q, %v; want %q", in, got, err, want)
		}
	}
	for _, bad := range []string{"/abs", "..", "../x"} {
		if _, err := CleanSubjectPath(bad); err == nil {
			t.Errorf("CleanSubjectPath(%q) accepted a path outside the repo", bad)
		}
	}
}

func TestDefaultSubject(t *testing.T) {
	dir := gitRepo(t, "git@github.com:Org/Repo.git")
	for rel, want := range map[string]Subject{
		".":               {Name: "repo", URL: "github.com/Org/Repo"},
		"packages/logger": {Name: "repo/packages/logger", URL: "github.com/Org/Repo", Path: "packages/logger"},
	} {
		loc, err := Locate(filepath.Join(dir, rel))
		if err != nil {
			t.Fatal(err)
		}
		got, err := DefaultSubject(loc)
		if err != nil {
			t.Fatalf("%s: %v", rel, err)
		}
		if got.Name != want.Name || got.URL != want.URL || got.Path != want.Path {
			t.Errorf("%s: DefaultSubject = %+v, want %+v", rel, *got, want)
		}
	}
}

func TestDefaultSubjectWithoutOrigin(t *testing.T) {
	dir := gitRepo(t, "")
	loc, err := Locate(filepath.Join(dir, "packages", "logger"))
	if err != nil {
		t.Fatal(err)
	}
	// PutSubject refuses a path with no url, so there is nothing to propose
	// in a package; the error points at the repo root instead.
	if _, err := DefaultSubject(loc); err == nil || !strings.Contains(err.Error(), loc.Toplevel) {
		t.Errorf("DefaultSubject in a package with no origin: err = %v, want one naming %s", err, loc.Toplevel)
	}
	loc, err = Locate(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := DefaultSubject(loc); err == nil || !strings.Contains(err.Error(), "pass --name") {
		t.Errorf("DefaultSubject outside git: err = %v, want one saying to pass --name", err)
	}
}
