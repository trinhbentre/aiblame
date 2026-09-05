package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"

	"github.com/trinhbentre/aiblame/internal/attrib"
	"github.com/trinhbentre/aiblame/internal/config"
	"github.com/trinhbentre/aiblame/internal/gitx"
)

const agentsHelp = `Usage: aiblame agents [--json]

List the coding agents and non-AI bots aiblame recognises, with the identity
strings (e-mails, names) that trigger detection. Extend the list per repo in
.aiblame.toml under [[detect.agents]] / [[detect.bots]].
`

func cmdAgents(_ context.Context, args []string, env Env) int {
	fs := newFlagSet("agents", env, agentsHelp)
	asJSON := fs.Bool("json", false, "JSON output")
	if _, err := parseInterspersed(fs, args); err != nil {
		return usageErr(env, fs, err)
	}
	type row struct {
		Name     string   `json:"name"`
		Vendor   string   `json:"vendor,omitempty"`
		Emails   []string `json:"emails,omitempty"`
		Suffixes []string `json:"email_suffixes,omitempty"`
		Names    []string `json:"names,omitempty"`
		Pattern  string   `json:"name_pattern,omitempty"`
		Bot      bool     `json:"bot"`
	}
	var rows []row
	for _, list := range [][]attrib.Identity{attrib.KnownAgents, attrib.KnownBots} {
		for _, id := range list {
			r := row{Name: id.Name, Vendor: id.Vendor, Emails: id.Emails, Suffixes: id.EmailSuffixes, Names: id.Names, Bot: id.IsBot}
			if id.NamePattern != nil {
				r.Pattern = id.NamePattern.String()
			}
			rows = append(rows, r)
		}
	}
	if *asJSON {
		enc := json.NewEncoder(env.Stdout)
		enc.SetIndent("", "  ")
		return exitOn(env, enc.Encode(rows))
	}
	fmt.Fprintf(env.Stdout, "%d coding agents, %d non-AI bots\n\n", len(attrib.KnownAgents), len(attrib.KnownBots))
	fmt.Fprintf(env.Stdout, "%-18s %-16s %s\n", "AGENT", "VENDOR", "MATCHED BY")
	for _, r := range rows {
		if r.Bot {
			continue
		}
		fmt.Fprintf(env.Stdout, "%-18s %-16s %s\n", r.Name, r.Vendor, matchedBy(r.Emails, r.Suffixes, r.Names, r.Pattern))
	}
	fmt.Fprintf(env.Stdout, "\n%-18s %s\n", "BOT (not AI)", "MATCHED BY")
	for _, r := range rows {
		if !r.Bot {
			continue
		}
		fmt.Fprintf(env.Stdout, "%-18s %s\n", r.Name, matchedBy(r.Emails, r.Suffixes, r.Names, r.Pattern))
	}
	keys := make([]string, 0, len(attrib.AITrailerKeys))
	for k := range attrib.AITrailerKeys {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	fmt.Fprintln(env.Stdout, "\nTrailer keys treated as AI evidence:", strings.Join(keys, ", "))
	return ExitOK
}

func matchedBy(emails, suffixes, names []string, pattern string) string {
	var parts []string
	if len(emails) > 0 {
		parts = append(parts, emails[0])
	} else if len(suffixes) > 0 {
		parts = append(parts, "*"+suffixes[0])
	}
	if len(names) > 0 {
		parts = append(parts, "name="+names[0])
	}
	if pattern != "" && len(parts) == 0 {
		parts = append(parts, "pattern="+pattern)
	}
	s := strings.Join(parts, ", ")
	if len(s) > 70 {
		s = s[:69] + "…"
	}
	return s
}

const initHelp = `Usage: aiblame init [--force] [-C DIR]

Write a commented starter .aiblame.toml to the repository root.
`

func cmdInit(ctx context.Context, args []string, env Env) int {
	fs := newFlagSet("init", env, initHelp)
	force := fs.Bool("force", false, "overwrite existing file")
	dir := fs.String("C", "", "repository directory")
	if _, err := parseInterspersed(fs, args); err != nil {
		return usageErr(env, fs, err)
	}
	target := *dir
	if target == "" {
		target = env.Cwd
	}
	if target == "" {
		target = "."
	}
	root := target
	if r, err := gitx.Open(ctx, target); err == nil {
		root = r.Dir
	}
	p := filepath.Join(root, config.FileName)
	if _, err := os.Stat(p); err == nil && !*force {
		fmt.Fprintf(env.Stderr, "aiblame: %s already exists (use --force to overwrite)\n", p)
		return ExitFailed
	} else if err != nil && !errors.Is(err, os.ErrNotExist) {
		return fail(env, err)
	}
	if err := os.WriteFile(p, []byte(config.Example), 0o644); err != nil {
		return fail(env, err)
	}
	fmt.Fprintf(env.Stdout, "wrote %s\n", p)
	return ExitOK
}

func cmdVersion(args []string, env Env) int {
	if len(args) > 0 && (args[0] == "--json" || args[0] == "-json") {
		enc := json.NewEncoder(env.Stdout)
		return exitOn(env, enc.Encode(map[string]string{"version": Version, "commit": Commit, "date": Date, "go": runtime.Version(), "os": runtime.GOOS, "arch": runtime.GOARCH}))
	}
	s := "aiblame " + Version
	if Commit != "" {
		s += " (" + Commit
		if Date != "" {
			s += ", " + Date
		}
		s += ")"
	}
	fmt.Fprintf(env.Stdout, "%s %s %s/%s\n", s, runtime.Version(), runtime.GOOS, runtime.GOARCH)
	return ExitOK
}
