package main

import (
	"flag"
	"fmt"
	"os"
	"runtime"
	"strings"

	"avyos.dev/lib/format"
	"avyos.dev/lib/graphics/app"
	gsettings "avyos.dev/lib/settings"
)

func init() {
	runtime.LockOSThread()
	flag.Usage = func() {
		fmt.Fprintln(os.Stderr, "settings - Graphical settings center with CLI access")
		fmt.Fprintln(os.Stderr)
		fmt.Fprintln(os.Stderr, "Usage:")
		fmt.Fprintln(os.Stderr, "  settings                    Launch the graphical settings center")
		fmt.Fprintln(os.Stderr, "  settings list   [options] [prefix]")
		fmt.Fprintln(os.Stderr, "  settings get    [options] <path>")
		fmt.Fprintln(os.Stderr, "  settings set    [options] <path> <value>")
		fmt.Fprintln(os.Stderr, "  settings delete [options] <path>")
		fmt.Fprintln(os.Stderr)
		fmt.Fprintln(os.Stderr, "Options:")
		fmt.Fprintln(os.Stderr, "  -scope user|system|all   Select settings scope")
	}
}

func main() {
	flag.Parse()
	if err := run(flag.Args(), gsettings.DefaultStore()); err != nil {
		format.Error("%s", err)
		os.Exit(1)
	}
}

func run(args []string, store gsettings.Store) error {
	if len(args) == 0 {
		app.Options.Title = "Settings"
		return app.Run(SettingsApp{Store: store})
	}

	switch args[0] {
	case "list":
		return runList(store, args[1:])
	case "get":
		return runGet(store, args[1:])
	case "set":
		return runSet(store, args[1:])
	case "delete", "remove", "unset":
		return runDelete(store, args[1:])
	case "gui":
		app.Options.Title = "Settings"
		return app.Run(SettingsApp{Store: store})
	default:
		return fmt.Errorf("unknown subcommand %q", args[0])
	}
}

func runList(store gsettings.Store, args []string) error {
	fs := flag.NewFlagSet("list", flag.ContinueOnError)
	scopeFlag := fs.String("scope", "all", "settings scope (user|system|all)")
	fs.SetOutput(os.Stderr)
	if err := fs.Parse(args); err != nil {
		return err
	}
	prefix := ""
	if extra := fs.Args(); len(extra) > 0 {
		prefix = extra[0]
	}

	scopes, err := parseListScopes(*scopeFlag)
	if err != nil {
		return err
	}

	table := format.NewTable("Scope", "Path", "Value")
	for _, scope := range scopes {
		entries, err := store.List(scope, prefix)
		if err != nil {
			return err
		}
		for _, entry := range entries {
			table.AddRow(scope.String(), entry.Path, gsettings.FormatValue(entry.Value))
		}
	}
	table.Print()
	return nil
}

func runGet(store gsettings.Store, args []string) error {
	fs := flag.NewFlagSet("get", flag.ContinueOnError)
	scopeFlag := fs.String("scope", "user", "settings scope (user|system)")
	fs.SetOutput(os.Stderr)
	if err := fs.Parse(args); err != nil {
		return err
	}
	if len(fs.Args()) != 1 {
		return fmt.Errorf("usage: settings get [-scope user|system] <path>")
	}
	scope, err := gsettings.ParseScope(*scopeFlag)
	if err != nil {
		return err
	}
	value, ok, err := store.Get(scope, fs.Args()[0])
	if err != nil {
		return err
	}
	if !ok {
		return fmt.Errorf("%s setting %q not found", scope.String(), fs.Args()[0])
	}
	fmt.Println(gsettings.FormatValue(value))
	return nil
}

func runSet(store gsettings.Store, args []string) error {
	fs := flag.NewFlagSet("set", flag.ContinueOnError)
	scopeFlag := fs.String("scope", "user", "settings scope (user|system)")
	fs.SetOutput(os.Stderr)
	if err := fs.Parse(args); err != nil {
		return err
	}
	if len(fs.Args()) < 2 {
		return fmt.Errorf("usage: settings set [-scope user|system] <path> <value>")
	}
	scope, err := gsettings.ParseScope(*scopeFlag)
	if err != nil {
		return err
	}
	path := fs.Args()[0]
	rawValue := strings.Join(fs.Args()[1:], " ")
	value, err := gsettings.ParseValue(rawValue)
	if err != nil {
		return fmt.Errorf("parse value: %w", err)
	}
	if err := store.Set(scope, path, value); err != nil {
		return err
	}
	format.Success("%s %s = %s", scope.String(), path, gsettings.FormatValue(value))
	return nil
}

func runDelete(store gsettings.Store, args []string) error {
	fs := flag.NewFlagSet("delete", flag.ContinueOnError)
	scopeFlag := fs.String("scope", "user", "settings scope (user|system)")
	fs.SetOutput(os.Stderr)
	if err := fs.Parse(args); err != nil {
		return err
	}
	if len(fs.Args()) != 1 {
		return fmt.Errorf("usage: settings delete [-scope user|system] <path>")
	}
	scope, err := gsettings.ParseScope(*scopeFlag)
	if err != nil {
		return err
	}
	if err := store.Delete(scope, fs.Args()[0]); err != nil {
		return err
	}
	format.Success("removed %s %s", scope.String(), fs.Args()[0])
	return nil
}

func parseListScopes(raw string) ([]gsettings.Scope, error) {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "", "all":
		return []gsettings.Scope{gsettings.ScopeUser, gsettings.ScopeSystem}, nil
	default:
		scope, err := gsettings.ParseScope(raw)
		if err != nil {
			return nil, err
		}
		return []gsettings.Scope{scope}, nil
	}
}
