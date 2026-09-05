<h1 align="center">aiblame</h1>

<p align="center"><strong>How much of this repo did AI write?</strong><br>
Zero-setup AI authorship statistics for any git repository.</p>

<p align="center">
  <a href="https://github.com/trinhbentre/aiblame/actions/workflows/ci.yml"><img alt="CI" src="https://github.com/trinhbentre/aiblame/actions/workflows/ci.yml/badge.svg"></a>
  <a href="https://github.com/trinhbentre/aiblame/releases/latest"><img alt="Release" src="https://img.shields.io/github/v/release/trinhbentre/aiblame?color=7c3aed"></a>
  <a href="https://pkg.go.dev/github.com/trinhbentre/aiblame"><img alt="Go Reference" src="https://pkg.go.dev/badge/github.com/trinhbentre/aiblame.svg"></a>
  <a href="LICENSE"><img alt="MIT" src="https://img.shields.io/badge/license-MIT-blue.svg"></a>
  <a href="https://github.com/trinhbentre/aiblame/blob/main/.github/badges/ai.json"><img alt="AI-written" src="https://img.shields.io/endpoint?url=https://raw.githubusercontent.com/trinhbentre/aiblame/main/.github/badges/ai.json"></a>
</p>

```
$ aiblame stats garrytan/gstack

  AI-written  ███████████████████░  97.5%  of 375,718 surviving lines
  366,192 AI (366,192 assisted + 0 agent-authored) · 9,526 human · 0 bot

                     total               AI  assisted  agent   human  bot
  Commits              373      343 (92.0%)       343      0      30    0
  Lines added      575,809  564,650 (98.1%)   564,650      0  11,159    0
  Surviving lines  375,718  366,192 (97.5%)   366,192      0   9,526    0

Agents
                  lines  share  commits  models
  Claude Code   365,904  97.4%      338  Opus 4.6, Opus 4.7, Opus 4.8, Opus 5, …
  Cursor         10,316   2.7%        7  Opus 4.8, Opus 5, …
  Hermes Agent    1,618   0.4%        1

Disclosure conventions
  co-authored-by  343 commits
```

Coding agents already sign their work: Claude Code, Codex, Cursor, Copilot and
aider add `Co-authored-by` trailers; the Linux kernel, Mesa, Zephyr and Fedora
require `Assisted-by`; Devin, Jules and the Copilot coding agent commit under
their own bot identities. **aiblame reads all of it** and turns it into a
number you can put in a README, a CI gate, or a PR comment.

- **Zero setup.** No database, no editor hooks, no daemon, no token. One
  static binary that shells out to `git`. Works on a repo you cloned five
  seconds ago, and on history written years before you installed it.
- **Every convention.** `Co-authored-by`, `Assisted-by`, `Generated-by`,
  `Coding-Agent`/`Model`, `AI-assistant`, 50+ agent identities, message
  markers like `🤖 Generated with [Claude Code]`. Non-AI bots (dependabot,
  renovate, github-actions) are kept out of the way.
- **Honest.** aiblame counts *disclosed* AI involvement and says so. No style
  heuristics, no guessing, no false positives by design.
- **Everything is a command.** Badge, Markdown, JSON, per-line blame, policy
  checks, a disclosure hook, a GitHub Action and an Agent Skill.

## Install

```sh
# macOS / Linux
curl -fsSL https://raw.githubusercontent.com/trinhbentre/aiblame/main/install.sh | sh

# Go 1.25+
go install github.com/trinhbentre/aiblame/cmd/aiblame@latest

# or grab a binary from the releases page (darwin/linux/windows, amd64/arm64)
```

Requires `git` 2.23+ on `PATH`.

## Quick start

```sh
aiblame                         # the repo you are in
aiblame stats ../other-repo     # any path
aiblame stats openai/codex      # any GitHub repo (cloned into your cache)
aiblame --since "6 months ago"  # recent history only
aiblame --no-blame              # instant answer on huge repos (lines added instead of blame)
aiblame --json | jq .headline_ai_share
```

## Measured, not claimed

aiblame on six repositories whose authors publicly said they were built with
AI (measured 2026-09-06 at the default branch):

| Repository | Author's claim | aiblame (disclosed AI) |
|---|---|---|
| [anthropics/claudes-c-compiler](https://github.com/anthropics/claudes-c-compiler) | "100% of the code … written by Claude Opus 4.6" | **100.0%** — 3,974 of 3,976 commits authored by Claude |
| [garrytan/gstack](https://github.com/garrytan/gstack) | "AI wrote most of it" | **97.5%** of surviving lines, all via `Co-authored-by` |
| [steveyegge/beads](https://github.com/steveyegge/beads) | "100% vibe coded" | **58.2%** of lines added carry a trailer (Claude Code, Amp, Copilot, Cursor, Codex) |
| [cloudflare/workers-oauth-provider](https://github.com/cloudflare/workers-oauth-provider) | "largely written with the help of Claude" | **7.4%** — the 2025 core was prompted in commit *messages*, not trailers |
| [mitsuhiko/sloppy-xml-py](https://github.com/mitsuhiko/sloppy-xml-py) | "100% AI generated with Claude Code" | **0%** — no disclosure in the history |
| [karpathy/llm-council](https://github.com/karpathy/llm-council) | "99% vibe coded" | **0%** — no disclosure in the history |

That last column is the point. aiblame reports a **floor**: what the history
discloses. When a project's claim and its history disagree, you learn
something about the project's disclosure habits, and `aiblame hook install`
fixes it going forward.

## Commands

| Command | What it does |
|---|---|
| `aiblame [stats] [PATH\|URL\|owner/repo]` | Full report: commits, lines added and surviving lines by human / AI-assisted / agent-authored / bot; per-agent, per-contributor, per-directory, per-file and per-month tables. `-f json` / `-f md`. |
| `aiblame blame FILE` | `git blame` with a `HUMAN` / `ASSIST` / `AGENT` / `BOT` tag and the agent on every line. |
| `aiblame log [--ai\|--human\|--bot] [--agent NAME] [--evidence]` | Commits with their classification and *why*. |
| `aiblame badge [--out ai.svg] [--shields ai.json]` | shields.io-style SVG, or an endpoint JSON for `img.shields.io/endpoint`. |
| `aiblame check [--max PCT] [--min PCT] [--forbid-agent-authored] [--require-trailer assisted-by]` | Policy gate for CI; exit 1 on violation. |
| `aiblame hook install\|uninstall\|status\|print\|detect` | `prepare-commit-msg` hook that adds a trailer when you commit from an agent session. |
| `aiblame agents` | The identity tables. |
| `aiblame init` | Write a commented `.aiblame.toml`. |

Run `aiblame help` for every flag. Exit codes: `0` ok, `1` check failed,
`2` usage, `3` error.

## What counts as AI

Each commit gets one kind. A **line** inherits the kind of the commit that
last touched it (`git blame`).

| Kind | Rule | AI? |
|---|---|---|
| `agent` | the commit author is an AI identity (`devin-ai-integration[bot]`, `copilot-swe-agent[bot]`, `Claude <noreply@anthropic.com>`, `… (aider)`) | ✅ |
| `assisted` | a human author plus a `Co-authored-by` naming an agent, any `Assisted-by` / `Generated-by` / `Coding-Agent` trailer, or a `Generated with …` marker | ✅ |
| `bot` | dependabot, renovate, github-actions, pre-commit.ci, … | reported, excluded from the denominator |
| `human` | everything else | ❌ |

**AI share = AI ÷ (AI + human).** Lockfiles, `vendor/`, `node_modules/`,
minified and generated files are excluded by default;
`.git-blame-ignore-revs` is honoured. Full details, edge cases and
limitations: [docs/METHODOLOGY.md](docs/METHODOLOGY.md). Every identity and
the source it came from: [docs/DETECTION.md](docs/DETECTION.md).

## Badge

```sh
aiblame badge --shields .github/badges/ai.json   # commit this file
```

```markdown
![AI-written](https://img.shields.io/endpoint?url=https://raw.githubusercontent.com/OWNER/REPO/main/.github/badges/ai.json)
```

Or skip shields and commit the SVG: `aiblame badge --out .github/badges/ai.svg`.
`--color auto` scales the colour with the percentage; `--metric commits`
switches the number.

## GitHub Action

```yaml
- uses: actions/checkout@v4
  with:
    fetch-depth: 0            # aiblame needs the full history
- uses: trinhbentre/aiblame@v1
  with:
    badge-path: .github/badges/ai.json          # optional
    check-args: --require-trailer co-authored-by # optional policy gate
```

The action prints the Markdown report to the job summary and exposes
`steps.<id>.outputs.ai-share`. Pair `badge-path` with any "commit changed
files" step to keep the badge current.

## Policy checks

```sh
# Kernel/Mesa style: humans sign off, AI is disclosed with Assisted-by
aiblame check --since 2026-01-01 --require-trailer assisted-by --forbid-agent-authored

# Keep AI below a ceiling, by surviving lines (default) or commits
aiblame check --max 60
aiblame check --max 30 --metric commits
```

Defaults can live in `.aiblame.toml` so CI just runs `aiblame check`.

## Disclose your own commits

Half of the repositories in the table above have no idea how much AI they
contain because nobody wrote it down. Fix that in one command:

```sh
aiblame hook install                    # Assisted-by: Claude Code
aiblame hook install --style co-authored-by
```

The hook is a 40-line POSIX shell script (`aiblame hook print`). It fires
only when the commit is made from inside an agent session, detected via the
variables agents set for their subprocesses: `CLAUDECODE` (Claude Code),
`CODEX_SANDBOX*` (Codex), `CURSOR_AGENT` (Cursor), `GEMINI_CLI`, `OPENCODE`.
Set `AIBLAME_MODEL` to record the model; `AIBLAME_AGENT` to override;
`AIBLAME_DISABLE=1` to turn it off. It never adds a second trailer when the
agent already added one.

## Configuration

`aiblame init` writes a commented `.aiblame.toml`:

```toml
[paths]
exclude = ["docs/generated/**"]

[[detect.agents]]           # your in-house agent
name = "Acme Copilot"
emails = ["acme-agent@acme.example"]

[check]
require_trailer = "assisted-by"

[badge]
label = "AI-written"
color = "auto"
```

## For coding agents

[`skills/aiblame/SKILL.md`](skills/aiblame/SKILL.md) is an
[Agent Skill](https://agentskills.io) that teaches Claude Code, Codex and
friends when and how to run aiblame, how to read the JSON, and how to
disclose their own commits. Drop it into your skills directory or point your
agent at this repository.

## How it compares

| | aiblame | [agentblame](https://github.com/mesa-dot-dev/agentblame) | `git shortlog` |
|---|---|---|---|
| Works on any existing repo, retroactively | ✅ | ❌ hooks must be installed before the commits | ✅ |
| Setup | none | Bun runtime, per-repo `init`, editor hooks, local DB, browser extension + token | none |
| Attribution granularity | commit (trailer-based) | line + prompt (editor hooks + git notes) | commit |
| Conventions understood | Co-authored-by, Assisted-by, Generated-by, agent identities, markers | its own notes | none |
| Policy gate / badge / Action | ✅ | ❌ | ❌ |

agentblame gives finer data if your whole team installs it first. aiblame
gives you a defensible number for the repo you have today.

## Limitations

- **Undisclosed AI use is invisible.** aiblame is not a detector of AI-looking
  code; it measures what the history says. See the table above.
- **Granularity is the commit.** One AI line in a 500-line commit makes 501
  AI-assisted lines. This is inherent to every trailer scheme.
- **Trailers can be forged or squashed away.** GitHub squash merges keep
  `Co-authored-by`; check your platform for `Assisted-by`.
- **Blame is O(files).** ~1,500 files take a few seconds; use `--no-blame`
  for an instant answer or `-j` to tune parallelism.

## Built with AI

This repository was written with Claude Code (Claude Fable 5.1), including
this sentence. Every commit carries the trailer; run `aiblame` on it.

## License

[MIT](LICENSE).
