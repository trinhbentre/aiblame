package stats

import (
	"context"
	"encoding/json"
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

func abs(f float64) float64 {
	if f < 0 {
		return -f
	}
	return f
}
