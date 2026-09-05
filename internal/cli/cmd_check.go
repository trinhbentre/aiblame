package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/trinhbentre/aiblame/internal/attrib"
	"github.com/trinhbentre/aiblame/internal/render"
	"github.com/trinhbentre/aiblame/internal/stats"
)

const checkHelp = `Usage: aiblame check [PATH|URL] [flags]

Enforce an AI-authorship policy. Exit code 1 when any rule fails.

Rules (flags override [check] in .aiblame.toml):
  --max PCT                  Fail when the AI share exceeds PCT
  --min PCT                  Fail when the AI share is below PCT
  --metric M                 lines (default) | churn | commits
  --forbid-agent-authored    Fail when an AI identity is the commit *author*
                             (DCO-style policies: only humans may sign off)
  --require-trailer NAME     Fail when an AI-assisted commit does not use this
                             convention: assisted-by | co-authored-by |
                             generated-by | coding-agent
  --max-violations N         Show at most N offending commits per rule (default 20)
Plus the common analysis flags. --since is useful to check only new commits:

  aiblame check --since 2026-01-01 --require-trailer assisted-by --forbid-agent-authored
  aiblame check --max 60
`

type checkResult struct {
	Rule    string   `json:"rule"`
	Pass    bool     `json:"pass"`
	Detail  string   `json:"detail"`
	Commits []string `json:"commits,omitempty"`
}

type checkOutput struct {
	Pass     bool          `json:"pass"`
	Metric   string        `json:"metric"`
	AIShare  float64       `json:"ai_share"`
	Results  []checkResult `json:"results"`
	Rev      string        `json:"rev_hash"`
	Since    string        `json:"since,omitempty"`
	Until    string        `json:"until,omitempty"`
	Version  string        `json:"version"`
	Warnings []string      `json:"warnings,omitempty"`
}

func cmdCheck(ctx context.Context, args []string, env Env) int {
	fs := newFlagSet("check", env, checkHelp)
	var c common
	c.bind(fs)
	maxShare := fs.Float64("max", -1, "max AI share")
	minShare := fs.Float64("min", -1, "min AI share")
	metric := fs.String("metric", "", "lines | churn | commits")
	forbidAgent := fs.Bool("forbid-agent-authored", false, "fail on agent-authored commits")
	requireTrailer := fs.String("require-trailer", "", "required convention")
	maxViol := fs.Int("max-violations", 20, "offending commits to list")
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
	target := ""
	if len(pos) == 1 {
		target = pos[0]
	}
	r, err := openTarget(ctx, target, env, c.quiet)
	if err != nil {
		return fail(env, err)
	}
	an, cfg, err := buildAnalyzer(&c, r, env, false)
	if err != nil {
		return fail(env, err)
	}
	if *metric == "" {
		*metric = cfg.Check.Metric
	}
	if *metric == "" {
		*metric = "lines"
	}
	switch *metric {
	case "lines", "churn", "commits":
	default:
		return usageErr(env, fs, fmt.Errorf("--metric must be lines, churn or commits"))
	}
	if *maxShare < 0 && cfg.Check.Max != nil {
		*maxShare = *cfg.Check.Max
	}
	if *minShare < 0 && cfg.Check.Min != nil {
		*minShare = *cfg.Check.Min
	}
	if !*forbidAgent {
		*forbidAgent = cfg.Check.ForbidAgentAuthored
	}
	if *requireTrailer == "" {
		*requireTrailer = cfg.Check.RequireTrailer
	}
	*requireTrailer = strings.ToLower(strings.TrimSpace(*requireTrailer))
	switch *requireTrailer {
	case "", "assisted-by", "co-authored-by", "generated-by", "coding-agent":
	default:
		return usageErr(env, fs, fmt.Errorf("--require-trailer %q is not a known convention", *requireTrailer))
	}
	if *maxShare < 0 && *minShare < 0 && !*forbidAgent && *requireTrailer == "" {
		return usageErr(env, fs, fmt.Errorf("no rules given: use --max, --min, --forbid-agent-authored or --require-trailer (or [check] in .aiblame.toml)"))
	}

	needBlame := *metric == "lines" && !c.noBlame
	an.Opts.Blame = needBlame
	if needBlame && !c.quiet && isTTY(env.Stderr) {
		an.Opts.Progress = progressPrinter(env.Stderr)
	}
	rep, err := an.Run(ctx)
	if err != nil {
		return fail(env, err)
	}
	var share float64
	switch *metric {
	case "commits":
		share = rep.Commits.AIShare
	case "churn":
		share = rep.Churn.AIShare
	default:
		share = rep.Headline
		if rep.Metric != "lines" {
			*metric = rep.Metric
		}
	}
	out := checkOutput{Pass: true, Metric: *metric, AIShare: share, Rev: rep.Rev, Since: rep.Since, Until: rep.Until, Version: Version, Warnings: rep.Warnings}
	add := func(res checkResult) {
		if !res.Pass {
			out.Pass = false
		}
		out.Results = append(out.Results, res)
	}
	if *maxShare >= 0 {
		add(checkResult{Rule: "max", Pass: share <= *maxShare, Detail: fmt.Sprintf("AI share %s <= %.1f%% (%s)", render.Pct(share), *maxShare, *metric)})
	}
	if *minShare >= 0 {
		add(checkResult{Rule: "min", Pass: share >= *minShare, Detail: fmt.Sprintf("AI share %s >= %.1f%% (%s)", render.Pct(share), *minShare, *metric)})
	}
	if *forbidAgent || *requireTrailer != "" {
		commits, err := an.Commits(ctx)
		if err != nil {
			return fail(env, err)
		}
		var agentAuthored, undisclosed []string
		for _, cc := range commits {
			if cc.IsMerge() && !c.includeMerges {
				continue
			}
			if in, _ := an.InWindow(ctx, cc.Commit); !in {
				continue
			}
			short := cc.Hash[:7] + " " + truncate(cc.Subject(), 60)
			if *forbidAgent && cc.Attr.Kind == attrib.Agent {
				agentAuthored = append(agentAuthored, short+" ("+cc.Attr.PrimaryAgent()+")")
			}
			if *requireTrailer != "" && cc.Attr.Kind == attrib.Assisted && !hasConvention(cc, *requireTrailer) {
				how := "message marker only"
				if len(cc.Attr.Conventions) > 0 {
					how = strings.Join(cc.Attr.Conventions, ",")
				}
				undisclosed = append(undisclosed, short+" ("+how+")")
			}
		}
		if *forbidAgent {
			add(checkResult{Rule: "forbid-agent-authored", Pass: len(agentAuthored) == 0,
				Detail: fmt.Sprintf("%d commit(s) authored directly by an AI identity", len(agentAuthored)), Commits: capList(agentAuthored, *maxViol)})
		}
		if *requireTrailer != "" {
			add(checkResult{Rule: "require-trailer", Pass: len(undisclosed) == 0,
				Detail: fmt.Sprintf("%d AI-assisted commit(s) without a %s trailer", len(undisclosed), *requireTrailer), Commits: capList(undisclosed, *maxViol)})
		}
	}

	if format == "json" {
		enc := json.NewEncoder(env.Stdout)
		enc.SetIndent("", "  ")
		_ = enc.Encode(out)
	} else {
		color := useColor(env.Stdout, c.noColor, env.Getenv)
		ok, bad := "PASS", "FAIL"
		if color {
			ok, bad = "\x1b[32mPASS\x1b[0m", "\x1b[31mFAIL\x1b[0m"
		}
		fmt.Fprintf(env.Stdout, "aiblame check · %s @ %s · AI share %s (%s)\n\n", rep.Repo, rep.Rev[:7], render.Pct(share), *metric)
		for _, res := range out.Results {
			mark := ok
			if !res.Pass {
				mark = bad
			}
			fmt.Fprintf(env.Stdout, "  %s  %-22s %s\n", mark, res.Rule, res.Detail)
			for _, cm := range res.Commits {
				fmt.Fprintf(env.Stdout, "          %s\n", cm)
			}
		}
		fmt.Fprintln(env.Stdout)
		if out.Pass {
			fmt.Fprintf(env.Stdout, "%s all rules passed\n", ok)
		} else {
			fmt.Fprintf(env.Stdout, "%s one or more rules failed\n", bad)
		}
		for _, w := range rep.Warnings {
			fmt.Fprintf(env.Stderr, "warning: %s\n", w)
		}
	}
	if !out.Pass {
		return ExitFailed
	}
	return ExitOK
}

func hasConvention(cc stats.ClassifiedCommit, want string) bool {
	for _, c := range cc.Attr.Conventions {
		if c == want {
			return true
		}
	}
	return false
}

func capList(s []string, n int) []string {
	if n <= 0 || len(s) <= n {
		return s
	}
	out := append([]string{}, s[:n]...)
	return append(out, fmt.Sprintf("… and %d more", len(s)-n))
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n-1] + "…"
}
