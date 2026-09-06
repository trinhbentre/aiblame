package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/trinhbentre/aiblame/internal/testrepo"
)

// run executes Main with a scrubbed environment and returns exit code,
// stdout and stderr.
func run(t *testing.T, cwd string, envMap map[string]string, args ...string) (int, string, string) {
	t.Helper()
	var out, errb bytes.Buffer
	getenv := func(k string) string { return envMap[k] }
	code := Main(context.Background(), args, Env{Stdout: &out, Stderr: &errb, Getenv: getenv, Cwd: cwd})
	return code, out.String(), errb.String()
}

func TestStatsJSONAndFormats(t *testing.T) {
	repo := testrepo.Standard(t)
	code, out, errs := run(t, repo.Dir, nil, "stats", repo.Dir, "--json", "-q")
	if code != ExitOK {
		t.Fatalf("exit %d: %s", code, errs)
	}
	var rep map[string]any
	if err := json.Unmarshal([]byte(out), &rep); err != nil {
		t.Fatalf("bad json: %v\n%s", err, out)
	}
	if h := rep["headline_ai_share"].(float64); h < 70.5 || h > 70.7 {
		t.Fatalf("headline = %v", h)
	}
	if rep["metric"] != "lines" {
		t.Fatalf("metric = %v", rep["metric"])
	}

	code, out, _ = run(t, repo.Dir, nil, "--no-color", "-q")
	if code != ExitOK || !strings.Contains(out, "AI-written") || !strings.Contains(out, "Claude Code") || strings.Contains(out, "\x1b[") {
		t.Fatalf("table output wrong (exit %d):\n%s", code, out)
	}
	code, out, _ = run(t, repo.Dir, nil, "stats", "-f", "md", "--compact", "-q")
	if code != ExitOK || !strings.HasPrefix(out, "## AI authorship") || strings.Contains(out, "### Directories") {
		t.Fatalf("markdown output wrong (exit %d):\n%s", code, out)
	}
	code, out, _ = run(t, repo.Dir, nil, "stats", "--no-blame", "--json", "-q")
	if code != ExitOK || !strings.Contains(out, `"metric": "churn"`) {
		t.Fatalf("no-blame output wrong (exit %d):\n%s", code, out)
	}
	// -o writes a file
	outFile := filepath.Join(t.TempDir(), "sub", "report.json")
	code, out, errs = run(t, repo.Dir, nil, "stats", "--json", "-o", outFile)
	if code != ExitOK || out != "" || !strings.Contains(errs, "wrote") {
		t.Fatalf("-o: exit %d out=%q err=%q", code, out, errs)
	}
	if b, err := os.ReadFile(outFile); err != nil || !bytes.Contains(b, []byte("headline_ai_share")) {
		t.Fatalf("output file missing: %v", err)
	}
	// FORCE_COLOR turns colours on even for a buffer
	_, out, _ = run(t, repo.Dir, map[string]string{"FORCE_COLOR": "1"}, "stats", "-q")
	if !strings.Contains(out, "\x1b[") {
		t.Fatal("FORCE_COLOR ignored")
	}
}

func TestUsageAndErrors(t *testing.T) {
	repo := testrepo.Standard(t)
	if code, _, errs := run(t, repo.Dir, nil, "stats", "--format", "yaml"); code != ExitUsage || !strings.Contains(errs, "unknown format") {
		t.Fatalf("bad format: exit %d %q", code, errs)
	}
	if code, _, _ := run(t, repo.Dir, nil, "stats", "a", "b"); code != ExitUsage {
		t.Fatalf("two paths: exit %d", code)
	}
	if code, _, _ := run(t, repo.Dir, nil, "stats", "--bogus"); code != ExitUsage {
		t.Fatalf("unknown flag: exit %d", code)
	}
	if code, out, _ := run(t, repo.Dir, nil, "help"); code != ExitOK || !strings.Contains(out, "Usage:") {
		t.Fatalf("help: exit %d", code)
	}
	if code, _, errs := run(t, repo.Dir, nil, "stats", "--help"); code != ExitOK || !strings.Contains(errs, "aiblame stats") {
		t.Fatalf("stats --help: exit %d %q", code, errs)
	}
	if code, out, _ := run(t, repo.Dir, nil, "version"); code != ExitOK || !strings.HasPrefix(out, "aiblame ") {
		t.Fatalf("version: exit %d %q", code, out)
	}
	if code, out, _ := run(t, repo.Dir, nil, "version", "--json"); code != ExitOK || !strings.Contains(out, `"version"`) {
		t.Fatalf("version json: exit %d %q", code, out)
	}
	notRepo := t.TempDir()
	if code, _, errs := run(t, notRepo, nil, "stats", notRepo); code != ExitError || !strings.Contains(errs, "not a git repository") {
		t.Fatalf("not a repo: exit %d %q", code, errs)
	}
	empty := testrepo.New(t)
	if code, _, errs := run(t, empty.Dir, nil, "stats", empty.Dir); code != ExitError || !strings.Contains(errs, "no commits") {
		t.Fatalf("empty repo: exit %d %q", code, errs)
	}
	if code, _, errs := run(t, repo.Dir, nil, "stats", filepath.Join(repo.Dir, "does-not-exist")); code != ExitError || !strings.Contains(errs, "not a directory") {
		t.Fatalf("missing path: exit %d %q", code, errs)
	}
	if code, _, errs := run(t, repo.Dir, nil, "stats", "--since", "whenever"); code != ExitError || !strings.Contains(errs, "unrecognised date") {
		t.Fatalf("bad date: exit %d %q", code, errs)
	}
}

func TestCheck(t *testing.T) {
	repo := testrepo.Standard(t)
	if code, out, _ := run(t, repo.Dir, nil, "check", "--max", "50", "--no-color", "-q"); code != ExitFailed || !strings.Contains(out, "FAIL") {
		t.Fatalf("max 50 should fail: exit %d\n%s", code, out)
	}
	if code, out, _ := run(t, repo.Dir, nil, "check", "--max", "90", "--min", "10", "--no-color", "-q"); code != ExitOK || !strings.Contains(out, "all rules passed") {
		t.Fatalf("max 90 should pass: exit %d\n%s", code, out)
	}
	code, out, _ := run(t, repo.Dir, nil, "check", "--require-trailer", "assisted-by", "--no-color", "-q")
	if code != ExitFailed || !strings.Contains(out, "2 AI-assisted commit(s) without a assisted-by trailer") || !strings.Contains(out, "Add AI module") {
		t.Fatalf("require-trailer: exit %d\n%s", code, out)
	}
	code, out, _ = run(t, repo.Dir, nil, "check", "--forbid-agent-authored", "--json", "-q")
	if code != ExitFailed {
		t.Fatalf("forbid-agent: exit %d\n%s", code, out)
	}
	var res struct {
		Pass    bool `json:"pass"`
		Results []struct {
			Rule    string   `json:"rule"`
			Pass    bool     `json:"pass"`
			Commits []string `json:"commits"`
		} `json:"results"`
	}
	if err := json.Unmarshal([]byte(out), &res); err != nil || res.Pass || len(res.Results) != 1 || res.Results[0].Rule != "forbid-agent-authored" || len(res.Results[0].Commits) != 1 || !strings.Contains(res.Results[0].Commits[0], "Devin") {
		t.Fatalf("check json = %+v err=%v", res, err)
	}
	// --since excludes the offending commits -> pass
	if code, _, _ := run(t, repo.Dir, nil, "check", "--forbid-agent-authored", "--since", testrepo.CommitTime(6).Format("2006-01-02T15:04:05Z"), "-q"); code != ExitOK {
		t.Fatalf("windowed forbid should pass: exit %d", code)
	}
	if code, _, errs := run(t, repo.Dir, nil, "check", "-q"); code != ExitUsage || !strings.Contains(errs, "no rules") {
		t.Fatalf("no rules: exit %d %q", code, errs)
	}
	// commit share = 4 AI / (4 AI + 2 human) = 66.7%; the bot commit is not in the denominator
	if code, _, _ := run(t, repo.Dir, nil, "check", "--metric", "commits", "--max", "70", "-q"); code != ExitOK {
		t.Fatalf("commit metric 66.7%% <= 70 should pass: exit %d", code)
	}
	if code, _, _ := run(t, repo.Dir, nil, "check", "--metric", "commits", "--max", "60", "-q"); code != ExitFailed {
		t.Fatalf("commit metric 66.7%% <= 60 should fail: exit %d", code)
	}
	// config-driven defaults
	repo.Write(".aiblame.toml", "[check]\nmax = 10\n")
	if code, _, _ := run(t, repo.Dir, nil, "check", "-q"); code != ExitFailed {
		t.Fatalf("config max should fail: exit %d", code)
	}
}

func TestLog(t *testing.T) {
	repo := testrepo.Standard(t)
	code, out, _ := run(t, repo.Dir, nil, "log", "--ai", "--no-color")
	if code != ExitOK {
		t.Fatalf("exit %d", code)
	}
	if n := strings.Count(out, "\n"); n != 4 {
		t.Fatalf("expected 4 AI commits, got %d:\n%s", n, out)
	}
	if !strings.Contains(out, "AGENT ") || !strings.Contains(out, "ASSIST") {
		t.Fatalf("tags missing:\n%s", out)
	}
	_, out, _ = run(t, repo.Dir, nil, "log", "--agent", "codex", "--no-color")
	if strings.Count(out, "\n") != 1 || !strings.Contains(out, "Codex") {
		t.Fatalf("agent filter:\n%s", out)
	}
	_, out, _ = run(t, repo.Dir, nil, "log", "--kind", "bot", "--evidence", "--no-color")
	if strings.Count(out, "Bump deps") != 1 || !strings.Contains(out, "dependabot") {
		t.Fatalf("bot filter:\n%s", out)
	}
	_, out, _ = run(t, repo.Dir, nil, "log", "--json", "-n", "2")
	var entries []map[string]any
	if err := json.Unmarshal([]byte(out), &entries); err != nil || len(entries) != 2 {
		t.Fatalf("json log: %v %d", err, len(entries))
	}
	if code, _, _ := run(t, repo.Dir, nil, "log", "--kind", "alien"); code != ExitUsage {
		t.Fatalf("bad kind: exit %d", code)
	}
	if _, _, errs := run(t, repo.Dir, nil, "log", "--agent", "nobody"); !strings.Contains(errs, "no matching commits") {
		t.Fatalf("expected empty notice, got %q", errs)
	}
}

func TestBlame(t *testing.T) {
	repo := testrepo.Standard(t)
	code, out, errs := run(t, repo.Dir, nil, "blame", "src/ai.go", "--no-color")
	if code != ExitOK {
		t.Fatalf("exit %d %s", code, errs)
	}
	if !strings.Contains(out, "ASSIST") || !strings.Contains(out, "Claude Code") || !strings.Contains(out, "100.0% AI-written") {
		t.Fatalf("blame output:\n%s", out)
	}
	if strings.Count(out, "| ai line") != 30 {
		t.Fatalf("expected 30 content lines:\n%s", out)
	}
	_, out, _ = run(t, repo.Dir, nil, "blame", filepath.Join(repo.Dir, "src", "human.go"), "--summary", "--no-color")
	if !strings.Contains(out, "0.0% AI-written") || strings.Contains(out, "| human line") {
		t.Fatalf("summary:\n%s", out)
	}
	_, out, _ = run(t, repo.Dir, nil, "blame", "src/devin.py", "--json")
	var res map[string]any
	if err := json.Unmarshal([]byte(out), &res); err != nil || res["agent"].(float64) != 12 || res["ai_share"].(float64) != 100 {
		t.Fatalf("blame json: %v %v", err, res)
	}
	if code, _, _ := run(t, repo.Dir, nil, "blame"); code != ExitUsage {
		t.Fatalf("no file: exit %d", code)
	}
	outside := filepath.Join(t.TempDir(), "x.go")
	os.WriteFile(outside, []byte("x"), 0o644)
	if code, _, _ := run(t, repo.Dir, nil, "blame", outside); code != ExitError {
		t.Fatalf("outside repo: exit %d", code)
	}
}

func TestBadge(t *testing.T) {
	repo := testrepo.Standard(t)
	dir := t.TempDir()
	svg := filepath.Join(dir, "b", "ai.svg")
	js := filepath.Join(dir, "ai.json")
	code, out, errs := run(t, repo.Dir, nil, "badge", "--out", svg, "--shields", js, "-q")
	if code != ExitOK || out != "" {
		t.Fatalf("exit %d out=%q err=%q", code, out, errs)
	}
	b, err := os.ReadFile(svg)
	if err != nil || !bytes.Contains(b, []byte("<svg")) || !bytes.Contains(b, []byte("71%")) {
		t.Fatalf("svg: %v %s", err, b)
	}
	var sh map[string]any
	jb, _ := os.ReadFile(js)
	if err := json.Unmarshal(jb, &sh); err != nil || sh["message"] != "71%" || sh["label"] != "AI-written" || sh["schemaVersion"].(float64) != 1 {
		t.Fatalf("shields: %v %s", err, jb)
	}
	code, out, _ = run(t, repo.Dir, nil, "badge", "--precision", "1", "--label", "vibe", "--color", "auto", "--metric", "commits", "-q")
	if code != ExitOK || !strings.Contains(out, "<svg") || !strings.Contains(out, "66.7%") || !strings.Contains(out, "vibe") {
		t.Fatalf("stdout badge:\n%s", out)
	}
	if code, _, _ := run(t, repo.Dir, nil, "badge", "--metric", "bytes"); code != ExitUsage {
		t.Fatalf("bad metric: exit %d", code)
	}
}

func TestHookCommands(t *testing.T) {
	repo := testrepo.Standard(t)
	if code, out, _ := run(t, repo.Dir, nil, "hook", "status"); code != ExitFailed || !strings.Contains(out, "not installed") {
		t.Fatalf("status before: exit %d %q", code, out)
	}
	if code, out, _ := run(t, repo.Dir, nil, "hook", "install", "--style", "co-authored-by"); code != ExitOK || !strings.Contains(out, "installed") {
		t.Fatalf("install: exit %d %q", code, out)
	}
	if code, out, _ := run(t, repo.Dir, nil, "hook", "status"); code != ExitOK || !strings.Contains(out, "co-authored-by") {
		t.Fatalf("status after: exit %d %q", code, out)
	}
	if code, out, _ := run(t, "", nil, "hook", "print"); code != ExitOK || !strings.Contains(out, "# aiblame-hook v1") {
		t.Fatalf("print: exit %d", code)
	}
	if code, out, _ := run(t, repo.Dir, map[string]string{"CLAUDECODE": "1"}, "hook", "detect"); code != ExitOK || !strings.Contains(out, "Claude Code") {
		t.Fatalf("detect: exit %d %q", code, out)
	}
	if code, out, _ := run(t, repo.Dir, nil, "hook", "detect"); code != ExitFailed || !strings.Contains(out, "no coding-agent session") {
		t.Fatalf("detect none: exit %d %q", code, out)
	}
	if code, _, _ := run(t, repo.Dir, nil, "hook", "install", "--style", "nope"); code != ExitUsage {
		t.Fatalf("bad style: exit %d", code)
	}
	if code, out, _ := run(t, repo.Dir, nil, "hook", "uninstall"); code != ExitOK || !strings.Contains(out, "removed") {
		t.Fatalf("uninstall: exit %d %q", code, out)
	}
	if code, _, _ := run(t, repo.Dir, nil, "hook"); code != ExitUsage {
		t.Fatalf("missing subcommand: exit %d", code)
	}
}

func TestInitAndAgents(t *testing.T) {
	repo := testrepo.Standard(t)
	if code, out, _ := run(t, repo.Dir, nil, "init"); code != ExitOK || !strings.Contains(out, ".aiblame.toml") {
		t.Fatalf("init: exit %d %q", code, out)
	}
	if _, err := os.Stat(filepath.Join(repo.Dir, ".aiblame.toml")); err != nil {
		t.Fatal(err)
	}
	if code, _, errs := run(t, repo.Dir, nil, "init"); code != ExitFailed || !strings.Contains(errs, "already exists") {
		t.Fatalf("init twice: exit %d %q", code, errs)
	}
	if code, _, _ := run(t, repo.Dir, nil, "init", "--force"); code != ExitOK {
		t.Fatalf("init --force: exit %d", code)
	}
	// the generated config must load and the report still works
	if code, _, errs := run(t, repo.Dir, nil, "stats", "--json", "-q"); code != ExitOK {
		t.Fatalf("stats with example config: exit %d %s", code, errs)
	}
	code, out, _ := run(t, repo.Dir, nil, "agents")
	if code != ExitOK || !strings.Contains(out, "Claude Code") || !strings.Contains(out, "Dependabot") || !strings.Contains(out, "assisted-by") {
		t.Fatalf("agents: exit %d\n%s", code, out)
	}
	_, out, _ = run(t, repo.Dir, nil, "agents", "--json")
	var rows []map[string]any
	if err := json.Unmarshal([]byte(out), &rows); err != nil || len(rows) < 40 {
		t.Fatalf("agents json: %v %d", err, len(rows))
	}
}

func TestConfigExcludeApplied(t *testing.T) {
	repo := testrepo.Standard(t)
	repo.Write(".aiblame.toml", "[paths]\nexclude = [\"src/ai.go\"]\n")
	_, out, _ := run(t, repo.Dir, nil, "stats", "--json", "-q")
	var rep struct {
		Lines struct {
			Assisted float64 `json:"assisted"`
		} `json:"lines"`
	}
	if err := json.Unmarshal([]byte(out), &rep); err != nil || rep.Lines.Assisted != 18 {
		t.Fatalf("exclude not applied: %v %+v", err, rep)
	}
	repo.Write(".aiblame.toml", "[paths]\nexclude = 5\n")
	if code, _, errs := run(t, repo.Dir, nil, "stats", "-q"); code != ExitError || !strings.Contains(errs, ".aiblame.toml") {
		t.Fatalf("bad config: exit %d %q", code, errs)
	}
}

func TestCheckPoliciesAndNewRules(t *testing.T) {
	repo := testrepo.Standard(t)
	// Mesa policy: the Claude/Codex Co-authored-by commits violate the
	// forbid-trailer rule and the require-trailer rule; devin is agent-authored.
	code, out, _ := run(t, repo.Dir, nil, "check", repo.Dir, "--policy", "mesa", "--no-blame", "--no-color", "-q")
	if code != ExitFailed {
		t.Fatalf("mesa policy should fail (exit %d):\n%s", code, out)
	}
	for _, want := range []string{"policy mesa", "FAIL  forbid-trailer", "FAIL  require-trailer", "FAIL  forbid-agent-authored", "PASS  forbid-agent-signoff", "co-authored-by"} {
		if !strings.Contains(out, want) {
			t.Fatalf("missing %q in:\n%s", want, out)
		}
	}
	// Kubernetes policy in JSON: every trailer is forbidden.
	code, out, _ = run(t, repo.Dir, nil, "check", repo.Dir, "--policy", "kubernetes", "--no-blame", "--json", "-q")
	if code != ExitFailed {
		t.Fatalf("kubernetes policy exit %d", code)
	}
	var res map[string]any
	if err := json.Unmarshal([]byte(out), &res); err != nil {
		t.Fatalf("bad json: %v\n%s", err, out)
	}
	if res["policy"] != "kubernetes" || res["pass"] != false {
		t.Fatalf("json = %v", res)
	}
	// An explicit forbid-trailer for a convention nobody used passes.
	code, out, _ = run(t, repo.Dir, nil, "check", repo.Dir, "--forbid-trailer", "generated-by", "--forbid-agent-signoff", "--no-blame", "--no-color", "-q")
	if code != ExitOK || !strings.Contains(out, "PASS  forbid-trailer") {
		t.Fatalf("forbid unused convention: exit %d\n%s", code, out)
	}
	// require-trailer accepts alternatives.
	code, out, _ = run(t, repo.Dir, nil, "check", repo.Dir, "--require-trailer", "assisted-by,co-authored-by", "--no-blame", "--no-color", "-q")
	if code != ExitOK || !strings.Contains(out, "assisted-by or co-authored-by") {
		t.Fatalf("require alternatives: exit %d\n%s", code, out)
	}
	// Unknown convention and unknown policy are usage errors.
	if code, _, errs := run(t, repo.Dir, nil, "check", repo.Dir, "--require-trailer", "signed-by-robots"); code != ExitUsage || !strings.Contains(errs, "not a known convention") {
		t.Fatalf("unknown convention: exit %d %q", code, errs)
	}
	if code, _, errs := run(t, repo.Dir, nil, "check", repo.Dir, "--policy", "nope"); code != ExitUsage || !strings.Contains(errs, "unknown policy") {
		t.Fatalf("unknown policy: exit %d %q", code, errs)
	}
	// --list-policies needs no repository.
	code, out, _ = run(t, t.TempDir(), nil, "check", "--list-policies")
	if code != ExitOK || !strings.Contains(out, "kernel") || !strings.Contains(out, "kubernetes") || !strings.Contains(out, "source:") {
		t.Fatalf("list policies: exit %d\n%s", code, out)
	}
	// An AI Signed-off-by is caught.
	repo.Write("src/dco.go", "x\n")
	repo.Commit(testrepo.CommitOpts{Message: "fix: dco\n\nSigned-off-by: Claude <noreply@anthropic.com>"})
	code, out, _ = run(t, repo.Dir, nil, "check", repo.Dir, "--forbid-agent-signoff", "--no-blame", "--no-color", "-q")
	if code != ExitFailed || !strings.Contains(out, "FAIL  forbid-agent-signoff") {
		t.Fatalf("agent signoff: exit %d\n%s", code, out)
	}
	// Config-driven policy.
	if err := os.WriteFile(filepath.Join(repo.Dir, ".aiblame.toml"), []byte("[check]\npolicy = \"asf\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	code, out, _ = run(t, repo.Dir, nil, "check", repo.Dir, "--no-blame", "--no-color", "-q")
	if code != ExitFailed || !strings.Contains(out, "policy asf") || !strings.Contains(out, "without a generated-by trailer") {
		t.Fatalf("config policy: exit %d\n%s", code, out)
	}
}

func TestRangeAndPrivacyFlags(t *testing.T) {
	repo := testrepo.Standard(t)
	code, out, errs := run(t, repo.Dir, nil, "stats", repo.Dir, "--rev", "HEAD~3..HEAD", "--json", "-q")
	if code != ExitOK {
		t.Fatalf("range stats exit %d: %s", code, errs)
	}
	var rep map[string]any
	if err := json.Unmarshal([]byte(out), &rep); err != nil {
		t.Fatal(err)
	}
	if rep["base_rev"] != "HEAD~3" || rep["rev"] != "HEAD~3..HEAD" || rep["headline_ai_share"].(float64) != 100 {
		t.Fatalf("range json = %v", rep)
	}
	code, out, _ = run(t, repo.Dir, nil, "stats", repo.Dir, "--rev", "HEAD~3..HEAD", "--no-color", "-q")
	if code != ExitOK || !strings.Contains(out, "range HEAD~3..HEAD") {
		t.Fatalf("range table:\n%s", out)
	}
	code, out, _ = run(t, repo.Dir, nil, "check", repo.Dir, "--rev", "HEAD~3..HEAD", "--forbid-agent-authored", "--no-blame", "--no-color", "-q")
	if code != ExitFailed || !strings.Contains(out, "@ HEAD~3..HEAD") {
		t.Fatalf("range check: exit %d\n%s", code, out)
	}
	code, out, _ = run(t, repo.Dir, nil, "log", repo.Dir, "--rev", "HEAD~3..HEAD", "--json")
	if code != ExitOK || strings.Count(out, `"hash"`) != 3 {
		t.Fatalf("range log: exit %d\n%s", code, out)
	}
	code, out, _ = run(t, repo.Dir, nil, "stats", repo.Dir, "--no-authors", "--no-color", "-q")
	if code != ExitOK || strings.Contains(out, "Contributors") {
		t.Fatalf("--no-authors still printed contributors:\n%s", out)
	}
	code, out, _ = run(t, repo.Dir, nil, "stats", repo.Dir, "--no-color", "-q")
	if code != ExitOK || !strings.Contains(out, "Still in the tree") {
		t.Fatalf("survival line missing:\n%s", out)
	}
}

func TestProvenanceThroughCLI(t *testing.T) {
	repo := testrepo.Standard(t)
	hashes := strings.Fields(repo.Git("rev-list", "HEAD"))
	initial := hashes[len(hashes)-1]
	note := "src/human.go\n  s_1::t_1 1-5\n---\n" +
		`{"schema_version":"authorship/3.0.0","base_commit_sha":"","prompts":{},"sessions":{"s_1":{"agent_id":{"tool":"cursor","id":"x","model":"claude-sonnet-4-5"},"human_author":"human@example.com"}}}`
	repo.Git("notes", "--ref=ai", "add", "-m", note, initial)

	code, out, _ := run(t, repo.Dir, nil, "stats", repo.Dir, "--no-color", "-q")
	if code != ExitOK || !strings.Contains(out, "Cursor") || !strings.Contains(out, "sidecar data: git-ai-notes 1") || !strings.Contains(out, "1 commits attributed line by line") {
		t.Fatalf("stats with notes:\n%s", out)
	}
	code, out, _ = run(t, repo.Dir, nil, "stats", repo.Dir, "--no-provenance", "--no-color", "-q")
	if code != ExitOK || strings.Contains(out, "Cursor") {
		t.Fatalf("--no-provenance still read notes:\n%s", out)
	}
	// blame shows the five Cursor lines and 15 human lines.
	code, out, _ = run(t, repo.Dir, nil, "blame", filepath.Join(repo.Dir, "src", "human.go"), "--no-color")
	if code != ExitOK || strings.Count(out, "ASSIST Cursor") != 5 || !strings.Contains(out, "25.0% AI-written (5 assisted") {
		t.Fatalf("blame with notes:\n%s", out)
	}
	// log --evidence explains what was not counted.
	repo.Write("docs/x.md", "x\n")
	repo.Commit(testrepo.CommitOpts{Message: "docs\n\nGenerated-By: StageFreight\nAssisted-by: Viktor Szakats"})
	code, out, _ = run(t, repo.Dir, nil, "log", repo.Dir, "--evidence", "-n", "1", "--no-color")
	if code != ExitOK || !strings.Contains(out, "HUMAN") || !strings.Contains(out, "not counted trailer:Generated-By: StageFreight") || !strings.Contains(out, "names a person") {
		t.Fatalf("log evidence:\n%s", out)
	}
	code, out, _ = run(t, repo.Dir, nil, "stats", repo.Dir, "--no-blame", "--no-color", "-q")
	if code != ExitOK || !strings.Contains(out, "StageFreight (1)") {
		t.Fatalf("unrecognised hint missing:\n%s", out)
	}
}

func TestRemoteURLHosts(t *testing.T) {
	cases := map[string]string{
		"codeberg.org/forgejo/forgejo": "https://codeberg.org/forgejo/forgejo.git",
		"gitlab.com/gitlab-org/cli":    "https://gitlab.com/gitlab-org/cli.git",
		"owner/repo":                   "https://github.com/owner/repo.git",
	}
	for in, want := range cases {
		if got, ok := remoteURL(in); !ok || got != want {
			t.Errorf("remoteURL(%q) = %q,%v want %q", in, got, ok, want)
		}
	}
	for _, bad := range []string{"-x/owner/repo", "codeberg.org/-owner/repo", "a/b/c", "./x/y", ".hidden/x/y"} {
		if _, ok := remoteURL(bad); ok {
			t.Errorf("remoteURL(%q) should be rejected", bad)
		}
	}
}

func TestTruncateIsRuneSafe(t *testing.T) {
	s := "fix: sửa lỗi hiển thị tiếng Việt và emoji 🤖🤖🤖"
	got := truncate(s, 12)
	if !utf8.ValidString(got) || utf8.RuneCountInString(got) != 12 || !strings.HasSuffix(got, "…") {
		t.Fatalf("truncate = %q", got)
	}
	if truncate("short", 10) != "short" || truncate("ab", 1) != "ab" {
		t.Fatal("truncate changed short strings")
	}
}

func TestLockPath(t *testing.T) {
	p := filepath.Join(t.TempDir(), "x.lock")
	unlock, err := lockPath(context.Background(), p)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(p); err != nil {
		t.Fatal("lock file missing")
	}
	// a stale lock (old mtime) is reclaimed
	unlock()
	os.WriteFile(p, []byte("stale"), 0o644)
	old := time.Now().Add(-time.Hour)
	os.Chtimes(p, old, old)
	unlock2, err := lockPath(context.Background(), p)
	if err != nil {
		t.Fatalf("stale lock not reclaimed: %v", err)
	}
	unlock2()
	// a fresh lock held by someone else makes us wait until ctx is cancelled
	os.WriteFile(p, []byte("held"), 0o644)
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()
	if _, err := lockPath(ctx, p); err == nil {
		t.Fatal("expected to give up on held lock")
	}
}

func TestRemoteURL(t *testing.T) {
	cases := map[string]string{
		"https://github.com/a/b": "https://github.com/a/b",
		"git@github.com:a/b.git": "git@github.com:a/b.git",
		"openai/codex":           "https://github.com/openai/codex.git",
		"owner/repo.git":         "https://github.com/owner/repo.git",
		"./relative":             "",
		"just-a-name":            "",
		"a/b/c":                  "",
		"../x":                   "",
		"C:\\path\\repo":         "",
	}
	for in, want := range cases {
		got, ok := remoteURL(in)
		if (want == "") == ok || got != want {
			t.Errorf("remoteURL(%q) = %q,%v want %q", in, got, ok, want)
		}
	}
}
