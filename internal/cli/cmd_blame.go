package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/trinhbentre/aiblame/internal/attrib"
	"github.com/trinhbentre/aiblame/internal/gitx"
	"github.com/trinhbentre/aiblame/internal/render"
	"github.com/trinhbentre/aiblame/internal/stats"
)

const blameHelp = `Usage: aiblame blame FILE [flags]

Like git blame, but each line is tagged HUMAN / ASSIST / AGENT / BOT with the
agent that touched it.

Flags:
  --rev REV       Revision (default HEAD)
  -w              Ignore whitespace
  --format FMT    table (default) | json
  --no-color
  --summary       Only print the per-file totals
`

type blameLineOut struct {
	Line    int         `json:"line"`
	Hash    string      `json:"hash"`
	Kind    attrib.Kind `json:"kind"`
	Agent   string      `json:"agent,omitempty"`
	Author  string      `json:"author"`
	Content string      `json:"content"`
}

func cmdBlame(ctx context.Context, args []string, env Env) int {
	fs := newFlagSet("blame", env, blameHelp)
	var c common
	c.bind(fs)
	summary := fs.Bool("summary", false, "only totals")
	pos, err := parseInterspersed(fs, args)
	if err != nil {
		return usageErr(env, fs, err)
	}
	if len(pos) != 1 {
		return usageErr(env, fs, fmt.Errorf("expected exactly one FILE"))
	}
	format, err := c.resolvedFormat("table")
	if err != nil {
		return usageErr(env, fs, err)
	}
	file := pos[0]
	if !filepath.IsAbs(file) && env.Cwd != "" {
		file = filepath.Join(env.Cwd, file)
	}
	abs, err := filepath.Abs(file)
	if err != nil {
		return fail(env, err)
	}
	r, err := gitx.Open(ctx, filepath.Dir(abs))
	if err != nil {
		return fail(env, err)
	}
	// git reports the real top-level path; resolve symlinks (e.g. macOS
	// /var -> /private/var) on both sides before computing the relative path.
	realAbs, realRoot := abs, r.Dir
	if p, err := filepath.EvalSymlinks(abs); err == nil {
		realAbs = p
	}
	if p, err := filepath.EvalSymlinks(r.Dir); err == nil {
		realRoot = p
	}
	rel, err := filepath.Rel(realRoot, realAbs)
	if err != nil || strings.HasPrefix(rel, "..") {
		return fail(env, fmt.Errorf("%s is outside the repository %s", abs, r.Dir))
	}
	rel = filepath.ToSlash(rel)
	an, _, err := buildAnalyzer(&c, r, env, false)
	if err != nil {
		return fail(env, err)
	}
	hash, err := r.ResolveRev(ctx, stats.HeadOfRange(an.Opts.Rev))
	if err != nil {
		return fail(env, err)
	}
	commits, err := an.Commits(ctx)
	if err != nil {
		return fail(env, err)
	}
	store := an.Provenance()
	byHash := map[string]*stats.ClassifiedCommit{}
	for i := range commits {
		byHash[commits[i].Hash] = &commits[i]
	}
	bopts := gitx.BlameOptions{Rev: hash, IgnoreWhitespace: c.ignoreWS, Content: true}
	if !c.noIgnoreRevs {
		bopts.IgnoreRevsFile = r.IgnoreRevsFile()
	}
	res, err := r.Blame(ctx, rel, bopts)
	if err != nil {
		return fail(env, err)
	}
	var out []blameLineOut
	var totals [4]int
	for _, l := range res.Lines {
		k := attrib.Human
		agent, author := "", ""
		if cc := byHash[l.Hash]; cc != nil {
			k = cc.Attr.Kind
			agent = cc.Attr.PrimaryAgent()
			author = cc.AuthorName
			// A git-ai authorship log knows which lines of the commit the
			// agent wrote; use it when present.
			if ov := stats.LineKind(store, cc, l, rel); ov != nil {
				k = *ov
				if !k.IsAI() {
					agent = ""
				}
			}
		}
		totals[k]++
		out = append(out, blameLineOut{Line: l.LineNo, Hash: l.Hash, Kind: k, Agent: agent, Author: author, Content: l.Content})
	}
	ai := totals[attrib.Assisted] + totals[attrib.Agent]
	den := ai + totals[attrib.Human]
	pct := 0.0
	if den > 0 {
		pct = float64(ai) * 100 / float64(den)
	}
	if format == "json" {
		enc := json.NewEncoder(env.Stdout)
		enc.SetIndent("", "  ")
		if out == nil {
			out = []blameLineOut{}
		}
		return exitOn(env, enc.Encode(map[string]any{
			"file": rel, "rev_hash": hash, "ai_share": pct,
			"human": totals[attrib.Human], "assisted": totals[attrib.Assisted], "agent": totals[attrib.Agent], "bot": totals[attrib.Bot],
			"lines": out,
		}))
	}
	color := useColor(env.Stdout, c.noColor, env.Getenv)
	if !*summary {
		width := 0
		for _, l := range out {
			if n := len(l.Agent); n > width {
				width = n
			}
		}
		if width < 5 {
			width = 5
		}
		for _, l := range out {
			agent := render.Sanitize(l.Agent)
			if agent == "" {
				agent = "-"
			}
			prefix := fmt.Sprintf("%-6s %-*s %s %4d", kindTag(l.Kind), width, agent, l.Hash[:7], l.Line)
			if color {
				prefix = colorKind(l.Kind) + prefix + "\x1b[0m"
			}
			fmt.Fprintf(env.Stdout, "%s | %s\n", prefix, render.Sanitize(l.Content))
		}
		fmt.Fprintln(env.Stdout)
	}
	fmt.Fprintf(env.Stdout, "%s: %s AI-written (%d assisted + %d agent, %d human, %d bot of %d lines)\n",
		rel, render.Pct(pct), totals[attrib.Assisted], totals[attrib.Agent], totals[attrib.Human], totals[attrib.Bot], len(out))
	return ExitOK
}
