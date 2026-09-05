package cli

import (
	"context"
	"fmt"

	"github.com/trinhbentre/aiblame/internal/render"
)

const statsHelp = `Usage: aiblame stats [PATH|URL|owner/repo] [flags]

Analyse a repository and print how much of it was written with AI.
See "aiblame help" for the common flags.

Examples:
  aiblame                          # current repo, full report
  aiblame stats ../other-repo --json
  aiblame stats openai/codex       # clones into the cache and analyses it
  aiblame --since "6 months ago" --top 5
  aiblame --no-blame               # fast: skip git blame, use lines added
  aiblame -f md > report.md        # Markdown for a PR comment or job summary
`

func cmdStats(ctx context.Context, args []string, env Env) int {
	fs := newFlagSet("stats", env, statsHelp)
	var c common
	c.bind(fs)
	compact := fs.Bool("compact", false, "hide directory, file and timeline sections")
	pos, err := parseInterspersed(fs, args)
	if err != nil {
		return usageErr(env, fs, err)
	}
	if len(pos) > 1 {
		return usageErr(env, fs, fmt.Errorf("expected at most one PATH, got %d", len(pos)))
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
	an, _, err := buildAnalyzer(&c, r, env, !c.noBlame)
	if err != nil {
		return fail(env, err)
	}
	rep, err := an.Run(ctx)
	if err != nil {
		return fail(env, err)
	}
	rep.Warnings = append(warns, rep.Warnings...)
	w, closeFn, err := openOutput(c.output, env)
	if err != nil {
		return fail(env, err)
	}
	toStdout := c.output == "" || c.output == "-"
	ropts := render.Options{Color: toStdout && useColor(env.Stdout, c.noColor, env.Getenv), Top: c.top, Compact: *compact}
	switch format {
	case "json":
		if err := render.JSON(w, rep); err != nil {
			_ = closeFn()
			return fail(env, err)
		}
	case "md":
		render.Markdown(w, rep, ropts)
	default:
		render.Table(w, rep, ropts)
	}
	// Close errors matter for -o: a full disk surfaces here, not on Write.
	if err := closeFn(); err != nil {
		return fail(env, fmt.Errorf("writing %s: %w", c.output, err))
	}
	if !toStdout && !c.quiet {
		fmt.Fprintf(env.Stderr, "wrote %s\n", c.output)
	}
	return ExitOK
}
