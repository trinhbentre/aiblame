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
			kind:   Assisted, agents: []string{"cursor-agent"}, conv: []string{"generated-by"},
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
