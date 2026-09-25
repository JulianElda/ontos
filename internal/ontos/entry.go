package ontos

import (
	"crypto/rand"
	"encoding/base32"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

// Entry is one piece of knowledge, about the size of a doc section. It says
// how things are now; a change overwrites it and nothing keeps the old text.
type Entry struct {
	Schema    int      `json:"schema"`
	ID        string   `json:"id"`
	Title     string   `json:"title"`
	Category  string   `json:"category"`
	Subjects  []string `json:"subjects"`
	Trigger   string   `json:"trigger"`
	Tags      []string `json:"tags"`
	Certainty string   `json:"certainty"`
	Checked   string   `json:"checked"`
	Sources   []string `json:"sources"`
	Related   []string `json:"related"`
	Body      string   `json:"body"`
}

// Certainties, from most to least sure. `confirmed` is for what was run or
// read in code; a suspected bug is never written as fact.
var Certainties = []string{"confirmed", "inferred", "suspected"}

// normalize writes empty lists as [] rather than null, so a file reads the
// same whether a list was never set or was cleared.
func (e *Entry) normalize() {
	for _, l := range []*[]string{&e.Subjects, &e.Tags, &e.Sources, &e.Related} {
		if *l == nil {
			*l = []string{}
		}
	}
}

// Entry looks up one entry by id.
func (s *Store) Entry(id string) (*Entry, error) {
	if e, ok := s.Entries[id]; ok {
		return e, nil
	}
	return nil, fmt.Errorf("no entry %q (fix: `ontos search` lists the ids)", id)
}

// validate reports every problem at once, so one retry fixes them all.
func (s *Store) validate(e *Entry) error {
	var errs []error
	for field, v := range map[string]string{"title": e.Title, "trigger": e.Trigger, "body": e.Body} {
		if strings.TrimSpace(v) == "" {
			errs = append(errs, fmt.Errorf("%s is required", field))
		}
	}
	for field, v := range map[string]string{"title": e.Title, "trigger": e.Trigger, "checked": e.Checked} {
		if strings.Contains(v, "\n") {
			errs = append(errs, fmt.Errorf("%s must be one line", field))
		}
	}
	if !slices.Contains(Categories, e.Category) {
		errs = append(errs, fmt.Errorf("category %q is not one of: %s", e.Category, strings.Join(Categories, ", ")))
	}
	if !slices.Contains(Certainties, e.Certainty) {
		errs = append(errs, fmt.Errorf("certainty %q is not one of: %s", e.Certainty, strings.Join(Certainties, ", ")))
	}
	if len(e.Subjects) == 0 {
		errs = append(errs, errors.New("at least one subject is required"))
	}
	for _, name := range e.Subjects {
		if _, ok := s.Subjects[name]; !ok {
			errs = append(errs, fmt.Errorf("no subject %q (fix: `ontos subject list`, or `ontos subject add`)", name))
		}
	}
	for _, id := range e.Related {
		if _, ok := s.Entries[id]; !ok {
			errs = append(errs, fmt.Errorf("related entry %q does not exist", id))
		}
	}
	slices.SortFunc(errs, func(a, b error) int { return strings.Compare(a.Error(), b.Error()) })
	return errors.Join(errs...)
}

// PutEntry validates e and writes it, giving it a new id if it has none.
func (s *Store) PutEntry(e *Entry) error {
	if err := s.Writable(); err != nil {
		return err
	}
	e.Schema = Schema
	e.normalize()
	if err := s.validate(e); err != nil {
		return err
	}
	if e.ID == "" {
		id, err := s.newID()
		if err != nil {
			return err
		}
		e.ID = id
	}
	if err := s.writeJSON(s.entryPath(e.ID), e); err != nil {
		return err
	}
	s.Entries[e.ID] = e
	return nil
}

// DeleteEntry removes an entry and every pointer to it, and returns the ids of
// the entries whose `related` it rewrote. The pointers go first: if the delete
// then fails, what is left is an entry nothing points to, not a dangling id.
func (s *Store) DeleteEntry(id string) ([]string, error) {
	if err := s.Writable(); err != nil {
		return nil, err
	}
	if _, err := s.Entry(id); err != nil {
		return nil, err
	}
	var rewritten []string
	for _, other := range s.sortedEntries() {
		if other.ID == id || !slices.Contains(other.Related, id) {
			continue
		}
		updated := *other
		updated.Related = slices.DeleteFunc(slices.Clone(other.Related), func(r string) bool { return r == id })
		if err := s.writeJSON(s.entryPath(other.ID), &updated); err != nil {
			return rewritten, err
		}
		s.Entries[other.ID] = &updated
		rewritten = append(rewritten, other.ID)
	}
	if err := os.Remove(s.entryPath(id)); err != nil {
		return rewritten, err
	}
	delete(s.Entries, id)
	return rewritten, nil
}

func (s *Store) entryPath(id string) string { return filepath.Join(s.entriesDir(), id+".json") }

// sortedEntries is every entry in id order, for anything whose output or
// side effects should not depend on map order.
func (s *Store) sortedEntries() []*Entry {
	out := make([]*Entry, 0, len(s.Entries))
	for _, e := range s.Entries {
		out = append(out, e)
	}
	slices.SortFunc(out, func(a, b *Entry) int { return strings.Compare(a.ID, b.ID) })
	return out
}

// idEncoding is lowercase base32 (a-z, 2-7): no 0/1/8/9 to confuse with
// letters, and nothing that needs quoting when Claude types it.
var idEncoding = base32.NewEncoding("abcdefghijklmnopqrstuvwxyz234567").WithPadding(base32.NoPadding)

// NewID returns 8 random characters (40 bits). Five random bytes encode to
// exactly eight base32 characters.
func NewID() (string, error) {
	var b [5]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	return idEncoding.EncodeToString(b[:]), nil
}

// newID retries until the id is free both in memory and on disk; the disk
// check covers a file that did not load and so is not in Entries.
func (s *Store) newID() (string, error) {
	for range 100 {
		id, err := NewID()
		if err != nil {
			return "", err
		}
		if _, ok := s.Entries[id]; ok {
			continue
		}
		if _, err := os.Stat(s.entryPath(id)); !errors.Is(err, fs.ErrNotExist) {
			continue
		}
		return id, nil
	}
	return "", errors.New("could not find a free entry id after 100 tries")
}
