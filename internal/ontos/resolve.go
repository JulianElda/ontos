package ontos

import (
	"fmt"
	"slices"
	"strings"
)

// Resolution is how a directory became a subject, step by step, so `subject
// which` can show exactly where it went wrong when it does. Subject is nil
// when nothing matched.
type Resolution struct {
	Location *Location `json:"location"`
	URL      string    `json:"url"`
	Steps    []string  `json:"steps"`
	Subject  *Subject  `json:"subject"`
}

func (r *Resolution) step(format string, args ...any) {
	r.Steps = append(r.Steps, fmt.Sprintf(format, args...))
}

// Resolve finds the subject for dir: the subject with the repo's url whose
// path is the longest prefix of dir's path in the repo, or failing that, a
// root subject named after the repo's directory.
func (s *Store) Resolve(dir string) (*Resolution, error) {
	loc, err := Locate(dir)
	if err != nil {
		return nil, err
	}
	r := &Resolution{Location: loc}
	r.step("directory: %s", loc.Dir)
	if loc.Toplevel == "" {
		r.step("not inside a git repository, so no subject resolves from here")
		return r, nil
	}
	r.step("toplevel: %s, path in repo: %s", loc.Toplevel, orRoot(loc.Rel))

	if loc.Remote == "" {
		r.step("no remote.origin.url")
	} else {
		r.URL = NormalizeURL(loc.Remote)
		r.step("remote.origin.url: %s, normalized: %s", loc.Remote, r.URL)
		if sub := s.byURL(r, loc.Rel); sub != nil {
			r.Subject = sub
			return r, nil
		}
	}

	name := loc.RepoName()
	if sub, ok := s.Subjects[name]; ok && sub.Path == "" {
		r.step("fallback: subject %q is named after the toplevel and has no path", name)
		r.Subject = sub
		return r, nil
	}
	r.step("fallback: no subject named %q with an empty path", name)
	return r, nil
}

func (s *Store) byURL(r *Resolution, rel string) *Subject {
	candidates := s.WithURL(r.URL)
	if len(candidates) == 0 {
		r.step("no subject has url %s", r.URL)
		return nil
	}
	var names []string
	var best *Subject
	for _, c := range candidates {
		names = append(names, fmt.Sprintf("%s at %s", c.Name, orRoot(c.Path)))
		if contains(c.Path, rel) && (best == nil || len(c.Path) > len(best.Path)) {
			best = c
		}
	}
	r.step("subjects with that url: %s", strings.Join(names, ", "))
	if best == nil {
		r.step("none has a path containing %s", orRoot(rel))
		return nil
	}
	r.step("longest path containing %s: %s", orRoot(rel), best.Name)
	return best
}

// contains reports whether the package at dir contains rel, both relative to
// the repo root. The root (empty) contains everything; packages/log does not
// contain packages/logger.
func contains(dir, rel string) bool {
	return dir == "" || rel == dir || strings.HasPrefix(rel, dir+"/")
}

// WithURL is every subject with url u, root first, then by path.
func (s *Store) WithURL(u string) []*Subject {
	var out []*Subject
	if u == "" {
		return out
	}
	for _, sub := range s.Subjects {
		if sub.URL == u {
			out = append(out, sub)
		}
	}
	slices.SortFunc(out, func(a, b *Subject) int { return strings.Compare(a.Path, b.Path) })
	return out
}

// Root is the repo-root subject for a package subject: the same url, no path.
func (s *Store) Root(sub *Subject) *Subject {
	for _, c := range s.WithURL(sub.URL) {
		if c.Path == "" {
			return c
		}
	}
	return nil
}

func orRoot(p string) string {
	if p == "" {
		return "(root)"
	}
	return p
}
