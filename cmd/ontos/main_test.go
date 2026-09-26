package main

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/JulianElda/ontos/internal/ontos"
)

// cli runs ontos in-process against a scratch store, the way the hook and
// Claude run the binary.
type cli struct {
	t     *testing.T
	store string
}

func newCLI(t *testing.T) *cli {
	t.Helper()
	dir := t.TempDir()
	for _, sub := range []string{"entries", "subjects"} {
		if err := os.MkdirAll(filepath.Join(dir, sub), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	cfg := filepath.Join(dir, "config.toml")
	if err := os.WriteFile(cfg, []byte("store = \""+dir+"\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("ONTOS_CONFIG", cfg)
	c := &cli{t: t, store: dir}
	c.ok("", "subject", "add", "--name", "app", "--url", "")
	return c
}

func (c *cli) run(stdin string, args ...string) (code int, stdout, stderr string) {
	var out, errOut bytes.Buffer
	code = run(args, strings.NewReader(stdin), &out, &errOut)
	return code, out.String(), errOut.String()
}

func (c *cli) ok(stdin string, args ...string) string {
	c.t.Helper()
	code, out, errOut := c.run(stdin, args...)
	if code != 0 {
		c.t.Fatalf("ontos %v exited %d\nstderr: %s", args, code, errOut)
	}
	return out
}

func (c *cli) add(title string, extra ...string) string {
	c.t.Helper()
	args := append([]string{"add", "--title", title, "--category", "gotcha", "--subject", "app",
		"--trigger", "when " + title, "--certainty", "confirmed", "--body-file", "-"}, extra...)
	return strings.TrimSpace(c.ok("body of "+title+"\n", args...))
}

func (c *cli) get(id string) ontos.Entry {
	c.t.Helper()
	var entries []ontos.Entry
	if err := json.Unmarshal([]byte(c.ok("", "get", id, "--json")), &entries); err != nil {
		c.t.Fatal(err)
	}
	return entries[0]
}

func TestUpdateLeavesUngivenFieldsAlone(t *testing.T) {
	c := newCLI(t)
	other := c.add("other")
	id := c.add("original", "--tag", "a", "--source", "src/x.go", "--related", other, "--checked", "abc1234")
	before := c.get(id)

	c.ok("", "update", id, "--tag", "b", "--tag", "c")
	after := c.get(id)

	if want := []string{"b", "c"}; !reflect.DeepEqual(after.Tags, want) {
		t.Errorf("tags = %v, want %v: a list flag replaces the list", after.Tags, want)
	}
	after.Tags = before.Tags
	if !reflect.DeepEqual(after, before) {
		t.Errorf("update --tag changed other fields:\n before %+v\n after  %+v", before, after)
	}

	c.ok("", "update", id, "--tag", "")
	if got := c.get(id).Tags; len(got) != 0 {
		t.Errorf("--tag '' left tags %v, want them cleared", got)
	}
}

func TestUpdateWithNoFlagFails(t *testing.T) {
	c := newCLI(t)
	id := c.add("x")
	if code, _, errOut := c.run("", "update", id); code != 1 || !strings.Contains(errOut, "nothing to update") {
		t.Errorf("update with no flags: exit %d, stderr %q", code, errOut)
	}
}

func TestDeleteRemovesRelatedPointers(t *testing.T) {
	c := newCLI(t)
	gone := c.add("gone")
	keep := c.add("keep")
	pointer := c.add("pointer", "--related", gone, "--related", keep)

	out := c.ok("", "delete", gone)
	if !strings.Contains(out, "related list of: "+pointer) || !strings.Contains(out, "deleted "+gone) {
		t.Errorf("delete output does not name the rewritten entry:\n%s", out)
	}
	if got := c.get(pointer).Related; !reflect.DeepEqual(got, []string{keep}) {
		t.Errorf("related = %v, want only %s", got, keep)
	}
	if code, _, _ := c.run("", "get", gone); code != 1 {
		t.Errorf("get of a deleted entry exited %d, want 1", code)
	}
}

func TestAddRejectsUnknownNames(t *testing.T) {
	c := newCLI(t)
	for bad, args := range map[string][]string{
		`category "maps"`: {"--category", "maps", "--subject", "app"},
		`subject "nope"`:  {"--category", "map", "--subject", "nope"},
	} {
		args = append([]string{"add", "--title", "t", "--trigger", "t", "--certainty", "confirmed", "--body-file", "-"}, args...)
		code, out, errOut := c.run("body", args...)
		if code != 1 || out != "" {
			t.Errorf("add with %s: exit %d, stdout %q", bad, code, out)
		}
		if !strings.Contains(errOut, bad) {
			t.Errorf("add with %s: stderr does not name it: %q", bad, errOut)
		}
	}
	if files, _ := os.ReadDir(filepath.Join(c.store, "entries")); len(files) != 0 {
		t.Errorf("a rejected add wrote %d file(s)", len(files))
	}
}

func TestAddNeedsEveryRequiredFlag(t *testing.T) {
	newCLI(t)
	var out, errOut bytes.Buffer
	if code := run([]string{"add", "--title", "t"}, strings.NewReader(""), &out, &errOut); code != 1 {
		t.Errorf("exit %d, want 1", code)
	}
	if !strings.Contains(errOut.String(), "--category, --subject, --trigger, --certainty, --body-file") {
		t.Errorf("stderr does not list the missing flags: %q", errOut.String())
	}
}

func TestFlagsAfterPositional(t *testing.T) {
	c := newCLI(t)
	id := c.add("x")
	// --json after the id must still apply: Go's flag package alone would
	// treat it as a second id.
	if out := c.ok("", "get", id, "--json"); !strings.HasPrefix(out, "[") {
		t.Errorf("get <id> --json printed markdown:\n%s", out)
	}
}

func TestContextWithoutConfigExitsZero(t *testing.T) {
	t.Setenv("ONTOS_CONFIG", filepath.Join(t.TempDir(), "missing.toml"))
	var out, errOut bytes.Buffer
	code := run([]string{"context", t.TempDir()}, strings.NewReader(""), &out, &errOut)
	if code != 0 {
		t.Errorf("context exited %d, want 0", code)
	}
	for _, want := range []string{"no config at", "setup.sh", "## Maintenance rules", "Never store secrets"} {
		if !strings.Contains(out.String(), want) {
			t.Errorf("context output lacks %q:\n%s", want, out.String())
		}
	}
}

func TestContextBadArgumentsExitZero(t *testing.T) {
	var out, errOut bytes.Buffer
	if code := run([]string{"context", "a", "b", "--bogus"}, strings.NewReader(""), &out, &errOut); code != 0 {
		t.Errorf("context with bad arguments exited %d, want 0", code)
	}
	if !strings.Contains(out.String(), "## Maintenance rules") {
		t.Errorf("rules missing:\n%s", out.String())
	}
}

// gitRepo makes a git repo at <tmp>/repo with a packages/logger directory and
// origin git@github.com:Org/Repo.git.
func gitRepo(t *testing.T) string {
	t.Helper()
	dir := filepath.Join(t.TempDir(), "repo")
	if err := os.MkdirAll(filepath.Join(dir, "packages", "logger"), 0o755); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{{"init", "-q"}, {"remote", "add", "origin", "git@github.com:Org/Repo.git"}} {
		if out, err := exec.Command("git", append([]string{"-C", dir}, args...)...).CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	return dir
}

func TestWhichProposesWhatSubjectAddAdds(t *testing.T) {
	c := newCLI(t)
	t.Chdir(filepath.Join(gitRepo(t), "packages", "logger"))

	out := c.ok("", "subject", "which")
	_, proposed, ok := strings.Cut(out, " adds ")
	proposed = strings.TrimSuffix(proposed, ").\n")
	if want := "repo/packages/logger: github.com/Org/Repo, path packages/logger"; !ok || proposed != want {
		t.Fatalf("which proposed %q, want %q\n%s", proposed, want, out)
	}
	if added := c.ok("", "subject", "add"); added != "added subject "+proposed+"\n" {
		t.Errorf("subject add printed %q, but which proposed %q", added, proposed)
	}
	if out := c.ok("", "subject", "which"); !strings.Contains(out, "Subject: "+proposed) {
		t.Errorf("which after add:\n%s", out)
	}
}

func TestWhichNamesTheSubjectInTheWay(t *testing.T) {
	c := newCLI(t)
	// Another checkout named repo, whose package has the name this one would get.
	c.ok("", "subject", "add", "--name", "repo/packages/logger", "--url", "github.com/Other/Repo", "--path", "packages/logger")
	out := c.ok("", "subject", "which", filepath.Join(gitRepo(t), "packages", "logger"))
	for _, want := range []string{"would fail", "repo/packages/logger: github.com/Other/Repo", "`ontos subject update repo/packages/logger`"} {
		if !strings.Contains(out, want) {
			t.Errorf("which output lacks %q:\n%s", want, out)
		}
	}
}

func TestWhichOutsideGitSaysToPassName(t *testing.T) {
	c := newCLI(t)
	out := c.ok("", "subject", "which", t.TempDir())
	if !strings.Contains(out, "pass --name") || strings.Contains(out, "`ontos subject add` in") {
		t.Errorf("which outside git should say to pass --name, not to run subject add there:\n%s", out)
	}
}
