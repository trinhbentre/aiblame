// Package provenance reads the attribution data that hook-based tools leave
// in a repository outside commit messages, so that aiblame can count it
// without any of those tools being installed:
//
//   - git-ai authorship logs: refs/notes/ai (Authorship Log v3 text format)
//     and the older refs/ai/authorship/<sha> JSON blobs (schema 0.0.1). These
//     carry line ranges per file, which gives line-level attribution.
//   - Exceeds Ink: refs/notes/exceeds-ink, same v3 format.
//   - Entire CLI checkpoints: refs/entire/checkpoints/<shard>/<id> (or the
//     entire/checkpoints/v1 branch), referenced from commits by an
//     Entire-Checkpoint trailer. Their metadata names the agent and model.
//   - Claudit conversations: refs/notes/claude-conversations (presence only).
//
// Everything here is read with plain git plumbing; nothing is inferred from
// code style. Data that does not parse is skipped and reported as a warning.
package provenance

import (
	"context"
	"fmt"
	"path"
	"sort"
	"strings"

	"github.com/trinhbentre/aiblame/internal/attrib"
	"github.com/trinhbentre/aiblame/internal/gitx"
)

// Source names. They double as disclosure-convention names in reports.
const (
	SourceGitAI      = "git-ai-notes"
	SourceExceedsInk = "exceeds-ink-notes"
	SourceEntire     = "entire-checkpoint"
	SourceClaudit    = "claude-conversations-notes"
)

// EntireTrailerSource is the evidence source the classifier records for an
// Entire-Checkpoint trailer; Load resolves the agent behind it.
const EntireTrailerSource = "trailer:Entire-Checkpoint"

// MaxBlobSize is the largest sidecar blob aiblame will parse. Authorship
// logs and checkpoint metadata are kilobytes; anything bigger is skipped
// with a warning rather than fed to the JSON decoder.
const MaxBlobSize = 4 << 20

// LineRange is an inclusive 1-based range of lines.
type LineRange struct{ Start, End int }

// Contains reports whether n falls inside the range.
func (r LineRange) Contains(n int) bool { return n >= r.Start && n <= r.End }

// FileAttribution is the per-line provenance of one file as committed.
type FileAttribution struct {
	AI    []LineRange
	Human []LineRange
}

// Kind reports whether a line was written by an agent. known is false for
// lines the log does not mention ("untracked": provenance unknown).
func (f *FileAttribution) Kind(line int) (ai, known bool) {
	if f == nil {
		return false, false
	}
	for _, r := range f.AI {
		if r.Contains(line) {
			return true, true
		}
	}
	for _, r := range f.Human {
		if r.Contains(line) {
			return false, true
		}
	}
	return false, false
}

// Record is what a sidecar says about one commit.
type Record struct {
	Source string
	// Evidence has one entry per agent (and model) that touched the commit.
	Evidence []attrib.Evidence
	// Files maps the path as committed to its line attribution. Nil when the
	// source only knows about the commit as a whole.
	Files map[string]*FileAttribution
	// AILines / HumanLines are the attested line totals.
	AILines, HumanLines int
}

// LineLevel reports whether the record can attribute individual lines.
func (r *Record) LineLevel() bool { return r != nil && len(r.Files) > 0 }

// Checkpoint is the part of an Entire checkpoint aiblame uses.
type Checkpoint struct {
	ID       string
	Agent    string // canonical agent name ("Claude Code"), "" when unknown
	Model    string
	Sessions []Session
}

// Session is one agent session inside a checkpoint.
type Session struct {
	Agent, Model, SessionID string
	FilesTouched            []string
	AgentLines              int
	HumanAdded              int
	HumanModified           int
	AgentPercentage         float64
}

// Store holds everything Load found.
type Store struct {
	records     map[string]*Record
	checkpoints map[string]*Checkpoint
	lineLevel   int
	// Sources counts commits per source.
	Sources map[string]int
	// Warnings are problems that did not stop the load (unparseable notes,
	// checkpoints referenced but not fetched).
	Warnings []string
	// EntireStorage reports whether any Entire checkpoint data exists.
	EntireStorage bool
}

// Options tunes Load.
type Options struct {
	// ExtraAgents are user-defined identities used to canonicalise tool names.
	ExtraAgents []attrib.Identity
	// CheckpointIDs are the Entire checkpoint ids referenced by commits; only
	// these are read.
	CheckpointIDs []string
}

// ForCommit returns the sidecar record for a commit hash, or nil.
func (s *Store) ForCommit(hash string) *Record {
	if s == nil {
		return nil
	}
	return s.records[hash]
}

// Checkpoint returns the resolved Entire checkpoint for an id, or nil.
func (s *Store) Checkpoint(id string) *Checkpoint {
	if s == nil {
		return nil
	}
	return s.checkpoints[strings.TrimSpace(id)]
}

// HasLineLevel reports whether any record carries line ranges (so blame
// should keep per-line detail).
func (s *Store) HasLineLevel() bool { return s != nil && s.lineLevel > 0 }

// LineLevelCommits is the number of commits with line-level attribution.
func (s *Store) LineLevelCommits() int {
	if s == nil {
		return 0
	}
	return s.lineLevel
}

// Empty reports whether nothing was found.
func (s *Store) Empty() bool {
	return s == nil || (len(s.records) == 0 && len(s.checkpoints) == 0)
}

// Load reads every supported sidecar from the repository.
func Load(ctx context.Context, r *gitx.Runner, opts Options) (*Store, error) {
	s := &Store{records: map[string]*Record{}, checkpoints: map[string]*Checkpoint{}, Sources: map[string]int{}}
	if err := s.loadNotes(ctx, r, "ai", SourceGitAI, opts); err != nil {
		return nil, fmt.Errorf("provenance: refs/notes/ai: %w", err)
	}
	if err := s.loadAuthorshipRefs(ctx, r, opts); err != nil {
		return nil, fmt.Errorf("provenance: refs/ai/authorship: %w", err)
	}
	if err := s.loadNotes(ctx, r, "exceeds-ink", SourceExceedsInk, opts); err != nil {
		return nil, fmt.Errorf("provenance: refs/notes/exceeds-ink: %w", err)
	}
	if err := s.loadPresenceNotes(ctx, r, "claude-conversations", SourceClaudit, "Claude Code"); err != nil {
		return nil, fmt.Errorf("provenance: refs/notes/claude-conversations: %w", err)
	}
	if err := s.loadEntire(ctx, r, opts); err != nil {
		return nil, fmt.Errorf("provenance: Entire checkpoints: %w", err)
	}
	for _, rec := range s.records {
		if rec.LineLevel() {
			s.lineLevel++
		}
	}
	return s, nil
}

// loadNotes reads a notes ref whose blobs are Authorship Logs.
func (s *Store) loadNotes(ctx context.Context, r *gitx.Runner, ref, source string, opts Options) error {
	notes, err := r.NotesList(ctx, ref)
	if err != nil {
		return err
	}
	if len(notes) == 0 {
		return nil
	}
	oids := make([]string, 0, len(notes))
	byOID := map[string][]string{}
	for commit, oid := range notes {
		if _, seen := byOID[oid]; !seen {
			oids = append(oids, oid)
		}
		byOID[oid] = append(byOID[oid], commit)
	}
	sort.Strings(oids)
	blobs, err := r.CatFileBatch(ctx, oids)
	if err != nil {
		return err
	}
	bad := 0
	for _, oid := range oids {
		if len(blobs[oid]) > MaxBlobSize {
			bad++
			continue
		}
		rec, err := ParseAuthorshipLog(blobs[oid], opts.ExtraAgents)
		if err != nil {
			bad++
			continue
		}
		rec.Source = source
		for i := range rec.Evidence {
			rec.Evidence[i].Source = "notes:" + ref
		}
		for _, commit := range byOID[oid] {
			if _, exists := s.records[commit]; !exists {
				s.records[commit] = rec
				s.Sources[source]++
			}
		}
	}
	if bad > 0 {
		s.Warnings = append(s.Warnings, fmt.Sprintf("%d note(s) in refs/notes/%s could not be parsed as authorship logs and were skipped", bad, ref))
	}
	return nil
}

// loadAuthorshipRefs reads git-ai's older refs/ai/authorship/<sha> blobs.
func (s *Store) loadAuthorshipRefs(ctx context.Context, r *gitx.Runner, opts Options) error {
	refs, err := r.ForEachRef(ctx, "refs/ai/authorship/")
	if err != nil {
		return err
	}
	if len(refs) == 0 {
		return nil
	}
	var specs []string
	commitOf := map[string]string{}
	for _, ref := range refs {
		if ref.Type != "blob" {
			continue
		}
		commit := path.Base(ref.Name)
		if _, exists := s.records[commit]; exists {
			continue
		}
		specs = append(specs, ref.Object)
		commitOf[ref.Object] = commit
	}
	blobs, err := r.CatFileBatch(ctx, specs)
	if err != nil {
		return err
	}
	bad := 0
	for _, oid := range specs {
		if len(blobs[oid]) > MaxBlobSize {
			bad++
			continue
		}
		rec, err := ParseAuthorshipLog(blobs[oid], opts.ExtraAgents)
		if err != nil {
			bad++
			continue
		}
		rec.Source = SourceGitAI
		for i := range rec.Evidence {
			rec.Evidence[i].Source = "refs/ai/authorship"
		}
		s.records[commitOf[oid]] = rec
		s.Sources[SourceGitAI]++
	}
	if bad > 0 {
		s.Warnings = append(s.Warnings, fmt.Sprintf("%d refs/ai/authorship entries could not be parsed and were skipped", bad))
	}
	return nil
}

// loadPresenceNotes treats every commit that has a note in ref as
// AI-assisted by agent (used for conversation-transcript notes).
func (s *Store) loadPresenceNotes(ctx context.Context, r *gitx.Runner, ref, source, agent string) error {
	notes, err := r.NotesList(ctx, ref)
	if err != nil {
		return err
	}
	for commit := range notes {
		if _, exists := s.records[commit]; exists {
			continue
		}
		s.records[commit] = &Record{Source: source, Evidence: []attrib.Evidence{{Source: "notes:" + ref, Value: "conversation transcript attached", Agent: agent}}}
		s.Sources[source]++
	}
	return nil
}

// loadEntire resolves the referenced checkpoint ids.
func (s *Store) loadEntire(ctx context.Context, r *gitx.Runner, opts Options) error {
	ids := uniqueSorted(opts.CheckpointIDs)
	refs, err := r.ForEachRef(ctx, "refs/entire/checkpoints/")
	if err != nil {
		return err
	}
	refOf := map[string]string{}
	for _, ref := range refs {
		refOf[path.Base(ref.Name)] = ref.Name
	}
	var branch string
	for _, b := range []string{"refs/heads/entire/checkpoints/v1", "refs/remotes/origin/entire/checkpoints/v1"} {
		if r.RefExists(ctx, b) {
			branch = b
			break
		}
	}
	s.EntireStorage = len(refOf) > 0 || branch != ""
	if len(ids) == 0 {
		return nil
	}
	if !s.EntireStorage {
		s.Warnings = append(s.Warnings, fmt.Sprintf("%d commit(s) reference Entire checkpoints but no checkpoint data is present; fetch it with: git fetch origin '+refs/entire/checkpoints/*:refs/entire/checkpoints/*' '+refs/heads/entire/checkpoints/v1:refs/remotes/origin/entire/checkpoints/v1'", len(ids)))
		return nil
	}
	// Every candidate location for every id, in one batch.
	var specs []string
	specOwner := map[string]string{}
	for _, id := range ids {
		for _, base := range entireBases(id, refOf[id], branch) {
			spec := base + "/metadata.json"
			if !strings.Contains(base, ":") {
				spec = base + ":metadata.json"
			}
			specs = append(specs, spec)
			specOwner[spec] = id
		}
	}
	blobs, err := r.CatFileBatch(ctx, specs)
	if err != nil {
		return err
	}
	baseOf := map[string]string{}
	tops := map[string]*entireTop{}
	for _, spec := range specs {
		b, ok := blobs[spec]
		if !ok {
			continue
		}
		id := specOwner[spec]
		if _, done := tops[id]; done {
			continue
		}
		if len(b) > MaxBlobSize {
			s.Warnings = append(s.Warnings, fmt.Sprintf("Entire checkpoint %s: metadata.json larger than %d bytes, skipped", id, MaxBlobSize))
			continue
		}
		top, err := parseEntireTop(b)
		if err != nil {
			s.Warnings = append(s.Warnings, fmt.Sprintf("Entire checkpoint %s: %v", id, err))
			continue
		}
		tops[id] = top
		baseOf[id] = strings.TrimSuffix(strings.TrimSuffix(spec, "/metadata.json"), ":metadata.json")
	}
	// Session metadata (agent, model, attribution) lives in per-session files.
	var sessSpecs []string
	sessOwner := map[string]string{}
	for id, top := range tops {
		var paths []string
		for _, sess := range top.Sessions {
			if p := strings.TrimPrefix(sess.Metadata, "/"); p != "" {
				paths = append(paths, p)
			}
		}
		if len(paths) == 0 {
			// Older layouts kept a single session under 0/.
			paths = []string{"0/metadata.json"}
		}
		for _, p := range paths {
			spec := joinSpec(baseOf[id], p)
			sessSpecs = append(sessSpecs, spec)
			sessOwner[spec] = id
		}
	}
	sort.Strings(sessSpecs)
	sessBlobs, err := r.CatFileBatch(ctx, sessSpecs)
	if err != nil {
		return err
	}
	for _, id := range ids {
		top, ok := tops[id]
		if !ok {
			continue
		}
		cp := &Checkpoint{ID: id, Agent: canonicalAgent(top.Agent, opts.ExtraAgents), Model: top.Model}
		for _, spec := range sessSpecs {
			if sessOwner[spec] != id {
				continue
			}
			b, ok := sessBlobs[spec]
			if !ok || len(b) > MaxBlobSize {
				continue
			}
			sess, err := parseEntireSession(b)
			if err != nil {
				s.Warnings = append(s.Warnings, fmt.Sprintf("Entire checkpoint %s: %v", id, err))
				continue
			}
			sess.Agent = canonicalAgent(sess.Agent, opts.ExtraAgents)
			cp.Sessions = append(cp.Sessions, sess)
			if cp.Agent == "" {
				cp.Agent = sess.Agent
			}
			if cp.Model == "" {
				cp.Model = sess.Model
			}
		}
		s.checkpoints[id] = cp
		s.Sources[SourceEntire]++
	}
	if missing := len(ids) - len(tops); missing > 0 {
		s.Warnings = append(s.Warnings, fmt.Sprintf("%d Entire checkpoint(s) referenced by commits were not found locally (run `git fetch origin '+refs/entire/checkpoints/*:refs/entire/checkpoints/*'`)", missing))
	}
	return nil
}

// entireBases lists the object-spec prefixes where a checkpoint may live:
// its own ref (ULID ids, sharded by the last two characters) or the v1
// branch (12-hex ids, sharded by the first two characters).
func entireBases(id, ref, branch string) []string {
	var out []string
	if ref != "" {
		out = append(out, ref)
	}
	if branch != "" && len(id) > 2 {
		out = append(out, branch+":"+id[:2]+"/"+id[2:])
	}
	return out
}

// joinSpec appends a tree path to a base that is either "<ref>" or
// "<ref>:<dir>".
func joinSpec(base, p string) string {
	if strings.Contains(base, ":") {
		return base + "/" + p
	}
	return base + ":" + p
}

// canonicalAgent maps a tool label from a sidecar ("claude", "Claude Code",
// "cursor", "copilot-cli") to aiblame's identity name.
func canonicalAgent(tool string, extra []attrib.Identity) string {
	t := strings.TrimSpace(strings.ReplaceAll(tool, "_", "-"))
	if t == "" {
		return ""
	}
	if id := attrib.LookupAgent(t, "", extra); id != nil && id.Name != "Generic AI" {
		return id.Name
	}
	if id := attrib.LookupAgent(strings.ReplaceAll(t, "-", " "), "", extra); id != nil && id.Name != "Generic AI" {
		return id.Name
	}
	return t
}

func uniqueSorted(in []string) []string {
	seen := map[string]bool{}
	var out []string
	for _, s := range in {
		s = strings.TrimSpace(s)
		if s == "" || seen[s] {
			continue
		}
		seen[s] = true
		out = append(out, s)
	}
	sort.Strings(out)
	return out
}
