# Changelog

All notable changes to aiblame are documented here. The format follows
[Keep a Changelog](https://keepachangelog.com/en/1.1.0/) and the project uses
[Semantic Versioning](https://semver.org/).

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

[1.0.0]: https://github.com/trinhbentre/aiblame/releases/tag/v1.0.0
