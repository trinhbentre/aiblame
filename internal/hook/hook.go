// Package hook installs a prepare-commit-msg hook that appends an AI
// attribution trailer when a commit is made from inside a coding-agent
// session.
//
// Detection is environment based: Claude Code sets CLAUDECODE=1 in every
// subprocess, Codex sets CODEX_SANDBOX*, Cursor's agent sets CURSOR_AGENT=1,
// Gemini CLI sets GEMINI_CLI=1 and OpenCode sets OPENCODE=1. AIBLAME_AGENT /
// AIBLAME_MODEL override detection; AIBLAME_DISABLE=1 turns the hook off.
package hook

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/trinhbentre/aiblame/internal/gitx"
)

// Style selects the trailer convention the hook writes.
type Style string

const (
	// AssistedBy writes "Assisted-by: <agent> (<model>)" — the convention
	// used by the Linux kernel, Mesa, Zephyr, Fedora and Apache Calcite.
	AssistedBy Style = "assisted-by"
	// CoAuthoredBy writes "Co-authored-by: <agent> <email>" — what Claude
	// Code, Codex, Cursor and aider emit themselves.
	CoAuthoredBy Style = "co-authored-by"
	// GeneratedBy writes "Generated-by: <agent> (model: <model>)".
	GeneratedBy Style = "generated-by"
)

// ParseStyle validates a style string.
func ParseStyle(s string) (Style, error) {
	switch Style(strings.ToLower(strings.TrimSpace(s))) {
	case "", AssistedBy:
		return AssistedBy, nil
	case CoAuthoredBy:
		return CoAuthoredBy, nil
	case GeneratedBy:
		return GeneratedBy, nil
	}
	return "", fmt.Errorf("unknown style %q (want assisted-by, co-authored-by or generated-by)", s)
}

// Marker identifies hooks written by aiblame.
const Marker = "# aiblame-hook v1"

// EnvRule maps an environment variable to an agent.
type EnvRule struct {
	Var   string
	Agent string
	Email string
	// Note documents the source of the variable.
	Note string
}

// EnvRules lists the variables checked, in priority order.
var EnvRules = []EnvRule{
	{Var: "CLAUDECODE", Agent: "Claude Code", Email: "noreply@anthropic.com", Note: "set to 1 in every subprocess Claude Code spawns"},
	{Var: "CODEX_SANDBOX", Agent: "Codex", Email: "noreply@openai.com", Note: "set by Codex CLI's sandbox (e.g. seatbelt)"},
	{Var: "CODEX_SANDBOX_NETWORK_DISABLED", Agent: "Codex", Email: "noreply@openai.com", Note: "set by Codex CLI inside its sandbox"},
	{Var: "CODEX_THREAD_ID", Agent: "Codex", Email: "noreply@openai.com", Note: "set by Codex CLI for the active thread"},
	{Var: "CURSOR_AGENT", Agent: "Cursor", Email: "cursoragent@cursor.com", Note: "set to 1 by the Cursor agent (not by Cursor's human terminal)"},
	{Var: "GEMINI_CLI", Agent: "Gemini CLI", Email: "gemini-cli@google.com", Note: "set to 1 for shell commands run by Gemini CLI"},
	{Var: "OPENCODE", Agent: "OpenCode", Email: "noreply@opencode.ai", Note: "set for the lifetime of an OpenCode process"},
}

// Detected is the result of environment detection.
type Detected struct {
	Agent string
	Email string
	Model string
	Var   string
}

// Detect inspects the environment (via getenv) for an agent session.
func Detect(getenv func(string) string) (Detected, bool) {
	if getenv("AIBLAME_DISABLE") != "" {
		return Detected{}, false
	}
	model := getenv("AIBLAME_MODEL")
	if a := getenv("AIBLAME_AGENT"); a != "" {
		return Detected{Agent: a, Email: getenv("AIBLAME_EMAIL"), Model: model, Var: "AIBLAME_AGENT"}, true
	}
	for _, r := range EnvRules {
		if getenv(r.Var) != "" {
			return Detected{Agent: r.Agent, Email: r.Email, Model: model, Var: r.Var}, true
		}
	}
	// Generic fallback: AI_AGENT="claude-code_2-1-261_agent" style values.
	if a := getenv("AI_AGENT"); a != "" {
		name := a
		if i := strings.IndexByte(name, '_'); i > 0 {
			name = name[:i]
		}
		name = strings.ReplaceAll(name, "-", " ")
		return Detected{Agent: name, Model: model, Var: "AI_AGENT"}, true
	}
	return Detected{}, false
}

// Trailer formats the trailer line for a detection in the given style.
func Trailer(d Detected, style Style) string {
	agent := strings.TrimSpace(d.Agent)
	switch style {
	case CoAuthoredBy:
		email := d.Email
		if email == "" {
			// No canonical address: fall back to Assisted-by so we never
			// invent an e-mail identity.
			return Trailer(d, AssistedBy)
		}
		return fmt.Sprintf("Co-authored-by: %s <%s>", agent, email)
	case GeneratedBy:
		if d.Model != "" {
			return fmt.Sprintf("Generated-by: %s (model: %s)", agent, d.Model)
		}
		return fmt.Sprintf("Generated-by: %s", agent)
	default:
		if d.Model != "" {
			return fmt.Sprintf("Assisted-by: %s (%s)", agent, d.Model)
		}
		return fmt.Sprintf("Assisted-by: %s", agent)
	}
}

// Script returns the POSIX sh prepare-commit-msg hook for a style.
func Script(style Style) string {
	var b strings.Builder
	b.WriteString("#!/bin/sh\n")
	b.WriteString(Marker + "\n")
	b.WriteString("# Appends an AI attribution trailer when this commit is created from inside a\n")
	b.WriteString("# coding-agent session (Claude Code, Codex, Cursor, Gemini CLI, OpenCode).\n")
	b.WriteString("# Style: " + string(style) + ". Manage with: aiblame hook install|uninstall|status\n")
	b.WriteString("# Override: AIBLAME_AGENT / AIBLAME_MODEL / AIBLAME_EMAIL. Disable: AIBLAME_DISABLE=1\n")
	b.WriteString(`msgfile="$1"; source="${2:-}"
case "$source" in merge|squash) exit 0 ;; esac
[ -n "${AIBLAME_DISABLE:-}" ] && exit 0
[ -f "$msgfile" ] || exit 0
agent=""; email=""; model="${AIBLAME_MODEL:-}"
if [ -n "${AIBLAME_AGENT:-}" ]; then
  agent="$AIBLAME_AGENT"; email="${AIBLAME_EMAIL:-}"
`)
	for _, r := range EnvRules {
		fmt.Fprintf(&b, "elif [ -n \"${%s:-}\" ]; then\n  agent=%q; email=%q\n", r.Var, r.Agent, r.Email)
	}
	b.WriteString(`elif [ -n "${AI_AGENT:-}" ]; then
  agent=$(printf '%s' "$AI_AGENT" | sed 's/_.*//; s/-/ /g')
fi
[ -z "$agent" ] && exit 0
# Do not add a second trailer when the agent already disclosed itself.
if grep -qiE '^(assisted-by|generated-by|generated-with|coding-agent|ai-agent|ai-assistant|ai-assisted-by):' "$msgfile" 2>/dev/null; then exit 0; fi
if grep -qiE '^co-authored-by:.*(noreply@anthropic\.com|noreply@openai\.com|cursoragent@cursor\.com|copilot(\[bot\])?@users\.noreply\.github\.com|noreply@aider\.chat|noreply@opencode\.ai|@ampcode\.com|gemini-cli@google\.com)' "$msgfile" 2>/dev/null; then exit 0; fi
`)
	switch style {
	case CoAuthoredBy:
		b.WriteString(`if [ -n "$email" ]; then
  trailer="Co-authored-by: $agent <$email>"
elif [ -n "$model" ]; then
  trailer="Assisted-by: $agent ($model)"
else
  trailer="Assisted-by: $agent"
fi
`)
	case GeneratedBy:
		b.WriteString(`if [ -n "$model" ]; then
  trailer="Generated-by: $agent (model: $model)"
else
  trailer="Generated-by: $agent"
fi
`)
	default:
		b.WriteString(`if [ -n "$model" ]; then
  trailer="Assisted-by: $agent ($model)"
else
  trailer="Assisted-by: $agent"
fi
`)
	}
	b.WriteString(`if command -v git >/dev/null 2>&1 && git interpret-trailers --in-place --if-exists addIfDifferent --trailer "$trailer" "$msgfile" 2>/dev/null; then
  exit 0
fi
printf '\n%s\n' "$trailer" >> "$msgfile" || echo "aiblame-hook: could not append trailer to $msgfile" >&2
exit 0
`)
	return b.String()
}

// ErrExists is returned when a foreign prepare-commit-msg hook exists.
var ErrExists = errors.New("a prepare-commit-msg hook not managed by aiblame already exists (use --force to replace it)")

// Path returns the prepare-commit-msg hook path for the repository.
func Path(ctx context.Context, r *gitx.Runner) (string, error) {
	dir, err := r.HooksDir(ctx)
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "prepare-commit-msg"), nil
}

// Install writes the hook. Existing aiblame hooks are replaced; foreign hooks
// are left alone unless force is set.
func Install(ctx context.Context, r *gitx.Runner, style Style, force bool) (string, error) {
	p, err := Path(ctx, r)
	if err != nil {
		return "", err
	}
	if b, err := os.ReadFile(p); err == nil {
		if !strings.Contains(string(b), Marker) && !force {
			return p, ErrExists
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return p, err
	}
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		return p, err
	}
	if err := os.WriteFile(p, []byte(Script(style)), 0o755); err != nil {
		return p, err
	}
	return p, nil
}

// Uninstall removes an aiblame-managed hook. It refuses to delete foreign
// hooks and reports whether anything was removed.
func Uninstall(ctx context.Context, r *gitx.Runner) (removed bool, path string, err error) {
	p, err := Path(ctx, r)
	if err != nil {
		return false, "", err
	}
	b, err := os.ReadFile(p)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return false, p, nil
		}
		return false, p, err
	}
	if !strings.Contains(string(b), Marker) {
		return false, p, fmt.Errorf("%s is not managed by aiblame; not removing", p)
	}
	return true, p, os.Remove(p)
}

// Status describes the current hook.
type Status struct {
	Path      string
	Installed bool
	Managed   bool
	Style     Style
}

// Inspect reports the state of the hook file.
func Inspect(ctx context.Context, r *gitx.Runner) (Status, error) {
	p, err := Path(ctx, r)
	if err != nil {
		return Status{}, err
	}
	st := Status{Path: p}
	b, err := os.ReadFile(p)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return st, nil
		}
		return st, err
	}
	st.Installed = true
	s := string(b)
	st.Managed = strings.Contains(s, Marker)
	for _, sty := range []Style{AssistedBy, CoAuthoredBy, GeneratedBy} {
		if strings.Contains(s, "# Style: "+string(sty)+".") {
			st.Style = sty
		}
	}
	return st, nil
}
