package ontos

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// newStore makes an empty store in a temp dir and points ONTOS_CONFIG at a
// config naming it, with extra appended to the config (a [context] table).
func newStore(t *testing.T, extra string) *Store {
	t.Helper()
	dir := t.TempDir()
	for _, sub := range []string{"entries", "subjects"} {
		if err := os.MkdirAll(filepath.Join(dir, sub), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	cfg := filepath.Join(dir, "config.toml")
	if err := os.WriteFile(cfg, []byte("store = \""+dir+"\"\n"+extra), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("ONTOS_CONFIG", cfg)
	return mustLoad(t, dir)
}

func mustLoad(t *testing.T, dir string) *Store {
	t.Helper()
	s, err := Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func mustSubject(t *testing.T, s *Store, sub *Subject) {
	t.Helper()
	if err := s.PutSubject(sub); err != nil {
		t.Fatal(err)
	}
}

// mustEntry adds an entry with every required field filled in, overridable
// by the caller through the fields it sets.
func mustEntry(t *testing.T, s *Store, e Entry) *Entry {
	t.Helper()
	if e.Trigger == "" {
		e.Trigger = "when testing"
	}
	if e.Certainty == "" {
		e.Certainty = "confirmed"
	}
	if e.Body == "" {
		e.Body = "body of " + e.Title
	}
	if err := s.PutEntry(&e); err != nil {
		t.Fatal(err)
	}
	return &e
}

// gitRepo makes a git repo at <tmp>/repo with a packages/logger/src tree and,
// if remote is not empty, an origin remote.
func gitRepo(t *testing.T, remote string) string {
	t.Helper()
	dir := filepath.Join(t.TempDir(), "repo")
	if err := os.MkdirAll(filepath.Join(dir, "packages", "logger", "src"), 0o755); err != nil {
		t.Fatal(err)
	}
	run := func(args ...string) {
		if out, err := exec.Command("git", append([]string{"-C", dir}, args...)...).CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	run("init", "-q")
	if remote != "" {
		run("remote", "add", "origin", remote)
	}
	return dir
}
