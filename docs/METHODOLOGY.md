# Methodology

aiblame measures **disclosed** AI involvement. It never guesses from code
style. If a commit does not say an agent was involved, aiblame counts it as
human. That makes the numbers reproducible and auditable, and it means the
true AI share of a repository is *at least* what aiblame reports.

## Classification of a commit

Each commit gets exactly one **kind**:

| Kind | Rule | Counts as AI |
|---|---|---|
| `agent` | The commit **author** (name or e-mail) is a known AI identity, e.g. `devin-ai-integration[bot]`, `copilot-swe-agent[bot]`, `Claude <noreply@anthropic.com>`, or a name ending in `(aider)`. | yes |
| `assisted` | A human author plus AI evidence in the message: a `Co-authored-by` trailer naming a known agent, any `Assisted-by` / `Generated-by` / `Coding-Agent` / `AI-assistant` (…) trailer, or a free-text marker such as `🤖 Generated with [Claude Code]`. | yes |
| `bot` | The author is known non-AI automation: dependabot, renovate, github-actions, pre-commit.ci, release-please, weblate, … | no (reported separately) |
| `human` | Everything else. | no |

Evidence order: author identity → committer identity (opt-in) → trailers →
message markers. Markers are only consulted when no structured evidence
exists, and the built-in marker list is deliberately conservative (it will
not match "Document how to use Claude in CI"). A human co-author named
"Ai Chen" is not an agent; the generic `ai`/`llm`/`agent` name rule only
applies to noreply-style addresses.

The exact identity tables are in [DETECTION.md](DETECTION.md) and printed by
`aiblame agents`.

## Metrics

| Metric | Definition | Source |
|---|---|---|
| **Surviving lines** (default headline) | For every tracked text file at the analysed revision, `git blame --porcelain` attributes each line to the commit that last changed it; the line inherits that commit's kind. | `git blame` per file, in parallel |
| **Lines added** ("churn") | Sum of `--numstat` insertions per commit, by kind. Cheap; used as the headline with `--no-blame`. | `git log --numstat -M -C` |
| **Commits** | Count of non-merge commits by kind (`--include-merges` to include merges). | `git log` |

**AI share** = `AI / (AI + human)` where `AI = assisted + agent`. Bot lines
are excluded from the denominator because they are authored by neither a
person nor a coding agent; they are still shown.

Per-agent lines use the same blame data: a line written in a commit
co-authored by two agents is credited to both, so agent rows can sum to more
than the AI total.

Per-contributor rows list human authors only. Their "AI share" is the share
of their surviving lines (or churn) that came from AI-assisted commits.

The month timeline uses lines added (churn), because blame has no time axis.

## What is counted

- Files present at the analysed revision (`--rev`, default `HEAD`).
- Text blobs ≤ 1 MiB (`--max-file-size`). Binary files (as reported by git or
  by extension) are skipped.
- Everything except the default excludes: lockfiles (`package-lock.json`,
  `go.sum`, `Cargo.lock`, …), `vendor/`, `node_modules/`, build output,
  minified bundles, protobuf/codegen outputs, snapshots, data files. See
  `stats.DefaultExcludes`. Disable with `--no-default-excludes`; extend with
  `--exclude` / `--include` or `.aiblame.toml`.
- `.git-blame-ignore-revs` is honoured automatically (mass reformatting
  commits are attributed to the previous author), unless `--no-ignore-revs`.
- `--since` / `--until` filter **commit and churn** metrics and the timeline.
  Blame is a snapshot of the tree and is not windowed.

## Known limitations

- **Undisclosed AI use is invisible.** Tab-completion, copy-paste from a chat,
  or agents configured to drop their trailer leave no trace. aiblame reports a
  floor, not the truth. `aiblame hook install` raises the floor for your own
  commits.
- **Squash merges** keep `Co-authored-by` trailers only if the platform
  preserves them (GitHub does by default; the squash commit body lists
  co-authors). `Assisted-by` trailers survive only if the squash message
  keeps the body. Rebases preserve messages.
- **Trailer spoofing.** Anyone can write a trailer. aiblame measures what the
  history claims; it is not a forensic tool.
- **Attribution granularity** is the commit. A commit with one AI-written
  line and 500 human lines counts as 501 AI-assisted lines. Smaller commits
  give better numbers; this is a property of every trailer-based scheme,
  including the Linux kernel's.
- **Agent-authored vs assisted** depends on how the agent commits. Claude
  Code commits as the human and adds a trailer (assisted); Devin commits as
  itself (agent). Both count as AI; the split is informational.
- Blame on very large repositories takes time (one `git blame` per file).
  Use `--no-blame` for a quick answer or `-j` to tune parallelism.
