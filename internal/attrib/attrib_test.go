package attrib

import (
	"reflect"
	"testing"
)

func TestParseAgentRef(t *testing.T) {
	cases := []struct {
		in          string
		tool, model string
		email       string
	}{
		{"Claude <noreply@anthropic.com>", "Claude", "", "noreply@anthropic.com"},
		{"Claude Opus 4.1 <noreply@anthropic.com>", "Claude", "Opus 4.1", "noreply@anthropic.com"},
		{"Claude Sonnet 4.6 <noreply@anthropic.com>", "Claude", "Sonnet 4.6", "noreply@anthropic.com"},
		{"Claude Opus 4.6 (1M context) <noreply@anthropic.com>", "Claude", "Opus 4.6", "noreply@anthropic.com"},
		{"Claude Opus 5 (1M context) <noreply@anthropic.com>", "Claude", "Opus 5", "noreply@anthropic.com"},
		{"Claude:claude-fable-5 [Cursor]", "Claude", "claude-fable-5", ""},
		{"Codex <noreply@openai.com>", "Codex", "", "noreply@openai.com"},
		{"Codex CLI (gpt-5-codex) <noreply@openai.com>", "Codex CLI", "gpt-5-codex", "noreply@openai.com"},
		{"Cursor <cursoragent@cursor.com>", "Cursor", "", "cursoragent@cursor.com"},
		{"Copilot <175728472+Copilot@users.noreply.github.com>", "Copilot", "", "175728472+Copilot@users.noreply.github.com"},
		{"Claude Code (claude-opus-4-6)", "Claude Code", "claude-opus-4-6", ""},
		{"Claude:claude-3-opus coccinelle sparse", "Claude", "claude-3-opus", ""},
		{"LLM coccinelle sparse", "LLM", "", ""},
		{"cursor-agent/0.42 (model: claude-sonnet-4-5; operator: a@b.c)", "cursor-agent", "claude-sonnet-4-5", ""},
		{"OpenCode v1.0.203 (Claude Opus 4.5)", "OpenCode", "Claude Opus 4.5", ""},
		{"aider (gpt-4o) <noreply@aider.chat>", "aider", "gpt-4o", "noreply@aider.chat"},
		{"Claude Code", "Claude Code", "", ""},
		{"Gemini CLI", "Gemini CLI", "", ""},
		// Shapes seen on GitHub in 2026 (gh search commits corpus, Sept 2026).
		{"Hermes Agent:gpt-5.6-sol", "Hermes Agent", "gpt-5.6-sol", ""},
		{"openai-codex:gpt-5.5 [read,bash,edit,write]", "openai-codex", "gpt-5.5", ""},
		{"claude-code:fable-5.1#high (orchestrator, reviewer)", "claude-code", "fable-5.1", ""},
		{"claude-code/claude-fable-5", "claude-code", "claude-fable-5", ""},
		{"devx/3066e945-4358-448d-b2e2-8ee7c94ce548", "devx", "", ""},
		{"GPT-5.6 Sol via Codex", "Codex", "GPT-5.6 Sol", ""},
		{"GPT via Codex", "Codex", "", ""}, // a bare family name is not a model
		{"OpenCode GPT-5.6 Sol <noreply@opencode.ai>", "OpenCode", "GPT-5.6 Sol", "noreply@opencode.ai"},
		{"OpenAI GPT-5 Codex", "OpenAI Codex", "GPT-5", ""},
		{"OpenAI GPT-5.6 Sol", "GPT", "GPT-5.6 Sol", ""},
		{"GPT-5.5", "GPT", "GPT-5.5", ""},
		{"Cursor Grok 4.6 <noreply@anthropic.com>", "Cursor", "Grok 4.6", "noreply@anthropic.com"},
		{"ChatGPT (OpenAI GPT-5.5)", "ChatGPT", "GPT-5.5", ""},
		{"Grok (xAI)", "Grok", "", ""},
		{"Claude (Anthropic)", "Claude", "", ""},
		{"claude-opus-5", "Claude", "opus-5", ""},
		{"Copilot:github-copilot/gpt-5.6-luna", "Copilot", "github-copilot/gpt-5.6-luna", ""},
		{"Greptile:undisclosed", "Greptile", "", ""},
		{"Nitori (gensokyo-skills:nitori-reverse-engineering)", "Nitori", "", ""},
		{"antigravity:gemini-3.6-flash [read,bash,edit,write]", "antigravity", "gemini-3.6-flash", ""},
		{"Claude Code (<synthetic>)", "Claude Code", "", ""},
		{"Claude Code (gpt-5-6-thinking,)", "Claude Code", "gpt-5-6-thinking", ""},
		{"Claude Opus 4.8 (1M context) <noreply@anthropic.com>", "Claude", "Opus 4.8", "noreply@anthropic.com"},
		{"Codex Test <codex@example.com>", "Codex Test", "", "codex@example.com"},
		{"Claude:Opus-4.5", "Claude", "Opus-4.5", ""},                          // Artsy RFC
		{"Claude:claude-opus-4.6 coccinelle", "Claude", "claude-opus-4.6", ""}, // Zephyr
		{"Copilot:claude-sonnet-4-6", "Copilot", "claude-sonnet-4-6", ""},      // VS Code proposal
		{"ChatGPTv5", "ChatGPTv5", "", ""},                                     // Fedora example
		{"generic LLM chatbot", "generic LLM chatbot", "", ""},                 // Fedora example
		{"Claude Code:claude-opus-4-8", "Claude Code", "claude-opus-4-8", ""},
		{"Codex:gpt-5.3-codex-spark", "Codex", "gpt-5.3-codex-spark", ""},
	}
	for _, c := range cases {
		got := ParseAgentRef(c.in)
		if got.Tool != c.tool || got.Model != c.model || got.Email != c.email {
			t.Errorf("ParseAgentRef(%q) = %+v, want tool=%q model=%q email=%q", c.in, got, c.tool, c.model, c.email)
		}
	}
}

func TestParseTrailers(t *testing.T) {
	msg := "Add feature\n\nBody text: not a trailer? Actually it matches, caller filters.\nhttps://example.com\n\nCo-Authored-By: Claude <noreply@anthropic.com>\nSigned-off-by: Dev <d@x.y>\n"
	got := ParseTrailers(msg)
	var keys []string
	for _, tr := range got {
		keys = append(keys, tr.Key)
	}
	want := []string{"Co-Authored-By", "Signed-off-by"}
	if !reflect.DeepEqual(keys, want) {
		t.Fatalf("keys = %v, want %v", keys, want)
	}
	// The subject line is never a trailer: conventional-commit scopes such
	// as "agent: …" (18 of them in entireio/cli) must not become evidence.
	if got := ParseTrailers("agent: dedupe transcript scanning in opencode and copilot-cli\n\nbody"); len(got) != 0 {
		t.Fatalf("subject parsed as trailer: %+v", got)
	}
	if got := classify(t, Commit{AuthorName: "J", AuthorEmail: "j@example.com", Message: "agent: move ParseHookEvent to HookSupport\n\nRefactor only."}); got.Kind != Human || len(got.Ignored) != 0 {
		t.Fatalf("subject scope classified: %+v", got)
	}
	// Prose after a weak key is not a tool name either.
	if got := classify(t, Commit{AuthorName: "J", AuthorEmail: "j@example.com", Message: "x\n\nAgent: decided to refactor the copilot integration after review."}); got.Kind != Human || len(got.Ignored) != 1 || got.Ignored[0].Reason != "value is not a tool name" {
		t.Fatalf("prose after weak key: %+v", got)
	}
	// A hook that repeated the key.
	if ref := ParseAgentRef("Assisted-by: Claude Opus 4.6 <noreply@anthropic.com>"); ref.Tool != "Claude" || ref.Model != "Opus 4.6" {
		t.Fatalf("doubled key: %+v", ref)
	}
	// Unknown AI gives way to a named agent on the same commit.
	got2 := classify(t, Commit{AuthorName: "J", AuthorEmail: "j@example.com", Message: "x\n\nCo-Authored-By: Claude Fable 5 <noreply@anthropic.com>\nEntire-Checkpoint: 01KXGTTNGCEACC83QZEJ5YAF0D"})
	if !reflect.DeepEqual(got2.Agents, []string{"Claude Code"}) || len(got2.Evidence) != 2 {
		t.Fatalf("unknown should be dropped: %+v", got2)
	}
	for in, want := range map[string]string{`"gpt-5"`: "gpt-5", "<synthetic>": "", "unknown": "", "Claude": "", "claude-opus-5": "claude-opus-5", "fable-5.1#high": "fable-5.1", "OpenAI GPT-5.5": "GPT-5.5", "GPT": ""} {
		if got := CleanModel(in); got != want {
			t.Errorf("CleanModel(%q) = %q, want %q", in, got, want)
		}
	}
}

func classify(t *testing.T, cm Commit) Attribution {
	t.Helper()
	return NewClassifier(Options{}).Classify(cm)
}

func TestClassifyRealWorld(t *testing.T) {
	human := Commit{AuthorName: "Jane Doe", AuthorEmail: "jane@example.com", CommitterName: "Jane Doe", CommitterEmail: "jane@example.com"}
	cases := []struct {
		name   string
		commit Commit
		kind   Kind
		agents []string
		conv   []string
	}{
		{
			name:   "plain human",
			commit: with(human, "Fix typo\n\nSigned-off-by: Jane Doe <jane@example.com>"),
			kind:   Human,
		},
		{
			name:   "human co-author is not AI",
			commit: with(human, "Pair work\n\nCo-authored-by: Bob Smith <bob@example.com>"),
			kind:   Human,
		},
		{
			name:   "claude code co-author",
			commit: with(human, "Add parser\n\n🤖 Generated with [Claude Code](https://claude.com/claude-code)\n\nCo-Authored-By: Claude <noreply@anthropic.com>"),
			kind:   Assisted, agents: []string{"Claude Code"}, conv: []string{"co-authored-by"},
		},
		{
			name:   "claude with model in name",
			commit: with(human, "Refactor\n\nCo-Authored-By: Claude Fable 5.1 <noreply@anthropic.com>"),
			kind:   Assisted, agents: []string{"Claude Code"}, conv: []string{"co-authored-by"},
		},
		{
			name:   "codex default trailer",
			commit: with(human, "Implement thing\n\nCo-authored-by: Codex <noreply@openai.com>"),
			kind:   Assisted, agents: []string{"Codex"}, conv: []string{"co-authored-by"},
		},
		{
			name:   "cursor trailer",
			commit: with(human, "Update UI\n\nCo-authored-by: Cursor <cursoragent@cursor.com>"),
			kind:   Assisted, agents: []string{"Cursor"}, conv: []string{"co-authored-by"},
		},
		{
			name:   "copilot trailer",
			commit: with(human, "Tweak\n\nCo-authored-by: Copilot <175728472+Copilot@users.noreply.github.com>"),
			kind:   Assisted, agents: []string{"GitHub Copilot"}, conv: []string{"co-authored-by"},
		},
		{
			name:   "aider trailer",
			commit: with(human, "feat: x\n\nCo-authored-by: aider (gpt-4o) <noreply@aider.chat>"),
			kind:   Assisted, agents: []string{"aider"}, conv: []string{"co-authored-by"},
		},
		{
			name:   "aider author suffix",
			commit: Commit{AuthorName: "Jane Doe (aider)", AuthorEmail: "jane@example.com", Message: "feat: y"},
			kind:   Agent, agents: []string{"aider"},
		},
		{
			name:   "kernel assisted-by",
			commit: with(human, "mm: fix leak\n\nAssisted-by: Claude:claude-3-opus coccinelle sparse\nSigned-off-by: Jane Doe <jane@example.com>"),
			kind:   Assisted, agents: []string{"Claude Code"}, conv: []string{"assisted-by"},
		},
		{
			name:   "kernel assisted-by generic LLM",
			commit: with(human, "net: thing\n\nAssisted-by: LLM coccinelle sparse"),
			kind:   Assisted, agents: []string{"Unknown AI"}, conv: []string{"assisted-by"},
		},
		{
			name:   "calcite assisted-by",
			commit: with(human, "[CALCITE-1] x\n\nAssisted-by: Claude Code (claude-opus-4-6)"),
			kind:   Assisted, agents: []string{"Claude Code"}, conv: []string{"assisted-by"},
		},
		{
			name:   "generated-by crash override style",
			commit: with(human, "x\n\nGenerated-By: cursor-agent/0.42 (model: claude-sonnet-4-5; operator: jane@example.com)"),
			kind:   Assisted, agents: []string{"Cursor"}, conv: []string{"generated-by"},
		},
		{
			name:   "coding-agent + model",
			commit: with(human, "x\n\nCoding-Agent: Claude Code\nModel: claude-opus-4-6"),
			kind:   Assisted, agents: []string{"Claude Code"}, conv: []string{"coding-agent"},
		},
		{
			name:   "ai-assistant proposal",
			commit: with(human, "x\n\nAI-assistant: OpenCode v1.0.203 (Claude Opus 4.5)"),
			kind:   Assisted, agents: []string{"OpenCode"}, conv: []string{"ai-assistant"},
		},
		{
			name:   "devin bot author",
			commit: Commit{AuthorName: "devin-ai-integration[bot]", AuthorEmail: "158243242+devin-ai-integration[bot]@users.noreply.github.com", Message: "fix"},
			kind:   Agent, agents: []string{"Devin"},
		},
		{
			name:   "copilot swe agent author",
			commit: Commit{AuthorName: "copilot-swe-agent[bot]", AuthorEmail: "198982749+Copilot@users.noreply.github.com", Message: "Initial plan"},
			kind:   Agent, agents: []string{"GitHub Copilot"},
		},
		{
			name:   "jules author",
			commit: Commit{AuthorName: "google-labs-jules[bot]", AuthorEmail: "161369871+google-labs-jules[bot]@users.noreply.github.com", Message: "x"},
			kind:   Agent, agents: []string{"Jules"},
		},
		{
			name:   "gemini code assist author",
			commit: Commit{AuthorName: "gemini-code-assist[bot]", AuthorEmail: "176961590+gemini-code-assist[bot]@users.noreply.github.com", Message: "x"},
			kind:   Agent, agents: []string{"Gemini CLI"},
		},
		{
			name:   "dependabot is bot not AI",
			commit: Commit{AuthorName: "dependabot[bot]", AuthorEmail: "49699333+dependabot[bot]@users.noreply.github.com", Message: "Bump lodash"},
			kind:   Bot,
		},
		{
			name:   "renovate is bot",
			commit: Commit{AuthorName: "renovate[bot]", AuthorEmail: "29139614+renovate[bot]@users.noreply.github.com", Message: "chore(deps)"},
			kind:   Bot,
		},
		{
			name:   "github-actions is bot",
			commit: Commit{AuthorName: "github-actions[bot]", AuthorEmail: "41898282+github-actions[bot]@users.noreply.github.com", Message: "release"},
			kind:   Bot,
		},
		{
			name:   "message marker only",
			commit: with(human, "Add tests\n\n🤖 Generated with [Claude Code](https://claude.ai/code)"),
			kind:   Assisted, agents: []string{"Claude Code"},
		},
		{
			name:   "made with cursor marker",
			commit: with(human, "Landing page\n\nMade with Cursor"),
			kind:   Assisted, agents: []string{"Cursor"},
		},
		{
			name:   "human named Ai Chen is human",
			commit: Commit{AuthorName: "Ai Chen", AuthorEmail: "ai.chen@example.com", Message: "fix"},
			kind:   Human,
		},
		{
			name:   "human with Claude in subject is human",
			commit: with(human, "Document how to use Claude in CI"),
			kind:   Human,
		},
		{
			name:   "bot committer does not flip human author",
			commit: Commit{AuthorName: "Jane Doe", AuthorEmail: "jane@example.com", CommitterName: "GitHub", CommitterEmail: "noreply@github.com", Message: "Update README.md"},
			kind:   Human,
		},
		{
			name:   "claude author directly",
			commit: Commit{AuthorName: "Claude", AuthorEmail: "noreply@anthropic.com", Message: "x"},
			kind:   Agent, agents: []string{"Claude Code"},
		},
		{
			name:   "unknown ai bot keeps its own name",
			commit: Commit{AuthorName: "agent-think[bot]", AuthorEmail: "298805081+agent-think[bot]@users.noreply.github.com", Message: "fix: x\n\nCo-authored-by: agent-think[bot] <agent-think[bot]@users.noreply.github.com>"},
			kind:   Agent, agents: []string{"agent-think[bot]"},
		},
		{
			name:   "gas town crew commits as anthropic noreply",
			commit: Commit{AuthorName: "sfgastown/crew/dunks", AuthorEmail: "noreply@anthropic.com", Message: "fix: y\n\nCo-Authored-By: Claude Opus 4.6 (1M context) <noreply@anthropic.com>"},
			kind:   Agent, agents: []string{"Claude Code"},
		},
		{
			name:   "two agents",
			commit: with(human, "x\n\nCo-authored-by: Claude <noreply@anthropic.com>\nCo-authored-by: Codex <noreply@openai.com>"),
			kind:   Assisted, agents: []string{"Claude Code", "Codex"}, conv: []string{"co-authored-by"},
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := classify(t, c.commit)
			if got.Kind != c.kind {
				t.Fatalf("kind = %v, want %v (evidence %+v)", got.Kind, c.kind, got.Evidence)
			}
			if !reflect.DeepEqual(got.Agents, c.agents) {
				t.Fatalf("agents = %v, want %v", got.Agents, c.agents)
			}
			if c.conv != nil && !reflect.DeepEqual(got.Conventions, c.conv) {
				t.Fatalf("conventions = %v, want %v", got.Conventions, c.conv)
			}
		})
	}
}

func with(c Commit, msg string) Commit { c.Message = msg; return c }

// TestClassifyCorpus2026 covers shapes collected from public GitHub commits
// in September 2026 (gh search commits for Assisted-by, Generated-by,
// Co-authored-by, Claude-Session; curl's history for human Assisted-by).
func TestClassifyCorpus2026(t *testing.T) {
	human := Commit{AuthorName: "Jane Doe", AuthorEmail: "jane@example.com"}
	cases := []struct {
		name    string
		commit  Commit
		kind    Kind
		agents  []string
		models  []string
		conv    []string
		ignored int
		signoff bool
	}{
		// curl uses Assisted-by for human helpers (279 commits by Sept 2026).
		{name: "curl human assisted-by", commit: with(human, "curl_fopen: restore the uid and gid checks\n\nReported-by: Stanislav Fort\nAssisted-by: Viktor Szakats\nCloses #22843"), kind: Human, ignored: 1},
		{name: "curl handle on github", commit: with(human, "fix\n\nAssisted-by: riastradh on github"), kind: Human, ignored: 1},
		{name: "curl github noreply address", commit: with(human, "fix\n\nAssisted-by: tholin@users.noreply.github.com"), kind: Human, ignored: 1},
		{name: "curl accented name", commit: with(human, "fix\n\nAssisted-by: Marc Hörsken"), kind: Human, ignored: 1},
		{name: "assisted-by none is negative", commit: with(human, "fix\n\nAssisted-by: none"), kind: Human, ignored: 1},
		{
			name:   "cinatra template with none plus real agents",
			commit: with(human, "lifecycle-b W6 part 1 (#3019)\n\nGate-suite: cinatra-core@2026.08.4\nAccountable: Sandro Groganz <sandro@cinatra.ai> (@groganz)\nAssisted-by: Claude Code\nAssisted-by: Claude Code (claude-fable-5)\nAssisted-by: Codex (gpt-5.6-sol)\nAssisted-by: none"),
			kind:   Assisted, agents: []string{"Claude Code", "Codex"}, models: []string{"claude-fable-5", "gpt-5.6-sol"}, conv: []string{"assisted-by"}, ignored: 1,
		},
		{name: "kernel 7.3 generic LLM", commit: with(human, "mm: fix\n\nAssisted-by: LLM coccinelle sparse\nSigned-off-by: Jane Doe <jane@example.com>"), kind: Assisted, agents: []string{UnknownAI}, conv: []string{"assisted-by"}},
		{name: "fedora generic chatbot", commit: with(human, "fix\n\nAssisted-by: generic LLM chatbot"), kind: Assisted, agents: []string{UnknownAI}},
		{name: "hermes agent with model", commit: with(human, "fix\n\nAssisted-by: Hermes Agent:gpt-5.6-sol"), kind: Assisted, agents: []string{"Hermes Agent"}, models: []string{"gpt-5.6-sol"}},
		{name: "openai-codex kernel style", commit: with(human, "fix\n\nAssisted-by: openai-codex:gpt-5.5 [read,bash,edit,write]"), kind: Assisted, agents: []string{"Codex"}, models: []string{"gpt-5.5"}},
		{name: "claude-code effort and roles", commit: with(human, "style: wrap\n\nAssisted-by: claude-code:opus-5#medium (implementer)\nAssisted-by: claude-code:fable-5.1#high (orchestrator, reviewer)"), kind: Assisted, agents: []string{"Claude Code"}, models: []string{"fable-5.1", "opus-5"}},
		{name: "unknown single-token tool counts", commit: with(human, "fix\n\nAssisted-by: devx/c380b28c-ab53-4c64-a5d7-b1e4b72559e9"), kind: Assisted, agents: []string{"devx"}},
		{name: "greptile undisclosed model", commit: with(human, "fix\n\nAssisted-by: Greptile:undisclosed"), kind: Assisted, agents: []string{"Greptile"}},
		{name: "gpt via codex", commit: with(human, "fix\n\nAssisted-by: GPT-5.6 Sol via Codex"), kind: Assisted, agents: []string{"Codex"}, models: []string{"GPT-5.6 Sol"}},
		{name: "bare model is GPT", commit: with(human, "fix\n\nAssisted-by: OpenAI GPT-5.6 Sol"), kind: Assisted, agents: []string{"GPT"}, models: []string{"GPT-5.6 Sol"}},
		{name: "grok vendor note", commit: with(human, "fix\n\nAssisted-by: Grok (xAI)"), kind: Assisted, agents: []string{"Grok Build"}},
		{name: "antigravity kernel style", commit: with(human, "fix\n\nAssisted-by: antigravity:gemini-3.6-flash"), kind: Assisted, agents: []string{"Antigravity"}, models: []string{"gemini-3.6-flash"}},
		{name: "chatgpt with vendor model", commit: with(human, "fix\n\nAssisted-by: ChatGPT (OpenAI GPT-5.5)"), kind: Assisted, agents: []string{"ChatGPT"}, models: []string{"GPT-5.5"}},
		{name: "assisted-by with email", commit: with(human, "fix\n\nAssisted-by: Claude Fable 5 <noreply@anthropic.com>"), kind: Assisted, agents: []string{"Claude Code"}, models: []string{"Fable 5"}},

		// Generated-by is shared with non-AI tooling.
		{name: "generated-by stagefreight is not AI", commit: with(human, "docs: refresh generated docs and badges\n\nGenerated-By: StageFreight"), kind: Human, ignored: 1},
		{name: "generated-by maka is an agent workspace", commit: with(human, "refactor: share catalog projection\n\nGenerated-by: Maka"), kind: Assisted, agents: []string{"Maka"}, conv: []string{"generated-by"}},
		{name: "generated-by posthog desktop", commit: with(human, "feat: x\n\nGenerated-by: PostHog Desktop"), kind: Assisted, agents: []string{"PostHog Desktop"}},
		{name: "generated-by codex", commit: with(human, "feat: x\n\nGenerated-by: Codex"), kind: Assisted, agents: []string{"Codex"}},
		{name: "made-with cursor trailer", commit: with(human, "feat: x\n\nMade-with: Cursor"), kind: Assisted, agents: []string{"Cursor"}, conv: []string{"made-with"}},
		{name: "made-with love is not AI", commit: with(human, "feat: x\n\nMade-with: love"), kind: Human, ignored: 1},
		{name: "ai-used-for code", commit: with(human, "fix\n\nAI-used-for: code, tests"), kind: Assisted, agents: []string{UnknownAI}, conv: []string{"ai-used-for"}},
		{name: "ai-used-for research only", commit: with(human, "fix\n\nAI-used-for: research"), kind: Human, ignored: 1},

		// Session-link trailers.
		{name: "claude-session alone", commit: with(human, "Rebuild after merging dev\n\nClaude-Session: https://claude.ai/code/session_0169ULqYpWT4UKwiizM2obZf"), kind: Assisted, agents: []string{"Claude Code"}, conv: []string{"claude-session"}},
		{name: "claude-session with co-author", commit: with(human, "x\n\nCo-Authored-By: Claude Opus 5 <noreply@anthropic.com>\nClaude-Session: https://claude.ai/code/session_01SscSgdPZx1dhGfeEvU58d3"), kind: Assisted, agents: []string{"Claude Code"}, models: []string{"Opus 5"}, conv: []string{"claude-session", "co-authored-by"}},
		{name: "entire checkpoint", commit: with(human, "fix: y\n\nEntire-Checkpoint: 01KXGTTNGCEACC83QZEJ5YAF0D"), kind: Assisted, agents: []string{UnknownAI}, conv: []string{"entire-checkpoint"}},
		{name: "amp thread id", commit: with(human, "fix\n\nAmp-Thread-ID: T-0c8c2a1e\nCo-authored-by: Amp <amp@ampcode.com>"), kind: Assisted, agents: []string{"Amp"}, conv: []string{"amp-thread-id", "co-authored-by"}},
		{
			name:   "copilot coding agent with logs url",
			commit: Commit{AuthorName: "copilot-swe-agent[bot]", AuthorEmail: "198982749+Copilot@users.noreply.github.com", Message: "Fix\n\nCo-authored-by: jane <jane@example.com>\nAgent-Logs-Url: https://github.com/o/r/sessions/abc"},
			kind:   Agent, agents: []string{"GitHub Copilot"}, conv: []string{"agent-logs-url"},
		},

		// Co-authored-by identities.
		{name: "cursor running grok under anthropic address", commit: with(human, "x\n\nCo-authored-by: Cursor Grok 4.6 <noreply@anthropic.com>"), kind: Assisted, agents: []string{"Cursor"}, models: []string{"Grok 4.6"}},
		{name: "claude code running gpt", commit: with(human, "x\n\nCo-authored-by: GPT-5.6 Sol <noreply@anthropic.com>"), kind: Assisted, agents: []string{"Claude Code"}, models: []string{"GPT-5.6 Sol"}},
		{name: "chatgpt co-author", commit: with(human, "x\n\nCo-authored-by: ChatGPT <noreply@openai.com>"), kind: Assisted, agents: []string{"ChatGPT"}},
		{name: "codebuff co-author", commit: with(human, "x\n\nCo-authored-by: Codebuff <noreply@codebuff.com>"), kind: Assisted, agents: []string{"Codebuff"}},
		{name: "claude github app", commit: with(human, "x\n\nCo-authored-by: claude[bot] <209825114+claude[bot]@users.noreply.github.com>"), kind: Assisted, agents: []string{"Claude Code"}},
		{name: "grok co-author", commit: with(human, "x\n\nCo-authored-by: Grok 4.6 <grok@x.ai>"), kind: Assisted, agents: []string{"Grok Build"}, models: []string{"Grok 4.6"}},
		{name: "copilot autofix", commit: with(human, "x\n\nCo-authored-by: Copilot Autofix powered by AI <175728472+Copilot@users.noreply.github.com>"), kind: Assisted, agents: []string{"GitHub Copilot"}},
		{name: "antigravity co-author", commit: with(human, "x\n\nCo-authored-by: Antigravity Agent <antigravity@gemini.ai>"), kind: Assisted, agents: []string{"Antigravity"}},
		{name: "greptile co-author", commit: with(human, "x\n\nCo-authored-by: greptile-apps[bot] <12345+greptile-apps[bot]@users.noreply.github.com>"), kind: Assisted, agents: []string{"Greptile"}},
		{name: "cursor placeholder address", commit: with(human, "x\n\nCo-authored-by: Cursor <cursor@agent>"), kind: Assisted, agents: []string{"Cursor"}},
		{name: "human co-author named Gemini stays human", commit: with(human, "x\n\nCo-authored-by: Gemini Rodriguez <gemini.r@example.com>"), kind: Human},
		{name: "personal helper email is human", commit: with(human, "x\n\nAssisted-by: Sam Lee <sam@example.com>"), kind: Human, ignored: 1},

		// Authors.
		{name: "codex cloud placeholder author", commit: Commit{AuthorName: "Codex Test", AuthorEmail: "codex@example.com", Message: "fix"}, kind: Agent, agents: []string{"Codex"}},
		{name: "greptile author", commit: Commit{AuthorName: "greptile-apps[bot]", AuthorEmail: "12345+greptile-apps[bot]@users.noreply.github.com", Message: "fix"}, kind: Agent, agents: []string{"Greptile"}},
		{name: "stainless is a bot", commit: Commit{AuthorName: "stainless-app[bot]", AuthorEmail: "142633134+stainless-app[bot]@users.noreply.github.com", Message: "release"}, kind: Bot},
		{name: "stagefreight author is a bot", commit: Commit{AuthorName: "stagefreight", AuthorEmail: "ci@example.com", Message: "docs: refresh generated docs and badges\n\nGenerated-By: StageFreight"}, kind: Bot, ignored: 1},
		{name: "amazon q developer bot", commit: Commit{AuthorName: "amazon-q-developer[bot]", AuthorEmail: "1+amazon-q-developer[bot]@users.noreply.github.com", Message: "fix"}, kind: Agent, agents: []string{"Kiro"}},

		// DCO: an AI must not sign off.
		{name: "agent signed off", commit: with(human, "fix\n\nSigned-off-by: Claude <noreply@anthropic.com>"), kind: Assisted, agents: []string{"Claude Code"}, conv: []string{"signed-off-by"}, signoff: true},

		// Free-text markers.
		{name: "codebuff marker", commit: with(human, "feat: add\n\n🤖 Generated with Codebuff"), kind: Assisted, agents: []string{"Codebuff"}},
		{name: "generated with help of", commit: with(human, "feat: add\n\nGenerated with help of Claude Code"), kind: Assisted, agents: []string{"Claude Code"}},
		{name: "generated with devin link", commit: with(human, "feat: add\n\nGenerated with [Devin](https://devin.ai)"), kind: Assisted, agents: []string{"Devin"}},
		{name: "generated with love is human", commit: with(human, "feat: add\n\nGenerated with love"), kind: Human},
		{name: "generated with unknown ai agent note", commit: with(human, "feat: add\n\nGenerated with omp (AI coding agent)"), kind: Assisted, agents: []string{"omp"}},
		{name: "vibe coded by", commit: with(human, "feat: add\n\nvibe-coded by Claude Code"), kind: Assisted, agents: []string{"Claude Code"}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := classify(t, c.commit)
			if got.Kind != c.kind {
				t.Fatalf("kind = %v, want %v (evidence %+v ignored %+v)", got.Kind, c.kind, got.Evidence, got.Ignored)
			}
			if c.agents != nil && !reflect.DeepEqual(got.Agents, c.agents) {
				t.Fatalf("agents = %v, want %v (evidence %+v)", got.Agents, c.agents, got.Evidence)
			}
			if c.models != nil && !reflect.DeepEqual(got.Models, c.models) {
				t.Fatalf("models = %v, want %v", got.Models, c.models)
			}
			if c.conv != nil && !reflect.DeepEqual(got.Conventions, c.conv) {
				t.Fatalf("conventions = %v, want %v", got.Conventions, c.conv)
			}
			if len(got.Ignored) != c.ignored {
				t.Fatalf("ignored = %+v, want %d entries", got.Ignored, c.ignored)
			}
			if got.AgentSignedOff != c.signoff {
				t.Fatalf("agent_signed_off = %v, want %v", got.AgentSignedOff, c.signoff)
			}
		})
	}
}

// TestVendorEmployeesStayHuman: a person at an agent vendor committing from
// a corporate mailbox is not the agent (review finding, 1.1.0).
func TestVendorEmployeesStayHuman(t *testing.T) {
	for _, c := range []Commit{
		{AuthorName: "Jane Doe", AuthorEmail: "jane.doe@codeium.com", Message: "fix"},
		{AuthorName: "Li Wei", AuthorEmail: "li.wei@moonshot.ai", Message: "fix"},
		{AuthorName: "Sam Human", AuthorEmail: "sam@tabnine.com", Message: "fix"},
		{AuthorName: "Ann", AuthorEmail: "ann@devin.ai", Message: "fix"},
		{AuthorName: "Bob", AuthorEmail: "bob@aider.chat", Message: "fix"},
		{AuthorName: "Cy", AuthorEmail: "cy@continue.dev", Message: "fix"},
	} {
		if got := classify(t, c); got.Kind != Human {
			t.Errorf("%s <%s> classified as %v %v", c.AuthorName, c.AuthorEmail, got.Kind, got.Agents)
		}
	}
	// Documented no-reply addresses and GitHub bot suffixes still match.
	for _, c := range []Commit{
		{AuthorName: "x", AuthorEmail: "noreply@aider.chat", Message: "fix"},
		{AuthorName: "x", AuthorEmail: "noreply@cline.bot", Message: "fix"},
		{AuthorName: "x", AuthorEmail: "198982749+Copilot@users.noreply.github.com", Message: "fix"},
		{AuthorName: "sfgastown/crew/dunks", AuthorEmail: "noreply@anthropic.com", Message: "fix"},
	} {
		if got := classify(t, c); got.Kind != Agent {
			t.Errorf("%s should be an agent, got %v", c.AuthorEmail, got.Kind)
		}
	}
	// A one-word human name with a personal address in Assisted-by is a person.
	got := classify(t, Commit{AuthorName: "J", AuthorEmail: "j@example.com", Message: "fix bug\n\nAssisted-by: Fabrice <fabrice@example.com>"})
	if got.Kind != Human || len(got.Ignored) != 1 || got.Ignored[0].Reason != ReasonPerson {
		t.Fatalf("mononym helper: %+v", got)
	}
	// …but an unknown tool with an automated address still counts.
	got = classify(t, Commit{AuthorName: "J", AuthorEmail: "j@example.com", Message: "fix\n\nAssisted-by: Acme <noreply@acme.example>"})
	if got.Kind != Assisted || got.PrimaryAgent() != "Acme" {
		t.Fatalf("unknown tool with noreply address: %+v", got)
	}
}

func TestLooksLikePerson(t *testing.T) {
	yes := []string{"Jay Satiro", "Viktor Szakats", "Marc Hörsken", "riastradh on github", "tholin@users.noreply.github.com", "Sam Lee <sam@example.com>", "marc-groundctl@users.noreply.github.com"}
	// Known agents ("Amazon Q", "Claude Code") are excluded by the caller
	// before this check; two plain words can still be a tool name, which is
	// why the identity table matters.
	no := []string{"Codex", "Hermes Agent", "generic LLM chatbot", "Local LLM fuzzer", "Bynario AI", "devx/abc12345", "Claude:claude-opus-5", "Codex (gpt-5)", "LLM coccinelle sparse", "Claude <noreply@anthropic.com>", "claude[bot] <1+claude[bot]@users.noreply.github.com>"}
	for _, s := range yes {
		if !LooksLikePerson(s) {
			t.Errorf("%q should look like a person", s)
		}
	}
	for _, s := range no {
		if LooksLikePerson(s) {
			t.Errorf("%q should not look like a person", s)
		}
	}
}

func TestMergeAndSetAgent(t *testing.T) {
	att := classify(t, Commit{AuthorName: "Jane", AuthorEmail: "jane@example.com", Message: "fix"})
	att.Merge(Evidence{Source: "notes:git-ai", Value: "s_1", Agent: "Cursor", Model: "claude-sonnet-4-5"}, "git-ai-notes")
	if att.Kind != Assisted || att.PrimaryAgent() != "Cursor" || !att.HasConvention("git-ai-notes") || att.Models[0] != "claude-sonnet-4-5" {
		t.Fatalf("merge: %+v", att)
	}
	bot := classify(t, Commit{AuthorName: "dependabot[bot]", AuthorEmail: "1+dependabot[bot]@users.noreply.github.com", Message: "bump"})
	bot.Merge(Evidence{Source: "notes:git-ai", Agent: "Cursor"}, "git-ai-notes")
	if bot.Kind != Bot {
		t.Fatalf("bot kind changed by merge: %v", bot.Kind)
	}
	cp := classify(t, Commit{AuthorName: "Jane", AuthorEmail: "jane@example.com", Message: "fix\n\nEntire-Checkpoint: 01KX"})
	cp.SetAgent("trailer:Entire-Checkpoint", "Claude Code", "claude-fable-5")
	if !reflect.DeepEqual(cp.Agents, []string{"Claude Code"}) || !reflect.DeepEqual(cp.Models, []string{"claude-fable-5"}) {
		t.Fatalf("set agent: %+v", cp)
	}
}

// TestEveryIdentityHasAnExample guards the rule in AGENTS.md: each built-in
// agent must be reachable from at least one name or e-mail in the table.
func TestEveryIdentityIsMatchable(t *testing.T) {
	for _, id := range KnownAgents {
		if id.Name == "Generic AI" {
			continue
		}
		var ok bool
		for _, n := range id.Names {
			if got := LookupAgent(n, "", nil); got != nil && got.Name == id.Name {
				ok = true
				break
			}
		}
		for _, e := range id.Emails {
			if got := LookupAgent("", e, nil); got != nil && got.Name == id.Name {
				ok = true
			}
		}
		if !ok {
			t.Errorf("identity %q cannot be matched by any of its own names/e-mails (shadowed by an earlier entry?)", id.Name)
		}
	}
}

func TestClassifyExtraIdentitiesAndTrailers(t *testing.T) {
	cl := NewClassifier(Options{
		ExtraAgents:      []Identity{{Name: "CorpBot", Emails: []string{"agent@corp.example"}}},
		ExtraBots:        []Identity{{Name: "CI", Emails: []string{"ci@corp.example"}, IsBot: true}},
		ExtraTrailerKeys: []string{"X-Generated-By"},
		MessageMarkers:   []string{`(?i)\[(?P<agent>robo)\]`},
	})
	if got := cl.Classify(Commit{AuthorName: "x", AuthorEmail: "agent@corp.example"}); got.Kind != Agent || got.PrimaryAgent() != "CorpBot" {
		t.Fatalf("extra agent: %+v", got)
	}
	if got := cl.Classify(Commit{AuthorName: "x", AuthorEmail: "ci@corp.example"}); got.Kind != Bot {
		t.Fatalf("extra bot: %+v", got)
	}
	if got := cl.Classify(Commit{AuthorName: "h", AuthorEmail: "h@h", Message: "x\n\nX-Generated-By: MyTool (m1)"}); got.Kind != Assisted || got.PrimaryAgent() != "MyTool" || got.Models[0] != "m1" {
		t.Fatalf("extra trailer: %+v", got)
	}
	if got := cl.Classify(Commit{AuthorName: "h", AuthorEmail: "h@h", Message: "[robo] did it"}); got.Kind != Assisted || got.PrimaryAgent() != "robo" {
		t.Fatalf("extra marker: %+v", got)
	}
}

func TestKindText(t *testing.T) {
	for _, k := range Kinds() {
		var back Kind
		b, _ := k.MarshalText()
		if err := back.UnmarshalText(b); err != nil || back != k {
			t.Fatalf("round trip %v failed: %v", k, err)
		}
	}
	var k Kind
	if err := k.UnmarshalText([]byte("nope")); err == nil {
		t.Fatal("expected error")
	}
	if !Assisted.IsAI() || !Agent.IsAI() || Human.IsAI() || Bot.IsAI() {
		t.Fatal("IsAI wrong")
	}
}
