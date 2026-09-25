package ontos

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/BurntSushi/toml"
)

// Categories are the eight kinds of entry, in the order Claude meets them:
// arriving, reasoning, editing, using a tool, doing a task, debugging, judging
// a bug, looking outside. Listings sort by this order, not alphabetically.
var Categories = []string{"map", "mechanism", "invariant", "gotcha", "procedure", "diagnosis", "gap", "reference"}

// Config is local to one machine and never synced. It names the store and
// which categories `context` prints in full, as one line each, or not at all.
type Config struct {
	Store   string        `toml:"store"`
	Context ContextConfig `toml:"context"`
}

type ContextConfig struct {
	Full  []string `toml:"full"`
	Index []string `toml:"index"`
}

// DefaultContext is what `context` loads when the config has no [context]:
// the map in full, everything else as one line per entry.
func DefaultContext() ContextConfig {
	return ContextConfig{Full: []string{"map"}, Index: slices.Clone(Categories[1:])}
}

// SetupHint names the command that creates a missing config or store.
const SetupHint = "run ./scripts/setup.sh in the ontos checkout"

// ConfigPath is ~/.config/ontos/config.toml, overridable with ONTOS_CONFIG for
// tests and scratch stores.
func ConfigPath() string {
	if p := os.Getenv("ONTOS_CONFIG"); p != "" {
		return p
	}
	dir, err := os.UserConfigDir()
	if err != nil {
		return Expand("~/.config/ontos/config.toml")
	}
	return filepath.Join(dir, "ontos", "config.toml")
}

func LoadConfig() (*Config, error) {
	path := ConfigPath()
	var c Config
	md, err := toml.DecodeFile(path, &c)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, fmt.Errorf("no config at %s (fix: %s)", path, SetupHint)
		}
		return nil, fmt.Errorf("reading %s: %w", path, err)
	}
	if undecoded := md.Undecoded(); len(undecoded) > 0 {
		return nil, fmt.Errorf("%s: unknown key %q (known: store, context.full, context.index)", path, undecoded[0].String())
	}

	c.Store = Expand(c.Store)
	if c.Store == "" {
		return nil, fmt.Errorf("%s: `store` is unset; set it to the folder that holds the entries and subjects", path)
	}
	if !md.IsDefined("context") {
		c.Context = DefaultContext()
	}
	seen := map[string]string{}
	for _, list := range []struct {
		key   string
		names []string
	}{{"full", c.Context.Full}, {"index", c.Context.Index}} {
		for _, name := range list.names {
			if !slices.Contains(Categories, name) {
				return nil, fmt.Errorf("%s: context.%s names unknown category %q (known: %s)", path, list.key, name, strings.Join(Categories, ", "))
			}
			if other, ok := seen[name]; ok {
				return nil, fmt.Errorf("%s: category %q is in both context.%s and context.%s; keep it in one", path, name, other, list.key)
			}
			seen[name] = list.key
		}
	}
	return &c, nil
}

func Expand(p string) string {
	if p != "~" && !strings.HasPrefix(p, "~/") {
		return p
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return p
	}
	return filepath.Join(home, strings.TrimPrefix(p, "~"))
}
