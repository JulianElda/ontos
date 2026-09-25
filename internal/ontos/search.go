package ontos

import (
	"cmp"
	"fmt"
	"slices"
	"strings"
)

// Summary is what a listing shows of an entry: enough to decide whether to
// `get` it, never the body, so a search costs lines rather than pages.
type Summary struct {
	ID       string `json:"id"`
	Title    string `json:"title"`
	Category string `json:"category"`
	Trigger  string `json:"trigger"`
}

func (e *Entry) Summary() Summary {
	return Summary{ID: e.ID, Title: e.Title, Category: e.Category, Trigger: e.Trigger}
}

// Filter narrows a search. Each field is an exact match and they combine with
// AND; an empty field does not filter.
type Filter struct {
	Subject  string
	Category string
	Tag      string
}

// Check rejects a filter that could only ever match nothing because it names
// a category or subject that does not exist. A tag has no registry, so an
// unused one just matches nothing.
func (s *Store) Check(f Filter) error {
	if f.Category != "" && !slices.Contains(Categories, f.Category) {
		return fmt.Errorf("category %q is not one of: %s", f.Category, strings.Join(Categories, ", "))
	}
	if f.Subject != "" {
		if _, err := s.Subject(f.Subject); err != nil {
			return err
		}
	}
	return nil
}

func (f Filter) match(e *Entry) bool {
	return (f.Subject == "" || slices.Contains(e.Subjects, f.Subject)) &&
		(f.Category == "" || e.Category == f.Category) &&
		(f.Tag == "" || slices.Contains(e.Tags, f.Tag))
}

// Search matches every whitespace-separated term, case-insensitively, as a
// substring of the title, trigger or body; all terms must match. A term scores
// 3 in the title, 2 in the trigger and 1 in the body, added up per field it
// appears in. With no terms it is a filtered listing, in category order.
func (s *Store) Search(query string, f Filter) []Summary {
	terms := strings.Fields(strings.ToLower(query))
	type hit struct {
		e     *Entry
		score int
	}
	var hits []hit
	for _, e := range s.Entries {
		if !f.match(e) {
			continue
		}
		if score, ok := scoreEntry(e, terms); ok {
			hits = append(hits, hit{e, score})
		}
	}

	slices.SortFunc(hits, func(a, b hit) int {
		if len(terms) > 0 {
			if c := cmp.Compare(b.score, a.score); c != 0 {
				return c
			}
		} else if c := cmp.Compare(categoryRank(a.e.Category), categoryRank(b.e.Category)); c != 0 {
			return c
		}
		return cmp.Or(strings.Compare(a.e.Title, b.e.Title), strings.Compare(a.e.ID, b.e.ID))
	})

	out := make([]Summary, len(hits))
	for i, h := range hits {
		out[i] = h.e.Summary()
	}
	return out
}

func scoreEntry(e *Entry, terms []string) (int, bool) {
	title, trigger, body := strings.ToLower(e.Title), strings.ToLower(e.Trigger), strings.ToLower(e.Body)
	score := 0
	for _, t := range terms {
		s := 0
		if strings.Contains(title, t) {
			s += 3
		}
		if strings.Contains(trigger, t) {
			s += 2
		}
		if strings.Contains(body, t) {
			s++
		}
		if s == 0 {
			return 0, false
		}
		score += s
	}
	return score, true
}

func categoryRank(c string) int { return slices.Index(Categories, c) }
