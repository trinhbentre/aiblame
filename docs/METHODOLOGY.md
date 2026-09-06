# Methodology

aiblame measures **disclosed** AI involvement. It never guesses from code
style. If nothing in the repository says an agent was involved, aiblame counts
the commit as human. That makes the numbers reproducible and auditable, and it
means the true AI share of a repository is *at least* what aiblame reports.

"Disclosed" covers two kinds of evidence:

- what people and agents write into commits: author identities, trailers
  (`Co-authored-by`, `Assisted-by`, `Generated-by`, `Claude-Session`, …) and
  free-text markers;
- what hook-based tools leave next to commits ("sidecar data"): git-ai
  authorship logs in `refs/notes/ai`, Entire checkpoints in
  `refs/entire/checkpoints/*`, Exceeds Ink and Claudit notes.

Both are read with plain `git`; nothing needs to be installed in the
repository.

## Classification of a commit

Each commit gets exactly one **kind**:

| Kind | Rule | Counts as AI |
|---|---|---|
| `agent` | The commit **author** (name or e-mail) is a known AI identity, e.g. `devin-ai-integration[bot]`, `copilot-swe-agent[bot]`, `Claude <noreply@anthropic.com>`, `Codex Test <codex@example.com>`, or a name ending in `(aider)`. | yes |
| `assisted` | A human author plus AI evidence: a `Co-authored-by` trailer naming a known agent, a disclosure trailer (`Assisted-by`, `Generated-by`, `AI-used-for`, …), a session-link trailer (`Claude-Session`, `Entire-Checkpoint`, `Agent-Logs-Url`, …), a git-ai / Entire sidecar record, or a free-text marker such as `🤖 Generated with [Claude Code]`. | yes |
| `bot` | The author is known non-AI automation: dependabot, renovate, github-actions, pre-commit.ci, release-please, Stainless, StageFreight, … | no (reported separately) |
| `human` | Everything else. | no |

Evidence order: author identity → committer identity (opt-in) → trailers →
sidecar data → message markers. Markers are only consulted when nothing
structured was found, and the built-in marker list is deliberately
conservative (it will not match "Document how to use Claude in CI"). A human
co-author named "Ai Chen" is not an agent; the generic `ai`/`llm`/`agent` name
rule only applies to noreply-style addresses; name *patterns* such as
`^gemini…` only apply when the e-mail is a noreply or agent address, so a
person called Gemini with her own mailbox stays human.

### Trailer keys come in three strengths

Real-world commit corpora (see [DETECTION.md](DETECTION.md)) show that the same
trailer key means different things in different projects, so aiblame treats
keys differently:

**Strong keys** (`Assisted-by`, `AI-assisted`, `AI-assisted-by`, `AI-agent`,
`AI-assistant`, `AI-generated`, `AI-used-for`, `LLM-assisted-by`,
`Coding-Agent`, `Commit-generated-by`, …) exist only to disclose AI. Their
value is AI evidence even when the tool is unknown (`Assisted-by: devx/…`
becomes agent "devx"), **except** when:

- the value is a negative — `Assisted-by: none`, `n/a`, `no`;
- the value names a **person**. curl uses `Assisted-by:` for the human who
  helped with a change (279 commits by September 2026: `Assisted-by: Jay
  Satiro`, `Assisted-by: riastradh on github`,
  `Assisted-by: tholin@users.noreply.github.com`). A value is treated as a
  person when it is two to five alphabetic words with no digits, colons,
  slashes or parentheses and no AI vocabulary, or a personal e-mail address
  (GitHub's `login@users.noreply.github.com` addresses are people unless the
  login ends in `[bot]`). Known agents ("Claude Code", "Amazon Q") are checked
  first, so a two-word tool name in the identity table is never mistaken for a
  person;
- for `AI-used-for` (QEMU), the value only mentions research, review or
  translation — the code itself was not AI-written.

**Weak keys** (`Generated-by`, `Generated-with`, `Made-with`, `Agent`) are
also used by non-AI tooling: SDK generators, release bots (`Generated-By:
StageFreight`), "Made-with: love". Their value counts only when it resolves to
a known agent (`Generated-by: Codex`, `Generated-by: Maka`, `Made-with:
Cursor`) or is a short tool-like value that uses AI/model vocabulary
(`Generated-by: Acme LLM bot`). Anything else is listed under **unrecognised
tools** in the report so the repository owner can add it to `.aiblame.toml`
if it is an agent.

**Session-link keys** (`Claude-Session`, `Claude-Session-Path`,
`Claude-Sessions-Id`, `Turbocommit-Session`, `Agent-Logs-Url`,
`Amp-Thread-ID`, `Entire-Checkpoint`, `Agent-Conversation`,
`Agent-Transcript`) carry a URL or an id, not a tool name. Their presence is
AI evidence; the agent comes from the key (`Claude-Session` → Claude Code,
`Agent-Logs-Url` → GitHub Copilot, `Amp-Thread-ID` → Amp). `Entire-Checkpoint`
is resolved to the real agent and model from the checkpoint metadata when it
is available (see below); otherwise the agent is "Unknown AI". When the same
commit also names an agent (a `Co-authored-by: Claude …` next to an
`Entire-Checkpoint`), the unknown placeholder is dropped.

`Co-authored-by` / `Co-developed-by` are AI evidence only when the person is a
known agent; `Signed-off-by` naming an agent is AI evidence and is flagged
(`agent_signed_off`) because DCO-based projects forbid it. `Model:` is a
companion to `Coding-Agent:`. The subject line is never a trailer, so a
conventional-commit scope such as `agent: refactor hooks` is not evidence.

### Trailer values are normalised

The value after the key is parsed into a tool, an optional model and an
optional e-mail. Shapes seen in public history and handled:

```
Claude Opus 4.8 (1M context) <noreply@anthropic.com>   tool Claude, model Opus 4.8
Claude Code (claude-opus-5)                            Calcite / Mesa style
Claude:claude-fable-5-1 coccinelle sparse              kernel 7.0 / Zephyr style
LLM coccinelle sparse                                  kernel 7.3+ style (agent unknown)
Hermes Agent:gpt-5.6-sol                               tool with a space before the colon
openai-codex:gpt-5.5 [read,bash,edit,write]            bracketed tool list dropped
claude-code:fable-5.1#high (orchestrator, reviewer)    effort suffix and role note dropped
claude-code/claude-fable-5                             tool/model
devx/3066e945-4358-448d-b2e2-8ee7c94ce548              tool/session-id
GPT-5.6 Sol via Codex                                  model via tool
OpenCode GPT-5.6 Sol <noreply@opencode.ai>             tool with an embedded model
cursor-agent/0.42 (model: claude-sonnet-4-5; operator: a@b.c)
```

Bare model names (`Claude Opus 5`, `GPT-5.5`) are credited to the model's
family ("Claude Code", "GPT"). Placeholders (`<synthetic>`, `unknown`,
`undisclosed`) are dropped from the model list.

## Sidecar data (provenance)

When a repository contains attribution written by another tool, aiblame reads
it. Nothing is installed; the data is fetched like any other ref.

| Source | Where | What it gives |
|---|---|---|
| git-ai | `refs/notes/ai` (Authorship Log v3 text) and older `refs/ai/authorship/<sha>` JSON blobs (schema 0.0.1) | per file, the line ranges each agent session wrote and which lines a human wrote; agent tool and model |
| Exceeds Ink | `refs/notes/exceeds-ink` (same v3 format) | same |
| Entire CLI | `Entire-Checkpoint: <id>` trailer → `refs/entire/checkpoints/<shard>/<id>` or the `entire/checkpoints/v1` branch; `metadata.json` and per-session `N/metadata.json` | agent, model, files touched, agent/human line counts per session |
| Claudit | `refs/notes/claude-conversations` | the commit was produced in a Claude Code session (presence only) |

A commit covered by a sidecar record becomes `assisted` (the author stays
human) and the source is listed under **Disclosure conventions** as
`git-ai-notes`, `entire-checkpoint`, `exceeds-ink-notes` or
`claude-conversations-notes`. A git-ai record with an empty attestation
section (the tool looked and attributed nothing to an agent) leaves the
commit as it was.

**Line-level attribution.** git-ai logs say which lines of which file an
agent wrote. When such a log exists for a commit, `git blame` runs with
per-line detail and each surviving line is attributed on its own: a line in
an agent range is AI, a line in a human range is human, and a line the log
does not mention ("untracked") is human unless the commit has other AI
evidence (a trailer, an agent author), in which case the commit's kind
applies. Files the log does not mention are treated as untracked. The report
says how many commits were attributed this way (`line_level_commits`).

Entire checkpoints carry per-session line counts but no line ranges, so they
resolve the agent and model of the commit without changing its lines.

Notes and checkpoint refs are not fetched by a plain `git clone`. Repositories
aiblame clones itself (`aiblame stats owner/repo`) fetch them automatically;
for a local checkout run

```sh
git fetch origin '+refs/notes/*:refs/notes/*' '+refs/ai/*:refs/ai/*' '+refs/entire/*:refs/entire/*'
```

aiblame prints this hint when it sees `Entire-Checkpoint` trailers whose
checkpoints are missing. `--no-provenance` (or `detect.provenance = false`)
turns sidecar reading off.

## Metrics

| Metric | Definition | Source |
|---|---|---|
| **Surviving lines** (default headline) | For every tracked text file at the analysed revision, `git blame --porcelain` attributes each line to the commit that last changed it; the line inherits that commit's kind (or its own kind, with line-level sidecar data). | `git blame` per file, in parallel |
| **Lines added** ("churn") | Sum of `--numstat` insertions per commit, by kind. Cheap; used as the headline with `--no-blame`. | `git log --numstat -M -C` |
| **Commits** | Count of non-merge commits by kind (`--include-merges` to include merges). | `git log` |
| **Survival** | Surviving lines ÷ lines added, per kind and per agent: how much of what was written is still in the tree. Follows the line-survival measure of Rahman & Shihab (EASE 2026) and GitClear's churn studies. Capped at 100% (renames and `.git-blame-ignore-revs` can make blame credit more lines than numstat counted) and only computed without `--since`/`--until`, because churn is windowed and blame is not. | both of the above |

**AI share** = `AI / (AI + human)` where `AI = assisted + agent`. Bot lines
are excluded from the denominator because they are authored by neither a
person nor a coding agent; they are still shown.

Per-agent lines use the same blame data: a line written in a commit
co-authored by two agents is credited to both, so agent rows can sum to more
than the AI total.

Per-contributor rows list human authors only. Their "AI share" is the share
of their surviving lines (or churn) that came from AI-assisted commits.
`--no-authors` leaves the table out for reports shared outside the team.

The month timeline uses lines added (churn), because blame has no time axis.

## Range mode (pull requests)

`--rev BASE..HEAD` analyses one range the way a reviewer sees a pull request:

- commits and lines added come from `git log BASE..HEAD`;
- blame runs at `HEAD` but only on files the range touched, and lines that
  belong to commits outside the range are dropped from every count, so the
  headline is "of the lines this change introduced that survive, how many are
  AI";
- `aiblame check --rev origin/main..HEAD --policy kernel` gates only the new
  commits.

The report records the base as `base_rev`. `A..` means `A..HEAD`.

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
  agents configured to drop their trailer, or projects whose policy forbids
  trailers (Kubernetes asks for disclosure in the PR description instead)
  leave no trace in git. aiblame reports a floor, not the truth.
  `aiblame hook install` raises the floor for your own commits.
- **Squash merges** keep `Co-authored-by` trailers only if the platform
  preserves them (GitHub does by default; the squash commit body lists
  co-authors). `Assisted-by` trailers survive only if the squash message
  keeps the body. Rebases preserve messages. git-ai rewrites its notes after
  squashes and rebases; Entire keeps the checkpoint id in the trailer.
- **Trailer spoofing.** Anyone can write a trailer. aiblame measures what the
  history claims; it is not a forensic tool.
- **Attribution granularity** is the commit unless a git-ai authorship log
  exists for it. A commit with one AI-written line and 500 human lines counts
  as 501 AI-assisted lines. Smaller commits give better numbers; this is a
  property of every trailer-based scheme, including the Linux kernel's.
- **Heuristics can miss.** A two-word unknown tool without AI vocabulary in an
  `Assisted-by` value ("Acme Helper") looks like a person and is not counted;
  a `Generated-by` naming an unknown tool is not counted. Both are listed in
  `aiblame log --evidence` and (for weak keys) under unrecognised tools, and
  both are fixed by one `[[detect.agents]]` entry.
- **Agent-authored vs assisted** depends on how the agent commits. Claude
  Code commits as the human and adds a trailer (assisted); Devin commits as
  itself (agent). Both count as AI; the split is informational.
- **Sidecar data must be present.** Checkpoints and notes that were never
  pushed, or that the tool pruned, cannot be read; the report says how many
  were referenced but missing.
- Blame on very large repositories takes time (one `git blame` per file).
  Use `--no-blame` for a quick answer or `-j` to tune parallelism.
