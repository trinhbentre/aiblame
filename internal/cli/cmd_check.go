package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/trinhbentre/aiblame/internal/attrib"
	"github.com/trinhbentre/aiblame/internal/config"
	"github.com/trinhbentre/aiblame/internal/policy"
	"github.com/trinhbentre/aiblame/internal/render"
	"github.com/trinhbentre/aiblame/internal/stats"
)

const checkHelp = `Usage: aiblame check [PATH|URL] [flags]

Enforce an AI-authorship policy. Exit code 1 when any rule fails.

Rules (flags override [check] in .aiblame.toml):
  --policy NAME              Start from a project's published policy: kernel,
                             zephyr, mesa, openinfra, llvm, asf, fedora, artsy,
                             kubernetes (--list-policies shows the sources)
  --max PCT                  Fail when the AI share exceeds PCT
  --min PCT                  Fail when the AI share is below PCT
  --metric M                 lines (default) | churn | commits
  --forbid-agent-authored    Fail when an AI identity is the commit *author*
                             (DCO-style policies: only humans may sign off)
  --forbid-agent-signoff     Fail when a Signed-off-by trailer names an AI
  --require-trailer CONV     Fail when an AI-assisted commit does not use one of
                             these conventions: assisted-by, co-authored-by,
                             generated-by, coding-agent, … or a comma list
                             ("assisted-by,generated-by" = either)
  --forbid-trailer CONV      Fail when an AI-touched commit uses this
                             convention (repeatable), e.g. co-authored-by
  --max-violations N         Show at most N offending commits per rule (default 20)
  --list-policies            Print the presets and where they come from
Plus the common analysis flags. --rev BASE..HEAD checks one range (a pull
request); --since checks only recent commits:

  aiblame check --policy kernel --rev origin/main..HEAD
  aiblame check --since 2026-01-01 --require-trailer assisted-by --forbid-agent-authored
  aiblame check --forbid-trailer co-authored-by      # Mesa: AI is not a co-author
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
	Policy   string        `json:"policy,omitempty"`
	Metric   string        `json:"metric"`
	AIShare  float64       `json:"ai_share"`
	Results  []checkResult `json:"results"`
	Rev      string        `json:"rev_hash"`
	BaseRev  string        `json:"base_rev,omitempty"`
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
	forbidSignoff := fs.Bool("forbid-agent-signoff", false, "fail when an AI signs off")
	requireTrailer := fs.String("require-trailer", "", "required convention(s)")
	var forbidTrailers stringList
	fs.Var(&forbidTrailers, "forbid-trailer", "forbidden convention (repeatable)")
	policyName := fs.String("policy", "", "policy preset")
	listPolicies := fs.Bool("list-policies", false, "print presets")
	maxViol := fs.Int("max-violations", 20, "offending commits to list")
	pos, err := parseInterspersed(fs, args)
	if err != nil {
		return usageErr(env, fs, err)
	}
	if *listPolicies {
		printPolicies(env)
		return ExitOK
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
	r, warns, err := openTarget(ctx, target, env, c.quiet)
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
	*forbidAgent = *forbidAgent || cfg.Check.ForbidAgentAuthored
	*forbidSignoff = *forbidSignoff || cfg.Check.ForbidAgentSignoff
	if *requireTrailer == "" {
		*requireTrailer = cfg.Check.RequireTrailer
	}
	require := config.SplitConventions(*requireTrailer)
	forbid := config.SplitConventions(strings.Join(append([]string(forbidTrailers), cfg.Check.ForbidTrailers...), ","))
	if *policyName == "" {
		*policyName = cfg.Check.Policy
	}
	if *policyName != "" {
		p := policy.Find(*policyName)
		if p == nil {
			return usageErr(env, fs, fmt.Errorf("unknown policy %q (have %s)", *policyName, strings.Join(policy.Names(), ", ")))
		}
		*policyName = p.Name
		if len(require) == 0 {
			require = append(require, p.RequireTrailer...)
		}
		forbid = append(forbid, p.ForbidTrailers...)
		*forbidAgent = *forbidAgent || p.ForbidAgentAuthored
		*forbidSignoff = *forbidSignoff || p.ForbidAgentSignoff
	}
	for _, conv := range append(append([]string{}, require...), forbid...) {
		if !attrib.IsKnownConvention(conv) {
			return usageErr(env, fs, fmt.Errorf("%q is not a known convention (run `aiblame agents` for the list)", conv))
		}
	}
	forbid = dedupe(forbid)
	if *maxShare < 0 && *minShare < 0 && !*forbidAgent && !*forbidSignoff && len(require) == 0 && len(forbid) == 0 {
		return usageErr(env, fs, fmt.Errorf("no rules given: use --policy, --max, --min, --forbid-agent-authored, --forbid-agent-signoff, --require-trailer or --forbid-trailer (or [check] in .aiblame.toml)"))
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
	rep.Warnings = append(warns, rep.Warnings...)
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
	out := checkOutput{Pass: true, Policy: *policyName, Metric: *metric, AIShare: share, Rev: rep.Rev, BaseRev: rep.BaseRev, Since: rep.Since, Until: rep.Until, Version: Version, Warnings: rep.Warnings}
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
	if *forbidAgent || *forbidSignoff || len(require) > 0 || len(forbid) > 0 {
		commits, err := an.Commits(ctx)
		if err != nil {
			return fail(env, err)
		}
		var agentAuthored, signedOff, undisclosed, forbidden []string
		for _, cc := range commits {
			if cc.IsMerge() && !c.includeMerges {
				continue
			}
			if in, _ := an.InWindow(ctx, cc.Commit); !in {
				continue
			}
			short := cc.Hash[:7] + " " + truncate(render.Sanitize(cc.Subject()), 60)
			if *forbidAgent && cc.Attr.Kind == attrib.Agent {
				agentAuthored = append(agentAuthored, short+" ("+cc.Attr.PrimaryAgent()+")")
			}
			if *forbidSignoff && cc.Attr.AgentSignedOff {
				signedOff = append(signedOff, short+" ("+cc.Attr.PrimaryAgent()+")")
			}
			if len(require) > 0 && cc.Attr.Kind == attrib.Assisted && !hasAnyConvention(cc, require) {
				how := "message marker only"
				if len(cc.Attr.Conventions) > 0 {
					how = strings.Join(cc.Attr.Conventions, ",")
				}
				undisclosed = append(undisclosed, short+" ("+how+")")
			}
			if len(forbid) > 0 && cc.Attr.Kind.IsAI() {
				var used []string
				for _, conv := range forbid {
					if cc.Attr.HasConvention(conv) {
						used = append(used, conv)
					}
				}
				if len(used) > 0 {
					forbidden = append(forbidden, short+" ("+strings.Join(used, ",")+")")
				}
			}
		}
		if *forbidAgent {
			add(checkResult{Rule: "forbid-agent-authored", Pass: len(agentAuthored) == 0,
				Detail: fmt.Sprintf("%d commit(s) authored directly by an AI identity", len(agentAuthored)), Commits: capList(agentAuthored, *maxViol)})
		}
		if *forbidSignoff {
			add(checkResult{Rule: "forbid-agent-signoff", Pass: len(signedOff) == 0,
				Detail: fmt.Sprintf("%d commit(s) with a Signed-off-by naming an AI identity", len(signedOff)), Commits: capList(signedOff, *maxViol)})
		}
		if len(require) > 0 {
			add(checkResult{Rule: "require-trailer", Pass: len(undisclosed) == 0,
				Detail: fmt.Sprintf("%d AI-assisted commit(s) without a %s trailer", len(undisclosed), strings.Join(require, " or ")), Commits: capList(undisclosed, *maxViol)})
		}
		if len(forbid) > 0 {
			add(checkResult{Rule: "forbid-trailer", Pass: len(forbidden) == 0,
				Detail: fmt.Sprintf("%d AI-touched commit(s) using a forbidden convention (%s)", len(forbidden), strings.Join(forbid, ", ")), Commits: capList(forbidden, *maxViol)})
		}
	}

	if format == "json" {
		enc := json.NewEncoder(env.Stdout)
		enc.SetIndent("", "  ")
		if err := enc.Encode(out); err != nil {
			return fail(env, err)
		}
	} else {
		color := useColor(env.Stdout, c.noColor, env.Getenv)
		ok, bad := "PASS", "FAIL"
		if color {
			ok, bad = "\x1b[32mPASS\x1b[0m", "\x1b[31mFAIL\x1b[0m"
		}
		scope := rep.Rev[:7]
		if rep.BaseRev != "" {
			scope = rep.RevName
		}
		head := fmt.Sprintf("aiblame check · %s @ %s · AI share %s (%s)", rep.Repo, scope, render.Pct(share), *metric)
		if *policyName != "" {
			head += " · policy " + *policyName
		}
		fmt.Fprintf(env.Stdout, "%s\n\n", head)
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

func printPolicies(env Env) {
	fmt.Fprintln(env.Stdout, "Policy presets for `aiblame check --policy NAME` (sources last checked 2026-09-06):")
	for _, p := range policy.All() {
		var rules []string
		if len(p.RequireTrailer) > 0 {
			rules = append(rules, "require "+strings.Join(p.RequireTrailer, "|"))
		}
		if len(p.ForbidTrailers) > 0 {
			rules = append(rules, "forbid "+strings.Join(p.ForbidTrailers, ","))
		}
		if p.ForbidAgentAuthored {
			rules = append(rules, "no agent authors")
		}
		if p.ForbidAgentSignoff {
			rules = append(rules, "no agent sign-off")
		}
		fmt.Fprintf(env.Stdout, "\n  %-11s %s\n              rules: %s\n              source: %s\n", p.Name, p.Description, strings.Join(rules, "; "), p.Source)
	}
}

func hasAnyConvention(cc stats.ClassifiedCommit, want []string) bool {
	for _, w := range want {
		if cc.Attr.HasConvention(w) {
			return true
		}
	}
	return false
}

func dedupe(in []string) []string {
	seen := map[string]bool{}
	var out []string
	for _, s := range in {
		if s == "" || seen[s] {
			continue
		}
		seen[s] = true
		out = append(out, s)
	}
	return out
}

func capList(s []string, n int) []string {
	if n <= 0 || len(s) <= n {
		return s
	}
	out := append([]string{}, s[:n]...)
	return append(out, fmt.Sprintf("… and %d more", len(s)-n))
}

// truncate shortens s to at most n runes (not bytes) so multi-byte subjects
// stay valid UTF-8.
func truncate(s string, n int) string {
	if n <= 1 || utf8.RuneCountInString(s) <= n {
		return s
	}
	r := []rune(s)
	return string(r[:n-1]) + "…"
}
