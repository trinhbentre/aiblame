package attrib

import (
	"regexp"
	"strings"
)

// Identity describes one known coding agent (or one known non-AI bot).
type Identity struct {
	// Name is the canonical display name, e.g. "Claude Code".
	Name string
	// Vendor is the organisation behind the agent.
	Vendor string
	// Emails are exact (case-insensitive) e-mail addresses used by the agent.
	Emails []string
	// EmailSuffixes match the end of an e-mail address, e.g.
	// "+copilot@users.noreply.github.com".
	EmailSuffixes []string
	// Names are exact (case-insensitive) author / trailer names.
	Names []string
	// NamePattern is an optional regexp applied to the lower-cased name.
	NamePattern *regexp.Regexp
	// TrailerEmail is the e-mail aiblame uses when it writes a
	// Co-authored-by trailer for this agent (see the hook subcommand).
	TrailerEmail string
	// IsBot marks non-AI automation (dependabot, renovate…). These are
	// classified as Bot, not as AI.
	IsBot bool
}

func rx(s string) *regexp.Regexp { return regexp.MustCompile(s) }

// KnownAgents is the built-in table of AI coding agents. Order matters only
// for display; matching checks all entries.
//
// Sources for each entry are documented in docs/DETECTION.md.
var KnownAgents = []Identity{
	{
		Name: "Claude Code", Vendor: "Anthropic",
		Emails:       []string{"noreply@anthropic.com"},
		Names:        []string{"claude", "claude code", "claude[bot]", "claude-code"},
		NamePattern:  rx(`^claude(\s+(code|opus|sonnet|haiku|fable|mythos)\b.*)?$`),
		TrailerEmail: "noreply@anthropic.com",
	},
	{
		Name: "Codex", Vendor: "OpenAI",
		Emails:       []string{"noreply@openai.com", "codex@openai.com"},
		Names:        []string{"codex", "openai codex", "codex cli", "codex[bot]", "chatgpt-codex-connector[bot]"},
		NamePattern:  rx(`^(openai\s+)?codex(\s+cli)?(\s*\(.*\))?$|^chatgpt-codex`),
		TrailerEmail: "noreply@openai.com",
	},
	{
		Name: "Cursor", Vendor: "Cursor",
		Emails:       []string{"cursoragent@cursor.com", "agent@cursor.com"},
		Names:        []string{"cursor", "cursor agent", "cursor[bot]", "cursoragent"},
		TrailerEmail: "cursoragent@cursor.com",
	},
	{
		Name: "GitHub Copilot", Vendor: "GitHub",
		EmailSuffixes: []string{"+copilot@users.noreply.github.com", "+copilot[bot]@users.noreply.github.com", "+copilot-swe-agent[bot]@users.noreply.github.com"},
		Names:         []string{"copilot", "github copilot", "copilot-swe-agent[bot]", "copilot[bot]", "copilot-swe-agent"},
		TrailerEmail:  "175728472+Copilot@users.noreply.github.com",
	},
	{
		Name: "Devin", Vendor: "Cognition",
		EmailSuffixes: []string{"+devin-ai-integration[bot]@users.noreply.github.com"},
		Names:         []string{"devin", "devin-ai-integration[bot]", "devin ai"},
		NamePattern:   rx(`^devin(-ai)?(\s|-|\[|$)`),
		TrailerEmail:  "devin-ai-integration[bot]@users.noreply.github.com",
	},
	{
		Name: "Gemini CLI", Vendor: "Google",
		Names:         []string{"gemini", "gemini cli", "gemini-cli", "gemini code assist", "gemini-code-assist[bot]"},
		EmailSuffixes: []string{"+gemini-code-assist[bot]@users.noreply.github.com"},
		TrailerEmail:  "gemini-cli@google.com",
	},
	{
		Name: "Jules", Vendor: "Google",
		Names:         []string{"jules", "google-labs-jules[bot]", "google jules"},
		EmailSuffixes: []string{"+google-labs-jules[bot]@users.noreply.github.com"},
		TrailerEmail:  "google-labs-jules[bot]@users.noreply.github.com",
	},
	{
		Name: "Antigravity", Vendor: "Google",
		Names:        []string{"antigravity", "google antigravity"},
		TrailerEmail: "antigravity@google.com",
	},
	{
		Name: "aider", Vendor: "Aider AI",
		Emails:       []string{"noreply@aider.chat"},
		Names:        []string{"aider"},
		NamePattern:  rx(`^aider(\s*\(.*\))?$|\(aider\)$`),
		TrailerEmail: "noreply@aider.chat",
	},
	{
		Name: "OpenCode", Vendor: "SST / Anomaly",
		Emails:       []string{"noreply@opencode.ai", "opencode@sst.dev"},
		Names:        []string{"opencode", "opencode-agent[bot]", "open code"},
		NamePattern:  rx(`^opencode(\s+v?\d.*)?$`),
		TrailerEmail: "noreply@opencode.ai",
	},
	{
		Name: "Amp", Vendor: "Sourcegraph",
		Emails:       []string{"amp@ampcode.com", "noreply@ampcode.com"},
		Names:        []string{"amp", "amp agent", "amp-agent"},
		TrailerEmail: "amp@ampcode.com",
	},
	{
		Name: "Cline", Vendor: "Cline",
		Emails:       []string{"noreply@cline.bot"},
		Names:        []string{"cline", "cline[bot]"},
		TrailerEmail: "noreply@cline.bot",
	},
	{
		Name: "Roo Code", Vendor: "Roo Code",
		Names:        []string{"roo code", "roo-code", "roocode", "roo"},
		TrailerEmail: "noreply@roocode.com",
	},
	{
		Name: "Kilo Code", Vendor: "Kilo",
		Names:        []string{"kilo code", "kilocode", "kilo-code"},
		TrailerEmail: "noreply@kilocode.ai",
	},
	{
		Name: "Windsurf", Vendor: "Windsurf",
		Names:        []string{"windsurf", "cascade", "windsurf cascade", "codeium"},
		TrailerEmail: "noreply@windsurf.com",
	},
	{
		Name: "Kiro", Vendor: "AWS",
		Names:        []string{"kiro", "kiro agent", "amazon q", "amazon q developer"},
		TrailerEmail: "noreply@kiro.dev",
	},
	{
		Name: "Junie", Vendor: "JetBrains",
		Names:        []string{"junie", "jetbrains junie"},
		TrailerEmail: "noreply@jetbrains.com",
	},
	{
		Name: "Goose", Vendor: "Block",
		Names:        []string{"goose", "block goose", "codename goose"},
		TrailerEmail: "noreply@block.xyz",
	},
	{
		Name: "Warp", Vendor: "Warp",
		Names:        []string{"warp", "warp agent", "oz"},
		TrailerEmail: "noreply@warp.dev",
	},
	{
		Name: "Zed Agent", Vendor: "Zed",
		Names:        []string{"zed", "zed agent", "zed ai"},
		TrailerEmail: "noreply@zed.dev",
	},
	{
		Name: "Augment", Vendor: "Augment Code",
		Names:        []string{"augment", "augment code", "augment-agent", "auggie"},
		TrailerEmail: "noreply@augmentcode.com",
	},
	{
		Name: "Droid", Vendor: "Factory",
		Names:        []string{"droid", "factory droid", "factory"},
		TrailerEmail: "noreply@factory.ai",
	},
	{
		Name: "Hermes Agent", Vendor: "Nous Research",
		Names:        []string{"hermes", "hermes agent", "hermes-agent"},
		TrailerEmail: "noreply@nousresearch.com",
	},
	{
		Name: "Pi", Vendor: "pi.dev",
		Names:        []string{"pi", "pi coding agent", "pi-coding-agent"},
		TrailerEmail: "noreply@pi.dev",
	},
	{
		Name: "Kimi Code", Vendor: "Moonshot AI",
		Names:        []string{"kimi", "kimi code", "kimi-code", "kimi cli"},
		TrailerEmail: "noreply@moonshot.cn",
	},
	{
		Name: "Qwen Code", Vendor: "Alibaba",
		Names:        []string{"qwen", "qwen code", "qwen-code"},
		TrailerEmail: "noreply@alibabacloud.com",
	},
	{
		Name: "DeepSeek Harness", Vendor: "DeepSeek",
		Names:        []string{"deepseek", "deepseek harness", "dsh"},
		TrailerEmail: "noreply@deepseek.com",
	},
	{
		Name: "Grok Build", Vendor: "xAI",
		Names:        []string{"grok", "grok build", "grok-build"},
		TrailerEmail: "noreply@x.ai",
	},
	{
		Name: "Mistral Vibe", Vendor: "Mistral AI",
		Names:        []string{"mistral vibe", "vibe", "devstral"},
		TrailerEmail: "noreply@mistral.ai",
	},
	{
		Name: "Trae", Vendor: "ByteDance",
		Names:        []string{"trae", "trae agent", "trae-agent"},
		TrailerEmail: "noreply@trae.ai",
	},
	{
		Name: "Replit Agent", Vendor: "Replit",
		Names:        []string{"replit", "replit agent", "replit-agent"},
		TrailerEmail: "noreply@replit.com",
	},
	{
		Name: "Lovable", Vendor: "Lovable",
		Names:         []string{"lovable", "lovable-dev[bot]", "gpt-engineer-app[bot]"},
		EmailSuffixes: []string{"+lovable-dev[bot]@users.noreply.github.com", "+gpt-engineer-app[bot]@users.noreply.github.com"},
		TrailerEmail:  "noreply@lovable.dev",
	},
	{
		Name: "Bolt", Vendor: "StackBlitz",
		Names:        []string{"bolt", "bolt.new", "bolt-new"},
		TrailerEmail: "noreply@bolt.new",
	},
	{
		Name: "v0", Vendor: "Vercel",
		Names:         []string{"v0", "v0[bot]", "v0.dev"},
		EmailSuffixes: []string{"+v0[bot]@users.noreply.github.com"},
		TrailerEmail:  "noreply@v0.dev",
	},
	{
		Name: "Sweep", Vendor: "Sweep",
		Names:         []string{"sweep", "sweep-ai[bot]", "sweep ai"},
		EmailSuffixes: []string{"+sweep-ai[bot]@users.noreply.github.com"},
		TrailerEmail:  "sweep-ai[bot]@users.noreply.github.com",
	},
	{
		Name: "CodeRabbit", Vendor: "CodeRabbit",
		Names:         []string{"coderabbit", "coderabbitai[bot]", "coderabbitai"},
		EmailSuffixes: []string{"+coderabbitai[bot]@users.noreply.github.com"},
		TrailerEmail:  "coderabbitai[bot]@users.noreply.github.com",
	},
	{
		Name: "Sourcery", Vendor: "Sourcery",
		Names:         []string{"sourcery", "sourcery-ai[bot]", "sourcery-ai"},
		EmailSuffixes: []string{"+sourcery-ai[bot]@users.noreply.github.com"},
		TrailerEmail:  "sourcery-ai[bot]@users.noreply.github.com",
	},
	{
		Name: "Ellipsis", Vendor: "Ellipsis",
		Names:         []string{"ellipsis", "ellipsis-dev[bot]"},
		EmailSuffixes: []string{"+ellipsis-dev[bot]@users.noreply.github.com"},
		TrailerEmail:  "ellipsis-dev[bot]@users.noreply.github.com",
	},
	{
		Name: "Codegen", Vendor: "Codegen",
		Names:         []string{"codegen", "codegen-sh[bot]", "codegen-sh"},
		EmailSuffixes: []string{"+codegen-sh[bot]@users.noreply.github.com"},
		TrailerEmail:  "codegen-sh[bot]@users.noreply.github.com",
	},
	{
		Name: "Qodo", Vendor: "Qodo",
		Names:         []string{"qodo", "qodo-merge-pro[bot]", "codiumai", "pr-agent"},
		EmailSuffixes: []string{"+qodo-merge-pro[bot]@users.noreply.github.com"},
		TrailerEmail:  "noreply@qodo.ai",
	},
	{
		Name: "Cody", Vendor: "Sourcegraph",
		Names:        []string{"cody", "sourcegraph cody"},
		TrailerEmail: "noreply@sourcegraph.com",
	},
	{
		Name: "Continue", Vendor: "Continue",
		Names:        []string{"continue", "continue agent", "continue-agent"},
		TrailerEmail: "noreply@continue.dev",
	},
	{
		Name: "Tabnine", Vendor: "Tabnine",
		Names:        []string{"tabnine", "tabnine agent"},
		TrailerEmail: "noreply@tabnine.com",
	},
	{
		Name: "Blackbox", Vendor: "Blackbox AI",
		Names:        []string{"blackbox", "blackbox ai", "blackboxai"},
		TrailerEmail: "noreply@blackbox.ai",
	},
	{
		Name: "Mentat", Vendor: "AbanteAI",
		Names:        []string{"mentat", "mentatbot", "mentatbot[bot]"},
		TrailerEmail: "noreply@mentat.ai",
	},
	{
		Name: "OpenHands", Vendor: "All Hands AI",
		Names:        []string{"openhands", "openhands-agent", "openhands[bot]", "opendevin"},
		Emails:       []string{"openhands@all-hands.dev"},
		TrailerEmail: "openhands@all-hands.dev",
	},
	{
		Name: "SWE-agent", Vendor: "Princeton / Stanford",
		Names:        []string{"swe-agent", "sweagent"},
		TrailerEmail: "noreply@swe-agent.com",
	},
	{
		Name: "Manus", Vendor: "Manus",
		Names:        []string{"manus", "manus agent"},
		TrailerEmail: "noreply@manus.im",
	},
	{
		Name: "Ona", Vendor: "Ona (Gitpod)",
		Names:        []string{"ona", "ona agent", "gitpod agent"},
		TrailerEmail: "noreply@ona.com",
	},
	{
		Name: "GitLab Duo", Vendor: "GitLab",
		Names:        []string{"gitlab duo", "duo", "gitlab-duo"},
		TrailerEmail: "noreply@gitlab.com",
	},
	{
		Name: "Generic AI", Vendor: "",
		NamePattern:  rx(`(^|[\s\-_])(ai|llm|gpt|assistant|agent)([\s\-_\[]|$)`),
		TrailerEmail: "",
	},
}

// KnownBots lists non-AI automation identities. They are reported as Bot so
// that dependency bumps and CI commits do not inflate the human bucket.
var KnownBots = []Identity{
	{Name: "Dependabot", Names: []string{"dependabot", "dependabot[bot]", "dependabot-preview[bot]"}, EmailSuffixes: []string{"+dependabot[bot]@users.noreply.github.com", "@dependabot.com"}, IsBot: true},
	{Name: "Renovate", Names: []string{"renovate", "renovate[bot]", "renovate-bot", "self-hosted renovate"}, EmailSuffixes: []string{"+renovate[bot]@users.noreply.github.com", "@renovateapp.com", "@renovatebot.com", "@mend.io"}, IsBot: true},
	{Name: "GitHub Actions", Names: []string{"github-actions", "github-actions[bot]", "github actions"}, Emails: []string{"41898282+github-actions[bot]@users.noreply.github.com", "actions@github.com"}, EmailSuffixes: []string{"+github-actions[bot]@users.noreply.github.com"}, IsBot: true},
	{Name: "pre-commit.ci", Names: []string{"pre-commit-ci[bot]", "pre-commit-ci"}, EmailSuffixes: []string{"+pre-commit-ci[bot]@users.noreply.github.com"}, IsBot: true},
	{Name: "Mergify", Names: []string{"mergify[bot]", "mergify"}, EmailSuffixes: []string{"+mergify[bot]@users.noreply.github.com"}, IsBot: true},
	{Name: "release-please", Names: []string{"release-please[bot]", "release-please"}, EmailSuffixes: []string{"+release-please[bot]@users.noreply.github.com"}, IsBot: true},
	{Name: "semantic-release", Names: []string{"semantic-release-bot", "semantic-release"}, Emails: []string{"semantic-release-bot@martynus.net"}, IsBot: true},
	{Name: "Snyk", Names: []string{"snyk-bot", "snyk[bot]"}, Emails: []string{"snyk-bot@snyk.io"}, IsBot: true},
	{Name: "Greenkeeper", Names: []string{"greenkeeper[bot]", "greenkeeperio-bot"}, IsBot: true},
	{Name: "ImgBot", Names: []string{"imgbot[bot]", "imgbot"}, IsBot: true},
	{Name: "allcontributors", Names: []string{"allcontributors[bot]"}, IsBot: true},
	{Name: "Weblate", Names: []string{"weblate", "weblate (bot)", "hosted weblate"}, EmailSuffixes: []string{"@weblate.org"}, IsBot: true},
	{Name: "Transifex", Names: []string{"transifex-integration[bot]", "transifex"}, IsBot: true},
	{Name: "Crowdin", Names: []string{"crowdin-bot", "crowdin bot"}, IsBot: true},
	{Name: "Codecov", Names: []string{"codecov[bot]", "codecov-commenter"}, IsBot: true},
	{Name: "Stale", Names: []string{"stale[bot]"}, IsBot: true},
	{Name: "Yoshi", Names: []string{"yoshi-automation", "yoshi-code-bot", "gcf-owl-bot[bot]", "release-please[bot]"}, EmailSuffixes: []string{"@google.com"}, NamePattern: rx(`^(yoshi|gcf-owl-bot)`), IsBot: true},
	{Name: "Generic bot", NamePattern: rx(`\[bot\]$|(^|[\s\-_])bot$`), IsBot: true},
}

// matchIdentity reports whether name/email (already lower-cased and trimmed)
// match the identity.
func (id Identity) matches(name, email string) bool {
	for _, e := range id.Emails {
		if email != "" && email == strings.ToLower(e) {
			return true
		}
	}
	for _, s := range id.EmailSuffixes {
		if email != "" && strings.HasSuffix(email, strings.ToLower(s)) {
			return true
		}
	}
	for _, n := range id.Names {
		if name != "" && name == strings.ToLower(n) {
			return true
		}
	}
	if id.NamePattern != nil && name != "" && id.NamePattern.MatchString(name) {
		return true
	}
	return false
}

// LookupAgent returns the canonical agent identity for a raw name/email pair,
// or nil when it is not a known AI agent. The Generic AI entry is only
// consulted when the e-mail is empty or a noreply address, to avoid
// misclassifying humans with "ai" in their name.
func LookupAgent(name, email string, extra []Identity) *Identity {
	n := normName(name)
	e := strings.ToLower(strings.TrimSpace(email))
	for i := range extra {
		if extra[i].matches(n, e) {
			return &extra[i]
		}
	}
	for i := range KnownAgents {
		id := &KnownAgents[i]
		if id.Name == "Generic AI" {
			if !looksAutomated(e) {
				continue
			}
			// Guard: only accept the generic rule for short tool-like names.
			if len(strings.Fields(n)) > 3 {
				continue
			}
		}
		if id.matches(n, e) {
			return id
		}
	}
	return nil
}

// LookupBot returns the matching non-AI bot identity, or nil.
func LookupBot(name, email string, extra []Identity) *Identity {
	n := normName(name)
	e := strings.ToLower(strings.TrimSpace(email))
	for i := range extra {
		if extra[i].matches(n, e) {
			return &extra[i]
		}
	}
	for i := range KnownBots {
		if KnownBots[i].matches(n, e) {
			return &KnownBots[i]
		}
	}
	return nil
}

// looksAutomated reports whether an e-mail looks like a noreply / bot
// address rather than a personal mailbox.
func looksAutomated(email string) bool {
	if email == "" {
		return true
	}
	return strings.Contains(email, "noreply") || strings.Contains(email, "no-reply") ||
		strings.Contains(email, "[bot]") || strings.HasSuffix(email, "@localhost")
}

func normName(name string) string {
	n := strings.ToLower(strings.TrimSpace(name))
	n = strings.Join(strings.Fields(n), " ")
	return n
}
