package attrib

import (
	"regexp"
	"strings"
)

// Trailer is one "Key: value" line found in a commit message.
type Trailer struct {
	Key   string // canonical key as written, e.g. "Co-authored-by"
	Value string // trimmed value
}

// AITrailerKeys lists trailer keys (lower-case) that carry AI attribution.
// Co-authored-by is only AI evidence when the value names a known agent;
// the other keys are AI evidence on their own.
var AITrailerKeys = map[string]bool{
	"assisted-by":        true, // Linux kernel, Mesa, Zephyr, Fedora, Calcite
	"generated-by":       true, // Crash Override recommendation, misc tools
	"generated-with":     true,
	"coding-agent":       true, // fabiorehm proposal
	"ai-agent":           true,
	"ai-assistant":       true, // Bence Ferdinandy proposal
	"ai-assisted-by":     true,
	"ai-generated":       true,
	"ai-generated-by":    true,
	"llm-assisted-by":    true,
	"agent":              true,
	"co-developed-by-ai": true,
}

// trailerLine matches "Key: value" where Key is a dash-separated token. Git
// itself is lenient about where trailers appear; so are we, because many
// agents put the trailer in the middle of a multi-paragraph body.
var trailerLine = regexp.MustCompile(`^\s*([A-Za-z][A-Za-z0-9-]*)\s*:\s*(\S.*?)\s*$`)

// ParseTrailers extracts trailer-like lines from a commit message body.
// Lines that look like URLs ("https://…") or prose ("Note: …" with long
// values) are filtered by the caller via key allow-lists.
func ParseTrailers(message string) []Trailer {
	var out []Trailer
	for _, raw := range strings.Split(message, "\n") {
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
	modelKVRx     = regexp.MustCompile(`(?i)\bmodel\s*[:=]\s*([^;,)\s]+)`)
	versionRx     = regexp.MustCompile(`/[vV]?\d[\w.\-]*$`)
	claudeModelRx = regexp.MustCompile(`(?i)^claude\s+((opus|sonnet|haiku|fable|mythos)\b.*)$`)
)

// ParseAgentRef normalises the many trailer value shapes seen in the wild:
//
//	Claude <noreply@anthropic.com>
//	Claude Opus 4.1 <noreply@anthropic.com>
//	Codex CLI (gpt-5-codex) <noreply@openai.com>
//	Claude Code (claude-opus-4-6)                 -- Calcite style
//	Claude:claude-3-opus coccinelle sparse         -- early kernel RFC style
//	LLM coccinelle sparse                          -- kernel doc example
//	cursor-agent/0.42 (model: claude-sonnet-4-5; operator: a@b.c)
//	OpenCode v1.0.203 (Claude Opus 4.5)
func ParseAgentRef(value string) AgentRef {
	var ref AgentRef
	p := ParsePerson(value)
	ref.Email = p.Email
	tool := p.Name
	if tool == "" {
		tool = value
	}

	// model: key=value form wins when present.
	if m := modelKVRx.FindStringSubmatch(tool); m != nil {
		ref.Model = m[1]
	}
	// Parenthesised model, e.g. "Codex CLI (gpt-5-codex)". Notes such as
	// "(1M context)" that Claude Code appends are not models and are dropped.
	if m := parenRx.FindStringSubmatch(tool); m != nil {
		inner := strings.TrimSpace(m[1])
		if ref.Model == "" && inner != "" && !isNonModelNote(inner) {
			ref.Model = inner
		}
		tool = strings.TrimSpace(parenRx.ReplaceAllString(tool, ""))
	}
	// "Agent:model tool1 tool2" kernel-RFC style. Only treat as such when the
	// colon is inside the first token and there is no space before it.
	if first, rest, ok := strings.Cut(tool, " "); ok || tool != "" {
		if !ok {
			first, rest = tool, ""
		}
		if a, m, has := strings.Cut(first, ":"); has && a != "" && m != "" && !strings.Contains(a, "@") {
			tool = a
			if ref.Model == "" {
				ref.Model = m
			}
			_ = rest // companion tools (coccinelle, sparse…) are ignored
		} else if has && a != "" && m == "" {
			tool = a
		}
	}
	// "Claude Opus 4.1" -> tool Claude, model "Opus 4.1". Must run before
	// version stripping so the "4.1" survives as part of the model.
	if m := claudeModelRx.FindStringSubmatch(strings.TrimSpace(tool)); m != nil {
		if ref.Model == "" {
			ref.Model = strings.TrimSpace(m[1])
		}
		tool = "Claude"
	}
	// Strip "/version".
	tool = versionRx.ReplaceAllString(tool, "")
	// "OpenCode v1.0.203" -> "OpenCode".
	fields := strings.Fields(tool)
	if len(fields) >= 2 {
		last := fields[len(fields)-1]
		if isVersionToken(last) {
			fields = fields[:len(fields)-1]
			tool = strings.Join(fields, " ")
		}
	}
	// Kernel "LLM tool1 tool2": first token is the tool when the rest are
	// known static analysers.
	if fields := strings.Fields(tool); len(fields) > 1 && strings.EqualFold(fields[0], "llm") {
		tool = fields[0]
	}
	ref.Tool = strings.TrimSpace(strings.Trim(tool, ",;"))
	return ref
}

var nonModelNoteRx = regexp.MustCompile(`(?i)\bcontext\b|\boperator\b|\bbeta\b$|\bpreview\b$`)

// isNonModelNote reports whether a parenthesised note is metadata rather
// than a model identifier, e.g. "1M context" or "operator: a@b.c".
func isNonModelNote(s string) bool {
	return nonModelNoteRx.MatchString(s)
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
