package ontos

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// git runs a git command in dir and returns trimmed stdout. A non-zero exit is
// an error; callers that treat failure as "unset" ignore it.
func git(dir string, args ...string) (string, error) {
	out, err := exec.Command("git", append([]string{"-C", dir}, args...)...).Output()
	return strings.TrimSpace(string(out)), err
}

// Location is where a directory sits in git terms: the repo it belongs to, its
// path inside that repo, and the repo's origin remote. Toplevel is empty
// outside a git repo, and Remote is empty when there is no origin.
type Location struct {
	Dir      string `json:"dir"`
	Toplevel string `json:"toplevel"`
	Rel      string `json:"rel"`
	Remote   string `json:"remote"`
}

// Locate works out dir's Location. Both sides have their symlinks resolved
// before comparing, because git reports the toplevel's real path while a
// shell's cwd may run through a link.
func Locate(dir string) (*Location, error) {
	abs, err := filepath.Abs(dir)
	if err != nil {
		return nil, err
	}
	if fi, err := os.Stat(abs); err != nil {
		return nil, err
	} else if !fi.IsDir() {
		abs = filepath.Dir(abs)
	}
	if real, err := filepath.EvalSymlinks(abs); err == nil {
		abs = real
	}
	loc := &Location{Dir: abs}

	top, err := git(abs, "rev-parse", "--show-toplevel")
	if err != nil || top == "" {
		return loc, nil
	}
	if real, err := filepath.EvalSymlinks(top); err == nil {
		top = real
	}
	loc.Toplevel = top
	if rel, err := filepath.Rel(top, abs); err == nil && rel != "." {
		loc.Rel = filepath.ToSlash(rel)
	}
	loc.Remote, _ = git(abs, "config", "remote.origin.url")
	return loc, nil
}

// RepoName is the name a repo's subject gets by default: the toplevel's
// directory name, which is also what resolution falls back to without a url.
func (l *Location) RepoName() string { return filepath.Base(l.Toplevel) }
