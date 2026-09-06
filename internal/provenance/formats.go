package provenance

import (
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/trinhbentre/aiblame/internal/attrib"
)

// ParseAuthorshipLog parses a git-ai / Exceeds Ink authorship log. Two
// formats exist in the wild:
//
//   - Authorship Log v3 (refs/notes/ai, spec git_ai_standard_v3.0.0.md): an
//     attestation section mapping files to "<key> <ranges>" lines, a "---"
//     divider, then a JSON metadata object. Keys are "s_<session>::t_<turn>"
//     (agent), "h_<hash>" (human) or a legacy 16-hex prompt id (agent).
//   - schema authorship/0.0.1 (refs/ai/authorship/<sha>): a JSON object with
//     files -> authors -> line lists, where the author is an agent label
//     ("Claude", "Cursor") or a human name.
//
// The returned Record has Evidence entries whose Source is
// "authorship-log"; callers replace it with the ref they read from.
func ParseAuthorshipLog(b []byte, extra []attrib.Identity) (*Record, error) {
	s := strings.TrimSpace(string(b))
	if s == "" {
		return nil, errors.New("empty authorship log")
	}
	if strings.HasPrefix(s, "{") {
		return parseLegacyAuthorship(s, extra)
	}
	return parseAuthorshipV3(s, extra)
}

type agentID struct {
	Tool  string `json:"tool"`
	ID    string `json:"id"`
	Model string `json:"model"`
}

type v3Meta struct {
	SchemaVersion string `json:"schema_version"`
	Sessions      map[string]struct {
		AgentID     agentID `json:"agent_id"`
		HumanAuthor string  `json:"human_author"`
	} `json:"sessions"`
	Humans map[string]struct {
		Author string `json:"author"`
	} `json:"humans"`
	Prompts map[string]struct {
		AgentID     agentID `json:"agent_id"`
		HumanAuthor string  `json:"human_author"`
	} `json:"prompts"`
}

// agentKey identifies an (agent, model) pair for evidence aggregation.
type agentKey struct{ agent, model string }

func parseAuthorshipV3(s string, extra []attrib.Identity) (*Record, error) {
	lines := strings.Split(s, "\n")
	div := -1
	for i, l := range lines {
		if strings.TrimRight(l, "\r") == "---" {
			div = i
			break
		}
	}
	if div < 0 {
		return nil, errors.New("authorship log has no --- divider")
	}
	var meta v3Meta
	if err := json.Unmarshal([]byte(strings.Join(lines[div+1:], "\n")), &meta); err != nil {
		return nil, fmt.Errorf("authorship log metadata: %w", err)
	}
	rec := &Record{Files: map[string]*FileAttribution{}}
	counts := map[agentKey]int{}
	cur := ""
	for _, raw := range lines[:div] {
		line := strings.TrimRight(raw, "\r")
		if strings.TrimSpace(line) == "" {
			continue
		}
		if !strings.HasPrefix(line, " ") && !strings.HasPrefix(line, "\t") {
			cur = unquoteLogPath(line)
			continue
		}
		f := strings.Fields(line)
		if len(f) < 2 || cur == "" {
			continue
		}
		ranges, n := parseRanges(f[1])
		if n == 0 {
			continue
		}
		fa := rec.Files[cur]
		if fa == nil {
			fa = &FileAttribution{}
			rec.Files[cur] = fa
		}
		key := f[0]
		switch {
		case strings.HasPrefix(key, "h_"):
			fa.Human = append(fa.Human, ranges...)
			rec.HumanLines += n
		default:
			var id agentID
			if strings.HasPrefix(key, "s_") {
				sid, _, _ := strings.Cut(key, "::")
				id = meta.Sessions[sid].AgentID
			} else if p, ok := meta.Prompts[key]; ok {
				id = p.AgentID
			}
			fa.AI = append(fa.AI, ranges...)
			rec.AILines += n
			counts[agentKey{agentOrUnknown(id.Tool, extra), attrib.CleanModel(id.Model)}] += n
		}
	}
	// An empty attestation section is a valid log: the tool looked at the
	// commit and attributed nothing to an agent. It stays a record (the
	// commit is covered) with no evidence and no line data.
	if len(rec.Files) == 0 {
		rec.Files = nil
	}
	rec.Evidence = evidenceFromCounts(counts, len(rec.Files))
	return rec, nil
}

type legacyAuthorship struct {
	SchemaVersion string `json:"schema_version"`
	Files         map[string]struct {
		File    string `json:"file"`
		Authors []struct {
			Author        string            `json:"author"`
			Lines         []json.RawMessage `json:"lines"`
			AgentMetadata map[string]any    `json:"agent_metadata"`
		} `json:"authors"`
	} `json:"files"`
}

func parseLegacyAuthorship(s string, extra []attrib.Identity) (*Record, error) {
	var doc legacyAuthorship
	if err := json.Unmarshal([]byte(s), &doc); err != nil {
		return nil, fmt.Errorf("authorship json: %w", err)
	}
	if doc.Files == nil {
		return nil, errors.New("authorship json has no files")
	}
	rec := &Record{Files: map[string]*FileAttribution{}}
	counts := map[agentKey]int{}
	for p, f := range doc.Files {
		if f.File != "" {
			p = f.File
		}
		fa := &FileAttribution{}
		for _, a := range f.Authors {
			agent, model := "", ""
			if t, ok := a.AgentMetadata["tool"].(string); ok && t != "" {
				agent = agentOrUnknown(t, extra)
				if m, ok := a.AgentMetadata["model"].(string); ok {
					model = attrib.CleanModel(m)
				}
			} else if id := attrib.LookupAgent(a.Author, "", extra); id != nil && id.Name != "Generic AI" {
				agent = id.Name
			}
			// Entries repeat and overlap in the wild ("186, 186, [182,183],
			// [183,183]"), so count distinct lines by merging intervals —
			// never by materialising them, since the blob is untrusted.
			var ranges []LineRange
			for _, raw := range a.Lines {
				if r, ok := parseRawRange(raw); ok {
					ranges = append(ranges, r)
				}
			}
			ranges, n := mergeRanges(ranges)
			if n == 0 {
				continue
			}
			if agent == "" {
				fa.Human = append(fa.Human, ranges...)
				rec.HumanLines += n
			} else {
				fa.AI = append(fa.AI, ranges...)
				rec.AILines += n
				counts[agentKey{agent, model}] += n
			}
		}
		if len(fa.AI)+len(fa.Human) > 0 {
			rec.Files[p] = fa
		}
	}
	rec.Evidence = evidenceFromCounts(counts, len(rec.Files))
	return rec, nil
}

// maxLine bounds line numbers accepted from sidecar data. No source file has
// ten million lines; anything above is a corrupt or hostile blob.
const maxLine = 10_000_000

// maxRangesPerEntry bounds how many ranges one author/attestation entry may
// carry before the rest is ignored.
const maxRangesPerEntry = 100_000

// parseRawRange decodes a 0.0.1 line entry: 22 or [8,34].
func parseRawRange(raw json.RawMessage) (LineRange, bool) {
	var n int64
	if err := json.Unmarshal(raw, &n); err == nil {
		if n <= 0 || n > maxLine {
			return LineRange{}, false
		}
		return LineRange{int(n), int(n)}, true
	}
	var pair []int64
	if err := json.Unmarshal(raw, &pair); err == nil && len(pair) == 2 && pair[0] > 0 && pair[1] >= pair[0] && pair[1] <= maxLine {
		return LineRange{int(pair[0]), int(pair[1])}, true
	}
	return LineRange{}, false
}

// mergeRanges sorts, deduplicates and merges overlapping ranges, returning
// the merged list and the number of distinct lines it covers. The cost is
// O(n log n) in the number of ranges, independent of their width.
func mergeRanges(in []LineRange) ([]LineRange, int) {
	if len(in) == 0 {
		return nil, 0
	}
	if len(in) > maxRangesPerEntry {
		in = in[:maxRangesPerEntry]
	}
	sort.Slice(in, func(i, j int) bool {
		if in[i].Start != in[j].Start {
			return in[i].Start < in[j].Start
		}
		return in[i].End < in[j].End
	})
	out := []LineRange{in[0]}
	for _, r := range in[1:] {
		last := &out[len(out)-1]
		if r.Start <= last.End+1 {
			if r.End > last.End {
				last.End = r.End
			}
			continue
		}
		out = append(out, r)
	}
	total := 0
	for _, r := range out {
		total += r.End - r.Start + 1
	}
	return out, total
}

// parseRanges decodes "1,2,19-222,300" and returns the ranges and the number
// of lines they cover. Malformed or absurd pieces are skipped.
func parseRanges(s string) ([]LineRange, int) {
	var out []LineRange
	for _, piece := range strings.Split(s, ",") {
		piece = strings.TrimSpace(piece)
		if piece == "" {
			continue
		}
		a, b, isRange := strings.Cut(piece, "-")
		start, err := strconv.Atoi(a)
		if err != nil || start <= 0 || start > maxLine {
			continue
		}
		end := start
		if isRange {
			end, err = strconv.Atoi(b)
			if err != nil || end < start || end > maxLine {
				continue
			}
		}
		out = append(out, LineRange{start, end})
		if len(out) >= maxRangesPerEntry {
			break
		}
	}
	return mergeRanges(out)
}

func unquoteLogPath(p string) string {
	p = strings.TrimSpace(p)
	if len(p) >= 2 && p[0] == '"' && p[len(p)-1] == '"' {
		if s, err := strconv.Unquote(p); err == nil {
			return s
		}
	}
	return p
}

func agentOrUnknown(tool string, extra []attrib.Identity) string {
	if a := canonicalAgent(tool, extra); a != "" {
		return a
	}
	return attrib.UnknownAI
}

func evidenceFromCounts(counts map[agentKey]int, files int) []attrib.Evidence {
	keys := make([]agentKey, 0, len(counts))
	for k := range counts {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool {
		if keys[i].agent != keys[j].agent {
			return keys[i].agent < keys[j].agent
		}
		return keys[i].model < keys[j].model
	})
	var out []attrib.Evidence
	for _, k := range keys {
		out = append(out, attrib.Evidence{
			Source: "authorship-log",
			Value:  fmt.Sprintf("%d line(s) in %d file(s)", counts[k], files),
			Agent:  k.agent,
			Model:  k.model,
		})
	}
	return out
}

// ---- Entire -------------------------------------------------------------

// entireTop is the checkpoint-level metadata.json.
type entireTop struct {
	CheckpointID string   `json:"checkpoint_id"`
	Agent        string   `json:"agent"`
	Model        string   `json:"model"`
	FilesTouched []string `json:"files_touched"`
	Sessions     []struct {
		Metadata string `json:"metadata"`
	} `json:"sessions"`
}

func parseEntireTop(b []byte) (*entireTop, error) {
	var top entireTop
	if err := json.Unmarshal(b, &top); err != nil {
		return nil, fmt.Errorf("metadata.json: %w", err)
	}
	return &top, nil
}

type entireSession struct {
	Agent              string   `json:"agent"`
	Model              string   `json:"model"`
	SessionID          string   `json:"session_id"`
	FilesTouched       []string `json:"files_touched"`
	InitialAttribution *struct {
		AgentLines      int     `json:"agent_lines"`
		HumanAdded      int     `json:"human_added"`
		HumanModified   int     `json:"human_modified"`
		AgentPercentage float64 `json:"agent_percentage"`
	} `json:"initial_attribution"`
}

func parseEntireSession(b []byte) (Session, error) {
	var es entireSession
	if err := json.Unmarshal(b, &es); err != nil {
		return Session{}, fmt.Errorf("session metadata.json: %w", err)
	}
	s := Session{Agent: strings.TrimSpace(es.Agent), Model: attrib.CleanModel(es.Model), SessionID: es.SessionID, FilesTouched: es.FilesTouched}
	if ia := es.InitialAttribution; ia != nil {
		s.AgentLines, s.HumanAdded, s.HumanModified, s.AgentPercentage = ia.AgentLines, ia.HumanAdded, ia.HumanModified, ia.AgentPercentage
	}
	return s, nil
}
