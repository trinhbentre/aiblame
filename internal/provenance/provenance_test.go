package provenance

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/trinhbentre/aiblame/internal/gitx"
	"github.com/trinhbentre/aiblame/internal/testrepo"
)

// From git_ai_standard_v3.0.0.md (sessions + humans + legacy prompt key).
const v3Note = `src/main.rs
  s_c9883b05a2487d::t_9f8e7d6c5b4a32 1-10,15-20
  h_31dce776f88375 42-50
"src/my file.rs"
  abcd1234abcd1234 1-3
src/lib.rs
  s_e7f2a90b31cc48::t_deadbeef012345 1-50
---
{
  "schema_version": "authorship/3.0.0",
  "git_ai_version": "1.4.5",
  "base_commit_sha": "7734793b756b3921c88db5375a8c156e9532447b",
  "prompts": {
    "abcd1234abcd1234": {"agent_id": {"tool": "claude", "id": "x", "model": "claude-opus-4-6"}, "human_author": "dev@example.com"}
  },
  "humans": {"h_31dce776f88375": {"author": "Developer <dev@example.com>"}},
  "sessions": {
    "s_c9883b05a2487d": {"agent_id": {"tool": "cursor", "id": "6ef2299e", "model": "claude-sonnet-4-5-20250514"}, "human_author": "dev@example.com"},
    "s_e7f2a90b31cc48": {"agent_id": {"tool": "copilot_cli", "id": "abc", "model": "gpt-5.6-sol"}, "human_author": "dev@example.com"}
  }
}`

// Real refs/ai/authorship/<sha> blob from git-ai-project/git-ai (schema 0.0.1).
const legacyNote = `{"files":{".github/workflows/release.yml":{"file":".github/workflows/release.yml","authors":[{"author":"Aidan Cunniffe","lines":[39,[8,34]],"agent_metadata":null}]},".vscode/settings.json":{"file":".vscode/settings.json","authors":[{"author":"Cursor","lines":[2],"agent_metadata":null}]},"agent-support/cursor/src/extension.ts":{"file":"agent-support/cursor/src/extension.ts","authors":[{"author":"Cursor","lines":[186,186,[182,183],[183,183],[182,183]],"agent_metadata":null},{"author":"Aidan Cunniffe","lines":[182,[158,160]],"agent_metadata":null}]}},"schema_version":"authorship/0.0.1"}`

// Real Entire checkpoint metadata (entireio/cli, July 2026), trimmed.
const entireTopJSON = `{
  "cli_version": "dev",
  "checkpoint_id": "01KXEWS08ZG9WXQPFE4QDA4GA3",
  "strategy": "manual-commit",
  "branch": "cli-onboard",
  "checkpoints_count": 1,
  "files_touched": ["cmd/entire/cli/onboarding_offer_actions.go"],
  "sessions": [{"metadata": "/0/metadata.json", "transcript": "/0/full.jsonl", "prompt": "/0/prompt.txt"}],
  "token_usage": {"input_tokens": 49, "output_tokens": 10057}
}`

const entireSessionJSON = `{
  "cli_version": "dev",
  "checkpoint_id": "01KXEWS08ZG9WXQPFE4QDA4GA3",
  "session_id": "50e3afb2-c325-45e7-aec5-3f54b10a12e7",
  "strategy": "manual-commit",
  "created_at": "2026-07-13T23:25:17.89354Z",
  "files_touched": ["cmd/entire/cli/onboarding_offer_actions.go"],
  "agent": "Claude Code",
  "model": "claude-fable-5",
  "initial_attribution": {"agent_lines": 2, "agent_removed": 17, "human_added": 0, "human_modified": 0, "agent_percentage": 100, "metric_version": 2}
}`

func TestParseAuthorshipLogV3(t *testing.T) {
	rec, err := ParseAuthorshipLog([]byte(v3Note), nil)
	if err != nil {
		t.Fatal(err)
	}
	if rec.AILines != 16+3+50 || rec.HumanLines != 9 {
		t.Fatalf("lines ai=%d human=%d", rec.AILines, rec.HumanLines)
	}
	if len(rec.Files) != 3 || rec.Files["src/my file.rs"] == nil {
		t.Fatalf("files = %v", rec.Files)
	}
	main := rec.Files["src/main.rs"]
	if ai, known := main.Kind(3); !ai || !known {
		t.Fatal("line 3 should be AI")
	}
	if ai, known := main.Kind(45); ai || !known {
		t.Fatal("line 45 should be human")
	}
	if _, known := main.Kind(12); known {
		t.Fatal("line 12 is untracked")
	}
	var agents []string
	for _, ev := range rec.Evidence {
		agents = append(agents, ev.Agent+"/"+ev.Model)
	}
	want := "Claude Code/claude-opus-4-6, Cursor/claude-sonnet-4-5-20250514, GitHub Copilot/gpt-5.6-sol"
	if got := strings.Join(agents, ", "); got != want {
		t.Fatalf("evidence = %q, want %q", got, want)
	}
	if rec.Evidence[1].Value != "16 line(s) in 3 file(s)" {
		t.Fatalf("value = %q", rec.Evidence[1].Value)
	}
	if _, err := ParseAuthorshipLog([]byte("no divider here"), nil); err == nil {
		t.Fatal("expected error without divider")
	}
	// An empty attestation section (git-ai found nothing to attribute) is a
	// valid record with no evidence and no line data — 469 of the notes in
	// git-ai's own repository look like this.
	empty, err := ParseAuthorshipLog([]byte("---\n{\"schema_version\":\"authorship/3.0.0\",\"base_commit_sha\":\"\",\"prompts\":{}}"), nil)
	if err != nil || len(empty.Evidence) != 0 || empty.LineLevel() {
		t.Fatalf("empty attestation: %+v err=%v", empty, err)
	}
	if _, err := ParseAuthorshipLog([]byte("src/a.rs\n  s_1::t_1 1-3\n---\nnot json"), nil); err == nil {
		t.Fatal("expected error for bad metadata")
	}
}

func TestParseAuthorshipLogLegacy(t *testing.T) {
	rec, err := ParseAuthorshipLog([]byte(legacyNote), nil)
	if err != nil {
		t.Fatal(err)
	}
	// Cursor: 1 (settings.json) + 186, 182-183 (extension.ts, duplicates collapsed) = 4
	if rec.AILines != 4 {
		t.Fatalf("ai lines = %d", rec.AILines)
	}
	// Aidan: 39, 8-34 (28) + 182, 158-160 (4) = 32
	if rec.HumanLines != 32 {
		t.Fatalf("human lines = %d", rec.HumanLines)
	}
	if len(rec.Evidence) != 1 || rec.Evidence[0].Agent != "Cursor" {
		t.Fatalf("evidence = %+v", rec.Evidence)
	}
	ext := rec.Files["agent-support/cursor/src/extension.ts"]
	if ai, known := ext.Kind(183); !ai || !known {
		t.Fatal("183 should be AI")
	}
	if ai, known := ext.Kind(159); ai || !known {
		t.Fatal("159 should be human")
	}
	if _, err := ParseAuthorshipLog([]byte(`{"schema_version":"authorship/0.0.1"}`), nil); err == nil {
		t.Fatal("expected error without files")
	}
}

// TestHostileRanges guards against a one-line denial of service: a legacy
// blob with [1, 9223372036854775807] must not be expanded line by line, and
// absurd line numbers are dropped.
func TestHostileRanges(t *testing.T) {
	hostile := `{"files":{"a.rs":{"authors":[{"author":"Cursor","lines":[[1,9223372036854775807],[5,10],[7,12],3,3]}]}},"schema_version":"authorship/0.0.1"}`
	rec, err := ParseAuthorshipLog([]byte(hostile), nil)
	if err != nil {
		t.Fatal(err)
	}
	// [1,MaxInt64] is dropped; 3, 5-10, 7-12 merge into 3 + 5-12 = 9 lines.
	if rec.AILines != 9 {
		t.Fatalf("ai lines = %d", rec.AILines)
	}
	fa := rec.Files["a.rs"]
	if ai, known := fa.Kind(11); !ai || !known {
		t.Fatal("11 should be AI after merging")
	}
	if _, known := fa.Kind(4); known {
		t.Fatal("4 is untracked")
	}
	v3 := "a.rs\n  s_1::t_1 1-99999999999,5-7,6-9\n---\n{\"schema_version\":\"authorship/3.0.0\",\"base_commit_sha\":\"\",\"prompts\":{},\"sessions\":{\"s_1\":{\"agent_id\":{\"tool\":\"cursor\",\"id\":\"x\",\"model\":\"m\"}}}}"
	rec, err = ParseAuthorshipLog([]byte(v3), nil)
	if err != nil || rec.AILines != 5 {
		t.Fatalf("v3 hostile: %+v err=%v", rec, err)
	}
	ranges, n := mergeRanges([]LineRange{{10, 12}, {1, 2}, {3, 4}, {11, 20}})
	if n != 15 || len(ranges) != 2 || ranges[0] != (LineRange{1, 4}) || ranges[1] != (LineRange{10, 20}) {
		t.Fatalf("merge = %v n=%d", ranges, n)
	}
}

func TestParseEntire(t *testing.T) {
	top, err := parseEntireTop([]byte(entireTopJSON))
	if err != nil || top.CheckpointID != "01KXEWS08ZG9WXQPFE4QDA4GA3" || len(top.Sessions) != 1 || top.Sessions[0].Metadata != "/0/metadata.json" {
		t.Fatalf("top = %+v err=%v", top, err)
	}
	sess, err := parseEntireSession([]byte(entireSessionJSON))
	if err != nil || sess.Agent != "Claude Code" || sess.Model != "claude-fable-5" || sess.AgentLines != 2 || sess.AgentPercentage != 100 {
		t.Fatalf("session = %+v err=%v", sess, err)
	}
	if canonicalAgent("claude-code", nil) != "Claude Code" || canonicalAgent("copilot_cli", nil) != "GitHub Copilot" || canonicalAgent("factoryai-droid", nil) != "Droid" || canonicalAgent("mystery", nil) != "mystery" {
		t.Fatal("canonicalAgent mapping")
	}
}

func TestLoadFromRepository(t *testing.T) {
	repo := testrepo.Standard(t)
	ctx := context.Background()
	r, err := gitx.Open(ctx, repo.Dir)
	if err != nil {
		t.Fatal(err)
	}
	hashes := strings.Fields(repo.Git("rev-list", "HEAD"))
	human := hashes[len(hashes)-1] // "Initial human commit"
	codex := hashes[len(hashes)-3]

	// Nothing yet.
	s, err := Load(ctx, r, Options{CheckpointIDs: []string{"01KXEWS08ZG9WXQPFE4QDA4GA3"}})
	if err != nil {
		t.Fatal(err)
	}
	if !s.Empty() || s.HasLineLevel() || len(s.Warnings) != 1 || !strings.Contains(s.Warnings[0], "no checkpoint data") {
		t.Fatalf("empty store = %+v", s)
	}

	// git-ai v3 note on the human commit, legacy blob on the codex commit.
	noteFile := filepath.Join(t.TempDir(), "note.txt")
	if err := os.WriteFile(noteFile, []byte(v3Note), 0o644); err != nil {
		t.Fatal(err)
	}
	repo.Git("notes", "--ref=ai", "add", "-F", noteFile, human)
	repo.Git("update-ref", "refs/ai/authorship/"+codex, repo.Blob(legacyNote))
	// A note that is not an authorship log must be skipped with a warning.
	repo.Git("notes", "--ref=ai", "add", "-m", "just a comment", hashes[0])
	// Claudit-style presence note.
	repo.Git("notes", "--ref=claude-conversations", "add", "-m", "transcript", hashes[1])

	// Entire checkpoint: its own ref (ULID, sharded by last two chars) and
	// the v1 branch layout (12-hex, sharded by first two chars).
	ulid := "01KXEWS08ZG9WXQPFE4QDA4GA3"
	cpTree := repo.Tree(map[string]string{"metadata.json": entireTopJSON, "0/metadata.json": entireSessionJSON, "0/prompt.txt": "lint is failing"})
	repo.Git("update-ref", "refs/entire/checkpoints/"+ulid[len(ulid)-2:]+"/"+ulid, repo.CommitTree(cpTree, "Finalize transcript for Checkpoint: "+ulid))
	hexID := "a3b2c4d5e6f7"
	v1Tree := repo.Tree(map[string]string{"a3/b2c4d5e6f7/metadata.json": strings.Replace(entireTopJSON, ulid, hexID, 1), "a3/b2c4d5e6f7/0/metadata.json": strings.Replace(entireSessionJSON, ulid, hexID, 1)})
	repo.Git("update-ref", "refs/heads/entire/checkpoints/v1", repo.CommitTree(v1Tree, "checkpoints"))

	s, err = Load(ctx, r, Options{CheckpointIDs: []string{ulid, hexID, "000000000000"}})
	if err != nil {
		t.Fatal(err)
	}
	if s.Sources[SourceGitAI] != 2 || s.Sources[SourceClaudit] != 1 || s.Sources[SourceEntire] != 2 {
		t.Fatalf("sources = %v", s.Sources)
	}
	if !s.HasLineLevel() || s.LineLevelCommits() != 2 {
		t.Fatalf("line level = %d", s.LineLevelCommits())
	}
	rec := s.ForCommit(human)
	if rec == nil || rec.Source != SourceGitAI || len(rec.Evidence) != 3 || rec.Evidence[0].Source != "notes:ai" {
		t.Fatalf("human record = %+v", rec)
	}
	if rec := s.ForCommit(codex); rec == nil || rec.Evidence[0].Agent != "Cursor" || rec.Evidence[0].Source != "refs/ai/authorship" {
		t.Fatalf("codex record = %+v", rec)
	}
	if rec := s.ForCommit(hashes[1]); rec == nil || rec.Source != SourceClaudit || rec.Evidence[0].Agent != "Claude Code" || rec.LineLevel() {
		t.Fatalf("claudit record = %+v", rec)
	}
	if s.ForCommit(hashes[0]) != nil {
		t.Fatal("unparseable note must not produce a record")
	}
	cp := s.Checkpoint(ulid)
	if cp == nil || cp.Agent != "Claude Code" || cp.Model != "claude-fable-5" || len(cp.Sessions) != 1 || cp.Sessions[0].AgentLines != 2 {
		t.Fatalf("ulid checkpoint = %+v", cp)
	}
	if cp := s.Checkpoint(hexID); cp == nil || cp.Agent != "Claude Code" {
		t.Fatalf("v1-branch checkpoint = %+v", cp)
	}
	if s.Checkpoint("000000000000") != nil {
		t.Fatal("unknown checkpoint should be nil")
	}
	joined := strings.Join(s.Warnings, "\n")
	if !strings.Contains(joined, "could not be parsed") || !strings.Contains(joined, "1 Entire checkpoint(s) referenced by commits were not found") {
		t.Fatalf("warnings = %q", joined)
	}
}
