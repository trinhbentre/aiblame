# Detection rules and their sources

`aiblame agents` prints the live tables. This page records where each rule
comes from so that it can be checked and updated. Everything here is data from
the tools' own documentation or from commits observed on GitHub; nothing is
inferred from code style.

## Trailer conventions

| Key | Meaning | Used by | Source |
|---|---|---|---|
| `Co-authored-by: <Agent> <email>` | GitHub's multi-author convention; AI evidence only when the person is a known agent | Claude Code (`Claude <noreply@anthropic.com>`, also `Claude Opus 4.1 <…>` etc.), Codex CLI (`Codex <noreply@openai.com>`), Cursor (`Cursor <cursoragent@cursor.com>`), GitHub Copilot (`Copilot <175728472+Copilot@users.noreply.github.com>`), aider (`aider (<model>) <noreply@aider.chat>`) | [GitHub docs](https://docs.github.com/en/pull-requests/committing-changes-to-your-project/creating-and-editing-commits/creating-a-commit-with-multiple-authors); [Codex `commit_attribution`](https://codex.danielvaughan.com/2026/03/28/codex-cli-commit-attribution/); [Cursor forum](https://forum.cursor.com/t/how-to-nicely-make-cursor-to-be-commit-co-author-on-github/115782); [VS Code `git.addAICoAuthor`](https://github.com/microsoft/vscode/issues/314311); [aider options](https://aider.chat/docs/config/options.html) |
| `Assisted-by: <tool> [(<model>)] [analysers…]` | Human remains the author; an LLM assisted. **Reserves `Co-authored-by` for humans.** | Linux kernel (`Documentation/process/coding-assistants.rst`), Mesa, Zephyr, Fedora, Rocky Linux, OpenInfra, Apache Calcite (`Assisted-by: <tool> (<model-id>)`) | [kernel.org](https://www.kernel.org/doc/html/latest/process/coding-assistants.html); [LWN](https://lwn.net/Articles/1031473/); [CALCITE-7752](http://www.mail-archive.com/issues@calcite.apache.org/msg66333.html) |
| `Generated-By: <agent>/<ver> (model: …; operator: …)` | Structured provenance trailer | Recommended by Crash Override; some in-house tooling | [Crash Override KB](https://crashoverride.com/resources/knowledge-base/code-ownership/attributing-ai-commits-git) |
| `Coding-Agent: <tool>` + `Model: <id>` | Two-field proposal | Fabio Rehm's proposal | [fabiorehm.com](https://fabiorehm.com/blog/2026/03/02/our-coding-agent-commits-deserve-better-than-co-authored-by/) |
| `AI-assistant: <tool> <ver> (<model>)` | Single-field proposal | Bence Ferdinandy (cited in the above) | same |
| `Generated-with`, `AI-Agent`, `AI-Assisted-By`, `AI-generated`, `LLM-Assisted-By`, `Agent` | Ad-hoc variants seen in the wild | various | observed |

Any key can be added per repo with `detect.extra_trailer_keys` in
`.aiblame.toml`.

## Agent identities (author or trailer person)

| Agent | Vendor | Matched by | Source |
|---|---|---|---|
| Claude Code | Anthropic | `noreply@anthropic.com`; names `Claude`, `Claude <model>`, `claude[bot]` | Claude Code default trailer; model name appended since 2026 ([explainx](https://www.explainx.ai/blog/claude-code-commit-co-author-attribution-disable-guide-2026)) |
| Codex | OpenAI | `noreply@openai.com`; names `Codex`, `Codex CLI (...)`, `chatgpt-codex-connector[bot]` | Codex `commit_attribution` default |
| Cursor | Cursor | `cursoragent@cursor.com`; names `Cursor`, `Cursor Agent`, `cursor[bot]` | Cursor commit attribution (on by default) |
| GitHub Copilot | GitHub | `*+Copilot@users.noreply.github.com`, `copilot-swe-agent[bot]` | VS Code AI co-author; Copilot coding agent commits |
| Devin | Cognition | `devin-ai-integration[bot]` | observed bot account |
| Jules | Google | `google-labs-jules[bot]` | [Jules changelog](https://jules.google/docs/changelog/) |
| Gemini CLI / Code Assist | Google | `gemini-code-assist[bot]`; names `Gemini`, `gemini-cli` | observed bot account (`176961590+gemini-code-assist[bot]@users.noreply.github.com`) |
| aider | Aider AI | author name suffix `(aider)`; `noreply@aider.chat` | [aider git integration](https://aider.chat/docs/git.html) |
| OpenCode, Amp, Cline, Roo Code, Kilo Code, Windsurf, Kiro, Junie, Goose, Warp, Zed, Augment, Droid (Factory), Hermes, Pi, Kimi Code, Qwen Code, DeepSeek Harness, Grok Build, Mistral Vibe, Trae, Replit, Lovable (`lovable-dev[bot]`, `gpt-engineer-app[bot]`), Bolt, v0 (`v0[bot]`), Sweep (`sweep-ai[bot]`), CodeRabbit (`coderabbitai[bot]`), Sourcery, Ellipsis, Codegen, Qodo, Cody, Continue, Tabnine, Blackbox, Mentat, OpenHands, SWE-agent, Manus, Ona, GitLab Duo | — | exact tool names as trailer/author names, plus bot accounts where they exist | tool names; bot accounts observed on GitHub |
| Generic AI | — | names containing `ai`, `llm`, `gpt`, `assistant`, `agent` **only** when the e-mail is a noreply/bot address | guard against humans with "Ai" in their name |

## Non-AI bots (kind `bot`)

dependabot, renovate, github-actions, pre-commit.ci, mergify, release-please,
semantic-release, snyk-bot, greenkeeper, imgbot, allcontributors, weblate,
transifex, crowdin, codecov, stale, Google yoshi/owl-bot, and any
`*[bot]` / `*-bot` name not matched above.

## Free-text markers

Consulted only when no trailer or identity matched:

- `Generated with [Claude Code|Codex|Cursor|Copilot|Gemini CLI|OpenCode|aider|…]`
- `🤖 Generated with [<Agent>]`
- `Made with Cursor|Claude Code|Codex|Windsurf|Bolt|Lovable|v0`
- `(generated|written|created|authored|implemented) [entirely|mostly|fully] by <agent|ChatGPT|GPT-x|an AI>`
- `[AI-generated]`

## Environment variables used by the hook

| Variable | Agent | Source |
|---|---|---|
| `CLAUDECODE=1` | Claude Code — set in every subprocess | [claude-code#531](https://github.com/anthropics/claude-code/issues/531), [env-vars docs](https://code.claude.com/docs/en/env-vars) |
| `CODEX_SANDBOX`, `CODEX_SANDBOX_NETWORK_DISABLED`, `CODEX_THREAD_ID` | Codex CLI sandbox | [Codex sandbox notes](https://codex.danielvaughan.com/2026/04/08/codex-sandbox-platform-implementation/) |
| `CURSOR_AGENT=1` | Cursor agent (not Cursor's human terminal, which sets `CURSOR_TRACE_ID`) | [Cursor forum](https://forum.cursor.com/t/cursor-cli-is-not-setting-cursor-agent-1-environment-variable-while-executing-bash-commands/132427) |
| `GEMINI_CLI=1` | Gemini CLI shell tool | [Gemini CLI shell docs](https://geminicli.com/docs/tools/shell/) |
| `OPENCODE=1` | OpenCode | [OpenCode env docs](https://deepwiki.com/tencent-source/opencode/5.3-environment-variables) |
| `AI_AGENT=<name>_<ver>_agent` | generic (observed: `claude-code_2-1-261_agent`) | observed |
| `AIBLAME_AGENT`, `AIBLAME_MODEL`, `AIBLAME_EMAIL`, `AIBLAME_DISABLE` | manual override / off switch | aiblame |
