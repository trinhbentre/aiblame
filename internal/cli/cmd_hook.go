package cli

import (
	"context"
	"errors"
	"fmt"

	"github.com/trinhbentre/aiblame/internal/gitx"
	"github.com/trinhbentre/aiblame/internal/hook"
)

const hookHelp = `Usage: aiblame hook <install|uninstall|status|print|detect> [flags]

Manage a prepare-commit-msg hook that appends an AI attribution trailer when
a commit is created from inside a coding-agent session. Detection uses the
environment variables agents set in their subprocesses:

  CLAUDECODE=1                     Claude Code
  CODEX_SANDBOX* / CODEX_THREAD_ID Codex CLI
  CURSOR_AGENT=1                   Cursor agent
  GEMINI_CLI=1                     Gemini CLI
  OPENCODE=1                       OpenCode
  AI_AGENT=<name>_<ver>_agent      generic fallback

Overrides: AIBLAME_AGENT, AIBLAME_MODEL, AIBLAME_EMAIL. Disable: AIBLAME_DISABLE=1.

Flags:
  --style S    assisted-by (default) | co-authored-by | generated-by
  --force      Replace a prepare-commit-msg hook that aiblame did not write
  -C DIR       Repository directory (default: current)

The hook is plain POSIX sh and needs no aiblame binary at commit time.
`

func cmdHook(ctx context.Context, args []string, env Env) int {
	fs := newFlagSet("hook", env, hookHelp)
	styleFlag := fs.String("style", "assisted-by", "trailer style")
	force := fs.Bool("force", false, "replace foreign hook")
	dir := fs.String("C", "", "repository directory")
	pos, err := parseInterspersed(fs, args)
	if err != nil {
		return usageErr(env, fs, err)
	}
	if len(pos) != 1 {
		return usageErr(env, fs, errors.New("expected one of install, uninstall, status, print, detect"))
	}
	style, err := hook.ParseStyle(*styleFlag)
	if err != nil {
		return usageErr(env, fs, err)
	}
	switch pos[0] {
	case "print":
		fmt.Fprint(env.Stdout, hook.Script(style))
		return ExitOK
	case "detect":
		d, ok := hook.Detect(env.Getenv)
		if !ok {
			fmt.Fprintln(env.Stdout, "no coding-agent session detected in the environment")
			return ExitFailed
		}
		fmt.Fprintf(env.Stdout, "agent: %s\nvia:   %s\ntrailer: %s\n", d.Agent, d.Var, hook.Trailer(d, style))
		return ExitOK
	}
	target := *dir
	if target == "" {
		target = env.Cwd
	}
	if target == "" {
		target = "."
	}
	r, err := gitx.Open(ctx, target)
	if err != nil {
		return fail(env, err)
	}
	switch pos[0] {
	case "install":
		p, err := hook.Install(ctx, r, style, *force)
		if err != nil {
			if errors.Is(err, hook.ErrExists) {
				fmt.Fprintf(env.Stderr, "aiblame: %v\n  existing hook: %s\n", err, p)
				return ExitFailed
			}
			return fail(env, err)
		}
		fmt.Fprintf(env.Stdout, "installed %s (style: %s)\n", p, style)
		if _, ok := hook.Detect(env.Getenv); ok {
			fmt.Fprintln(env.Stdout, "note: this shell is inside an agent session; commits made here will be tagged")
		}
		return ExitOK
	case "uninstall":
		removed, p, err := hook.Uninstall(ctx, r)
		if err != nil {
			return fail(env, err)
		}
		if removed {
			fmt.Fprintf(env.Stdout, "removed %s\n", p)
		} else {
			fmt.Fprintf(env.Stdout, "no aiblame hook at %s\n", p)
		}
		return ExitOK
	case "status":
		st, err := hook.Inspect(ctx, r)
		if err != nil {
			return fail(env, err)
		}
		switch {
		case !st.Installed:
			fmt.Fprintf(env.Stdout, "not installed (%s)\n", st.Path)
			return ExitFailed
		case !st.Managed:
			fmt.Fprintf(env.Stdout, "a prepare-commit-msg hook exists but was not written by aiblame (%s)\n", st.Path)
			return ExitFailed
		default:
			fmt.Fprintf(env.Stdout, "installed (%s, style: %s)\n", st.Path, st.Style)
			return ExitOK
		}
	}
	return usageErr(env, fs, fmt.Errorf("unknown hook subcommand %q", pos[0]))
}
