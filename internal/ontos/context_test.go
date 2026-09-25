package ontos

import (
	"path/filepath"
	"strings"
	"testing"
)

// contextRepo is a repo subject with a map and a gotcha, a logger package
// with its own gotcha, and a url-less subject the repo uses.
func contextRepo(t *testing.T, extraConfig string) string {
	t.Helper()
	s := newStore(t, extraConfig)
	mustSubject(t, s, &Subject{Name: "dev-vm"})
	mustSubject(t, s, &Subject{Name: "repo", URL: "github.com/Org/Repo", Uses: []string{"dev-vm"}})
	mustSubject(t, s, &Subject{Name: "repo/packages/logger", URL: "github.com/Org/Repo", Path: "packages/logger"})
	mustEntry(t, s, Entry{Title: "Repo layout", Category: "map", Subjects: []string{"repo"}, Body: "MAP-BODY"})
	mustEntry(t, s, Entry{Title: "Build caches", Category: "gotcha", Subjects: []string{"repo"},
		Trigger: "before a clean build", Body: "GOTCHA-BODY"})
	mustEntry(t, s, Entry{Title: "Log levels", Category: "gotcha", Subjects: []string{"repo/packages/logger"},
		Trigger: "before changing a level", Body: "LOGGER-BODY"})
	mustEntry(t, s, Entry{Title: "VM access", Category: "procedure", Subjects: []string{"dev-vm"},
		Trigger: "to get a shell on the VM", Body: "VM-BODY"})
	return gitRepo(t, "https://github.com/Org/Repo.git")
}

func TestContextAtRoot(t *testing.T) {
	dir := contextRepo(t, "")
	c := BuildContext(dir)
	if c.Missing != "" {
		t.Fatalf("Missing = %q", c.Missing)
	}
	md := c.Markdown()

	for _, want := range []string{
		"# ontos: repo\n",
		"MAP-BODY",                                // map is a full category by default
		"- Build caches: before a clean build (",  // gotcha is an index line
		"## dev-vm (used)",                        // uses are followed
		"- VM access: to get a shell on the VM (", // with their index lines
		"- repo/packages/logger: ontos context packages/logger\n",
		"## Maintenance rules",
	} {
		if !strings.Contains(md, want) {
			t.Errorf("context lacks %q:\n%s", want, md)
		}
	}
	for _, absent := range []string{"GOTCHA-BODY", "VM-BODY", "Log levels"} {
		if strings.Contains(md, absent) {
			t.Errorf("context has %q, which should not be printed at the root:\n%s", absent, md)
		}
	}
}

func TestContextFollowsConfig(t *testing.T) {
	dir := contextRepo(t, "[context]\nfull = [\"gotcha\"]\nindex = [\"procedure\"]\n")
	md := BuildContext(dir).Markdown()

	if !strings.Contains(md, "GOTCHA-BODY") {
		t.Errorf("gotcha is full in the config but its body is missing:\n%s", md)
	}
	if strings.Contains(md, "Repo layout") {
		t.Errorf("map is in neither list but was printed:\n%s", md)
	}
	if !strings.Contains(md, "- VM access: to get a shell on the VM (") {
		t.Errorf("procedure is an index category but has no line:\n%s", md)
	}
}

func TestContextInPackage(t *testing.T) {
	dir := contextRepo(t, "")
	md := BuildContext(filepath.Join(dir, "packages", "logger", "src")).Markdown()

	for _, want := range []string{
		"# ontos: repo/packages/logger\n",
		"- Log levels: before changing a level (",
		"## repo (repo root, index only)",
		"- Build caches: before a clean build (",
	} {
		if !strings.Contains(md, want) {
			t.Errorf("context lacks %q:\n%s", want, md)
		}
	}
	if strings.Contains(md, "MAP-BODY") {
		t.Errorf("the root's full entries are left out in a package:\n%s", md)
	}
}

func TestContextWithoutSubject(t *testing.T) {
	newStore(t, "")
	c := BuildContext(gitRepo(t, "git@github.com:Org/Other.git"))
	if !strings.Contains(c.Missing, "ontos subject add") {
		t.Errorf("Missing = %q, want the fix named", c.Missing)
	}
	if !strings.Contains(c.Markdown(), "## Maintenance rules") {
		t.Error("the rules are printed even without a subject")
	}
}
