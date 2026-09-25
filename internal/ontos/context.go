package ontos

import (
	"fmt"
	"slices"
	"strings"
)

// Context is what a session starts with: the resolved subject's knowledge,
// what it uses, where to go next, and the rules for keeping it current. It is
// built by BuildContext and never fails; anything missing is said in Missing.
type Context struct {
	Subject  string    `json:"subject,omitempty"`
	Uses     []string  `json:"uses,omitempty"`
	Sections []Section `json:"sections,omitempty"`
	Packages []Package `json:"packages,omitempty"`
	Problems []string  `json:"problems,omitempty"`
	Missing  string    `json:"missing,omitempty"`
	Rules    string    `json:"rules"`
}

// Section is one subject's share of the context. Role says why it is there:
// the resolved "subject", a "uses" subject, or the "repo root" of a package,
// which contributes index lines only.
type Section struct {
	Subject string    `json:"subject"`
	Role    string    `json:"role"`
	Full    []*Entry  `json:"full"`
	Index   []Summary `json:"index"`
}

// Package is one line of the root's package list: work that moves into the
// package runs `ontos context <path>` to load it.
type Package struct {
	Name string `json:"name"`
	Path string `json:"path"`
}

// BuildContext resolves dir and assembles its context. It runs from the
// SessionStart hook on every session, so it has no error path: a missing
// config, store or subject becomes one line naming the fix.
func BuildContext(dir string) *Context {
	c := &Context{Rules: Rules}
	cfg, err := LoadConfig()
	if err != nil {
		c.Missing = err.Error()
		return c
	}
	s, err := Load(cfg.Store)
	if err != nil {
		c.Missing = err.Error()
		return c
	}
	c.Problems = s.Report()
	r, err := s.Resolve(dir)
	if err != nil {
		c.Missing = fmt.Sprintf("cannot read %s: %v", dir, err)
		return c
	}
	if r.Subject == nil && r.Location.Toplevel == "" {
		c.Missing = fmt.Sprintf("%s is not inside a git repository, so no subject loads here; `ontos search` still reaches every entry.", r.Location.Dir)
		return c
	}
	if r.Subject == nil {
		c.Missing = fmt.Sprintf("no subject for %s (fix: `ontos subject add` there; `ontos subject which` shows how it resolved)", r.Location.Dir)
		return c
	}

	sub := r.Subject
	c.Subject, c.Uses = sub.Name, sub.Uses
	shown := map[string]bool{}
	c.Sections = append(c.Sections, s.section(sub.Name, "subject", cfg.Context, shown))
	for _, name := range sub.Uses {
		if _, ok := s.Subjects[name]; ok && name != sub.Name {
			c.Sections = append(c.Sections, s.section(name, "uses", cfg.Context, shown))
		}
	}

	if sub.Path == "" {
		for _, p := range s.WithURL(sub.URL) {
			if p.Path != "" {
				c.Packages = append(c.Packages, Package{Name: p.Name, Path: p.Path})
			}
		}
	} else if root := s.Root(sub); root != nil && !slices.Contains(sub.Uses, root.Name) {
		c.Sections = append(c.Sections, s.section(root.Name, "repo root", ContextConfig{Index: cfg.Context.Index}, shown))
	}
	return c
}

// section collects one subject's entries. An entry on several subjects is
// shown under the first section that reaches it, so it is never printed twice.
func (s *Store) section(name, role string, cc ContextConfig, shown map[string]bool) Section {
	sec := Section{Subject: name, Role: role, Full: []*Entry{}, Index: []Summary{}}
	for _, e := range s.Entries {
		if shown[e.ID] || !slices.Contains(e.Subjects, name) {
			continue
		}
		switch {
		case slices.Contains(cc.Full, e.Category):
			sec.Full = append(sec.Full, e)
		case slices.Contains(cc.Index, e.Category):
			sec.Index = append(sec.Index, e.Summary())
		default:
			continue
		}
		shown[e.ID] = true
	}
	// Full categories, then index categories, each in the order the config
	// lists them, and by title within a category.
	order := append(slices.Clone(cc.Full), cc.Index...)
	rank := func(cat string) int { return slices.Index(order, cat) }
	slices.SortFunc(sec.Full, func(a, b *Entry) int {
		if c := rank(a.Category) - rank(b.Category); c != 0 {
			return c
		}
		return strings.Compare(a.Title, b.Title)
	})
	slices.SortFunc(sec.Index, func(a, b Summary) int {
		if c := rank(a.Category) - rank(b.Category); c != 0 {
			return c
		}
		return strings.Compare(a.Title, b.Title)
	})
	return sec
}

// Report is everything wrong with the store that a reader should hear about,
// one line each: conflict copies, files that did not load, and names in
// `uses` that are not subjects.
func (s *Store) Report() []string {
	var out []string
	for _, c := range s.Conflicts {
		out = append(out, fmt.Sprintf("sync conflict copy: %s (fix: compare it with the file it shadows, keep one, delete the other)", c))
	}
	for _, p := range s.Problems {
		out = append(out, "did not load: "+p)
	}
	for _, sub := range s.SortedSubjects() {
		for _, name := range s.UnknownUses(sub) {
			out = append(out, fmt.Sprintf("subject %q uses %q, which is not a subject (fix: `ontos subject add --name %s`, or `ontos subject update %s --uses …`)", sub.Name, name, name, sub.Name))
		}
	}
	return out
}

// Markdown renders the context for the SessionStart hook, whose stdout goes
// straight into Claude's context.
func (c *Context) Markdown() string {
	var b strings.Builder
	if c.Missing != "" {
		fmt.Fprintf(&b, "# ontos\n\n%s\n", c.Missing)
	} else {
		fmt.Fprintf(&b, "# ontos: %s\n", c.Subject)
		if len(c.Uses) > 0 {
			fmt.Fprintf(&b, "\nUses: %s.\n", strings.Join(c.Uses, ", "))
		}
		for _, sec := range c.Sections {
			sec.markdown(&b)
		}
		if len(c.Packages) > 0 {
			b.WriteString("\n## Packages\n\nWhen the work moves into a package, load its context:\n\n")
			for _, p := range c.Packages {
				fmt.Fprintf(&b, "- %s: ontos context %s\n", p.Name, p.Path)
			}
		}
	}
	if len(c.Problems) > 0 {
		b.WriteString("\n## Problems\n\n")
		for _, p := range c.Problems {
			fmt.Fprintf(&b, "- %s\n", p)
		}
	}
	fmt.Fprintf(&b, "\n## Maintenance rules\n\n%s", c.Rules)
	return b.String()
}

func (sec Section) markdown(b *strings.Builder) {
	switch sec.Role {
	case "subject":
		fmt.Fprintf(b, "\n## %s\n", sec.Subject)
	case "uses":
		fmt.Fprintf(b, "\n## %s (used)\n", sec.Subject)
	default:
		fmt.Fprintf(b, "\n## %s (%s, index only)\n", sec.Subject, sec.Role)
	}
	if len(sec.Full) == 0 && len(sec.Index) == 0 {
		// Not "no entries": there may be some in categories not loaded here.
		fmt.Fprintf(b, "\nNothing in the categories loaded here; `ontos search --subject %s` lists all of its entries.\n", sec.Subject)
		return
	}
	category := ""
	for _, e := range sec.Full {
		if e.Category != category {
			category = e.Category
			fmt.Fprintf(b, "\n### %s\n", category)
		}
		fmt.Fprintf(b, "\n#### %s (%s, %s)\n\n%s\n", e.Title, e.ID, e.Certainty, strings.TrimSpace(e.Body))
	}
	for _, e := range sec.Index {
		if e.Category != category {
			category = e.Category
			fmt.Fprintf(b, "\n### %s\n\n", category)
		}
		fmt.Fprintf(b, "- %s: %s (%s)\n", e.Title, e.Trigger, e.ID)
	}
}
