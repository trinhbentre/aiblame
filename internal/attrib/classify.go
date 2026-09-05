package attrib

import (
	"regexp"
	"sort"
	"strings"
)

// Commit is the minimal commit view the classifier needs.
type Commit struct {
	Hash           string
	AuthorName     string
	AuthorEmail    string
	CommitterName  string
	CommitterEmail string
	Message        string // full message: subject, body and trailers
}

// Evidence records why a commit was classified the way it was.
type Evidence struct {
	Source string `json:"source"` // author | committer | trailer:<key> | message
	Value  string `json:"value"`  // the raw matched text
	Agent  string `json:"agent,omitempty"`
	Model  string `json:"model,omitempty"`
}

// Attribution is the classification result for one commit.
type Attribution struct {
	Kind     Kind       `json:"kind"`
	Agents   []string   `json:"agents,omitempty"` // canonical agent names, sorted
	Models   []string   `json:"models,omitempty"`
	Evidence []Evidence `json:"evidence,omitempty"`
	// Conventions lists the trailer keys that carried AI evidence
	// (lower-case), e.g. ["co-authored-by"], ["assisted-by"]. Empty for
	// commits detected only via author identity or message markers.
	Conventions []string `json:"conventions,omitempty"`
}

// PrimaryAgent returns the first agent name or "" when none.
func (a Attribution) PrimaryAgent() string {
	if len(a.Agents) == 0 {
		return ""
	}
	return a.Agents[0]
}

// Options tunes the classifier.
type Options struct {
	// ExtraAgents are user-defined AI identities (from config).
	ExtraAgents []Identity
	// ExtraBots are user-defined non-AI bot identities.
	ExtraBots []Identity
	// ExtraTrailerKeys are additional trailer keys (any case) treated as AI
	// evidence on their own.
	ExtraTrailerKeys []string
	// MessageMarkers are additional regular expressions matched against the
	// whole message (case-insensitive). A match marks the commit Assisted
	// with agent "Unknown AI" unless the pattern has a named group (?P<agent>…).
	MessageMarkers []string
	// DisableMessageMarkers turns off the built-in free-text markers.
	DisableMessageMarkers bool
	// TreatCommitterAsEvidence classifies a commit as Agent when only the
	// committer (not the author) is an AI identity. Off by default because
	// GitHub's web-flow and CI often act as committer for human work.
	TreatCommitterAsEvidence bool
}

// Classifier classifies commits. It is safe for concurrent use.
type Classifier struct {
	opts        Options
	trailerKeys map[string]bool
	markers     []*regexp.Regexp
}

// DefaultMessageMarkers are free-text signatures agents leave in commit
// bodies when no trailer is used.
var DefaultMessageMarkers = []string{
	`(?i)generated\s+with\s+\[?(?P<agent>claude\s+code|codex|cursor|copilot|gemini\s+cli|opencode|aider|amp|cline|windsurf|kiro|junie)\b`,
	`(?i)🤖\s*generated\s+with\s+\[?(?P<agent>[A-Za-z][A-Za-z ]{1,20}?)\]`,
	`(?i)\bmade\s+with\s+(?P<agent>cursor|claude\s+code|codex|windsurf|bolt|lovable|v0)\b`,
	`(?i)\b(?:generated|written|created|authored|implemented)\s+(?:entirely\s+|mostly\s+|fully\s+)?by\s+(?P<agent>claude(?:\s+code)?|codex|cursor|copilot|gemini(?:\s+cli)?|devin|aider|opencode|chatgpt|gpt-?[45][\w.-]*|an?\s+ai(?:\s+agent)?|ai)\b`,
	`(?i)\[(?P<agent>ai)-generated\]`,
	`(?i)^\s*(?:🤖|\[bot\])\s*(?:this\s+)?commit\s+(?:was\s+)?(?:generated|created)\s+by\s+(?P<agent>[A-Za-z][A-Za-z ]{1,20}?)\b`,
}

// NewClassifier builds a classifier from options.
func NewClassifier(opts Options) *Classifier {
	c := &Classifier{opts: opts, trailerKeys: map[string]bool{}}
	for k := range AITrailerKeys {
		c.trailerKeys[k] = true
	}
	for _, k := range opts.ExtraTrailerKeys {
		c.trailerKeys[strings.ToLower(strings.TrimSpace(k))] = true
	}
	if !opts.DisableMessageMarkers {
		for _, p := range DefaultMessageMarkers {
			c.markers = append(c.markers, regexp.MustCompile(p))
		}
	}
	for _, p := range opts.MessageMarkers {
		if rx, err := regexp.Compile(p); err == nil {
			c.markers = append(c.markers, rx)
		}
	}
	return c
}

// Classify determines the Kind of a commit and the agents involved.
func (c *Classifier) Classify(cm Commit) Attribution {
	var att Attribution
	agents := map[string]bool{}
	models := map[string]bool{}
	convs := map[string]bool{}
	add := func(ev Evidence) {
		att.Evidence = append(att.Evidence, ev)
		if ev.Agent != "" {
			agents[ev.Agent] = true
		}
		if ev.Model != "" {
			models[ev.Model] = true
		}
	}

	// 1. Author identity.
	authorIsAgent := false
	if id := LookupAgent(cm.AuthorName, cm.AuthorEmail, c.opts.ExtraAgents); id != nil {
		authorIsAgent = true
		add(Evidence{Source: "author", Value: personString(cm.AuthorName, cm.AuthorEmail), Agent: displayName(id, cm.AuthorName), Model: modelFromName(cm.AuthorName)})
	}
	authorIsBot := false
	if !authorIsAgent {
		if id := LookupBot(cm.AuthorName, cm.AuthorEmail, c.opts.ExtraBots); id != nil {
			authorIsBot = true
			add(Evidence{Source: "author", Value: personString(cm.AuthorName, cm.AuthorEmail), Agent: ""})
		}
	}
	// 2. Committer identity (opt-in).
	committerIsAgent := false
	if c.opts.TreatCommitterAsEvidence && !authorIsAgent {
		if id := LookupAgent(cm.CommitterName, cm.CommitterEmail, c.opts.ExtraAgents); id != nil {
			committerIsAgent = true
			add(Evidence{Source: "committer", Value: personString(cm.CommitterName, cm.CommitterEmail), Agent: displayName(id, cm.CommitterName)})
		}
	}

	// 3. Trailers.
	trailerHit := false
	for _, t := range ParseTrailers(cm.Message) {
		key := strings.ToLower(t.Key)
		switch {
		case key == "co-authored-by" || key == "co-developed-by":
			p := ParsePerson(t.Value)
			if id := LookupAgent(p.Name, p.Email, c.opts.ExtraAgents); id != nil {
				ref := ParseAgentRef(t.Value)
				trailerHit = true
				convs[key] = true
				add(Evidence{Source: "trailer:" + t.Key, Value: t.Value, Agent: displayName(id, p.Name), Model: ref.Model})
			}
		case c.trailerKeys[key]:
			ref := ParseAgentRef(t.Value)
			name := canonicalAgentName(ref.Tool, ref.Email, c.opts.ExtraAgents)
			trailerHit = true
			convs[key] = true
			add(Evidence{Source: "trailer:" + t.Key, Value: t.Value, Agent: name, Model: ref.Model})
		case key == "model":
			// Companion trailer to Coding-Agent; only meaningful with an agent.
			if trailerHit || authorIsAgent {
				models[strings.TrimSpace(t.Value)] = true
			}
		}
	}

	// 4. Free-text markers (only if nothing structured was found).
	markerHit := false
	if !trailerHit && !authorIsAgent {
		for _, rx := range c.markers {
			m := rx.FindStringSubmatch(cm.Message)
			if m == nil {
				continue
			}
			agent := "Unknown AI"
			if idx := rx.SubexpIndex("agent"); idx >= 0 && idx < len(m) && m[idx] != "" {
				agent = canonicalAgentName(m[idx], "", c.opts.ExtraAgents)
			}
			markerHit = true
			add(Evidence{Source: "message", Value: strings.TrimSpace(m[0]), Agent: agent})
			break
		}
	}

	switch {
	case authorIsAgent || committerIsAgent:
		att.Kind = Agent
	case trailerHit || markerHit:
		att.Kind = Assisted
	case authorIsBot:
		att.Kind = Bot
	default:
		att.Kind = Human
	}
	att.Agents = sortedKeys(agents)
	att.Models = sortedKeys(models)
	att.Conventions = sortedKeys(convs)
	return att
}

// displayName returns the canonical identity name, except for the catch-all
// Generic AI rule where the raw bot name (e.g. "agent-think[bot]") is more
// informative than the bucket name.
func displayName(id *Identity, raw string) string {
	if id.Name != "Generic AI" {
		return id.Name
	}
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "Generic AI"
	}
	return raw
}

// canonicalAgentName maps a raw tool token to a canonical agent name using
// the identity tables, falling back to a tidied version of the raw token.
func canonicalAgentName(tool, email string, extra []Identity) string {
	tool = strings.TrimSpace(tool)
	if id := LookupAgent(tool, email, extra); id != nil && id.Name != "Generic AI" {
		return id.Name
	}
	if tool == "" {
		if email != "" {
			return email
		}
		return "Unknown AI"
	}
	if strings.EqualFold(tool, "llm") || strings.EqualFold(tool, "ai") || strings.EqualFold(tool, "an ai") || strings.EqualFold(tool, "an ai agent") {
		return "Unknown AI"
	}
	// Title-case simple tokens like "cursor-agent" -> "cursor-agent" stays; keep raw.
	return tool
}

func modelFromName(name string) string {
	if m := claudeModelRx.FindStringSubmatch(strings.TrimSpace(name)); m != nil {
		return m[1]
	}
	if m := parenRx.FindStringSubmatch(name); m != nil {
		return strings.TrimSpace(m[1])
	}
	return ""
}

func personString(name, email string) string {
	name = strings.TrimSpace(name)
	email = strings.TrimSpace(email)
	switch {
	case name != "" && email != "":
		return name + " <" + email + ">"
	case name != "":
		return name
	default:
		return email
	}
}

func sortedKeys(m map[string]bool) []string {
	if len(m) == 0 {
		return nil
	}
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}
