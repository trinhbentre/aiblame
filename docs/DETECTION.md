# Detection rules and their sources

`aiblame agents` prints the live tables. This page records where each rule
comes from so that it can be checked and updated. Everything here is data from
the tools' own documentation, from their source code, or from commits observed
on public forges; nothing is inferred from code style.

## How the rules were validated

The 1.1 rules were checked against real history (September 2026):

- `gh search commits` corpora of 100 commits each for `Assisted-by:`,
  `Generated-by:`, `Co-authored-by: Claude`, `Co-authored-by: Cursor`,
  `AI-assistant:` and `Generated-with:` — every distinct value shape found is
  a test case in `internal/attrib/attrib_test.go`;
- the full history of **curl** (279 `Assisted-by:` trailers, all naming
  people), **entireio/cli** (4,545 `Entire-Checkpoint` trailers, 57 fetched
  checkpoints, 18 `agent:` conventional-commit subjects) and
  **git-ai-project/git-ai** (11,252 authorship records in `refs/notes/ai`
  and `refs/ai/authorship`, including 469 empty attestations);
- the Linux kernel documentation at the 7.0 and 7.3 (master) revisions, and
  the published policies of Mesa, Zephyr, LLVM, ASF, Fedora, Kubernetes,
  QEMU, Artsy and the projects collected in
  [melissawm/open-source-ai-contribution-policies](https://github.com/melissawm/open-source-ai-contribution-policies).

## Trailer conventions

### Strong keys — AI on their own

| Key | Meaning | Used by | Source |
|---|---|---|---|
| `Assisted-by: LLM [TOOL1] [TOOL2]` | Kernel 7.3+: an LLM assisted; no tool or model is named ("free advertising"). | Linux kernel since commit `816d9992d9ed` (2026-07-01) | [coding-assistants.rst](https://www.kernel.org/doc/html/latest/process/coding-assistants.html) |
| `Assisted-by: AGENT:MODEL [TOOL…]` | Kernel 7.0 / Zephyr form, e.g. `Claude:claude-opus-4.6 coccinelle`. | Linux 7.0–7.2, Zephyr, Artsy (`Claude:Opus-4.5`) | [Zephyr guidelines](https://docs.zephyrproject.org/latest/contribute/guidelines.html); [LWN](https://lwn.net/Articles/1031473/) |
| `Assisted-by: TOOL (MODEL)` | Mesa / Calcite form, e.g. `Claude Code (claude-opus-5)`. | Mesa, Apache Calcite, OpenInfra, Fedora (`Assisted-by: generic LLM chatbot`), LLVM, IREE, QGIS, STAC, OpenTelemetry (`Claude Opus 4.5`) | [Mesa](https://docs.mesa3d.org/submittingpatches.html); [LLVM](https://llvm.org/docs/AIToolPolicy.html); [CALCITE-7752](http://www.mail-archive.com/issues@calcite.apache.org/msg66333.html) |
| `Assisted-by: <person>` | **Not AI.** curl credits the human who helped. | curl (279 commits) | curl history |
| `AI-used-for: code, tests` | What AI was used for; `research`/`review` alone do not make the code AI-written. | QEMU (2026 relaxation) | [qemu-devel](https://lists.nongnu.org/archive/html/qemu-devel/2026-05/msg07614.html) |
| `AI-assisted-by`, `AI-assisted`, `AI-agent`, `AI-generated`, `AI-generated-by`, `LLM-assisted-by`, `Assisted-by-AI`, `Co-developed-by-AI` | Ad-hoc variants seen in the wild | various; Open Delivery Spec (`AI-assisted`) | observed |
| `Coding-Agent: <tool>` + `Model: <id>` | Two-field proposal | Fabio Rehm | [fabiorehm.com](https://fabiorehm.com/blog/2026/03/02/our-coding-agent-commits-deserve-better-than-co-authored-by/) |
| `AI-assistant: <tool> <ver> (<model>)` | Single-field proposal | Bence Ferdinandy | same |
| `Commit-generated-by` | commitlint/gitlint plugin taxonomy | rai-lint | [rai-lint](https://github.com/CheckMarKDevTools/rai-lint) |

### Weak keys — AI only for a known agent or AI vocabulary

| Key | AI examples | Non-AI examples seen | Source |
|---|---|---|---|
| `Generated-by` | `Codex`, `OpenAI Codex`, `Maka` (Apache Maka agent workspace), `PostHog Desktop` (PostHog's Array agent), `cursor-agent/0.42 (model: …; operator: …)` | `StageFreight` (release automation) | [ASF guidance](https://www.apache.org/legal/generative-tooling.html); [Crash Override](https://crashoverride.com/resources/knowledge-base/code-ownership/attributing-ai-commits-git); gh corpus |
| `Generated-with` | `Claude Code` | — | observed |
| `Made-with` | `Cursor` (Cursor 2.6+ trailer) | "love" | [Cursor forum](https://forum.cursor.com/t/allow-disabling-made-with-cursor-commit-trailer/154494) |
| `Agent` | `Claude Code` | prose after a conventional-commit scope | observed |

### Session-link keys — presence is AI, the key names the agent

| Key | Agent | Writer | Source |
|---|---|---|---|
| `Claude-Session: https://claude.ai/code/session_…` | Claude Code | Claude Code ≥ 2.1.258 (also `attribution.sessionUrl`) | [claude-code#91546](https://github.com/anthropics/claude-code/issues/91546) |
| `Claude-Session`, `Claude-Session-Path` | Claude Code | gammons/ai-session | [ai-session](https://github.com/gammons/ai-session) |
| `Claude-Sessions-Id` | Claude Code | schpet/jjagent | [jjagent](https://github.com/schpet/jjagent) |
| `Turbocommit-Session` | Claude Code | searlsco/turbocommit | [turbocommit](https://github.com/searlsco/turbocommit) |
| `Agent-Logs-Url` | GitHub Copilot coding agent | GitHub (2026-03-20) | [GitHub changelog](https://github.blog/changelog/2026-03-20-trace-any-copilot-coding-agent-commit-to-its-session-logs/) |
| `Amp-Thread-ID` | Amp | Amp CLI (`AMP_DISABLE_AMP_THREAD_TRAILER`) | [Amp guide](https://www.verdent.ai/guides/agents/amp-coding-agent) |
| `Entire-Checkpoint: <id>` | resolved from the checkpoint; else unknown | Entire CLI | [entireio/cli](https://github.com/entireio/cli) |
| `Agent-Conversation` | unknown | AgentsRoom | [agentsroom.dev](https://agentsroom.dev/features/commit-context) |
| `Agent-Transcript` | unknown | .specstory exports | observed |

`Co-authored-by` / `Co-developed-by` count when the person is a known agent
([GitHub docs](https://docs.github.com/en/pull-requests/committing-changes-to-your-project/creating-and-editing-commits/creating-a-commit-with-multiple-authors)).
`Signed-off-by` naming an agent counts and sets `agent_signed_off` (the kernel,
Zephyr and Mesa forbid it). Any key can be added per repository with
`detect.extra_trailer_keys` (treated as strong).

## Agent identities (author or trailer person)

| Agent | Vendor | Matched by | Source |
|---|---|---|---|
| Claude Code | Anthropic | `noreply@anthropic.com`, `claude@anthropic.ai`, `+claude[bot]@users.noreply.github.com`; names `Claude`, `Claude <Model>` (`Claude Opus 5`, `Claude Fable 5.1`, `Claude Opus 4.8 (1M context)`), `Claude Code`, `claude[bot]`, `claude-code` | default `Co-Authored-By` trailer (model-named since 2026; `Claude Code` when the model is not a Claude model — [changelog](https://code.claude.com/docs/en/changelog)); GitHub App id 209825114 |
| Codex | OpenAI | `noreply@openai.com`, `codex@openai.com`, `codex@example.com` (cloud placeholder "Codex Test"), `chatgpt-codex-connector[bot]`; names `Codex`, `openai-codex`, `Codex CLI (…)`, anything containing the word `codex` | [Codex `commit_attribution`](https://codex.danielvaughan.com/2026/03/28/codex-cli-commit-attribution/); [codex#18095](https://github.com/openai/codex/issues/18095) |
| ChatGPT, GPT | OpenAI | `ChatGPT`, `ChatGPTv5`; bare model names `GPT-5.5`, `OpenAI GPT-5.6 Sol` | Fedora examples; gh corpus |
| Cursor | Cursor | `cursoragent@cursor.com`, `agent@cursor.com`, `cursor@agent`; names starting with `Cursor` (`Cursor Agent`, `Cursor Grok 4.6`) | [Cursor forum](https://forum.cursor.com/t/how-to-nicely-make-cursor-to-be-commit-co-author-on-github/115782); [community discussion](https://github.com/orgs/community/discussions/186158) |
| GitHub Copilot | GitHub | `copilot@github.com`, `*+Copilot@users.noreply.github.com` (ids 175728472, 198982749, 223556219), `copilot-swe-agent[bot]`, `github-advanced-security[bot]` (`Copilot Autofix powered by AI`) | [VS Code `git.addAICoAuthor`](https://github.com/microsoft/vscode/issues/314311); [Copilot coding agent](https://github.blog/changelog/2026-03-20-trace-any-copilot-coding-agent-commit-to-its-session-logs/) |
| Devin | Cognition | `devin-ai-integration[bot]`, `cognition-devin[bot]` | observed bot account; [Devin docs](https://docs.devin.ai/integrations/gh) |
| Gemini CLI / Code Assist | Google | `gemini-cli-agent@google.com`, `gemini-cli-robot@google.com`, `gemini-code-assist[bot]`, `gemini-cli[bot]`; names starting with `Gemini` | [gemini-cli#11447](https://github.com/google-gemini/gemini-cli/discussions/11447); observed |
| Jules | Google | `google-labs-jules[bot]` | [Jules changelog](https://jules.google/docs/changelog/) |
| Antigravity | Google | `antigravity@gemini.ai`; `antigravity:gemini-3.6-flash` | gh corpus |
| aider | Aider AI | author suffix `(aider)`; `noreply@aider.chat`, `aider@aider.chat`; `aider-chat-bot` | [aider git integration](https://aider.chat/docs/git.html) |
| Amp | Amp | `amp@ampcode.com`; `Amp-Thread-ID` trailer | Amp docs |
| Codebuff | Codebuff | `noreply@codebuff.com`; marker `🤖 Generated with Codebuff` | gh corpus (32 co-author trailers) |
| Greptile | Greptile | `greptile-apps[bot]`; `Assisted-by: Greptile:undisclosed` | observed |
| Maka | Apache | `Generated-by: Maka` | [apache/maka](https://github.com/apache/maka) |
| PostHog Desktop | PostHog | `Generated-by: PostHog Desktop` | [PostHog Desktop git integration](https://posthog.com/docs/posthog-desktop/git-github-integration) |
| Grok Build | xAI | `grok@x.ai`; `Grok 4.6`, `Grok (xAI)` | gh corpus |
| Hermes Agent | Nous Research | `Hermes Agent:gpt-5.6-sol` | gh corpus |
| Crush | Charm | `crush@charm.land` | vibe-coded-badge regexes |
| Rovo Dev, Firebender, Terragon, Supermaven, Phind, IBM Bob | — | tool names as trailer values / git-ai tool ids | git-ai agent list; Liu et al. 2026 rule file |
| OpenCode, Cline, Roo Code, Kilo Code, Windsurf, Kiro / Amazon Q (`amazon-q-developer[bot]`), Junie, Goose, Warp, Zed, Augment, Droid (`droid@factory.ai`), Pi (`pi@earendil.works`), Kimi, Qwen (`noreply@alibaba.com`), DeepSeek, Mistral Vibe, Trae, Replit, Lovable, Bolt, v0, Sweep, CodeRabbit, Sourcery, Ellipsis, Codegen, Qodo, Cody, Continue, Tabnine, Blackbox, Mentat, OpenHands, SWE-agent, Manus, Ona, GitLab Duo | — | exact tool names as trailer/author names, plus bot accounts and documented no-reply addresses where they exist. Bare company domains are deliberately **not** matched: an engineer at an agent vendor committing from a corporate mailbox is a person. | tool names; bot accounts observed on GitHub; [Liu et al. 2026](https://github.com/yueyueL/tech-debt-ai-coding) |
| Generic AI | — | names containing `ai`, `llm`, `gpt`, `assistant`, `agent` **only** when the e-mail is a noreply/bot address | guard against humans with "Ai" in their name |

Matching order: user-defined identities, then exact names across the whole
table, then name patterns (only with a noreply/agent e-mail, except aider's
`(aider)` suffix), then exact e-mails, then e-mail suffixes (only for
addresses that already look automated), then the generic rule. So `Cursor Grok 4.6
<noreply@anthropic.com>` is Cursor running Grok, and `GPT-5.6 Sol
<noreply@anthropic.com>` is Claude Code running GPT.

## Non-AI bots (kind `bot`)

dependabot, renovate, github-actions, pre-commit.ci, mergify, release-please,
semantic-release, snyk-bot, greenkeeper, imgbot, allcontributors, weblate,
transifex, crowdin, codecov, stale, Stainless (`stainless-app[bot]`), Fern
(`fern-api[bot]`), StageFreight, Google yoshi/owl-bot, and any `*[bot]` /
`*-bot` name not matched above.

## Free-text markers

Consulted only when no trailer, identity or sidecar record matched. The
generic patterns require the captured tool to be a known agent, so "Generated
with love" is not AI:

- `Generated with [Claude Code|Codex|Cursor|Codebuff|Devin|…]`, including
  `Generated with help of …` and `🤖 Generated with …`
- `Generated with <tool> (AI coding agent)`
- `Made with Cursor|Claude Code|Codex|…`
- `(generated|written|created|authored|implemented|vibe-coded) [entirely|mostly|fully] by <agent|ChatGPT|GPT-x|an AI>`
- `[AI-generated]`

## Sidecar data

| Tool | Ref | Format | Source |
|---|---|---|---|
| git-ai | `refs/notes/ai` | Authorship Log v3: attestation lines (`<path>`, `  s_<session>::t_<turn> 1-10,15-20`, `  h_<hash> 42-50`, legacy 16-hex prompt ids), `---`, JSON metadata (`sessions[].agent_id.{tool,model}`, `humans`, `prompts`) | [git_ai_standard_v3.0.0.md](https://github.com/git-ai-project/git-ai/blob/main/specs/git_ai_standard_v3.0.0.md) |
| git-ai (older) | `refs/ai/authorship/<commit>` blob | JSON `authorship/0.0.1`: `files.<path>.authors[].{author, lines:[n \| [a,b]], agent_metadata}`; the author is an agent label (`Claude`, `Cursor`) or a person | git-ai repository history |
| Exceeds Ink | `refs/notes/exceeds-ink` | Authorship Log v3 | [Exceeds](https://blog.exceeds.ai/track-ai-code-contributions-git/) |
| Entire CLI | `refs/entire/checkpoints/<last two chars>/<ULID>` (commit; tree has `metadata.json`, `N/metadata.json`, transcripts) or branch `entire/checkpoints/v1` at `<id[:2]>/<id[2:]>/` for 12-hex ids | `metadata.json`: `checkpoint_id`, `sessions[].metadata`; session: `agent` (`Claude Code`, `claude-code`, `codex`, `copilot-cli`, `cursor`, `factoryai-droid`, `gemini`, `opencode`, `pi`), `model`, `files_touched`, `initial_attribution.{agent_lines,human_added,human_modified,agent_percentage}` | [entireio/cli](https://github.com/entireio/cli); [checkpoint storage](https://docs.entire.io/guides/configuration/checkpoint-storage) |
| Claudit | `refs/notes/claude-conversations` | presence of a note | [re-cinq/claudit](https://github.com/re-cinq/claudit) |

## Policy presets (`aiblame check --policy`)

| Preset | Rules | Source (checked 2026-09-06) |
|---|---|---|
| `kernel` | require `assisted-by`; forbid `co-authored-by`, `co-developed-by`; no agent authors; no agent sign-off | [kernel.org](https://www.kernel.org/doc/html/latest/process/coding-assistants.html) |
| `zephyr` | same as kernel | [Zephyr](https://docs.zephyrproject.org/latest/contribute/guidelines.html) |
| `mesa` | require `assisted-by` or `generated-by`; forbid `co-authored-by`; no agent authors; no agent sign-off | [Mesa](https://docs.mesa3d.org/submittingpatches.html) |
| `openinfra` | require `assisted-by` or `generated-by`; forbid `co-authored-by` | OpenInfra Foundation |
| `llvm` | require `assisted-by`; no agent authors | [LLVM](https://llvm.org/docs/AIToolPolicy.html) |
| `asf` | require `generated-by` | [ASF](https://www.apache.org/legal/generative-tooling.html) |
| `fedora` | require `assisted-by` | Fedora Council policy (2025-10-22) |
| `artsy` | require `assisted-by`; forbid `co-authored-by` | Artsy RFC (merged 2026-06-08) |
| `kubernetes` | forbid every AI trailer (disclosure goes in the PR description); no agent authors; no agent sign-off | [Kubernetes](https://www.kubernetes.dev/blog/2026/06/26/open-source-maintainership-in-the-age-of-ai/) |

## Environment variables used by the hook

| Variable | Agent | Source |
|---|---|---|
| `CLAUDECODE=1` | Claude Code — set in every subprocess; there is no variable for the active model ([claude-code#37817](https://github.com/anthropics/claude-code/issues/37817), closed as not planned) | [claude-code#531](https://github.com/anthropics/claude-code/issues/531), [env-vars docs](https://code.claude.com/docs/en/env-vars) |
| `CODEX_SANDBOX`, `CODEX_SANDBOX_NETWORK_DISABLED`, `CODEX_THREAD_ID` | Codex CLI sandbox | [Codex sandbox notes](https://codex.danielvaughan.com/2026/04/08/codex-sandbox-platform-implementation/) |
| `CURSOR_AGENT=1` | Cursor agent (not Cursor's human terminal, which sets `CURSOR_TRACE_ID`) | [Cursor forum](https://forum.cursor.com/t/cursor-cli-is-not-setting-cursor-agent-1-environment-variable-while-executing-bash-commands/132427) |
| `GEMINI_CLI=1` | Gemini CLI shell tool | [Gemini CLI shell docs](https://geminicli.com/docs/tools/shell/) |
| `OPENCODE=1` | OpenCode | [OpenCode env docs](https://deepwiki.com/tencent-source/opencode/5.3-environment-variables) |
| `AI_AGENT=<name>_<ver>_agent` | generic (observed: `claude-code_2-1-261_agent`) | observed |
| `AIBLAME_AGENT`, `AIBLAME_MODEL`, `AIBLAME_EMAIL`, `AIBLAME_DISABLE` | manual override / off switch | aiblame |

GitHub Copilot CLI exposes only an OpenTelemetry `traceparent` to child
processes, which is not specific enough to use; set `AIBLAME_AGENT="GitHub
Copilot"` there.
