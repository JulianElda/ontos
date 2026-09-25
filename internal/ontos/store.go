package ontos

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// Schema is the version every file this build writes carries. A file with any
// other value was written by a different ontos and is not read.
const Schema = 1

const tmpPrefix = ".ontos-tmp-"

// Store is the whole store, loaded into memory. Hundreds of entries are well
// under a megabyte, so there is no index: every command reads everything.
type Store struct {
	Dir      string
	Entries  map[string]*Entry
	Subjects map[string]*Subject
	// Conflicts are sync clients' conflict copies, relative to Dir. They are
	// never loaded, and always reported: one nobody mentions sits unresolved.
	Conflicts []string
	// Problems are files that could not be loaded, one line each naming the
	// file. Reads carry on without them; writes refuse (see Writable).
	Problems []string
}

func (s *Store) entriesDir() string  { return filepath.Join(s.Dir, "entries") }
func (s *Store) subjectsDir() string { return filepath.Join(s.Dir, "subjects") }

// Load reads both folders whole. It fails only when the store itself is not
// there; a bad file becomes a Problem so the rest stays readable.
func Load(dir string) (*Store, error) {
	s := &Store{Dir: dir, Entries: map[string]*Entry{}, Subjects: map[string]*Subject{}}

	if err := s.loadDir("entries", func(name string, data []byte) error {
		var e Entry
		if err := decode(data, &e.Schema, &e); err != nil {
			return err
		}
		if want := e.ID + ".json"; name != want {
			return fmt.Errorf("id %q does not match the file name (want %s)", e.ID, want)
		}
		e.normalize()
		s.Entries[e.ID] = &e
		return nil
	}); err != nil {
		return nil, err
	}

	if err := s.loadDir("subjects", func(name string, data []byte) error {
		var sub Subject
		if err := decode(data, &sub.Schema, &sub); err != nil {
			return err
		}
		if want := subjectFile(sub.Name); name != want {
			return fmt.Errorf("name %q does not match the file name (want %s)", sub.Name, want)
		}
		sub.normalize()
		s.Subjects[sub.Name] = &sub
		return nil
	}); err != nil {
		return nil, err
	}
	return s, nil
}

func (s *Store) loadDir(sub string, load func(name string, data []byte) error) error {
	dir := filepath.Join(s.Dir, sub)
	files, err := os.ReadDir(dir)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return fmt.Errorf("no store at %s (fix: %s, or point `store` in %s at it)", dir, SetupHint, ConfigPath())
		}
		return err
	}
	for _, f := range files {
		name := f.Name()
		switch {
		case f.IsDir(), strings.HasPrefix(name, tmpPrefix), !strings.HasSuffix(name, ".json"):
			// A leftover temp file is a write that died before its rename; the
			// file it was replacing is still intact, so there is nothing to read.
			continue
		case IsConflictCopy(name):
			s.Conflicts = append(s.Conflicts, filepath.Join(sub, name))
			continue
		}
		data, err := os.ReadFile(filepath.Join(dir, name))
		if err == nil {
			err = load(name, data)
		}
		if err != nil {
			s.Problems = append(s.Problems, fmt.Sprintf("%s: %v", filepath.Join(sub, name), err))
		}
	}
	return nil
}

// decode checks the schema before decoding the rest, so a file from a newer
// ontos is reported as that rather than as whatever field failed to parse.
func decode(data []byte, schema *int, v any) error {
	var head struct {
		Schema *int `json:"schema"`
	}
	if err := json.Unmarshal(data, &head); err != nil {
		return fmt.Errorf("not valid JSON: %w", err)
	}
	if head.Schema == nil {
		return fmt.Errorf("no schema field; this build reads schema %d", Schema)
	}
	if *head.Schema != Schema {
		return fmt.Errorf("schema %d; this build reads schema %d (fix: rebuild ontos from an up-to-date checkout)", *head.Schema, Schema)
	}
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	if err := dec.Decode(v); err != nil {
		return fmt.Errorf("not a valid file: %w", err)
	}
	*schema = Schema
	return nil
}

// Writable is the check every write makes first. A file that did not load is
// one a write could contradict without knowing, so nothing is written until it
// is fixed or removed.
func (s *Store) Writable() error {
	if len(s.Problems) == 0 {
		return nil
	}
	return fmt.Errorf("the store has files that did not load, so nothing is written until they are fixed or removed:\n  %s",
		strings.Join(s.Problems, "\n  "))
}

func (s *Store) writeJSON(path string, v any) error {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	return writeAtomic(path, append(data, '\n'))
}

// writeAtomic replaces path in one rename, so a sync client (or a reader) only
// ever sees the old file or the new one, never half of either.
func writeAtomic(path string, data []byte) (err error) {
	f, err := os.CreateTemp(filepath.Dir(path), tmpPrefix+"*")
	if err != nil {
		return err
	}
	defer func() {
		if err != nil {
			_ = f.Close()
			_ = os.Remove(f.Name())
		}
	}()
	if _, err = f.Write(data); err != nil {
		return err
	}
	if err = f.Sync(); err != nil {
		return err
	}
	if err = f.Close(); err != nil {
		return err
	}
	return os.Rename(f.Name(), path)
}
