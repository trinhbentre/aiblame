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
	// PatternWithPersonalEmail lets NamePattern match even when the e-mail
	// is a personal mailbox. Off by default so that a person called Gemini
	// stays human; on for aider, which appends "(aider)" to the human
	// author's own name and address.
	PatternWithPersonalEmail bool
	// TrailerEmail is the e-mail aiblame uses when it writes a
	// Co-authored-by trailer for this agent (see the hook subcommand).
	TrailerEmail string
	// IsBot marks non-AI automation (dependabot, renovate…). These are
	// classified as Bot, not as AI.
	IsBot bool
}

func rx(s string) *regexp.Regexp { return regexp.MustCompile(s) }

// KnownAgents is the built-in table of AI coding agents. Order matters for
// two things: display, and which identity wins when several match the same
// name (earlier wins). Matching by name/pattern is tried for every identity
// before matching by e-mail, so "Cursor Grok 4.6 <noreply@anthropic.com>" is
// Cursor, not Claude Code.
//
// Rules of the table (learned from Liu et al. 2026 and from false positives
// seen in the wild): never match a bare person-like name without a domain or
// "[bot]" anchor unless the word is unambiguous as an author name; e-mail
// patterns always contain "@"; do not match a vendor's employees
// (no "@sourcegraph.com"-style suffixes for tools that share a company
// domain with humans).
//
// Sources for each entry are documented in docs/DETECTION.md. Every entry
// has at least one real-world example in attrib_test.go.
var KnownAgents = []Identity{
	{
		Name: "Claude Code", Vendor: "Anthropic",
		Emails:        []string{"noreply@anthropic.com", "claude@anthropic.ai", "claude@users.noreply.github.com"},
		EmailSuffixes: []string{"+claude[bot]@users.noreply.github.com"},
		Names:         []string{"claude", "claude code", "claude[bot]", "claude-code", "claudecode", "claude code agent"},
		NamePattern:   rx(`^claude([\s-]+(code|opus|sonnet|haiku|fable|mythos)\b.*)?$`),
		TrailerEmail:  "noreply@anthropic.com",
	},
	{
		Name: "Codex", Vendor: "OpenAI",
		// codex@example.com is the placeholder identity early Codex cloud
		// environments committed with ("Codex Test").
		Emails:        []string{"noreply@openai.com", "codex@openai.com", "codex@example.com", "codex@agent"},
		EmailSuffixes: []string{"+codex[bot]@users.noreply.github.com", "+chatgpt-codex-connector[bot]@users.noreply.github.com"},
		Names:         []string{"codex", "openai codex", "openai-codex", "codex cli", "codex test", "codex[bot]", "chatgpt-codex-connector[bot]", "gpt-codex"},
		NamePattern:   rx(`^(openai[\s-]+)?codex(\b.*)?$|^chatgpt-codex|\bcodex\b`),
		TrailerEmail:  "noreply@openai.com",
	},
	{
		Name: "ChatGPT", Vendor: "OpenAI",
		Names:        []string{"chatgpt", "openai chatgpt", "chat gpt"},
		NamePattern:  rx(`^chatgpt\b`),
		TrailerEmail: "noreply@openai.com",
	},
	{
		// A bare OpenAI model name ("GPT-5.6 Sol", "gpt-5.5") with no tool.
		Name: "GPT", Vendor: "OpenAI",
		Names:        []string{"gpt", "openai", "openai gpt"},
		NamePattern:  rx(`^(openai\s+)?gpt[-\s]?\d`),
		TrailerEmail: "noreply@openai.com",
	},
	{
		Name: "Cursor", Vendor: "Cursor",
		Emails:        []string{"cursoragent@cursor.com", "agent@cursor.com", "cursor@agent"},
		EmailSuffixes: []string{"cursoragent@users.noreply.github.com"},
		Names:         []string{"cursor", "cursor agent", "cursor[bot]", "cursoragent", "cursor-agent", "cursor cli"},
		NamePattern:   rx(`^cursor(\b.*)?$`),
		TrailerEmail:  "cursoragent@cursor.com",
	},
	{
		Name: "GitHub Copilot", Vendor: "GitHub",
		Emails:        []string{"copilot@github.com"},
		EmailSuffixes: []string{"+copilot@users.noreply.github.com", "+copilot[bot]@users.noreply.github.com", "+copilot-swe-agent[bot]@users.noreply.github.com", "+github-advanced-security[bot]@users.noreply.github.com"},
		Names:         []string{"copilot", "github copilot", "copilot-swe-agent[bot]", "copilot[bot]", "copilot-swe-agent", "copilot cli", "copilot coding agent", "copilot-bot", "github-advanced-security[bot]"},
		NamePattern:   rx(`^(github\s+)?copilot(\b.*)?$`),
		TrailerEmail:  "175728472+Copilot@users.noreply.github.com",
	},
	{
		Name: "Devin", Vendor: "Cognition",
		EmailSuffixes: []string{"+devin-ai-integration[bot]@users.noreply.github.com"},
		Names:         []string{"devin", "devin-ai-integration[bot]", "devin ai", "cognition-devin[bot]"},
		NamePattern:   rx(`^devin(-ai)?(\s|-|\[|$)`),
		TrailerEmail:  "devin-ai-integration[bot]@users.noreply.github.com",
	},
	{
		Name: "Gemini CLI", Vendor: "Google",
		Emails:        []string{"gemini-cli-agent@google.com", "gemini-cli-robot@google.com", "gemini-cli@google.com", "gemini@google.com", "noreply@gemini.google.com"},
		EmailSuffixes: []string{"+gemini-code-assist[bot]@users.noreply.github.com", "+gemini-cli[bot]@users.noreply.github.com"},
		Names:         []string{"gemini", "gemini cli", "gemini-cli", "gemini code assist", "gemini-code-assist[bot]", "gemini-cli[bot]", "gemini[bot]", "google gemini", "gemini cli agent"},
		NamePattern:   rx(`^(google\s+)?gemini(\b.*)?$`),
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
		Emails:       []string{"antigravity@gemini.ai", "antigravity@google.com"},
		Names:        []string{"antigravity", "google antigravity", "antigravity agent", "antigravity cli"},
		NamePattern:  rx(`^antigravity\b`),
		TrailerEmail: "antigravity@google.com",
	},
	{
		Name: "aider", Vendor: "Aider AI",
		Emails:                   []string{"noreply@aider.chat", "aider@aider.chat"},
		Names:                    []string{"aider", "aider-chat-bot"},
		NamePattern:              rx(`^aider(\s*\(.*\))?$|\(aider\)$`),
		PatternWithPersonalEmail: true,
		TrailerEmail:             "noreply@aider.chat",
	},
	{
		Name: "OpenCode", Vendor: "SST / Anomaly",
		Emails:       []string{"noreply@opencode.ai", "opencode@sst.dev"},
		Names:        []string{"opencode", "opencode-agent[bot]", "open code"},
		NamePattern:  rx(`^opencode(\b.*)?$`),
		TrailerEmail: "noreply@opencode.ai",
	},
	{
		Name: "Amp", Vendor: "Amp (ex-Sourcegraph)",
		Emails:       []string{"amp@ampcode.com", "noreply@ampcode.com"},
		Names:        []string{"amp", "amp agent", "amp-agent", "amp code"},
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
		Names:        []string{"windsurf", "cascade", "windsurf cascade", "codeium", "codeium[bot]", "windsurf[bot]"},
		TrailerEmail: "noreply@windsurf.com",
	},
	{
		Name: "Kiro", Vendor: "AWS",
		Names:        []string{"kiro", "kiro agent", "kiro cli", "amazon q", "amazon q developer", "amazon-q-developer[bot]", "amazonq[bot]", "amazon-q[bot]", "amazon-codecatalyst[bot]"},
		NamePattern:  rx(`^amazon-?q\b`),
		TrailerEmail: "noreply@kiro.dev",
	},
	{
		Name: "Junie", Vendor: "JetBrains",
		Names:        []string{"junie", "jetbrains junie"},
		TrailerEmail: "noreply@jetbrains.com",
	},
	{
		Name: "Goose", Vendor: "Block",
		Emails:       []string{"goose@opensource.block.xyz", "goose@example.com"},
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
		Names:        []string{"augment", "augment code", "augment-agent", "auggie", "augmentcode[bot]"},
		TrailerEmail: "noreply@augmentcode.com",
	},
	{
		Name: "Droid", Vendor: "Factory",
		Emails:       []string{"droid@factory.ai"},
		Names:        []string{"droid", "factory droid", "factory", "factoryai-droid", "factory ai droid"},
		TrailerEmail: "noreply@factory.ai",
	},
	{
		Name: "Hermes Agent", Vendor: "Nous Research",
		Names:        []string{"hermes", "hermes agent", "hermes-agent"},
		TrailerEmail: "noreply@nousresearch.com",
	},
	{
		Name: "Pi", Vendor: "pi.dev",
		Emails:       []string{"pi@earendil.works"},
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
		Emails:       []string{"noreply@alibaba.com"},
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
		Emails:       []string{"grok@x.ai"},
		Names:        []string{"grok", "grok build", "grok-build"},
		NamePattern:  rx(`^grok(\b.*)?$`),
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
		Name: "Crush", Vendor: "Charm",
		Emails:       []string{"crush@charm.land"},
		Names:        []string{"crush", "charm crush"},
		TrailerEmail: "crush@charm.land",
	},
	{
		Name: "Codebuff", Vendor: "Codebuff",
		Emails:       []string{"noreply@codebuff.com"},
		Names:        []string{"codebuff"},
		TrailerEmail: "noreply@codebuff.com",
	},
	{
		Name: "Rovo Dev", Vendor: "Atlassian",
		Names:        []string{"rovo dev", "rovodev", "rovo-dev", "rovo"},
		TrailerEmail: "noreply@atlassian.com",
	},
	{
		Name: "Terragon", Vendor: "Terragon Labs",
		Names:        []string{"terragon", "terragon labs"},
		NamePattern:  rx(`^terragon\b`),
		TrailerEmail: "noreply@terragonlabs.com",
	},
	{
		Name: "Firebender", Vendor: "Firebender",
		Names:        []string{"firebender"},
		TrailerEmail: "noreply@firebender.com",
	},
	{
		Name: "Supermaven", Vendor: "Supermaven",
		Names:        []string{"supermaven", "supermaven[bot]"},
		TrailerEmail: "noreply@supermaven.com",
	},
	{
		Name: "Phind", Vendor: "Phind",
		Names:        []string{"phind", "phind[bot]"},
		TrailerEmail: "noreply@phind.com",
	},
	{
		Name: "IBM Bob", Vendor: "IBM",
		Names:        []string{"ibm bob", "bob (ibm)"},
		TrailerEmail: "noreply@ibm.com",
	},
	{
		Name: "Replit Agent", Vendor: "Replit",
		Names:        []string{"replit", "replit agent", "replit-agent", "replit-agent[bot]", "replit[bot]"},
		TrailerEmail: "noreply@replit.com",
	},
	{
		Name: "Lovable", Vendor: "Lovable",
		Names:         []string{"lovable", "lovable-dev[bot]", "gpt-engineer-app[bot]", "lovable[bot]"},
		EmailSuffixes: []string{"+lovable-dev[bot]@users.noreply.github.com", "+gpt-engineer-app[bot]@users.noreply.github.com"},
		TrailerEmail:  "noreply@lovable.dev",
	},
	{
		Name: "Bolt", Vendor: "StackBlitz",
		Names:        []string{"bolt", "bolt.new", "bolt-new", "bolt-new-by-stackblitz[bot]", "stackblitz[bot]", "bolt.new[bot]"},
		TrailerEmail: "noreply@bolt.new",
	},
	{
		Name: "v0", Vendor: "Vercel",
		Names:         []string{"v0", "v0[bot]", "v0.dev"},
		EmailSuffixes: []string{"+v0[bot]@users.noreply.github.com"},
		TrailerEmail:  "noreply@v0.dev",
	},
	{
		Name: "Maka", Vendor: "Apache",
		Names:        []string{"maka", "apache maka"},
		TrailerEmail: "noreply@maka.apache.org",
	},
	{
		Name: "PostHog Desktop", Vendor: "PostHog",
		Names:        []string{"posthog desktop", "posthog array", "array agent", "posthog agent"},
		TrailerEmail: "noreply@posthog.com",
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
		Name: "Greptile", Vendor: "Greptile",
		Names:         []string{"greptile", "greptile-apps[bot]", "greptile-apps"},
		EmailSuffixes: []string{"+greptile-apps[bot]@users.noreply.github.com"},
		TrailerEmail:  "greptile-apps[bot]@users.noreply.github.com",
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
		Names:         []string{"qodo", "qodo-merge-pro[bot]", "qodo-code-review[bot]", "codiumai", "pr-agent"},
		EmailSuffixes: []string{"+qodo-merge-pro[bot]@users.noreply.github.com", "+qodo-code-review[bot]@users.noreply.github.com"},
		TrailerEmail:  "noreply@qodo.ai",
	},
	{
		Name: "Cody", Vendor: "Sourcegraph",
		Names:        []string{"cody", "sourcegraph cody", "sourcegraph-cody[bot]", "cody[bot]"},
		TrailerEmail: "noreply@sourcegraph.com",
	},
	{
		Name: "Continue", Vendor: "Continue",
		Names:        []string{"continue", "continue agent", "continue-agent", "continue[bot]"},
		TrailerEmail: "noreply@continue.dev",
	},
	{
		Name: "Tabnine", Vendor: "Tabnine",
		Names:        []string{"tabnine", "tabnine agent", "tabnine[bot]"},
		TrailerEmail: "noreply@tabnine.com",
	},
	{
		Name: "Blackbox", Vendor: "Blackbox AI",
		Names:        []string{"blackbox", "blackbox ai", "blackboxai", "blackboxai[bot]", "blackboxai-deploy[bot]"},
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
	// SDK generators and release automation that author commits themselves.
	{Name: "Stainless", Names: []string{"stainless-app[bot]", "stainless bot", "stainless"}, EmailSuffixes: []string{"+stainless-app[bot]@users.noreply.github.com"}, IsBot: true},
	{Name: "Fern", Names: []string{"fern-api[bot]", "fern-api"}, EmailSuffixes: []string{"+fern-api[bot]@users.noreply.github.com"}, IsBot: true},
	{Name: "StageFreight", Names: []string{"stagefreight"}, IsBot: true},
	{Name: "Yoshi", Names: []string{"yoshi-automation", "yoshi-code-bot", "gcf-owl-bot[bot]", "release-please[bot]"}, EmailSuffixes: []string{"@google.com"}, NamePattern: rx(`^(yoshi|gcf-owl-bot)`), IsBot: true},
	{Name: "Generic bot", NamePattern: rx(`\[bot\]$|(^|[\s\-_])bot$`), IsBot: true},
}

// matchesName reports whether the normalised name matches the identity's
// names or pattern.
func (id Identity) matchesName(name string) bool {
	if name == "" {
		return false
	}
	for _, n := range id.Names {
		if name == strings.ToLower(n) {
			return true
		}
	}
	return id.NamePattern != nil && id.NamePattern.MatchString(name)
}

// matchesExactName reports whether the normalised name is one of the
// identity's listed names (patterns are not consulted).
func (id Identity) matchesExactName(name string) bool {
	if name == "" {
		return false
	}
	for _, n := range id.Names {
		if name == strings.ToLower(n) {
			return true
		}
	}
	return false
}

// matchesEmailStrict is matchesEmail with suffix matching optional.
func (id Identity) matchesEmailStrict(email string, allowSuffix bool) bool {
	if email == "" {
		return false
	}
	for _, e := range id.Emails {
		if email == strings.ToLower(e) {
			return true
		}
	}
	if !allowSuffix {
		return false
	}
	for _, s := range id.EmailSuffixes {
		if strings.HasSuffix(email, strings.ToLower(s)) {
			return true
		}
	}
	return false
}

// matchesEmail reports whether the lower-cased e-mail matches the identity.
func (id Identity) matchesEmail(email string) bool {
	if email == "" {
		return false
	}
	for _, e := range id.Emails {
		if email == strings.ToLower(e) {
			return true
		}
	}
	for _, s := range id.EmailSuffixes {
		if strings.HasSuffix(email, strings.ToLower(s)) {
			return true
		}
	}
	return false
}

// matches reports whether name/email (already lower-cased and trimmed)
// match the identity.
func (id Identity) matches(name, email string) bool {
	return id.matchesEmail(email) || id.matchesName(name)
}

// LookupAgent returns the canonical agent identity for a raw name/email pair,
// or nil when it is not a known AI agent.
//
// User-defined identities win. Then names and patterns are tried across the
// whole built-in table before e-mails, so a trailer such as
// "Cursor Grok 4.6 <noreply@anthropic.com>" is attributed to the tool named
// in it rather than to the address. The Generic AI entry is consulted last
// and only when the e-mail is empty or a noreply address, to avoid
// misclassifying humans with "ai" in their name.
func LookupAgent(name, email string, extra []Identity) *Identity {
	n := normName(name)
	e := strings.ToLower(strings.TrimSpace(email))
	for i := range extra {
		if extra[i].matches(n, e) {
			return &extra[i]
		}
	}
	// Exact names always count; patterns ("^gemini…", "^cursor…") only when
	// there is no personal e-mail attached, so a person called Gemini with
	// her own address stays human.
	allowPattern := looksAutomated(e)
	for i := range KnownAgents {
		id := &KnownAgents[i]
		if id.Name == "Generic AI" {
			continue
		}
		if id.matchesExactName(n) || ((allowPattern || id.PatternWithPersonalEmail) && id.matchesName(n)) {
			return id
		}
	}
	// Exact addresses always count. Suffixes ("+copilot@users.noreply…")
	// only when the address already looks automated, so that an employee of
	// an agent vendor committing from a corporate mailbox stays human.
	for i := range KnownAgents {
		id := &KnownAgents[i]
		if id.Name != "Generic AI" && id.matchesEmailStrict(e, allowPattern) {
			return id
		}
	}
	generic := &KnownAgents[len(KnownAgents)-1]
	// Guard: only accept the generic rule for automated addresses and short
	// tool-like names.
	if looksAutomated(e) && len(strings.Fields(n)) <= 3 && generic.matchesName(n) {
		return generic
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

// AgentByName returns the built-in identity with the given canonical name,
// or nil.
func AgentByName(name string) *Identity {
	for i := range KnownAgents {
		if KnownAgents[i].Name == name {
			return &KnownAgents[i]
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
		strings.Contains(email, "[bot]") || strings.HasSuffix(email, "@localhost") || strings.HasSuffix(email, "@agent")
}

func normName(name string) string {
	n := strings.ToLower(strings.TrimSpace(name))
	n = strings.Join(strings.Fields(n), " ")
	return n
}
