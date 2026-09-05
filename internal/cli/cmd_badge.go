package cli

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/trinhbentre/aiblame/internal/render"
)

const badgeHelp = `Usage: aiblame badge [PATH|URL] [flags]

Render the AI share as an SVG badge and/or a shields.io endpoint JSON.

Flags:
  --out FILE        Write the SVG here ("-" = stdout; default when --shields is not set)
  --shields FILE    Write shields.io endpoint JSON here (serve it and use
                    https://img.shields.io/endpoint?url=<raw-url>)
  --label TEXT      Badge label (default "AI-written", or [badge].label in config)
  --color COLOR     Hex or shields name; "auto" scales with the percentage
  --style STYLE     flat (default) | flat-square
  --metric M        lines (default) | churn | commits
  --precision N     Decimal places in the percentage (default 0)
Plus the common analysis flags (see "aiblame help").

Examples:
  aiblame badge --out .github/badges/ai.svg
  aiblame badge --shields .github/badges/ai.json --color auto
`

func cmdBadge(ctx context.Context, args []string, env Env) int {
	fs := newFlagSet("badge", env, badgeHelp)
	var c common
	c.bind(fs)
	out := fs.String("out", "", "SVG output path")
	shields := fs.String("shields", "", "shields endpoint JSON path")
	label := fs.String("label", "", "badge label")
	color := fs.String("color", "", "badge colour")
	style := fs.String("style", "", "flat | flat-square")
	metric := fs.String("metric", "lines", "lines | churn | commits")
	precision := fs.Int("precision", 0, "decimal places")
	pos, err := parseInterspersed(fs, args)
	if err != nil {
		return usageErr(env, fs, err)
	}
	if len(pos) > 1 {
		return usageErr(env, fs, fmt.Errorf("expected at most one PATH"))
	}
	target := ""
	if len(pos) == 1 {
		target = pos[0]
	}
	switch *metric {
	case "lines", "churn", "commits":
	default:
		return usageErr(env, fs, fmt.Errorf("--metric must be lines, churn or commits"))
	}
	r, _, err := openTarget(ctx, target, env, c.quiet)
	if err != nil {
		return fail(env, err)
	}
	an, cfg, err := buildAnalyzer(&c, r, env, !c.noBlame && *metric == "lines")
	if err != nil {
		return fail(env, err)
	}
	rep, err := an.Run(ctx)
	if err != nil {
		return fail(env, err)
	}
	var pct float64
	switch *metric {
	case "commits":
		pct = rep.Commits.AIShare
	case "churn":
		pct = rep.Churn.AIShare
	default:
		pct = rep.Headline
	}
	if *label == "" {
		*label = cfg.Badge.Label
	}
	if *label == "" {
		*label = "AI-written"
	}
	if *color == "" {
		*color = cfg.Badge.Color
	}
	if *style == "" {
		*style = cfg.Badge.Style
	}
	if *style == "" {
		*style = "flat"
	}
	msg := fmt.Sprintf("%.*f%%", *precision, pct)
	colorHex := render.ResolveColor(*color, pct)

	wrote := 0
	if *shields != "" {
		b, err := render.ShieldsJSON(*label, msg, colorHex, *style)
		if err != nil {
			return fail(env, err)
		}
		if err := writeFile(*shields, append(b, '\n')); err != nil {
			return fail(env, err)
		}
		if !c.quiet {
			fmt.Fprintf(env.Stderr, "wrote %s\n", *shields)
		}
		wrote++
	}
	if *out != "" || wrote == 0 {
		svg := render.BadgeSVG(*label, msg, colorHex, *style)
		if *out == "" || *out == "-" {
			fmt.Fprintln(env.Stdout, svg)
		} else {
			if err := writeFile(*out, []byte(svg+"\n")); err != nil {
				return fail(env, err)
			}
			if !c.quiet {
				fmt.Fprintf(env.Stderr, "wrote %s (%s %s)\n", *out, *label, msg)
			}
		}
	}
	return ExitOK
}

func writeFile(path string, b []byte) error {
	if dir := filepath.Dir(path); dir != "" {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return err
		}
	}
	return os.WriteFile(path, b, 0o644)
}
