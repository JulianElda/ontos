package main

import (
	"fmt"

	"github.com/JulianElda/ontos/internal/ontos"
)

// cmdContext prints the session-start context. It returns nothing: whatever
// goes wrong, including a bad argument or a panic, is printed as the one
// line the context has room for, followed by the rules, and run exits 0.
func (a *app) cmdContext(args []string) {
	defer func() {
		if r := recover(); r != nil {
			c := &ontos.Context{Rules: ontos.Rules, Missing: fmt.Sprintf("ontos context failed: %v", r)}
			a.printf("%s", c.Markdown())
		}
	}()

	fs := a.newFlagSet("context", "ontos context [path] [--json]")
	asJSON := fs.Bool("json", false, "print JSON")
	positional, err := parseArgs(fs, args)
	var c *ontos.Context
	switch {
	case err != nil || len(positional) > 1:
		c = &ontos.Context{Rules: ontos.Rules, Missing: "usage: ontos context [path] [--json]"}
	case len(positional) == 1:
		c = ontos.BuildContext(positional[0])
	default:
		c = ontos.BuildContext(".")
	}

	if *asJSON {
		if err := a.printJSON(c); err == nil {
			return
		}
	}
	a.printf("%s", c.Markdown())
}
