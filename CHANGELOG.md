# Changelog

All notable changes to aiblame are documented here. The format follows
[Keep a Changelog](https://keepachangelog.com/en/1.1.0/) and the project uses
[Semantic Versioning](https://semver.org/).

## [1.1.0] - 2026-09-06

### Added

- **Sidecar provenance.** aiblame now reads the attribution other tools
  leave in a repository, with nothing installed: git-ai authorship logs
  (`refs/notes/ai`, Authorship Log v3, and the older
  `refs/ai/authorship/<sha>` JSON blobs), Exceeds Ink notes, Claudit
  conversation notes and Entire checkpoints (`Entire-Checkpoint` trailers
  resolved through `refs/entire/checkpoints/*` or the `entire/checkpoints/v1`
  branch). git-ai logs give **line-level attribution**: only the lines the
  agent wrote count as AI. New report fields `provenance`,
  `line_level_commits`; `--no-provenance` and `detect.provenance = false`
  turn it off. Repositories cloned by aiblame fetch these refs automatically.
- **Survival metric.** `survival {ai, human, all}` and `agents[].survival`:
  the share of lines added that is still in the tree, following the
  line-survival measure of Rahman & Shihab (EASE 2026) and GitClear's churn
  research. Shown in the table and Markdown reports.
- **Range mode.** `--rev BASE..HEAD` analyses one range the way a reviewer
  sees a pull request: only the range's commits, churn and surviving lines
  (`base_rev` in JSON). The GitHub Action gained `pr-range` and `pr-comment`
  inputs.
- **Policy presets and new check rules.** `aiblame check --policy
  kernel|zephyr|mesa|openinfra|llvm|asf|fedora|artsy|kubernetes`
  (`--list-policies` prints the sources), `--forbid-trailer CONV`
  (Mesa: no `Co-authored-by` for AI; Kubernetes: no trailers at all),
  `--forbid-agent-signoff` (DCO), and `--require-trailer a,b` for
  alternatives. Config: `check.policy`, `check.forbid_trailers`,
  `check.forbid_agent_signoff`.
- **Session-link trailers** as evidence: `Claude-Session` (Claude Code
  ≥ 2.1.258), `Agent-Logs-Url` (Copilot coding agent), `Amp-Thread-ID`,
  `Entire-Checkpoint`, `Turbocommit-Session`, `Claude-Sessions-Id`,
  `Agent-Conversation`, `Agent-Transcript`; new keys `Made-with` (Cursor),
  `AI-used-for` (QEMU), `AI-assisted`, `Commit-generated-by`; `Signed-off-by`
  naming an agent is counted and flagged (`agent_signed_off`).
- **Identities**: ChatGPT, GPT (bare OpenAI model names), Codebuff,
  Greptile, Maka (Apache), PostHog Desktop, Crush, Rovo Dev, Terragon,
  Firebender, Supermaven, Phind, IBM Bob; new addresses for Claude Code
  (`claude@anthropic.ai`, `claude[bot]`), Codex (`codex@example.com`,
  `openai-codex`), Copilot (`copilot@github.com`,
  `github-advanced-security[bot]`), Gemini (`gemini-cli-robot@google.com`,
  `gemini-cli[bot]`), Devin (`cognition-devin[bot]`), aider
  (`aider@aider.chat`), Grok (`grok@x.ai`), Amazon Q bots, and more; bots
  Stainless, Fern, StageFreight. Company domains are never matched on their
  own, so vendor employees stay human.
- `aiblame stats host/owner/repo` for GitLab, Codeberg, Gitea and friends.
- `--no-authors` leaves the contributor table out of shared reports.
- `aiblame hook install --style kernel` (`Assisted-by: LLM`, Linux 7.3+) and
  `--style agent-model` (`Assisted-by: Agent:model`, Zephyr / kernel 7.0).
  The hook no longer adds a trailer when the commit already carries a
  session-link trailer.
- `aiblame log --evidence` and the JSON `ignored` field explain trailers that
  were *not* counted and why; the report lists `unrecognised_tools` named in
  `Generated-by`/`Made-with` trailers so they can be added to the config.

### Changed

- Trailer values are parsed with the shapes found in 2026 history:
  `Hermes Agent:gpt-5.6-sol`, `openai-codex:gpt-5.5 [read,bash,edit,write]`,
  `claude-code:fable-5.1#high (orchestrator, reviewer)`,
  `claude-code/claude-fable-5`, `devx/<uuid>`, `GPT-5.6 Sol via Codex`,
  `OpenCode GPT-5.6 Sol <…>`, `Cursor Grok 4.6 <noreply@anthropic.com>`
  (Cursor running Grok), `GPT-5.6 Sol <noreply@anthropic.com>` (Claude Code
  running GPT). Placeholder models (`<synthetic>`, `unknown`, `undisclosed`)
  and bare family names are dropped from the models column.
- Identity matching tries exact names, then patterns (only with a noreply or
  agent address), then exact e-mails, then e-mail suffixes (only for
  automated addresses), so a person called Gemini, or an engineer at an
  agent vendor, stays human.
- Trailer keys have strengths: strong (`Assisted-by`, …), weak
  (`Generated-by`, `Generated-with`, `Made-with`, `Agent`: counted only for a
  known agent or AI vocabulary) and session-link. The subject line is never
  a trailer.
- Markdown reports escape backticks and pipes in paths, since the Action can
  now post them as pull-request comments.
- `aiblame agents` documents the key classes and sidecar sources.

### Fixed

- **`Assisted-by` naming a person is not AI.** curl uses `Assisted-by:` for
  the human who helped (279 commits); 1.0.0 counted all of them. Values that
  read like a person's name or carry a personal address are ignored and
  listed in `log --evidence`.
- `Assisted-by: none` is a negative, not a disclosure.
- `Generated-By: StageFreight` and other non-AI generators no longer count.
- Conventional-commit subjects such as `agent: refactor hooks` were parsed
  as an `Agent:` trailer and produced garbage agent rows.
- Doubled keys written by buggy hooks (`Assisted-by: Assisted-by: Claude
  Opus 4.6 <…>`) are parsed once.
- Untrusted sidecar blobs are size-capped and line ranges are merged
  arithmetically, so a hostile authorship log cannot exhaust memory.

## [1.0.0] - 2026-09-06

First stable release.

### Added

- `aiblame stats` — AI authorship report for any git repository: surviving
  lines (git blame), lines added and commit counts split into human,
  AI-assisted, agent-authored and non-AI bot; per-agent, per-contributor,
  per-directory, per-file and per-month breakdowns; table, Markdown and JSON
  output (`schema_version` 1). Accepts a path, a git URL or GitHub
  `owner/repo` (cloned into the user cache).
- Detection of every disclosure convention in use in 2026: `Co-authored-by`
  trailers from Claude Code, Codex, Cursor, GitHub Copilot and aider;
  `Assisted-by` (Linux kernel, Mesa, Zephyr, Fedora, Apache Calcite);
  `Generated-by`, `Coding-Agent`/`Model`, `AI-assistant` and other trailer
  keys; agent author identities (Devin, Copilot SWE agent, Jules, Gemini Code
  Assist, aider `(aider)` suffix, CodeRabbit, Sweep, OpenHands, …); free-text
  markers such as "🤖 Generated with [Claude Code]". Non-AI automation
  (dependabot, renovate, github-actions, pre-commit.ci, …) is classified as
  `bot` and kept out of the AI share denominator.
- `aiblame blame FILE` — per-line HUMAN / ASSIST / AGENT / BOT view.
- `aiblame log` — commits with classification, agents, models and evidence.
- `aiblame badge` — shields.io-style SVG badge and endpoint JSON.
- `aiblame check` — CI gate: `--max` / `--min` AI share,
  `--forbid-agent-authored`, `--require-trailer <convention>`; exit code 1 on
  violation.
- `aiblame hook install` — POSIX `prepare-commit-msg` hook that appends an
  `Assisted-by` (or `Co-authored-by` / `Generated-by`) trailer when the commit
  is made from a Claude Code, Codex, Cursor, Gemini CLI or OpenCode session.
- `aiblame agents`, `aiblame init`, `aiblame version`.
- `.aiblame.toml` for custom agents/bots, extra trailer keys, path
  include/exclude, check thresholds and badge defaults.
- Default excludes for lockfiles, vendored, minified and generated files;
  `.git-blame-ignore-revs` support; parallel blame.
- GitHub Action (`action.yml`), `install.sh`, cross-platform release
  binaries, and an Agent Skill (`skills/aiblame/SKILL.md`).

[1.1.0]: https://github.com/trinhbentre/aiblame/releases/tag/v1.1.0
[1.0.0]: https://github.com/trinhbentre/aiblame/releases/tag/v1.0.0
