package ontos

import (
	"fmt"
	"net/url"
	"path"
	"path/filepath"
	"slices"
	"strings"
)

// Subject is what knowledge is about: a repo, a package inside one, or
// something with no repo at all (a protocol, an environment) that comes into
// play through `uses`. Entries refer to it by name, so the name never changes;
// a renamed repo only touches `url`.
type Subject struct {
	Schema int      `json:"schema"`
	Name   string   `json:"name"`
	URL    string   `json:"url"`
	Path   string   `json:"path"`
	Uses   []string `json:"uses"`
}

func (sub *Subject) normalize() {
	if sub.Uses == nil {
		sub.Uses = []string{}
	}
}

// subjectFile escapes the name into one path segment, so repo/packages/logger
// is repo%2Fpackages%2Flogger.json. The name inside the file is the real key.
func subjectFile(name string) string { return url.PathEscape(name) + ".json" }

func (s *Store) Subject(name string) (*Subject, error) {
	if sub, ok := s.Subjects[name]; ok {
		return sub, nil
	}
	return nil, fmt.Errorf("no subject %q (fix: `ontos subject list`, or `ontos subject add`)", name)
}

// SortedSubjects is every subject by name.
func (s *Store) SortedSubjects() []*Subject {
	out := make([]*Subject, 0, len(s.Subjects))
	for _, sub := range s.Subjects {
		out = append(out, sub)
	}
	slices.SortFunc(out, func(a, b *Subject) int { return strings.Compare(a.Name, b.Name) })
	return out
}

// UnknownUses lists the names in sub.Uses that are not subjects. They are
// reported, never fatal: a subject can be added before what it uses.
func (s *Store) UnknownUses(sub *Subject) []string {
	var out []string
	for _, name := range sub.Uses {
		if _, ok := s.Subjects[name]; !ok {
			out = append(out, name)
		}
	}
	return out
}

// CleanSubjectPath turns a package path into the form stored: slash-separated,
// relative to the repo root, and empty for the root itself.
func CleanSubjectPath(p string) (string, error) {
	p = path.Clean(filepath.ToSlash(strings.TrimSpace(p)))
	switch {
	case p == ".":
		return "", nil
	case path.IsAbs(p), p == "..", strings.HasPrefix(p, "../"):
		return "", fmt.Errorf("path %q must be relative to the repo root and inside it", p)
	}
	return p, nil
}

// PutSubject validates sub and writes it. The same url and path on two
// subjects would make resolution depend on which loaded first, so that is
// refused; subjects with no url never resolve from a directory and can share
// an empty one.
func (s *Store) PutSubject(sub *Subject) error {
	if err := s.Writable(); err != nil {
		return err
	}
	sub.Schema = Schema
	sub.normalize()
	sub.Name = strings.TrimSpace(sub.Name)
	if sub.Name == "" {
		return fmt.Errorf("a subject needs a name")
	}
	if strings.ContainsAny(sub.Name, "\n\t") {
		return fmt.Errorf("subject name %q must be one line with no tabs", sub.Name)
	}
	p, err := CleanSubjectPath(sub.Path)
	if err != nil {
		return err
	}
	sub.Path = p
	sub.URL = NormalizeURL(sub.URL)
	if sub.Path != "" && sub.URL == "" {
		return fmt.Errorf("subject %q has a path but no url; a package path only means something inside a repo", sub.Name)
	}
	if slices.Contains(sub.Uses, sub.Name) {
		return fmt.Errorf("subject %q cannot use itself", sub.Name)
	}
	if sub.URL != "" {
		for _, other := range s.SortedSubjects() {
			if other.Name != sub.Name && other.URL == sub.URL && other.Path == sub.Path {
				return fmt.Errorf("subject %q already has url %s and path %q (fix: `ontos subject update %s` instead)", other.Name, sub.URL, sub.Path, other.Name)
			}
		}
	}
	if err := s.writeJSON(filepath.Join(s.subjectsDir(), subjectFile(sub.Name)), sub); err != nil {
		return err
	}
	s.Subjects[sub.Name] = sub
	return nil
}

// NormalizeURL reduces a git remote to host/path, so every way of writing
// one remote compares equal:
//
//	git@github.com:Org/Repo.git           -> github.com/Org/Repo
//	https://user@GitHub.com/Org/Repo.git  -> github.com/Org/Repo
//	ssh://git@github.com:22/Org/Repo      -> github.com/Org/Repo
//
// The host is lowercased because DNS is case-insensitive; the path is kept
// as written because a forge may not be. A port is dropped along with the
// scheme, since it names the transport, not the repo.
func NormalizeURL(raw string) string {
	u := strings.TrimSpace(raw)
	if u == "" {
		return ""
	}
	var host, rest string
	if _, afterScheme, ok := strings.Cut(u, "://"); ok {
		host, rest, _ = strings.Cut(afterScheme, "/")
		if i := strings.LastIndex(host, "@"); i >= 0 {
			host = host[i+1:]
		}
		if h, _, ok := strings.Cut(host, ":"); ok {
			host = h
		}
	} else if h, r, ok := strings.Cut(u, ":"); ok && !strings.Contains(h, "/") {
		// scp-like [user@]host:path. A colon after a slash is a local path.
		host, rest = h, r
		if i := strings.LastIndex(host, "@"); i >= 0 {
			host = host[i+1:]
		}
	} else {
		rest = u
	}
	rest = strings.TrimSuffix(strings.Trim(rest, "/"), ".git")
	if host == "" {
		return rest
	}
	return strings.ToLower(host) + "/" + strings.TrimPrefix(rest, "/")
}
