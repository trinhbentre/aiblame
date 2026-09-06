---
name: aiblame
description: Measure and disclose AI authorship in a git repository. Use when asked "how much of this repo/PR/file was written by AI", when a project requires Assisted-by / Co-authored-by disclosure trailers or forbids them, when generating an AI-authorship badge or report, when checking a pull request against a project's AI policy, or before committing so the commit discloses the agent that helped.
---

# aiblame

`aiblame` is a single-binary CLI that computes AI authorship statistics from
the attribution already present in a repository: `Co-authored-by` trailers
(Claude Code, Codex, Cursor, Copilot, aider), `Assisted-by` trailers (Linux
kernel, Mesa, Zephyr, LLVM, Fedora), `Generated-by` (ASF), session links
(`Claude-Session`, `Agent-Logs-Url`, `Amp-Thread-ID`, `Entire-Checkpoint`),
agent author identities (Devin, Copilot coding agent, Jules), "Generated with …"
markers, and the sidecar data hook-based tools write into refs (git-ai
authorship logs in `refs/notes/ai`, Entire checkpoints). It needs no setup,
database or network: only `git`.

## When to use

- "How much of this repository was written by AI?" → `aiblame stats`
- "How much of this branch / PR is AI?" → `aiblame stats --rev origin/main..HEAD`
- "Which files / which commits did the agent write?" → `aiblame blame FILE`, `aiblame log --ai --evidence`
- "Does this repo follow the kernel / Mesa / Kubernetes AI policy?" → `aiblame check --policy NAME`
- "Add an AI-written badge to the README" → `aiblame badge`
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
aiblame stats gitlab.com/o/r         # host/owner/repo for GitLab, Codeberg, Gitea
aiblame stats --rev origin/main..HEAD   # only this range (a pull request)
aiblame stats --no-blame             # fast mode on huge repos (lines added instead of blame)
aiblame stats --since "3 months ago" # restrict commit/churn metrics to a window
aiblame stats --no-authors           # leave people out of a shared report
aiblame log --ai --evidence          # every AI-touched commit, why, and what was NOT counted
aiblame blame src/app.go             # per-line HUMAN / ASSIST / AGENT / BOT view
aiblame badge --out ai.svg           # SVG badge; --shields ai.json for img.shields.io/endpoint
aiblame check --policy kernel        # presets: kernel zephyr mesa openinfra llvm asf fedora artsy kubernetes
aiblame check --max 60 --require-trailer assisted-by,generated-by --forbid-trailer co-authored-by --forbid-agent-signoff
aiblame check --list-policies        # presets with their source URLs
aiblame hook install [--style assisted-by|kernel|agent-model|co-authored-by|generated-by]
aiblame agents                       # recognised agents, trailer key classes, sidecar sources
```

Exit codes: 0 ok, 1 a `check` rule failed (or `hook status` not installed),
2 usage error, 3 runtime error.

## Reading the output

- **AI share** = AI lines / (AI + human lines). Non-AI bots (dependabot,
  renovate, github-actions, SDK generators) are shown but excluded from the
  denominator.
- **assisted** = human author + AI trailer / session link / sidecar record /
  marker. **agent** = the AI identity is the commit author. Both count as AI.
- Default metric is **surviving lines** (git blame at HEAD). `--no-blame`
  switches to **lines added**. Commit counts are always shown too.
- **Still in the tree** (survival) = surviving lines ÷ lines added, per kind
  and per agent. Not shown with `--since`/`--until`.
- With git-ai authorship logs the attribution is **line-level**: only the
  lines the agent wrote count. The footer says how many commits were
  attributed that way.
- `Assisted-by: <a person's name>` (curl style), `Assisted-by: none` and
  `Generated-by: <unknown non-AI tool>` are **not** counted; `log --evidence`
  lists them with the reason, and the report lists unrecognised tool names so
  they can be added to `.aiblame.toml` if they are agents.
- Lockfiles, `vendor/`, `node_modules/`, minified and generated files are
  excluded by default (`--no-default-excludes` to include them).
- aiblame only counts **disclosed** AI involvement. Undisclosed AI code is
  invisible to it; say so when reporting numbers.

## JSON

`aiblame stats --json` prints one object (`schema_version` 1) with:
`headline_ai_share` (percent), `metric` ("lines" | "churn"), `rev`, `rev_hash`,
`base_rev` (range mode), `commits` / `churn` / `lines` (each
`{human, assisted, agent, bot, total, ai, ai_share}`), `survival`
(`{ai, human, all}` percent), `agents[]` (`name, vendor, commits, churn, lines,
survival, models`), `authors[]`, `conventions[]` (`co-authored-by`,
`assisted-by`, `claude-session`, `entire-checkpoint`, `git-ai-notes`,
`agent-author`, …), `provenance[]` (sidecar sources and commit counts),
`line_level_commits`, `unrecognised_tools[]`, `dirs[]`, `files[]`, `months[]`
(`YYYY-MM`), `warnings[]`.

`aiblame check --json` prints `{pass, policy, metric, ai_share, results[]
{rule, pass, detail, commits[]}}`.

## Disclosing your own commits

Projects increasingly require disclosure, and they disagree on the form:

```
Assisted-by: LLM                                    # Linux kernel 7.3+ (no tool or model names)
Assisted-by: Claude Code:claude-opus-5              # Zephyr, kernel 7.0
Assisted-by: Claude Code (claude-opus-5)            # Mesa, Calcite, LLVM, Fedora
Generated-by: Claude Code (model: claude-opus-5)    # ASF, Mesa (almost all generated)
Co-authored-by: Claude <noreply@anthropic.com>      # GitHub style (Claude Code default)
```

Some projects forbid trailers: Kubernetes wants the disclosure in the PR
description and rejects AI co-authors, `Assisted-by` and `Co-developed-by`;
Mesa, pip and attrs reject `Co-authored-by` for tools. Run
`aiblame check --list-policies` or read the project's CONTRIBUTING before
choosing. An AI must never add `Signed-off-by`.

`aiblame hook install` writes a `prepare-commit-msg` hook that adds the
trailer automatically when the commit is made from an agent session
(detected via `CLAUDECODE`, `CODEX_SANDBOX*`, `CURSOR_AGENT`, `GEMINI_CLI`,
`OPENCODE`, or `AIBLAME_AGENT`). Set `AIBLAME_MODEL` to include the model.
Pass `--style` to match the project's policy.

## Config

Optional `.aiblame.toml` at the repo root (`aiblame init` writes a commented
template): extra `[[detect.agents]]`, `detect.provenance`, `paths.exclude`,
`[check]` policy and rules, `[badge]` label/colour.
