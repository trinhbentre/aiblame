package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/trinhbentre/aiblame/internal/attrib"
)

const logHelp = `Usage: aiblame log [PATH] [flags]

List commits with their AI classification, newest first.

Flags:
  --ai            Only AI-touched commits (assisted + agent-authored)
  --human         Only human commits
  --bot           Only non-AI bot commits
  --kind K        Comma-separated kinds: human,assisted,agent,bot
  --agent NAME    Only commits involving this agent (substring, case-insensitive)
  -n N            Show at most N commits (default 50, 0 = all)
  --evidence      Print why each commit was classified
Plus common flags: --rev --since --until --include-merges --format table|json --no-color
`

type logEntry struct {
	Hash        string            `json:"hash"`
	Date        string            `json:"date"`
	Author      string            `json:"author"`
	AuthorEmail string            `json:"author_email"`
	Subject     string            `json:"subject"`
	Kind        attrib.Kind       `json:"kind"`
	Agents      []string          `json:"agents,omitempty"`
	Models      []string          `json:"models,omitempty"`
	Conventions []string          `json:"conventions,omitempty"`
	Evidence    []attrib.Evidence `json:"evidence,omitempty"`
	Merge       bool              `json:"merge,omitempty"`
	Added       int               `json:"lines_added"`
	Deleted     int               `json:"lines_deleted"`
}

func cmdLog(ctx context.Context, args []string, env Env) int {
	fs := newFlagSet("log", env, logHelp)
	var c common
	c.bind(fs)
	onlyAI := fs.Bool("ai", false, "only AI commits")
	onlyHuman := fs.Bool("human", false, "only human commits")
	onlyBot := fs.Bool("bot", false, "only bot commits")
	kinds := fs.String("kind", "", "kinds filter")
	agent := fs.String("agent", "", "agent filter")
	limit := fs.Int("n", 50, "max commits")
	evidence := fs.Bool("evidence", false, "show evidence")
	pos, err := parseInterspersed(fs, args)
	if err != nil {
		return usageErr(env, fs, err)
	}
	if len(pos) > 1 {
		return usageErr(env, fs, fmt.Errorf("expected at most one PATH"))
	}
	format, err := c.resolvedFormat("table")
	if err != nil {
		return usageErr(env, fs, err)
	}
	want := map[attrib.Kind]bool{}
	if *onlyAI {
		want[attrib.Assisted], want[attrib.Agent] = true, true
	}
	if *onlyHuman {
		want[attrib.Human] = true
	}
	if *onlyBot {
		want[attrib.Bot] = true
	}
	for _, k := range strings.Split(*kinds, ",") {
		k = strings.TrimSpace(k)
		if k == "" {
			continue
		}
		var kk attrib.Kind
		if err := kk.UnmarshalText([]byte(k)); err != nil {
			return usageErr(env, fs, err)
		}
		want[kk] = true
	}
	target := ""
	if len(pos) == 1 {
		target = pos[0]
	}
	r, err := openTarget(ctx, target, env, true)
	if err != nil {
		return fail(env, err)
	}
	an, _, err := buildAnalyzer(&c, r, env, false)
	if err != nil {
		return fail(env, err)
	}
	if _, err := r.ResolveRev(ctx, an.Opts.Rev); err != nil {
		return fail(env, err)
	}
	commits, err := an.Commits(ctx)
	if err != nil {
		return fail(env, err)
	}
	var entries []logEntry
	agentQ := strings.ToLower(*agent)
	for _, cc := range commits {
		if cc.IsMerge() && !c.includeMerges {
			continue
		}
		if in, err := an.InWindow(ctx, cc.Commit); err != nil {
			return fail(env, err)
		} else if !in {
			continue
		}
		if len(want) > 0 && !want[cc.Attr.Kind] {
			continue
		}
		if agentQ != "" {
			hit := false
			for _, a := range cc.Attr.Agents {
				if strings.Contains(strings.ToLower(a), agentQ) {
					hit = true
				}
			}
			if !hit {
				continue
			}
		}
		e := logEntry{
			Hash: cc.Hash, Date: cc.AuthorTime.Format("2006-01-02"), Author: cc.AuthorName, AuthorEmail: cc.AuthorEmail,
			Subject: cc.Subject(), Kind: cc.Attr.Kind, Agents: cc.Attr.Agents, Models: cc.Attr.Models, Conventions: cc.Attr.Conventions, Merge: cc.IsMerge(),
		}
		if *evidence || format == "json" {
			e.Evidence = cc.Attr.Evidence
		}
		for _, f := range cc.Files {
			e.Added += f.Added
			e.Deleted += f.Deleted
		}
		entries = append(entries, e)
		if *limit > 0 && len(entries) >= *limit {
			break
		}
	}
	if format == "json" {
		enc := json.NewEncoder(env.Stdout)
		enc.SetIndent("", "  ")
		if entries == nil {
			entries = []logEntry{}
		}
		return exitOn(env, enc.Encode(entries))
	}
	color := useColor(env.Stdout, c.noColor, env.Getenv)
	for _, e := range entries {
		tag := fmt.Sprintf("%-6s", kindTag(e.Kind))
		if color {
			tag = colorKind(e.Kind) + tag + "\x1b[0m"
		}
		agents := strings.Join(e.Agents, ",")
		if agents == "" {
			agents = "-"
		}
		fmt.Fprintf(env.Stdout, "%s  %s  %s  %-14s  +%d/-%d  %s\n", e.Hash[:7], e.Date, tag, truncate(agents, 14), e.Added, e.Deleted, e.Subject)
		if *evidence {
			for _, ev := range e.Evidence {
				fmt.Fprintf(env.Stdout, "         └ %s: %s\n", ev.Source, ev.Value)
			}
		}
	}
	if len(entries) == 0 {
		fmt.Fprintln(env.Stderr, "no matching commits")
	}
	return ExitOK
}

func kindTag(k attrib.Kind) string {
	switch k {
	case attrib.Agent:
		return "AGENT"
	case attrib.Assisted:
		return "ASSIST"
	case attrib.Bot:
		return "BOT"
	default:
		return "HUMAN"
	}
}

func colorKind(k attrib.Kind) string {
	switch k {
	case attrib.Agent:
		return "\x1b[1;35m"
	case attrib.Assisted:
		return "\x1b[35m"
	case attrib.Bot:
		return "\x1b[33m"
	default:
		return "\x1b[36m"
	}
}

func exitOn(env Env, err error) int {
	if err != nil {
		return fail(env, err)
	}
	return ExitOK
}
