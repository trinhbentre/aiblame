package attrib

import (
	"regexp"
	"strings"
	"unicode"
)

// Trailer is one "Key: value" line found in a commit message.
type Trailer struct {
	Key   string // canonical key as written, e.g. "Co-authored-by"
	Value string // trimmed value
}

// StrongTrailerKeys are trailer keys (lower-case) that exist only to
// disclose AI involvement. Their value is AI evidence unless it is a negative
// ("none") or clearly names a person: curl, for example, uses Assisted-by
// for human helpers, so "Assisted-by: Jay Satiro" is not AI.
var StrongTrailerKeys = map[string]bool{
	"assisted-by":         true, // Linux kernel, Mesa, Zephyr, Fedora, LLVM, Calcite, Artsy
	"assisted-by-ai":      true,
	"ai-assisted":         true, // Open Delivery Spec
	"ai-assisted-by":      true,
	"ai-agent":            true,
	"ai-assistant":        true, // Bence Ferdinandy proposal
	"ai-generated":        true,
	"ai-generated-by":     true,
	"ai-used-for":         true, // QEMU: "AI-used-for: code, tests"
	"llm-assisted-by":     true,
	"coding-agent":        true, // fabiorehm proposal (with a Model: companion)
	"co-developed-by-ai":  true,
	"commit-generated-by": true, // rai-lint
}

// WeakTrailerKeys are also used by non-AI tooling (SDK generators, release
// bots, "Made-with: love"). Their value is AI evidence only when it names a
// known agent or uses AI/model vocabulary.
var WeakTrailerKeys = map[string]bool{
	"generated-by":   true, // ASF, Mesa, Crash Override — but also StageFreight
	"generated-with": true,
	"made-with":      true, // Cursor: "Made-with: Cursor"
	"agent":          true,
}

// SessionTrailerKeys link a commit to an agent session (a URL or an id).
// Their presence is AI evidence; the value carries no agent name, so the
// agent is taken from the key. An empty agent means "unknown AI".
var SessionTrailerKeys = map[string]string{
	"claude-session":      "Claude Code", // Claude Code ≥ 2.1.25x: https://claude.ai/code/session_…
	"claude-session-path": "Claude Code", // gammons/ai-session
	"claude-sessions-id":  "Claude Code", // schpet/jjagent
	"turbocommit-session": "Claude Code", // searlsco/turbocommit
	"agent-logs-url":      "GitHub Copilot",
	"amp-thread-id":       "Amp",
	"entire-checkpoint":   "", // Entire CLI; agent resolved from the checkpoint when available
	"agent-conversation":  "", // AgentsRoom
	"agent-transcript":    "", // .specstory exports
}

// AITrailerKeys lists every trailer key (lower-case) whose value can carry
// AI attribution: the union of the strong and weak keys. Co-authored-by is
// handled separately because it is AI evidence only when the person is a
// known agent.
var AITrailerKeys = func() map[string]bool {
	m := map[string]bool{}
	for k := range StrongTrailerKeys {
		m[k] = true
	}
	for k := range WeakTrailerKeys {
		m[k] = true
	}
	return m
}()

// IsKnownConvention reports whether c (lower-case) is a disclosure
// convention aiblame can recognise in a commit: a trailer key from the
// tables above, a co-author trailer, a DCO sign-off or a sidecar source.
func IsKnownConvention(c string) bool {
	c = strings.ToLower(strings.TrimSpace(c))
	switch c {
	case "co-authored-by", "co-developed-by", "signed-off-by", "agent-author", "message-marker",
		"git-ai-notes", "exceeds-ink-notes", "claude-conversations-notes":
		return true
	}
	if _, ok := SessionTrailerKeys[c]; ok {
		return true
	}
	return AITrailerKeys[c]
}

// trailerLine matches "Key: value" where Key is a dash-separated token. Git
// itself is lenient about where trailers appear; so are we, because many
// agents put the trailer in the middle of a multi-paragraph body.
var trailerLine = regexp.MustCompile(`^\s*([A-Za-z][A-Za-z0-9-]*)\s*:\s*(\S.*?)\s*$`)

// ParseTrailers extracts trailer-like lines from a commit message body.
// The subject line is never a trailer, so conventional-commit scopes such
// as "agent: move X" or "model: bump" are not mistaken for one. Lines that
// look like URLs ("https://…") or prose ("Note: …" with long values) are
// filtered by the caller via key allow-lists.
func ParseTrailers(message string) []Trailer {
	var out []Trailer
	for i, raw := range strings.Split(message, "\n") {
		if i == 0 {
			continue
		}
		m := trailerLine.FindStringSubmatch(raw)
		if m == nil {
			continue
		}
		key := m[1]
		// Skip scheme-like prefixes ("https: //") and single-word shouting.
		if strings.HasPrefix(strings.ToLower(m[2]), "//") {
			continue
		}
		out = append(out, Trailer{Key: key, Value: strings.TrimSpace(m[2])})
	}
	return out
}

// Person is the parsed form of "Name <email>".
type Person struct {
	Name  string
	Email string
}

var personRx = regexp.MustCompile(`^\s*(.*?)\s*<\s*([^<>\s]+@[^<>\s]+)\s*>\s*$`)

// ParsePerson splits "Name <email>" into its parts. When no e-mail is present
// the whole string becomes the name.
func ParsePerson(s string) Person {
	if m := personRx.FindStringSubmatch(s); m != nil {
		return Person{Name: strings.TrimSpace(m[1]), Email: strings.TrimSpace(m[2])}
	}
	return Person{Name: strings.TrimSpace(s)}
}

// AgentRef is the normalised content of an AI trailer value.
type AgentRef struct {
	Tool  string // raw tool token, e.g. "Claude Code", "cursor-agent"
	Model string // model id if present, e.g. "claude-opus-4-6"
	Email string // e-mail if present
}

var (
	parenRx       = regexp.MustCompile(`\(([^()]*)\)`)
	bracketTailRx = regexp.MustCompile(`\s*\[[^\]]*\]\s*$`)
	modelKVRx     = regexp.MustCompile(`(?i)\bmodel\s*[:=]\s*([^;,)\s]+)`)
	effortRx      = regexp.MustCompile(`#[A-Za-z0-9_.-]+$`)
	claudeModelRx = regexp.MustCompile(`(?i)^claude[\s-]+((opus|sonnet|haiku|fable|mythos)\b.*)$`)
	// embeddedModelRx finds a model name inside a tool phrase, e.g.
	// "OpenCode GPT-5.6 Sol", "Cursor Grok 4.6", "Google Gemini 3.1 Flash".
	embeddedModelRx = regexp.MustCompile(`(?i)\b((?:gpt|grok|gemini|llama|qwen|kimi|deepseek|glm|mistral|devstral|codestral)[\s-]?\d[\w.]*(?:[\s-](?:sol|luna|spark|mini|nano|pro|flash|lite|ultra|turbo|thinking|preview|high|medium|low|image|instruct|coder)\b)*)`)
	modelFamilyRx   = regexp.MustCompile(`(?i)\b(claude|gpt|opus|sonnet|haiku|fable|mythos|gemini|grok|llama|qwen|kimi|deepseek|glm|mistral|devstral|codestral|composer|o[1-9])\b`)
	vendorPrefixRx  = regexp.MustCompile(`(?i)^(openai|anthropic|google|xai|meta|alibaba|moonshot|deepseek|mistral|zhipu|z\.ai)\s+`)
	nonModelNoteRx  = regexp.MustCompile(`(?i)\bcontext\b|\boperator\b|\bbeta\b$|\bpreview\b$`)
	// aiVocabRx marks a free-text value as AI-related even when no known agent
	// is named: "generic LLM chatbot", "Local LLM fuzzer", "Bynario AI".
	aiVocabRx = regexp.MustCompile(`(?i)(^|[^a-z])(ai|llm|llms|gpt|model|agent|agents|copilot|codex|claude|gemini|assistant|chatbot|bot|llama|qwen|kimi|deepseek|mistral|grok|opus|sonnet|haiku|fable)([^a-z]|$)`)
	hexIDRx   = regexp.MustCompile(`^[0-9a-fA-F-]{8,}$`)
)

var vendorWords = map[string]bool{"openai": true, "google": true, "anthropic": true, "xai": true, "x.ai": true, "meta": true, "alibaba": true, "moonshot": true, "deepseek": true, "mistral": true, "zhipu": true}

// negativeValues are trailer values that explicitly say no AI was used.
var negativeValues = map[string]bool{"none": true, "n/a": true, "na": true, "no": true, "nil": true, "null": true, "-": true, "—": true, "false": true, "0": true, "not applicable": true, "not used": true, "nothing": true}

// unknownModelValues are model slots that carry no information.
var unknownModelValues = map[string]bool{"undisclosed": true, "unknown": true, "unspecified": true, "n/a": true, "none": true, "-": true, "?": true}

// IsNegativeValue reports whether a trailer value states that no AI was
// involved ("Assisted-by: none").
func IsNegativeValue(v string) bool {
	return negativeValues[strings.ToLower(strings.TrimSpace(v))]
}

// ParseAgentRef normalises the many trailer value shapes seen in the wild
// (every example below is from a real commit):
//
//	Claude <noreply@anthropic.com>
//	Claude Opus 4.8 (1M context) <noreply@anthropic.com>
//	Codex (gpt-5.6-sol) <noreply@openai.com>
//	Claude Code (claude-opus-5)                         -- Calcite / Mesa style
//	Claude:claude-fable-5-1 coccinelle sparse           -- kernel v7.0 / Zephyr style
//	LLM coccinelle sparse                               -- kernel 7.3+ style
//	Hermes Agent:gpt-5.6-sol                            -- tool with spaces before the colon
//	openai-codex:gpt-5.5 [read,bash,edit,write]         -- bracketed tool list
//	claude-code:fable-5.1#high (orchestrator, reviewer) -- effort suffix, role note
//	claude-code/claude-fable-5                          -- tool/model
//	devx/3066e945-4358-448d-b2e2-8ee7c94ce548           -- tool/session-id
//	GPT-5.6 Sol via Codex                               -- model via tool
//	OpenCode GPT-5.6 Sol <noreply@opencode.ai>          -- tool + embedded model
//	cursor-agent/0.42 (model: claude-sonnet-4-5; operator: a@b.c)
//	OpenCode v1.0.203 (Claude Opus 4.5)
func ParseAgentRef(value string) AgentRef {
	var ref AgentRef
	// "Assisted-by: Assisted-by: Claude Opus 4.6 <…>" — a hook that prefixed
	// the key twice. Drop the repeated key.
	value = strings.TrimSpace(value)
	if k, rest, ok := strings.Cut(value, ":"); ok {
		lk := strings.ToLower(strings.TrimSpace(k))
		if AITrailerKeys[lk] || lk == "co-authored-by" || lk == "co-developed-by" {
			value = strings.TrimSpace(rest)
		}
	}
	p := ParsePerson(value)
	ref.Email = p.Email
	tool := p.Name
	if tool == "" && p.Email == "" {
		tool = value
	}

	// model: key=value form wins when present.
	if m := modelKVRx.FindStringSubmatch(tool); m != nil {
		ref.Model = m[1]
	}
	// Trailing "[tool1,tool2]" lists. "[bot]" is part of an identity and stays.
	for {
		loc := bracketTailRx.FindStringIndex(tool)
		if loc == nil || strings.EqualFold(strings.TrimSpace(tool[loc[0]:loc[1]]), "[bot]") {
			break
		}
		tool = strings.TrimSpace(tool[:loc[0]])
	}
	// Parenthesised notes: a model when it looks like one ("gpt-5-codex",
	// "Claude Opus 4.5"); dropped otherwise ("1M context", "orchestrator",
	// "Anthropic", "AI coding agent").
	for _, m := range parenRx.FindAllStringSubmatch(tool, -1) {
		inner := strings.Trim(strings.TrimSpace(m[1]), ",; ")
		if ref.Model == "" && isModelLike(inner) {
			ref.Model = inner
		}
	}
	tool = strings.TrimSpace(parenRx.ReplaceAllString(tool, " "))
	// "MODEL via TOOL".
	if i := strings.Index(strings.ToLower(tool), " via "); i >= 0 {
		left, right := strings.TrimSpace(tool[:i]), strings.TrimSpace(tool[i+5:])
		if right != "" {
			if ref.Model == "" && left != "" && (isModelLike(left) || modelFamilyRx.MatchString(left)) {
				ref.Model = left
			}
			tool = right
		}
	}
	// "AGENT:MODEL [companion tools]" — the agent may contain spaces
	// ("Hermes Agent:gpt-5.6-sol"). URLs ("https://…") and e-mails are not
	// split.
	if c := strings.Index(tool, ":"); c > 0 && !strings.HasPrefix(tool[c:], "://") && !strings.Contains(tool[:c], "@") {
		left := strings.TrimSpace(tool[:c])
		rest := strings.TrimSpace(tool[c+1:])
		if left != "" {
			tool = left
			if first, _, _ := strings.Cut(rest, " "); first != "" && ref.Model == "" {
				ref.Model = first
			}
		}
	}
	// "TOOL/rest": a version ("cursor-agent/0.42"), a session id
	// ("devx/3066e945-…") or a model ("claude-code/claude-fable-5").
	if strings.Count(tool, "/") == 1 && !strings.Contains(tool, " ") {
		left, right, _ := strings.Cut(tool, "/")
		switch {
		case left == "":
		case isVersionToken(right) || hexIDRx.MatchString(right):
			tool = left
		case isModelLike(right):
			if ref.Model == "" {
				ref.Model = right
			}
			tool = left
		}
	}
	// "Claude Opus 4.1" -> tool Claude, model "Opus 4.1".
	if m := claudeModelRx.FindStringSubmatch(strings.TrimSpace(tool)); m != nil {
		if ref.Model == "" {
			ref.Model = strings.TrimSpace(m[1])
		}
		tool = "Claude"
	}
	// "OpenCode GPT-5.6 Sol" -> tool OpenCode, model GPT-5.6 Sol. A bare
	// model ("GPT-5.5") keeps its family name as the tool.
	if loc := embeddedModelRx.FindStringIndex(tool); loc != nil {
		if ref.Model == "" {
			ref.Model = strings.TrimSpace(tool[loc[0]:loc[1]])
		}
		rest := strings.Join(strings.Fields(tool[:loc[0]]+" "+tool[loc[1]:]), " ")
		if rest == "" || vendorWords[strings.ToLower(rest)] {
			rest = modelFamily(tool[loc[0]:loc[1]])
		}
		tool = rest
	}
	// "OpenCode v1.0.203" -> "OpenCode".
	if fields := strings.Fields(tool); len(fields) >= 2 && isVersionToken(fields[len(fields)-1]) {
		tool = strings.Join(fields[:len(fields)-1], " ")
	}
	// Kernel "LLM tool1 tool2": first token is the tool when the rest are
	// static analysers.
	if fields := strings.Fields(tool); len(fields) > 1 && strings.EqualFold(fields[0], "llm") {
		tool = fields[0]
	}
	ref.Tool = strings.Trim(strings.TrimSpace(tool), ",;:")
	ref.Model = cleanModel(ref.Model)
	return ref
}

// CleanModel normalises a model identifier: it strips quotes, effort
// suffixes ("#high"), vendor prefixes ("OpenAI ") and returns "" for
// placeholders ("unknown", "<synthetic>", "undisclosed").
func CleanModel(m string) string { return cleanModel(m) }

func cleanModel(m string) string {
	m = strings.Trim(strings.TrimSpace(m), ",; \"'`")
	m = effortRx.ReplaceAllString(m, "")
	m = vendorPrefixRx.ReplaceAllString(m, "")
	m = strings.Trim(strings.TrimSpace(m), ",; \"'`")
	if m == "" || strings.ContainsAny(m, "<>") || unknownModelValues[strings.ToLower(m)] {
		return ""
	}
	// A bare family name ("Claude", "GPT") says which vendor, not which model.
	if modelFamilyRx.MatchString(m) && !strings.ContainsAny(m, "0123456789") && len(strings.Fields(m)) == 1 {
		return ""
	}
	return m
}

// IsToolLikeName reports whether a free-text value is short enough to be a
// tool name rather than a sentence: at most four words and forty characters,
// no sentence punctuation.
func IsToolLikeName(s string) bool {
	s = strings.TrimSpace(s)
	if s == "" || len(s) > 40 || strings.ContainsAny(s, ".!?;") {
		return false
	}
	return len(strings.Fields(s)) <= 4
}

// modelFamily returns the tool-ish family name of a model string:
// "GPT-5.6 Sol" -> "GPT", "gemini-3.6-flash" -> "Gemini".
func modelFamily(model string) string {
	m := modelFamilyRx.FindString(model)
	if m == "" {
		return model
	}
	switch strings.ToLower(m) {
	case "gpt":
		return "GPT"
	case "gemini":
		return "Gemini"
	case "grok":
		return "Grok"
	case "claude", "opus", "sonnet", "haiku", "fable", "mythos":
		return "Claude"
	case "qwen":
		return "Qwen"
	case "kimi":
		return "Kimi"
	case "deepseek":
		return "DeepSeek"
	case "mistral", "devstral", "codestral":
		return "Mistral"
	}
	return m
}

// isModelLike reports whether a token is plausibly a model identifier
// rather than a note, a role or a vendor.
func isModelLike(s string) bool {
	s = strings.Trim(strings.TrimSpace(s), ",; ")
	if s == "" || strings.ContainsAny(s, "<>") || nonModelNoteRx.MatchString(s) || IsNegativeValue(s) || unknownModelValues[strings.ToLower(s)] {
		return false
	}
	if modelFamilyRx.MatchString(s) {
		return true
	}
	return strings.ContainsAny(s, "0123456789") && !strings.Contains(s, " ")
}

func isVersionToken(s string) bool {
	s = strings.TrimPrefix(strings.TrimPrefix(s, "v"), "V")
	if s == "" {
		return false
	}
	digits := 0
	for _, r := range s {
		switch {
		case r >= '0' && r <= '9':
			digits++
		case r == '.' || r == '-' || r == '+' || r == '_':
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z':
			// allow "1.2.3-beta"
		default:
			return false
		}
	}
	return digits > 0 && (s[0] >= '0' && s[0] <= '9')
}

// personalEmail reports whether an address belongs to a person: anything
// that is not a noreply/bot address, plus GitHub's per-user noreply
// addresses ("login@users.noreply.github.com"), which are people unless the
// login ends in "[bot]".
func personalEmail(e string) bool {
	e = strings.ToLower(strings.TrimSpace(e))
	if strings.Contains(e, "[bot]") {
		return false
	}
	if strings.HasSuffix(e, "@users.noreply.github.com") {
		return true
	}
	return !looksAutomated(e)
}

// HasAIVocabulary reports whether a free-text value talks about AI even
// when it names no known agent ("generic LLM chatbot").
func HasAIVocabulary(s string) bool { return aiVocabRx.MatchString(s) }

// LooksLikePerson reports whether a trailer value reads like a human name
// rather than a tool: two or more alphabetic words, no digits or structural
// punctuation, and no AI vocabulary; or a bare personal e-mail address.
// Known agents must be excluded by the caller first.
func LooksLikePerson(raw string) bool {
	p := ParsePerson(raw)
	name := strings.TrimSpace(p.Name)
	if name == "" {
		// "<someone@example.com>" or a bare address.
		return p.Email != "" && personalEmail(p.Email)
	}
	if strings.Contains(name, "@") && !strings.ContainsAny(name, " ()[]:/") {
		return personalEmail(name)
	}
	if strings.ContainsAny(name, "0123456789:/()[]<>#=") || HasAIVocabulary(name) {
		return false
	}
	// "Fabrice <fabrice@example.com>": with an address attached the address
	// decides, whatever the shape of the name.
	if p.Email != "" {
		return personalEmail(p.Email)
	}
	words := strings.Fields(name)
	if len(words) < 2 || len(words) > 5 {
		return false
	}
	for _, w := range words {
		for _, r := range w {
			if !(unicode.IsLetter(r) || r == '.' || r == '\'' || r == '-' || r == '’') {
				return false
			}
		}
	}
	return true
}
