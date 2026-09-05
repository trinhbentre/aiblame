// Package attrib classifies git commits by AI involvement.
//
// It recognises the attribution conventions that coding agents and open
// source projects actually use in 2025-2026: Co-authored-by trailers added by
// Claude Code, Codex, Cursor, Copilot and aider; the Assisted-by trailer
// adopted by the Linux kernel, Mesa, Zephyr and Fedora; Generated-by and
// Coding-Agent style trailers; bot author identities such as
// devin-ai-integration[bot]; and free-text markers like
// "Generated with Claude Code".
package attrib

import "strings"

// Kind is the coarse authorship category of a commit.
type Kind int

const (
	// Human is a commit with no AI evidence at all.
	Human Kind = iota
	// Assisted is a commit authored by a human that carries AI evidence
	// (a trailer or message marker naming an agent).
	Assisted
	// Agent is a commit whose author (or committer, when the author is
	// unknown) is itself an AI identity, e.g. devin-ai-integration[bot].
	Agent
	// Bot is non-AI automation: dependabot, renovate, github-actions and so on.
	Bot
)

var kindNames = [...]string{"human", "assisted", "agent", "bot"}

// String returns the lower-case machine name of the kind.
func (k Kind) String() string {
	if int(k) < 0 || int(k) >= len(kindNames) {
		return "unknown"
	}
	return kindNames[k]
}

// Label returns a human-friendly label.
func (k Kind) Label() string {
	switch k {
	case Human:
		return "Human"
	case Assisted:
		return "AI-assisted"
	case Agent:
		return "AI agent"
	case Bot:
		return "Bot"
	}
	return "Unknown"
}

// IsAI reports whether the kind counts toward AI authorship.
func (k Kind) IsAI() bool { return k == Assisted || k == Agent }

// MarshalText implements encoding.TextMarshaler.
func (k Kind) MarshalText() ([]byte, error) { return []byte(k.String()), nil }

// UnmarshalText implements encoding.TextUnmarshaler.
func (k *Kind) UnmarshalText(b []byte) error {
	s := strings.ToLower(strings.TrimSpace(string(b)))
	for i, n := range kindNames {
		if n == s {
			*k = Kind(i)
			return nil
		}
	}
	return &UnknownKindError{Value: s}
}

// UnknownKindError is returned when a kind name cannot be parsed.
type UnknownKindError struct{ Value string }

func (e *UnknownKindError) Error() string { return "attrib: unknown kind " + e.Value }

// Kinds lists every kind in display order.
func Kinds() []Kind { return []Kind{Agent, Assisted, Human, Bot} }
