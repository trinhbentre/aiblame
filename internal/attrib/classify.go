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
	Source string `json:"source"` // author | committer | trailer:<key> | message | notes:<tool> | checkpoint:<tool>
	Value  string `json:"value"`  // the raw matched text
	Agent  string `json:"agent,omitempty"`
	Model  string `json:"model,omitempty"`
	// Reason explains why a candidate was NOT counted (only set on
	// Attribution.Ignored entries).
	Reason string `json:"reason,omitempty"`
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
	// Ignored lists trailer values that looked like disclosure but were not
	// counted, with the reason: a person named in Assisted-by (curl style),
	// "Assisted-by: none", or a Generated-by tool aiblame does not know.
	Ignored []Evidence `json:"ignored,omitempty"`
	// AgentSignedOff is set when a Signed-off-by trailer names an AI
	// identity, which DCO-based projects (Linux, Zephyr, Mesa) forbid.
	AgentSignedOff bool `json:"agent_signed_off,omitempty"`
}

// PrimaryAgent returns the first agent name or "" when none.
func (a Attribution) PrimaryAgent() string {
	if len(a.Agents) == 0 {
		return ""
	}
	return a.Agents[0]
}

// HasConvention reports whether conv is one of the conventions that carried
// AI evidence for this commit.
func (a Attribution) HasConvention(conv string) bool {
	for _, c := range a.Conventions {
		if c == conv {
			return true
		}
	}
	return false
}

// Merge adds evidence discovered outside the commit message (a git-ai
// authorship log, an Entire checkpoint) and upgrades a Human commit to
// Assisted. conv is recorded as the disclosure convention, e.g.
// "git-ai-notes". Agent-authored and bot commits keep their kind.
func (a *Attribution) Merge(ev Evidence, conv string) {
	ev.Model = cleanModel(ev.Model)
	a.Evidence = append(a.Evidence, ev)
	if ev.Agent != "" {
		a.Agents = dropUnknownWhenNamed(insertSorted(a.Agents, ev.Agent))
	}
	if ev.Model != "" {
		a.Models = insertSorted(a.Models, ev.Model)
	}
	if conv != "" {
		a.Conventions = insertSorted(a.Conventions, conv)
	}
	if a.Kind == Human {
		a.Kind = Assisted
	}
}

// SetAgent replaces the agent recorded for every evidence entry with the
// given source (used when an Entire checkpoint reveals which agent a session
// belonged to) and recomputes the agent and model lists.
func (a *Attribution) SetAgent(source, agent, model string) {
	model = cleanModel(model)
	for i := range a.Evidence {
		if a.Evidence[i].Source == source {
			a.Evidence[i].Agent = agent
			if model != "" {
				a.Evidence[i].Model = model
			}
		}
	}
	agents := map[string]bool{}
	models := map[string]bool{}
	for _, ev := range a.Evidence {
		if ev.Agent != "" {
			agents[ev.Agent] = true
		}
		if ev.Model != "" {
			models[ev.Model] = true
		}
	}
	a.Agents = dropUnknownWhenNamed(sortedKeys(agents))
	a.Models = sortedKeys(models)
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

// Marker is a free-text signature agents leave in commit bodies.
type Marker struct {
	Pattern string
	// RequireKnownAgent only accepts a match when the captured agent resolves
	// to a known identity, so "Generated with love" is not AI.
	RequireKnownAgent bool
}

// Classifier classifies commits. It is safe for concurrent use.
type Classifier struct {
	opts    Options
	strong  map[string]bool
	weak    map[string]bool
	markers []compiledMarker
}

type compiledMarker struct {
	rx           *regexp.Regexp
	requireKnown bool
}

// UnknownAI is the agent name used when the evidence proves AI involvement
// but not which tool ("Assisted-by: LLM").
const UnknownAI = "Unknown AI"

// Reasons recorded on Attribution.Ignored entries. Callers compare against
// these constants rather than the text.
const (
	ReasonNegative         = "states no AI was used"
	ReasonPerson           = "names a person, not a tool"
	ReasonNotToolName      = "value is not a tool name"
	ReasonUnrecognisedTool = "tool not recognised as an AI agent (add it under [[detect.agents]])"
	ReasonAIUsedForOther   = "AI used for something other than the code"
)

// DefaultMessageMarkers are free-text signatures agents leave in commit
// bodies when no trailer is used. Every pattern is anchored on a verb phrase
// plus a tool name; generic patterns require the tool to be a known agent.
var DefaultMessageMarkers = []Marker{
	// "🤖 Generated with [Claude Code](https://…)", "Generated with Codebuff 🤖",
	// "Generated with help of Claude Code", "Generated with [Devin](https://devin.ai)".
	{Pattern: `(?i)\bgenerated\s+with\s+(?:the\s+)?(?:(?:help|assistance|support)\s+(?:of|from)\s+)?\[?(?P<agent>[A-Za-z][A-Za-z0-9 .\-]{1,30}?)\]?(?:\s*[\(\[]|\s*https?://|\s*🤖|\s*[.,;!]|\s*$|\s*\n)`, RequireKnownAgent: true},
	// "Generated with omp (AI coding agent)".
	{Pattern: `(?i)\bgenerated\s+with\s+\[?(?P<agent>[A-Za-z][A-Za-z0-9 .\-]{1,30}?)\]?\s*\((?:an?\s+)?ai\s+(?:coding\s+)?(?:agent|assistant|tool)\)`},
	{Pattern: `(?i)\bmade\s+with\s+(?P<agent>[A-Za-z][A-Za-z0-9 .\-]{1,30}?)(?:\s*[\(\[]|\s*https?://|\s*[.,;!]|\s*$|\s*\n)`, RequireKnownAgent: true},
	{Pattern: `(?i)\b(?:generated|written|created|authored|implemented|vibe[- ]?coded)\s+(?:entirely\s+|mostly\s+|fully\s+)?by\s+(?P<agent>claude(?:\s+code)?|codex|cursor|copilot|gemini(?:\s+cli)?|devin|aider|opencode|codebuff|chatgpt|gpt-?[45][\w.-]*|an?\s+ai(?:\s+agent|\s+assistant)?|ai|an?\s+llm)\b`},
	{Pattern: `(?i)\[(?P<agent>ai)-generated\]`},
	{Pattern: `(?i)^\s*(?:🤖|\[bot\])\s*(?:this\s+)?commit\s+(?:was\s+)?(?:generated|created)\s+by\s+(?P<agent>[A-Za-z][A-Za-z ]{1,20}?)\b`},
}

// NewClassifier builds a classifier from options.
func NewClassifier(opts Options) *Classifier {
	c := &Classifier{opts: opts, strong: map[string]bool{}, weak: map[string]bool{}}
	for k := range StrongTrailerKeys {
		c.strong[k] = true
	}
	for k := range WeakTrailerKeys {
		c.weak[k] = true
	}
	for _, k := range opts.ExtraTrailerKeys {
		c.strong[strings.ToLower(strings.TrimSpace(k))] = true
	}
	if !opts.DisableMessageMarkers {
		for _, m := range DefaultMessageMarkers {
			c.markers = append(c.markers, compiledMarker{rx: regexp.MustCompile(m.Pattern), requireKnown: m.RequireKnownAgent})
		}
	}
	for _, p := range opts.MessageMarkers {
		if rx, err := regexp.Compile(p); err == nil {
			c.markers = append(c.markers, compiledMarker{rx: rx})
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
	ignore := func(ev Evidence, reason string) {
		ev.Reason = reason
		att.Ignored = append(att.Ignored, ev)
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
		src := "trailer:" + t.Key
		switch {
		case key == "co-authored-by" || key == "co-developed-by":
			p := ParsePerson(t.Value)
			id := LookupAgent(p.Name, p.Email, c.opts.ExtraAgents)
			if id == nil {
				continue // a human co-author
			}
			ref := ParseAgentRef(t.Value)
			agent := displayName(id, p.Name)
			// "GPT-5.6 Sol <noreply@anthropic.com>": a model-only name with a
			// tool's address means the tool ran that model.
			if id.Name == "GPT" && p.Email != "" {
				if byEmail := LookupAgent("", p.Email, c.opts.ExtraAgents); byEmail != nil && byEmail.Name != "GPT" {
					agent = byEmail.Name
					if ref.Model == "" {
						ref.Model = cleanModel(p.Name)
					}
				}
			}
			trailerHit = true
			convs[key] = true
			add(Evidence{Source: src, Value: t.Value, Agent: agent, Model: ref.Model})

		case key == "signed-off-by":
			// Only humans may certify the DCO. An AI identity here is both
			// AI evidence and a policy violation.
			p := ParsePerson(t.Value)
			if id := LookupAgent(p.Name, p.Email, c.opts.ExtraAgents); id != nil {
				trailerHit = true
				att.AgentSignedOff = true
				convs[key] = true
				add(Evidence{Source: src, Value: t.Value, Agent: displayName(id, p.Name), Model: ParseAgentRef(t.Value).Model})
			}

		case c.strong[key]:
			ev := Evidence{Source: src, Value: t.Value}
			if IsNegativeValue(t.Value) {
				ignore(ev, ReasonNegative)
				continue
			}
			ref := ParseAgentRef(t.Value)
			id := LookupAgent(ref.Tool, ref.Email, c.opts.ExtraAgents)
			if id == nil && key == "ai-used-for" && !aiUsedForCode(t.Value) {
				ignore(ev, ReasonAIUsedForOther)
				continue
			}
			if id == nil && LooksLikePerson(t.Value) {
				ignore(ev, ReasonPerson)
				continue
			}
			trailerHit = true
			convs[key] = true
			ev.Agent = canonicalAgentName(ref.Tool, ref.Email, c.opts.ExtraAgents)
			if key == "ai-used-for" && id == nil {
				ev.Agent = UnknownAI
			}
			ev.Model = ref.Model
			add(ev)

		case c.weak[key]:
			ev := Evidence{Source: src, Value: t.Value}
			if IsNegativeValue(t.Value) {
				ignore(ev, ReasonNegative)
				continue
			}
			ref := ParseAgentRef(t.Value)
			id := LookupAgent(ref.Tool, ref.Email, c.opts.ExtraAgents)
			switch {
			case id != nil:
			case !IsToolLikeName(ref.Tool):
				// A sentence ("Agent: decided to refactor …") is prose, not a tool.
				ignore(ev, ReasonNotToolName)
				continue
			case !HasAIVocabulary(t.Value) && ref.Model == "":
				ignore(ev, ReasonUnrecognisedTool)
				continue
			}
			trailerHit = true
			convs[key] = true
			ev.Agent = canonicalAgentName(ref.Tool, ref.Email, c.opts.ExtraAgents)
			ev.Model = ref.Model
			add(ev)

		case key == "model":
			// Companion trailer to Coding-Agent; only meaningful with an agent.
			if trailerHit || authorIsAgent {
				if m := cleanModel(t.Value); m != "" {
					models[m] = true
				}
			}

		default:
			if agent, ok := SessionTrailerKeys[key]; ok {
				trailerHit = true
				convs[key] = true
				if agent == "" {
					agent = UnknownAI
				}
				add(Evidence{Source: src, Value: t.Value, Agent: agent})
			}
		}
	}

	// 4. Free-text markers (only if nothing structured was found).
	markerHit := false
	if !trailerHit && !authorIsAgent {
		for _, m := range c.markers {
			sm := m.rx.FindStringSubmatch(cm.Message)
			if sm == nil {
				continue
			}
			agent := UnknownAI
			raw := ""
			if idx := m.rx.SubexpIndex("agent"); idx >= 0 && idx < len(sm) && sm[idx] != "" {
				raw = strings.TrimSpace(sm[idx])
				agent = canonicalAgentName(raw, "", c.opts.ExtraAgents)
			}
			if m.requireKnown && (raw == "" || LookupAgent(raw, "", c.opts.ExtraAgents) == nil) {
				continue
			}
			markerHit = true
			add(Evidence{Source: "message", Value: strings.TrimSpace(sm[0]), Agent: agent})
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
	att.Agents = dropUnknownWhenNamed(sortedKeys(agents))
	att.Models = sortedKeys(models)
	att.Conventions = sortedKeys(convs)
	return att
}

// dropUnknownWhenNamed removes the "Unknown AI" placeholder when another
// piece of evidence named the agent: a commit with both an Entire-Checkpoint
// trailer and a Claude co-author trailer was written with Claude Code.
func dropUnknownWhenNamed(agents []string) []string {
	if len(agents) < 2 {
		return agents
	}
	out := agents[:0:0]
	for _, a := range agents {
		if a != UnknownAI {
			out = append(out, a)
		}
	}
	return out
}

// aiUsedForCode interprets QEMU-style "AI-used-for: code, tests" values.
// Values that only mention research, review or translation do not make the
// code itself AI-written.
func aiUsedForCode(v string) bool {
	for _, f := range strings.FieldsFunc(strings.ToLower(v), func(r rune) bool { return r == ',' || r == ';' || r == ' ' || r == '/' }) {
		switch strings.Trim(f, "()") {
		case "research", "review", "analysis", "translation", "message", "commit", "summary", "and", "or", "the", "of", "for", "":
			continue
		default:
			return true
		}
	}
	return false
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
		return UnknownAI
	}
	switch strings.ToLower(tool) {
	case "llm", "ai", "an ai", "an ai agent", "an ai assistant", "an llm", "chatbot", "generic llm chatbot", "llm chatbot", "local llm":
		return UnknownAI
	}
	return tool
}

func modelFromName(name string) string {
	if m := claudeModelRx.FindStringSubmatch(strings.TrimSpace(name)); m != nil {
		return cleanModel(m[1])
	}
	if m := parenRx.FindStringSubmatch(name); m != nil && isModelLike(m[1]) {
		return cleanModel(m[1])
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

func insertSorted(list []string, v string) []string {
	for _, x := range list {
		if x == v {
			return list
		}
	}
	list = append(list, v)
	sort.Strings(list)
	return list
}
