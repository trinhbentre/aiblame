---
name: aiblame
description: Measure and disclose AI authorship in a git repository. Use when asked "how much of this repo/PR/file was written by AI", when a project requires Assisted-by / Co-authored-by disclosure trailers, when generating an AI-authorship badge or report, or before committing so the commit discloses the agent that helped.
---

# aiblame

`aiblame` is a single-binary CLI that computes AI authorship statistics from
the attribution already present in git history: `Co-authored-by` trailers
(Claude Code, Codex, Cursor, Copilot, aider), `Assisted-by` trailers (Linux
kernel, Mesa, Fedora, Apache Calcite), `Generated-by` trailers, agent author
identities (Devin, Copilot SWE agent, Jules) and "Generated with …" markers.
It needs no setup, database or network: only `git`.

## When to use

- "How much of this repository was written by AI?" → `aiblame stats`
- "Which files / which commits did the agent write?" → `aiblame blame FILE`, `aiblame log --ai`
- "Add an AI-written badge to the README" → `aiblame badge`
- "Enforce that AI-assisted commits carry an Assisted-by trailer" → `aiblame check`
- "Make my commits disclose that an agent helped" → `aiblame hook install`

## Install

```sh
go install github.com/trinhbentre/aiblame/cmd/aiblame@latest
# or
curl -fsSL https://raw.githubusercontent.com/trinhbentre/aiblame/main/install.sh | sh
```

## Commands

```sh
aiblame                              # report for the current repo (surviving lines by git blame)
aiblame stats PATH --json            # machine-readable; see "JSON" below
aiblame stats owner/repo             # clone a GitHub repo into the cache and analyse it
aiblame stats --no-blame             # fast mode on huge repos (lines added instead of blame)
aiblame stats --since "3 months ago" # restrict commit/churn metrics to a window
aiblame log --ai --evidence          # every AI-touched commit and why it was classified
aiblame blame src/app.go             # per-line HUMAN / ASSIST / AGENT / BOT view
aiblame badge --out ai.svg           # SVG badge; --shields ai.json for img.shields.io/endpoint
aiblame check --max 60 --require-trailer assisted-by --forbid-agent-authored
aiblame hook install [--style assisted-by|co-authored-by|generated-by]
aiblame agents                       # list recognised agents/bots
```

Exit codes: 0 ok, 1 a `check` rule failed (or `hook status` not installed),
2 usage error, 3 runtime error.

## Reading the output

- **AI share** = AI lines / (AI + human lines). Non-AI bots (dependabot,
  renovate, github-actions) are shown but excluded from the denominator.
- **assisted** = human author + AI trailer/marker. **agent** = the AI identity
  is the commit author. Both count as AI.
- Default metric is **surviving lines** (git blame at HEAD). `--no-blame`
  switches to **lines added**. Commit counts are always shown too.
- Lockfiles, `vendor/`, `node_modules/`, minified and generated files are
  excluded by default (`--no-default-excludes` to include them).
- aiblame only counts **disclosed** AI involvement. Undisclosed AI code is
  invisible to it; say so when reporting numbers.

## JSON

`aiblame stats --json` prints one object (`schema_version` 1) with:
`headline_ai_share` (percent), `metric` ("lines" | "churn"),
`commits` / `churn` / `lines` (each `{human, assisted, agent, bot, total, ai, ai_share}`),
`agents[]` (`name, vendor, commits, churn, lines, models`),
`authors[]`, `conventions[]` (`co-authored-by`, `assisted-by`, `agent-author`, …),
`dirs[]`, `files[]`, `months[]` (`YYYY-MM`), `warnings[]`.

## Disclosing your own commits

Projects increasingly require disclosure. Two conventions dominate:

```
Assisted-by: Claude Code (claude-opus-4-6)          # kernel / Mesa / Fedora style
Co-authored-by: Claude <noreply@anthropic.com>      # GitHub style (Claude Code default)
```

`aiblame hook install` writes a `prepare-commit-msg` hook that adds the
trailer automatically when the commit is made from an agent session
(detected via `CLAUDECODE`, `CODEX_SANDBOX*`, `CURSOR_AGENT`, `GEMINI_CLI`,
`OPENCODE`, or `AIBLAME_AGENT`). Set `AIBLAME_MODEL` to include the model.
If the repo's policy says which convention to use, pass `--style`.

## Config

Optional `.aiblame.toml` at the repo root (`aiblame init` writes a commented
template): extra `[[detect.agents]]`, `paths.exclude`, `[check]` thresholds,
`[badge]` label/colour.
