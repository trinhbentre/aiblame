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

  AI-written  ███████████████████░  97.5%  of 381,474 surviving lines
  371,948 AI (371,948 assisted + 0 agent-authored) · 9,526 human · 0 bot

                     total               AI  assisted  agent   human  bot
  Commits              374      344 (92.0%)       344      0      30    0
  Lines added      581,747  570,588 (98.1%)   570,588      0  11,159    0
  Surviving lines  381,474  371,948 (97.5%)   371,948      0   9,526    0
  Still in the tree: 65.2% of AI lines added, 85.4% of human lines added

Agents
                  lines  share  commits  survival  models
  Claude Code   371,660  97.4%      339     65.2%  Fable 5, Fable 5.1, Haiku 4.5, Opus 4…
  Cursor         10,316   2.7%        7     92.4%  Fable 5, Opus 4.8, Opus 5
  Hermes Agent    1,618   0.4%        1    100.0%  Opus 4.7

Disclosure conventions
  co-authored-by  344 commits
  claude-session  1 commits
```

Coding agents already sign their work: Claude Code, Codex, Cursor, Copilot and
aider add `Co-authored-by` trailers; the Linux kernel, Mesa, Zephyr, LLVM and
Fedora require `Assisted-by`; Devin, Jules and the Copilot coding agent commit
under their own bot identities; Claude Code, Amp and Entire link commits to
their sessions; git-ai and Entire write line-level attribution into git refs.
**aiblame reads all of it** and turns it into a number you can put in a
README, a CI gate, or a PR comment.

- **Zero setup.** No database, no editor hooks, no daemon, no token. One
  static binary that shells out to `git`. Works on a repo you cloned five
  seconds ago, and on history written years before you installed it.
- **Every convention.** `Co-authored-by`, `Assisted-by`, `Generated-by`,
  `Made-with`, `AI-used-for`, `Claude-Session`, `Agent-Logs-Url`,
  `Amp-Thread-ID`, `Entire-Checkpoint`, 60+ agent identities, message
  markers like `🤖 Generated with [Claude Code]` — plus the sidecar data that
  hook-based tools leave behind: git-ai authorship logs (`refs/notes/ai`),
  Entire checkpoints, Exceeds Ink and Claudit notes. Non-AI bots (dependabot,
  renovate, github-actions, SDK generators) are kept out of the way.
- **Honest.** aiblame counts *disclosed* AI involvement and says so. No style
  heuristics, no guessing, no false positives by design: `Assisted-by: Jay
  Satiro` (curl's way of crediting a human) is not AI, `Generated-By:
  StageFreight` (a release bot) is not AI, and `Assisted-by: none` means none.
- **Everything is a command.** Badge, Markdown, JSON, per-line blame, policy
  presets for the kernel / Mesa / LLVM / ASF / Kubernetes rules, a PR range
  mode, a disclosure hook, a GitHub Action and an Agent Skill.

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
aiblame                              # the repo you are in
aiblame stats ../other-repo          # any path
aiblame stats openai/codex           # any GitHub repo (cloned into your cache)
aiblame stats codeberg.org/forgejo/forgejo   # GitLab, Codeberg, Gitea: host/owner/repo
aiblame --since "6 months ago"       # recent history only
aiblame --rev origin/main..HEAD      # only this branch / pull request
aiblame --no-blame                   # instant answer on huge repos (lines added instead of blame)
aiblame --json | jq .headline_ai_share
```

## Measured, not claimed

aiblame on repositories whose authors publicly said they were built with AI,
and on two repositories that carry machine-written attribution (measured
2026-09-06 at the default branch):

| Repository | Author's claim | aiblame (disclosed AI) | Still in the tree |
|---|---|---|---|
| [anthropics/claudes-c-compiler](https://github.com/anthropics/claudes-c-compiler) | "100% of the code … written by Claude Opus 4.6" | **99.9%** — 3,974 of 3,976 commits authored by Claude | 46.7% of AI lines added |
| [garrytan/gstack](https://github.com/garrytan/gstack) | "AI wrote most of it" | **97.5%** of surviving lines, all via `Co-authored-by` | 65.2% AI · 85.4% human |
| [entireio/cli](https://github.com/entireio/cli) | built with Entire's own agent workflow | **86.0%** — 4,545 commits carry `Entire-Checkpoint`, 3,089 a Claude co-author, 120 `Assisted-by` | 65.7% AI · 52.7% human |
| [git-ai-project/git-ai](https://github.com/git-ai-project/git-ai) | tracked line by line with git-ai | **69.7%** — 11,252 authorship logs read from `refs/notes/ai` and `refs/ai/authorship`; 9,443 commits attributed per line | 54.5% AI · 51.4% human |
| [steveyegge/beads](https://github.com/steveyegge/beads) | "100% vibe coded" | **58.4%** of surviving lines: 4,172 `Co-authored-by`, 569 `Amp-Thread-ID`, 55 `Claude-Session` commits | 51.4% AI · 51.3% human |
| [cloudflare/workers-oauth-provider](https://github.com/cloudflare/workers-oauth-provider) | "largely written with the help of Claude" | **7.4%** — the 2025 core was prompted in commit *messages*, not trailers | 82.6% AI · 76.3% human |
| [mitsuhiko/sloppy-xml-py](https://github.com/mitsuhiko/sloppy-xml-py) | "100% AI generated with Claude Code" | **0%** — no disclosure in the history | — |
| [karpathy/llm-council](https://github.com/karpathy/llm-council) | "99% vibe coded" | **0%** — no disclosure in the history | — |

The third column is the point. aiblame reports a **floor**: what the history
discloses. When a project's claim and its history disagree, you learn
something about the project's disclosure habits, and `aiblame hook install`
fixes it going forward. The last column is *survival*: the share of lines ever
added that is still in the tree, by kind — the measure Rahman & Shihab (EASE
2026) and GitClear use to ask whether AI code lasts.

## Commands

| Command | What it does |
|---|---|
| `aiblame [stats] [PATH\|URL\|owner/repo\|host/owner/repo]` | Full report: commits, lines added and surviving lines by human / AI-assisted / agent-authored / bot; survival; per-agent, per-contributor, per-directory, per-file and per-month tables. `-f json` / `-f md`. |
| `aiblame blame FILE` | `git blame` with a `HUMAN` / `ASSIST` / `AGENT` / `BOT` tag and the agent on every line; line-accurate when a git-ai authorship log exists. |
| `aiblame log [--ai\|--human\|--bot] [--agent NAME] [--evidence]` | Commits with their classification, *why*, and what was deliberately not counted. |
| `aiblame badge [--out ai.svg] [--shields ai.json]` | shields.io-style SVG, or an endpoint JSON for `img.shields.io/endpoint`. |
| `aiblame check [--policy NAME] [--max PCT] [--min PCT] [--forbid-agent-authored] [--forbid-agent-signoff] [--require-trailer a,b] [--forbid-trailer c]` | Policy gate for CI; exit 1 on violation. |
| `aiblame hook install\|uninstall\|status\|print\|detect [--style S]` | `prepare-commit-msg` hook that adds a trailer when you commit from an agent session. |
| `aiblame agents` | The identity tables, trailer key classes and sidecar sources. |
| `aiblame init` | Write a commented `.aiblame.toml`. |

Run `aiblame help` for every flag. Exit codes: `0` ok, `1` check failed,
`2` usage, `3` error.

## What counts as AI

Each commit gets one kind. A **line** inherits the kind of the commit that
last touched it (`git blame`) — or its own kind, when a git-ai authorship log
says which lines the agent wrote.

| Kind | Rule | AI? |
|---|---|---|
| `agent` | the commit author is an AI identity (`devin-ai-integration[bot]`, `copilot-swe-agent[bot]`, `Claude <noreply@anthropic.com>`, `Codex Test <codex@example.com>`, `… (aider)`) | ✅ |
| `assisted` | a human author plus: a `Co-authored-by` naming an agent; an `Assisted-by` / `Generated-by` / `AI-used-for` / `Coding-Agent` trailer; a session link (`Claude-Session`, `Agent-Logs-Url`, `Amp-Thread-ID`, `Entire-Checkpoint`); a git-ai / Entire record; or a `Generated with …` marker | ✅ |
| `bot` | dependabot, renovate, github-actions, pre-commit.ci, Stainless, … | reported, excluded from the denominator |
| `human` | everything else — including `Assisted-by: <a person's name>` and `Generated-by: <a tool aiblame does not know as AI>` | ❌ |

**AI share = AI ÷ (AI + human).** Lockfiles, `vendor/`, `node_modules/`,
minified and generated files are excluded by default;
`.git-blame-ignore-revs` is honoured. Full details, edge cases and
limitations: [docs/METHODOLOGY.md](docs/METHODOLOGY.md). Every identity,
trailer key and sidecar format with the source it came from:
[docs/DETECTION.md](docs/DETECTION.md).

## Reads what other tools write

If your team uses [git-ai](https://github.com/git-ai-project/git-ai),
[Entire](https://github.com/entireio/cli), Exceeds Ink or Claudit, aiblame
reads their data with nothing installed:

- **git-ai** authorship logs in `refs/notes/ai` (and the older
  `refs/ai/authorship/<sha>` blobs) give **line-level** attribution: only the
  lines the agent wrote count as AI, human edits inside an AI commit stay
  human.
- **Entire** `Entire-Checkpoint: <id>` trailers are resolved through
  `refs/entire/checkpoints/*` to the agent and model of the session.
- Coverage is reported as `sidecar data: git-ai-notes 1,706 · 9,443 commits
  attributed line by line`, and `aiblame log --evidence` shows the record
  behind each commit.

Repositories aiblame clones itself fetch these refs automatically. For a local
checkout run `git fetch origin '+refs/notes/*:refs/notes/*'
'+refs/ai/*:refs/ai/*' '+refs/entire/*:refs/entire/*'` once (aiblame prints
the hint when checkpoints are referenced but missing). `--no-provenance` turns
this off.

## Pull requests

```sh
aiblame stats --rev origin/main..HEAD          # AI share of this branch only
aiblame check --rev origin/main..HEAD --policy kernel
```

Range mode counts only the range's commits and only the surviving lines they
introduced, so the number answers "how much of *this change* is AI". In the
GitHub Action set `pr-range: true` and, optionally, `pr-comment: true` to post
the report on the pull request.

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
permissions:
  contents: read
  pull-requests: write          # only for pr-comment
steps:
  - uses: actions/checkout@v4
    with:
      fetch-depth: 0            # aiblame needs the full history
  - uses: trinhbentre/aiblame@v1
    with:
      badge-path: .github/badges/ai.json   # optional
      check-args: --policy kernel          # optional policy gate
      pr-range: true                       # analyse only the PR's commits
      pr-comment: true                     # post the report on the PR
```

The action prints the Markdown report to the job summary and exposes
`steps.<id>.outputs.ai-share`. Pair `badge-path` with any "commit changed
files" step to keep the badge current.

## Policy checks

Open source projects have written down what they want; aiblame ships their
rules as presets (`aiblame check --list-policies` shows the sources):

```sh
aiblame check --policy kernel        # Assisted-by required; no AI co-author, author or sign-off
aiblame check --policy mesa          # Assisted-by or Generated-by; Co-authored-by is for people
aiblame check --policy kubernetes    # no AI trailers at all; disclosure goes in the PR
aiblame check --policy asf           # Generated-by

# or compose your own
aiblame check --since 2026-01-01 --require-trailer assisted-by,generated-by --forbid-trailer co-authored-by --forbid-agent-signoff
aiblame check --max 60               # keep AI below a ceiling, by surviving lines (default) or --metric commits
```

Defaults can live in `.aiblame.toml` so CI just runs `aiblame check`. When
the gate must hold against untrusted pull requests, pass the rules as flags
(`check-args: --policy kernel`) rather than trusting the `.aiblame.toml` of
the branch being checked, which the PR itself could edit.

## Disclose your own commits

Half of the repositories in the table above have no idea how much AI they
contain because nobody wrote it down. Fix that in one command:

```sh
aiblame hook install                    # Assisted-by: Claude Code (model)
aiblame hook install --style kernel     # Assisted-by: LLM            (Linux 7.3+)
aiblame hook install --style agent-model    # Assisted-by: Claude Code:model (Zephyr)
aiblame hook install --style co-authored-by
```

The hook is a ~50-line POSIX shell script (`aiblame hook print`). It fires
only when the commit is made from inside an agent session, detected via the
variables agents set for their subprocesses: `CLAUDECODE` (Claude Code),
`CODEX_SANDBOX*` (Codex), `CURSOR_AGENT` (Cursor), `GEMINI_CLI`, `OPENCODE`.
Set `AIBLAME_MODEL` to record the model; `AIBLAME_AGENT` to override;
`AIBLAME_DISABLE=1` to turn it off. It never adds a second trailer when the
agent already disclosed itself, including via a `Claude-Session` or
`Amp-Thread-ID` line.

## Configuration

`aiblame init` writes a commented `.aiblame.toml`:

```toml
[paths]
exclude = ["docs/generated/**"]

[detect]
provenance = true           # read git-ai / Entire / Exceeds / Claudit data

[[detect.agents]]           # your in-house agent, or a tool from the
name = "Acme Copilot"       # "unrecognised tools" list in the report
emails = ["acme-agent@acme.example"]

[check]
policy = "mesa"
forbid_agent_signoff = true

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

| | aiblame | [git-ai](https://github.com/git-ai-project/git-ai) | [Entire CLI](https://github.com/entireio/cli) | [agentblame](https://github.com/mesa-dot-dev/agentblame) | `git shortlog` |
|---|---|---|---|---|---|
| Works on any existing repo, retroactively | ✅ | ❌ only commits made after install | ❌ | ❌ | ✅ |
| Setup | none | daemon + per-agent integration | per-repo `entire enable` + hooks | Bun, per-repo `init`, editor hooks, token | none |
| Attribution granularity | commit; **line** when git-ai / Exceeds notes exist | line (own checkpoints) | session per commit | line (editor hooks) | commit |
| Conventions understood | Co-authored-by, Assisted-by, Generated-by, session links, agent identities, markers, git-ai notes, Entire checkpoints | its own notes | its own checkpoints | its own notes | none |
| Policy gate / badge / Action / PR mode | ✅ | CI re-attribution scripts | ❌ | GitHub Action for notes | ❌ |

git-ai and Entire give richer data if your whole team installs them first;
aiblame reads what they wrote and gives you a defensible number for the repo
you have today.

## Limitations

- **Undisclosed AI use is invisible.** aiblame is not a detector of AI-looking
  code; it measures what the history says. See the table above.
- **Granularity is the commit** unless a git-ai authorship log exists. One AI
  line in a 500-line commit makes 501 AI-assisted lines. This is inherent to
  every trailer scheme.
- **Trailers can be forged or squashed away.** GitHub squash merges keep
  `Co-authored-by`; check your platform for `Assisted-by`.
- **Sidecar data must be pushed.** Checkpoints a tool pruned or never pushed
  cannot be read; the report says how many were referenced but missing.
- **Blame is O(files).** ~1,500 files take about ten seconds; use `--no-blame`
  for an instant answer or `-j` to tune parallelism.

## Built with AI

This repository was written with Claude Code (Claude Fable 5.1), including
this sentence. Every commit carries the trailer; run `aiblame` on it.

## License

[MIT](LICENSE).
