package stats

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/trinhbentre/aiblame/internal/attrib"
	"github.com/trinhbentre/aiblame/internal/gitx"
	"github.com/trinhbentre/aiblame/internal/testrepo"
)

func newAnalyzer(t *testing.T, dir string, opts Options) *Analyzer {
	t.Helper()
	r, err := gitx.Open(context.Background(), dir)
	if err != nil {
		t.Fatal(err)
	}
	return &Analyzer{Runner: r, Classifier: attrib.NewClassifier(attrib.Options{}), Opts: opts}
}

func TestRunStandardRepo(t *testing.T) {
	repo := testrepo.Standard(t)
	a := newAnalyzer(t, repo.Dir, Options{Blame: true, DefaultExcludes: true, Version: "test"})
	rep, err := a.Run(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	// commits: human(initial, logo)=2, assisted(claude, codex, kernel)=3, agent(devin)=1, bot(dependabot)=1
	if rep.Commits.Human != 2 || rep.Commits.Assisted != 3 || rep.Commits.Agent != 1 || rep.Commits.Bot != 1 {
		t.Fatalf("commits = %+v", rep.Commits)
	}
	if rep.Commits.Total != 7 || rep.Commits.AI != 4 {
		t.Fatalf("commit totals = %+v", rep.Commits)
	}
	// lines (blame): human README 5 + human.go 20 = 25; assisted 30+10+8 = 48; agent 12; bot lockfile excluded by default; png skipped
	if rep.Lines == nil {
		t.Fatal("expected blame lines")
	}
	if rep.Lines.Human != 25 || rep.Lines.Assisted != 48 || rep.Lines.Agent != 12 || rep.Lines.Bot != 0 {
		t.Fatalf("lines = %+v", *rep.Lines)
	}
	wantShare := float64(60) * 100 / 85
	if abs(rep.Lines.AIShare-wantShare) > 0.01 || rep.Metric != "lines" || abs(rep.Headline-wantShare) > 0.01 {
		t.Fatalf("share = %v want %v (metric %s headline %v)", rep.Lines.AIShare, wantShare, rep.Metric, rep.Headline)
	}
	if rep.FileCount != 6 || rep.SkippedFiles != 1 { // 8 files - lockfile excluded (not counted, not skipped) - png skipped
		t.Fatalf("files=%d skipped=%d", rep.FileCount, rep.SkippedFiles)
	}
	// agents
	if len(rep.Agents) != 3 {
		t.Fatalf("agents = %+v", rep.Agents)
	}
	if rep.Agents[0].Name != "Claude Code" || rep.Agents[0].Lines != 38 || rep.Agents[0].Commits != 2 {
		t.Fatalf("top agent = %+v", rep.Agents[0])
	}
	if rep.Agents[0].Models[0] != "claude-opus-4-6" {
		t.Fatalf("models = %v", rep.Agents[0].Models)
	}
	// authors: only the human, 5 commits (2 human + 3 assisted), 3 AI
	if len(rep.Authors) != 1 || rep.Authors[0].Commits != 5 || rep.Authors[0].AICommits != 3 {
		t.Fatalf("authors = %+v", rep.Authors)
	}
	if rep.Authors[0].Lines != 73 || rep.Authors[0].AILines != 48 {
		t.Fatalf("author lines = %+v", rep.Authors[0])
	}
	// conventions
	conv := map[string]int64{}
	for _, c := range rep.Conventions {
		conv[c.Name] = c.Commits
	}
	if conv["co-authored-by"] != 2 || conv["assisted-by"] != 1 || conv["agent-author"] != 1 {
		t.Fatalf("conventions = %v", conv)
	}
	// dirs: src/ and (root)
	if len(rep.Dirs) != 2 || rep.Dirs[0].Path != "src/" || rep.Dirs[0].AILines != 60 || rep.Dirs[0].Lines != 80 {
		t.Fatalf("dirs = %+v", rep.Dirs)
	}
	if rep.Files[0].Path != "src/ai.go" || rep.Files[0].AIShare != 100 {
		t.Fatalf("files = %+v", rep.Files)
	}
	// months: all in 2026-01
	if len(rep.Months) != 1 || rep.Months[0].Commits != 7 || rep.Months[0].AICommits != 4 {
		t.Fatalf("months = %+v", rep.Months)
	}
	// JSON round trip and stable keys
	b, err := json.Marshal(rep)
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatal(err)
	}
	for _, k := range []string{"headline_ai_share", "commits", "churn", "lines", "agents", "authors", "conventions", "dirs", "files", "months", "schema_version"} {
		if _, ok := m[k]; !ok {
			t.Errorf("json missing key %s", k)
		}
	}
}

func TestRunChurnOnlyAndFilters(t *testing.T) {
	repo := testrepo.Standard(t)
	a := newAnalyzer(t, repo.Dir, Options{Blame: false, DefaultExcludes: false, Include: []string{"src/**"}})
	rep, err := a.Run(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if rep.Lines != nil || rep.Metric != "churn" {
		t.Fatalf("expected churn metric, got %+v", rep)
	}
	// churn inside src/: human 20, assisted 48, agent 12; lockfile excluded by include filter
	if rep.Churn.Human != 20 || rep.Churn.Assisted != 48 || rep.Churn.Agent != 12 || rep.Churn.Bot != 0 {
		t.Fatalf("churn = %+v", rep.Churn)
	}
	// no default excludes + no include: lockfile counted as bot churn
	a2 := newAnalyzer(t, repo.Dir, Options{Blame: true, DefaultExcludes: false})
	rep2, err := a2.Run(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if rep2.Lines.Bot != 100 || rep2.Churn.Bot != 100 {
		t.Fatalf("expected lockfile counted as bot: %+v / %+v", *rep2.Lines, rep2.Churn)
	}
	// bot lines must not change the AI share denominator: 60 AI / (60 + 25 human)
	if want := float64(60) * 100 / 85; abs(rep2.Lines.AIShare-want) > 0.01 {
		t.Fatalf("bot lines changed share: %v want %v", rep2.Lines.AIShare, want)
	}
}

func TestSinceWindow(t *testing.T) {
	repo := testrepo.Standard(t)
	// commits are one hour apart starting 2026-01-01T01:00Z (testrepo.CommitTime);
	// since 04:30Z keeps commits 5..7 (devin, dependabot, logo)
	a := newAnalyzer(t, repo.Dir, Options{Blame: true, DefaultExcludes: true, Since: "2026-01-01T04:30:00Z"})
	rep, err := a.Run(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if rep.Commits.Total != 3 || rep.Commits.Agent != 1 || rep.Commits.Bot != 1 || rep.Commits.Human != 1 {
		t.Fatalf("windowed commits = %+v", rep.Commits)
	}
	// blame is not windowed
	if rep.Lines.Assisted != 48 {
		t.Fatalf("blame should ignore --since: %+v", *rep.Lines)
	}
	if _, err := newAnalyzer(t, repo.Dir, Options{Since: "yesterday-ish"}).Run(context.Background()); err == nil {
		t.Fatal("expected date parse error")
	}
	if _, err := a.resolveDate(context.Background(), "3 months ago"); err != nil {
		t.Fatal(err)
	}
}

func TestEmptyRepo(t *testing.T) {
	repo := testrepo.New(t)
	a := newAnalyzer(t, repo.Dir, Options{Blame: true})
	if _, err := a.Run(context.Background()); err == nil {
		t.Fatal("expected error on repo without commits")
	}
}

func TestDirKey(t *testing.T) {
	cases := map[[2]any]string{
		{"README.md", 1}:    "(root)",
		{"src/a.go", 1}:     "src/",
		{"src/pkg/a.go", 2}: "src/pkg/",
		{"src/pkg/a.go", 5}: "src/pkg/",
		{"a/b/c/d/e.go", 3}: "a/b/c/",
	}
	for in, want := range cases {
		if got := dirKey(in[0].(string), in[1].(int)); got != want {
			t.Errorf("dirKey(%v) = %q want %q", in, got, want)
		}
	}
}

func TestSurvival(t *testing.T) {
	repo := testrepo.Standard(t)
	a := newAnalyzer(t, repo.Dir, Options{Blame: true, DefaultExcludes: true})
	rep, err := a.Run(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if rep.Survival == nil || rep.Survival.AI != 100 || rep.Survival.Human != 100 || rep.Survival.All != 100 {
		t.Fatalf("survival = %+v", rep.Survival)
	}
	// A human trims a third of the Claude file: AI survival drops, human stays.
	repo.Write("src/ai.go", testrepo.Lines("ai", 20))
	repo.Commit(testrepo.CommitOpts{Message: "Trim AI module"})
	rep, err = newAnalyzer(t, repo.Dir, Options{Blame: true, DefaultExcludes: true}).Run(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	// AI surviving 20+10+8+12 = 50 of 60 added; human 25 of 25.
	if abs(rep.Survival.AI-83.33) > 0.01 || rep.Survival.Human != 100 {
		t.Fatalf("survival after trim = %+v", rep.Survival)
	}
	var claude *AgentStat
	for i := range rep.Agents {
		if rep.Agents[i].Name == "Claude Code" {
			claude = &rep.Agents[i]
		}
	}
	// Claude Code: 28 surviving of 38 added.
	if claude == nil || claude.Survival == nil || abs(*claude.Survival-73.68) > 0.01 {
		t.Fatalf("claude survival = %+v", claude)
	}
	// A time window disables survival (churn is windowed, blame is not).
	rep, err = newAnalyzer(t, repo.Dir, Options{Blame: true, DefaultExcludes: true, Since: "2026-01-01T02:30:00Z"}).Run(context.Background())
	if err != nil || rep.Survival != nil {
		t.Fatalf("windowed survival should be nil: %+v err=%v", rep.Survival, err)
	}
}

func TestProvenanceLineLevel(t *testing.T) {
	repo := testrepo.Standard(t)
	hashes := strings.Fields(repo.Git("rev-list", "HEAD"))
	initial := hashes[len(hashes)-1]
	// git-ai says Cursor wrote lines 1-5 of src/human.go in the initial commit.
	note := "src/human.go\n  s_1::t_1 1-5\n---\n" +
		`{"schema_version":"authorship/3.0.0","base_commit_sha":"","prompts":{},"sessions":{"s_1":{"agent_id":{"tool":"cursor","id":"x","model":"claude-sonnet-4-5"},"human_author":"human@example.com"}}}`
	repo.Git("notes", "--ref=ai", "add", "-m", note, initial)

	// Without provenance nothing changes.
	rep, err := newAnalyzer(t, repo.Dir, Options{Blame: true, DefaultExcludes: true}).Run(context.Background())
	if err != nil || rep.Lines.Human != 25 || len(rep.Provenance) != 0 {
		t.Fatalf("provenance off: %+v %v", rep.Lines, err)
	}

	rep, err = newAnalyzer(t, repo.Dir, Options{Blame: true, DefaultExcludes: true, Provenance: true}).Run(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	// The initial commit is now AI-assisted (Cursor) at commit level…
	if rep.Commits.Assisted != 4 || rep.Commits.Human != 1 {
		t.Fatalf("commits = %+v", rep.Commits)
	}
	// …but only the five attested lines count as AI; the other 15 lines of
	// human.go and all of README.md (not in the log) stay human.
	if rep.Lines.Assisted != 48+5 || rep.Lines.Human != 20 || rep.Lines.Agent != 12 {
		t.Fatalf("lines = %+v", *rep.Lines)
	}
	if rep.LineLevelCommits != 1 || len(rep.Provenance) != 1 || rep.Provenance[0].Name != "git-ai-notes" || rep.Provenance[0].Commits != 1 {
		t.Fatalf("provenance = %+v line-level=%d", rep.Provenance, rep.LineLevelCommits)
	}
	var cursor *AgentStat
	for i := range rep.Agents {
		if rep.Agents[i].Name == "Cursor" {
			cursor = &rep.Agents[i]
		}
	}
	if cursor == nil || cursor.Lines != 5 || cursor.Commits != 1 || cursor.Models[0] != "claude-sonnet-4-5" {
		t.Fatalf("cursor = %+v", cursor)
	}
	conv := map[string]int64{}
	for _, c := range rep.Conventions {
		conv[c.Name] = c.Commits
	}
	if conv["git-ai-notes"] != 1 {
		t.Fatalf("conventions = %v", conv)
	}
	// The human's AI share moves with the line-level data: 53 AI of 73 lines.
	if len(rep.Authors) != 1 || rep.Authors[0].AILines != 53 || rep.Authors[0].Lines != 73 {
		t.Fatalf("authors = %+v", rep.Authors)
	}
}

func TestProvenanceEntire(t *testing.T) {
	repo := testrepo.Standard(t)
	id := "01KXEWS08ZG9WXQPFE4QDA4GA3"
	repo.Write("src/entire.go", testrepo.Lines("entire", 7))
	repo.Commit(testrepo.CommitOpts{Message: "Add onboarding\n\nEntire-Checkpoint: " + id})

	// Trailer only: AI-assisted by an unknown agent, with a fetch hint.
	rep, err := newAnalyzer(t, repo.Dir, Options{Blame: true, DefaultExcludes: true, Provenance: true}).Run(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if rep.Commits.Assisted != 4 || !hasAgent(rep, attrib.UnknownAI) {
		t.Fatalf("unresolved checkpoint: %+v agents=%+v", rep.Commits, rep.Agents)
	}
	if !containsStr(rep.Warnings, "no checkpoint data") {
		t.Fatalf("warnings = %v", rep.Warnings)
	}

	// With the checkpoint fetched the agent and model are known.
	tree := repo.Tree(map[string]string{
		"metadata.json":   `{"checkpoint_id":"` + id + `","sessions":[{"metadata":"/0/metadata.json"}]}`,
		"0/metadata.json": `{"agent":"claude-code","model":"claude-fable-5","initial_attribution":{"agent_lines":7,"human_added":0,"agent_percentage":100}}`,
	})
	repo.Git("update-ref", "refs/entire/checkpoints/"+id[len(id)-2:]+"/"+id, repo.CommitTree(tree, "checkpoint"))
	rep, err = newAnalyzer(t, repo.Dir, Options{Blame: true, DefaultExcludes: true, Provenance: true}).Run(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if hasAgent(rep, attrib.UnknownAI) || !hasAgent(rep, "Claude Code") {
		t.Fatalf("agents = %+v", rep.Agents)
	}
	for _, a := range rep.Agents {
		if a.Name == "Claude Code" && (a.Commits != 3 || !containsStr(a.Models, "claude-fable-5")) {
			t.Fatalf("claude = %+v", a)
		}
	}
	if len(rep.Provenance) != 1 || rep.Provenance[0].Name != "entire-checkpoint" || rep.Lines.Assisted != 48+7 {
		t.Fatalf("provenance = %+v lines=%+v", rep.Provenance, rep.Lines)
	}
}

func TestRangeMode(t *testing.T) {
	repo := testrepo.Standard(t)
	// HEAD~3..HEAD = devin, dependabot, logo commits.
	rep, err := newAnalyzer(t, repo.Dir, Options{Blame: true, DefaultExcludes: true, Rev: "HEAD~3..HEAD"}).Run(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if rep.BaseRev != "HEAD~3" || rep.RevName != "HEAD~3..HEAD" {
		t.Fatalf("range fields = %q %q", rep.BaseRev, rep.RevName)
	}
	if rep.Commits.Total != 3 || rep.Commits.Agent != 1 || rep.Commits.Bot != 1 || rep.Commits.Human != 1 {
		t.Fatalf("range commits = %+v", rep.Commits)
	}
	// Only lines the range introduced count: src/devin.py (12, agent). The
	// lockfile is excluded by default and the png is binary.
	if rep.Lines.Agent != 12 || rep.Lines.Human != 0 || rep.Lines.Assisted != 0 || rep.Headline != 100 {
		t.Fatalf("range lines = %+v", *rep.Lines)
	}
	if rep.FileCount != 1 {
		t.Fatalf("range should blame only touched files, got %d", rep.FileCount)
	}
	if _, err := newAnalyzer(t, repo.Dir, Options{Rev: "nope..HEAD"}).Run(context.Background()); err == nil {
		t.Fatal("bad base must fail")
	}
	// "A.." means A..HEAD.
	rep, err = newAnalyzer(t, repo.Dir, Options{Rev: "HEAD~1.."}).Run(context.Background())
	if err != nil || rep.Commits.Total != 1 || rep.BaseRev != "HEAD~1" {
		t.Fatalf("open range: %+v err=%v", rep.Commits, err)
	}
}

func TestNoAuthors(t *testing.T) {
	repo := testrepo.Standard(t)
	rep, err := newAnalyzer(t, repo.Dir, Options{Blame: true, DefaultExcludes: true, NoAuthors: true}).Run(context.Background())
	if err != nil || len(rep.Authors) != 0 || rep.Lines.Human != 25 {
		t.Fatalf("no-authors: authors=%d lines=%+v err=%v", len(rep.Authors), rep.Lines, err)
	}
}

func TestUnrecognisedTools(t *testing.T) {
	repo := testrepo.Standard(t)
	repo.Write("docs/gen.md", "x\n")
	repo.Commit(testrepo.CommitOpts{Message: "docs: refresh\n\nGenerated-By: StageFreight"})
	rep, err := newAnalyzer(t, repo.Dir, Options{Blame: false}).Run(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(rep.Unrecognised) != 1 || rep.Unrecognised[0].Name != "StageFreight" || rep.Unrecognised[0].Commits != 1 {
		t.Fatalf("unrecognised = %+v", rep.Unrecognised)
	}
	if rep.Commits.Human != 3 { // initial, logo, docs
		t.Fatalf("commits = %+v", rep.Commits)
	}
}

func hasAgent(rep *Report, name string) bool {
	for _, a := range rep.Agents {
		if a.Name == name {
			return true
		}
	}
	return false
}

func containsStr(list []string, sub string) bool {
	for _, s := range list {
		if strings.Contains(s, sub) {
			return true
		}
	}
	return false
}

func abs(f float64) float64 {
	if f < 0 {
		return -f
	}
	return f
}
