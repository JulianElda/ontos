package ontos

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestEntryRoundTrip(t *testing.T) {
	s := newStore(t, "")
	mustSubject(t, s, &Subject{Name: "repo", URL: "github.com/Org/Repo"})
	other := mustEntry(t, s, Entry{Title: "other", Category: "map", Subjects: []string{"repo"}})
	want := mustEntry(t, s, Entry{
		Title:     "Spinner forever, no layout",
		Category:  "diagnosis",
		Subjects:  []string{"repo"},
		Trigger:   "the app shows a spinner and never a layout",
		Tags:      []string{"broker", "startup"},
		Certainty: "inferred",
		Checked:   "abc1234",
		Sources:   []string{"src/app.ts", "src/broker/"},
		Related:   []string{other.ID},
		Body:      "| symptom | check |\n| --- | --- |\n| spinner | `msg-tail` |\n\nUnicode survives: → ✓",
	})

	got, err := mustLoad(t, s.Dir).Entry(want.ID)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("round trip changed the entry:\n got  %+v\n want %+v", got, want)
	}
	if len(want.ID) != 8 || strings.Trim(want.ID, "abcdefghijklmnopqrstuvwxyz234567") != "" {
		t.Errorf("id %q is not 8 lowercase base32 characters", want.ID)
	}
}

func TestWriteLeavesNoTempFiles(t *testing.T) {
	s := newStore(t, "")
	mustSubject(t, s, &Subject{Name: "repo/packages/logger", URL: "github.com/Org/Repo", Path: "packages/logger"})
	e := mustEntry(t, s, Entry{Title: "t", Category: "gotcha", Subjects: []string{"repo/packages/logger"}})
	e.Title = "t2"
	if err := s.PutEntry(e); err != nil {
		t.Fatal(err)
	}
	for _, sub := range []string{"entries", "subjects"} {
		files, err := os.ReadDir(filepath.Join(s.Dir, sub))
		if err != nil {
			t.Fatal(err)
		}
		for _, f := range files {
			if strings.HasPrefix(f.Name(), tmpPrefix) {
				t.Errorf("temp file left behind: %s/%s", sub, f.Name())
			}
		}
	}
	if _, err := os.Stat(filepath.Join(s.Dir, "subjects", "repo%2Fpackages%2Flogger.json")); err != nil {
		t.Errorf("subject file not at its escaped name: %v", err)
	}
}

func TestConflictCopyExcludedAndReported(t *testing.T) {
	s := newStore(t, "")
	mustSubject(t, s, &Subject{Name: "repo"})
	e := mustEntry(t, s, Entry{Title: "t", Category: "map", Subjects: []string{"repo"}})
	data, err := os.ReadFile(s.entryPath(e.ID))
	if err != nil {
		t.Fatal(err)
	}
	copyName := "x (conflicted copy).json"
	if err := os.WriteFile(filepath.Join(s.Dir, "entries", copyName), data, 0o644); err != nil {
		t.Fatal(err)
	}

	loaded := mustLoad(t, s.Dir)
	if len(loaded.Entries) != 1 {
		t.Errorf("loaded %d entries, want 1: the conflict copy was read", len(loaded.Entries))
	}
	if want := []string{filepath.Join("entries", copyName)}; !reflect.DeepEqual(loaded.Conflicts, want) {
		t.Errorf("Conflicts = %v, want %v", loaded.Conflicts, want)
	}
	if len(loaded.Problems) != 0 {
		t.Errorf("a conflict copy is not a load problem: %v", loaded.Problems)
	}
	if report := strings.Join(loaded.Report(), "\n"); !strings.Contains(report, copyName) {
		t.Errorf("Report does not name the conflict copy:\n%s", report)
	}
}

func TestOtherSchemaIsALoadError(t *testing.T) {
	s := newStore(t, "")
	name := "aaaaaaaa.json"
	body := `{"schema": 2, "id": "aaaaaaaa", "title": "from the future"}`
	if err := os.WriteFile(filepath.Join(s.Dir, "entries", name), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}

	loaded := mustLoad(t, s.Dir)
	if len(loaded.Problems) != 1 || !strings.Contains(loaded.Problems[0], name) || !strings.Contains(loaded.Problems[0], "schema 2") {
		t.Fatalf("Problems = %v, want one naming %s and schema 2", loaded.Problems, name)
	}
	if len(loaded.Entries) != 0 {
		t.Error("the schema 2 entry was loaded")
	}
	err := loaded.Writable()
	if err == nil || !strings.Contains(err.Error(), name) {
		t.Errorf("Writable() = %v, want an error naming %s", err, name)
	}
}

func TestUnparseableFileIsALoadError(t *testing.T) {
	s := newStore(t, "")
	if err := os.WriteFile(filepath.Join(s.Dir, "subjects", "broken.json"), []byte("{"), 0o644); err != nil {
		t.Fatal(err)
	}
	loaded := mustLoad(t, s.Dir)
	if len(loaded.Problems) != 1 || !strings.Contains(loaded.Problems[0], "broken.json") {
		t.Errorf("Problems = %v, want one naming broken.json", loaded.Problems)
	}
}

func TestMissingStoreNamesTheFix(t *testing.T) {
	_, err := Load(filepath.Join(t.TempDir(), "nope"))
	if err == nil || !strings.Contains(err.Error(), "setup.sh") {
		t.Errorf("Load of a missing store = %v, want an error naming setup.sh", err)
	}
}
