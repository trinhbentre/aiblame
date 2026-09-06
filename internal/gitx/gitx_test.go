package gitx

import (
	"context"
	"os/exec"
	"strings"
	"testing"

	"github.com/trinhbentre/aiblame/internal/testrepo"
)

func TestParseNumStatAndRenames(t *testing.T) {
	in := "10\t2\tsrc/a.go\n-\t-\timg.png\n3\t0\tdir/{old => new}/f.go\n1\t1\told.txt => new.txt\n4\t4\t\"sp ace.go\"\n"
	got := ParseNumStat(in)
	if len(got) != 5 {
		t.Fatalf("got %d rows", len(got))
	}
	if got[0].Path != "src/a.go" || got[0].Added != 10 || got[0].Deleted != 2 {
		t.Errorf("row0 = %+v", got[0])
	}
	if !got[1].Binary || got[1].Path != "img.png" {
		t.Errorf("row1 = %+v", got[1])
	}
	if got[2].Path != "dir/new/f.go" {
		t.Errorf("rename brace = %q", got[2].Path)
	}
	if got[3].Path != "new.txt" {
		t.Errorf("rename arrow = %q", got[3].Path)
	}
	if got[4].Path != "sp ace.go" {
		t.Errorf("quoted = %q", got[4].Path)
	}
}

func TestParseBlamePorcelain(t *testing.T) {
	sha1 := "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	sha2 := "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
	out := sha1 + " 1 1 2\nauthor A\nauthor-mail <a@a>\nsummary s\nfilename f\n\tline one\n" +
		sha1 + " 2 2\n\tline two\n" +
		sha2 + " 1 3 1\nauthor B\nboundary\nfilename f\n\tline three\n"
	res, err := ParseBlamePorcelain([]byte(out), true)
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Lines) != 3 {
		t.Fatalf("lines = %d", len(res.Lines))
	}
	if res.Counts[sha1] != 2 || res.Counts[sha2] != 1 {
		t.Fatalf("counts = %v", res.Counts)
	}
	if res.Lines[2].Content != "line three" || res.Lines[2].LineNo != 3 {
		t.Fatalf("line3 = %+v", res.Lines[2])
	}
	// SHA-256 repositories emit 64-hex object names.
	sha256 := strings.Repeat("c", 64)
	res, err = ParseBlamePorcelain([]byte(sha256+" 1 1 1\nauthor C\n\tx\n"), false)
	if err != nil || res.Counts[sha256] != 1 {
		t.Fatalf("sha256 header not parsed: %v %v", err, res)
	}
	// A header without its content line means the output was cut short.
	if _, err := ParseBlamePorcelain([]byte(sha1+" 1 1 1\nauthor A\n"), false); err == nil {
		t.Fatal("expected truncation error")
	}
	if h, _, ok := parseBlameHeader("not a header line"); ok || h != "" {
		t.Fatal("garbage accepted as header")
	}
}

func TestIsShallow(t *testing.T) {
	repo := testrepo.Standard(t)
	ctx := context.Background()
	r, err := Open(ctx, repo.Dir)
	if err != nil {
		t.Fatal(err)
	}
	if r.IsShallow(ctx) {
		t.Fatal("full repo reported as shallow")
	}
	dst := t.TempDir()
	cmd := exec.Command("git", "clone", "--quiet", "--depth", "1", "file://"+repo.Dir, dst)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Skipf("shallow clone not supported here: %v %s", err, out)
	}
	sr, err := Open(ctx, dst)
	if err != nil {
		t.Fatal(err)
	}
	if !sr.IsShallow(ctx) {
		t.Fatal("depth-1 clone not reported as shallow")
	}
}

func TestLogRejectsOptionLikeRev(t *testing.T) {
	repo := testrepo.Standard(t)
	ctx := context.Background()
	r, err := Open(ctx, repo.Dir)
	if err != nil {
		t.Fatal(err)
	}
	// With the "--" separator git treats this as a (missing) revision, not as
	// an option that would write a file.
	if _, err := r.Log(ctx, LogOptions{Rev: "--output=" + t.TempDir() + "/pwned"}); err == nil {
		t.Fatal("option-like rev must not be accepted")
	}
}

func TestLogAndBlameIntegration(t *testing.T) {
	repo := testrepo.Standard(t)
	ctx := context.Background()
	r, err := Open(ctx, repo.Dir)
	if err != nil {
		t.Fatal(err)
	}
	commits, err := r.Log(ctx, LogOptions{NumStat: true})
	if err != nil {
		t.Fatal(err)
	}
	if len(commits) != 7 {
		t.Fatalf("commits = %d", len(commits))
	}
	// newest first: logo commit has a binary file
	var sawBinary bool
	for _, f := range commits[0].Files {
		if f.Binary && f.Path == "assets/logo.png" {
			sawBinary = true
		}
	}
	if !sawBinary {
		t.Fatalf("expected binary numstat row, got %+v", commits[0].Files)
	}
	last := commits[len(commits)-1]
	if last.Subject() != "Initial human commit" || last.AuthorEmail != "human@example.com" {
		t.Fatalf("oldest = %+v", last)
	}
	if !contains(commits[5].Message, "Co-Authored-By: Claude") {
		t.Fatalf("message lost trailer: %q", commits[5].Message)
	}
	files, err := r.ListFiles(ctx, "HEAD")
	if err != nil || len(files) != 8 {
		t.Fatalf("files = %v err=%v", files, err)
	}
	sizes, err := r.FileSizes(ctx, "HEAD")
	if err != nil || sizes["src/ai.go"] == 0 {
		t.Fatalf("sizes = %v err=%v", sizes, err)
	}
	bl, err := r.Blame(ctx, "src/ai.go", BlameOptions{})
	if err != nil {
		t.Fatal(err)
	}
	total := 0
	for _, n := range bl.Counts {
		total += n
	}
	// Aggregate blame keeps only the per-commit counts, not every line.
	if total != 30 || len(bl.Counts) != 1 || len(bl.Lines) != 0 {
		t.Fatalf("blame = %d lines kept, %v", len(bl.Lines), bl.Counts)
	}
	if _, err := r.ResolveRev(ctx, "nope-not-a-rev"); err == nil {
		t.Fatal("expected error for bad rev")
	}
	if _, err := Open(ctx, t.TempDir()); err == nil {
		t.Fatal("expected not-a-repo error")
	}
}

func TestBlameDetailKeepsOriginalLines(t *testing.T) {
	repo := testrepo.Standard(t)
	ctx := context.Background()
	r, err := Open(ctx, repo.Dir)
	if err != nil {
		t.Fatal(err)
	}
	bl, err := r.Blame(ctx, "src/ai.go", BlameOptions{Detail: true})
	if err != nil {
		t.Fatal(err)
	}
	if len(bl.Lines) != 30 {
		t.Fatalf("lines = %d", len(bl.Lines))
	}
	for i, l := range bl.Lines {
		if l.OrigLine != i+1 || l.OrigPath != "src/ai.go" || l.Content != "" {
			t.Fatalf("line %d = %+v", i+1, l)
		}
	}
	// Without Detail only the counts are kept.
	bl, err = r.Blame(ctx, "src/ai.go", BlameOptions{})
	if err != nil || len(bl.Lines) != 0 || len(bl.Counts) != 1 {
		t.Fatalf("plain blame: lines=%d counts=%v err=%v", len(bl.Lines), bl.Counts, err)
	}
}

func TestNotesRefsAndCatFile(t *testing.T) {
	repo := testrepo.Standard(t)
	ctx := context.Background()
	r, err := Open(ctx, repo.Dir)
	if err != nil {
		t.Fatal(err)
	}
	head := strings.TrimSpace(repo.Git("rev-parse", "HEAD"))
	// No notes yet: empty map, no error, ref does not exist.
	if r.RefExists(ctx, "refs/notes/ai") {
		t.Fatal("refs/notes/ai should not exist yet")
	}
	notes, err := r.NotesList(ctx, "ai")
	if err != nil || len(notes) != 0 {
		t.Fatalf("empty notes = %v err=%v", notes, err)
	}
	repo.Git("notes", "--ref=ai", "add", "-m", "src/a.go\n  h_1 1-2\n---\n{}", "HEAD")
	notes, err = r.NotesList(ctx, "ai")
	if err != nil || len(notes) != 1 || notes[head] == "" {
		t.Fatalf("notes = %v err=%v", notes, err)
	}
	blobs, err := r.CatFileBatch(ctx, []string{notes[head], "refs/notes/ai:" + head, "HEAD:src/ai.go", "HEAD:does/not/exist"})
	if err != nil {
		t.Fatal(err)
	}
	if !contains(string(blobs[notes[head]]), "h_1 1-2") {
		t.Fatalf("note blob = %q", blobs[notes[head]])
	}
	if !contains(string(blobs["refs/notes/ai:"+head]), "h_1 1-2") {
		t.Fatalf("note via path spec = %q", blobs["refs/notes/ai:"+head])
	}
	if !contains(string(blobs["HEAD:src/ai.go"]), "ai line 1") {
		t.Fatalf("file blob = %q", blobs["HEAD:src/ai.go"])
	}
	if _, ok := blobs["HEAD:does/not/exist"]; ok {
		t.Fatal("missing object should be absent, not present")
	}
	// Loose refs (git-ai's refs/ai/authorship/<sha>, Entire's refs/entire/…).
	repo.Git("update-ref", "refs/ai/authorship/"+head, notes[head])
	refs, err := r.ForEachRef(ctx, "refs/ai/", "refs/notes/")
	if err != nil || len(refs) != 2 {
		t.Fatalf("refs = %+v err=%v", refs, err)
	}
	if refs[0].Name != "refs/ai/authorship/"+head || refs[0].Type != "blob" {
		t.Fatalf("ref0 = %+v", refs[0])
	}
	if _, err := r.CatFileBatch(ctx, []string{"--help"}); err == nil {
		t.Fatal("option-like spec must be rejected")
	}
	if _, err := r.ForEachRef(ctx, "--format=x"); err == nil {
		t.Fatal("option-like prefix must be rejected")
	}
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && (func() bool {
		for i := 0; i+len(sub) <= len(s); i++ {
			if s[i:i+len(sub)] == sub {
				return true
			}
		}
		return false
	})()
}
