package render

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/trinhbentre/aiblame/internal/stats"
)

func sampleReport() *stats.Report {
	lines := stats.Totals{Human: 25, Assisted: 48, Agent: 12}
	lines.Finalize()
	commits := stats.Totals{Human: 2, Assisted: 3, Agent: 1, Bot: 1}
	commits.Finalize()
	churn := stats.Totals{Human: 25, Assisted: 48, Agent: 12, Bot: 100}
	churn.Finalize()
	return &stats.Report{
		Tool: "aiblame", Version: "1.0.0-test", SchemaVer: 1, Repo: "/tmp/repo", RevName: "HEAD", Rev: "0123456789abcdef",
		GeneratedAt: time.Date(2026, 9, 6, 0, 0, 0, 0, time.UTC), Metric: "lines", Headline: lines.AIShare,
		Commits: commits, Churn: churn, Lines: &lines,
		Agents:      []stats.AgentStat{{Name: "Claude Code", Vendor: "Anthropic", Commits: 2, Lines: 38, Models: []string{"claude-opus-4-6"}}, {Name: "Devin", Lines: 12, Commits: 1}},
		Authors:     []stats.AuthorStat{{Name: "Test Human", Email: "h@x", Commits: 5, AICommits: 3, Lines: 73, AILines: 48, AIShare: 65.75}},
		Conventions: []stats.ConventionStat{{Name: "co-authored-by", Commits: 2}, {Name: "assisted-by", Commits: 1}},
		Dirs:        []stats.PathStat{{Path: "src/", Lines: 80, AILines: 60, AIShare: 75}, {Path: "(root)", Lines: 5}},
		Files:       []stats.PathStat{{Path: "src/ai.go", Lines: 30, AILines: 30, AIShare: 100}},
		Months:      []stats.MonthStat{{Month: "2025-12", Commits: 3, AICommits: 1, AIShare: 30}, {Month: "2026-01", Commits: 4, AICommits: 3, AIShare: 70}},
		FileCount:   6, SkippedFiles: 1, DurationMS: 420,
	}
}

func TestTableContainsKeySections(t *testing.T) {
	var buf bytes.Buffer
	Table(&buf, sampleReport(), Options{})
	out := buf.String()
	for _, want := range []string{"AI-written", "70.6%", "Agents", "Claude Code", "Contributors", "Test Human", "Disclosure conventions", "co-authored-by", "Directories", "src/", "Most AI-written files", "Timeline", "6 files blamed", "1 skipped"} {
		if !strings.Contains(out, want) {
			t.Errorf("table missing %q\n%s", want, out)
		}
	}
	if strings.Contains(out, "\x1b[") {
		t.Error("colour codes emitted without Color option")
	}
	var cbuf bytes.Buffer
	Table(&cbuf, sampleReport(), Options{Color: true, Compact: true})
	if !strings.Contains(cbuf.String(), "\x1b[") || strings.Contains(cbuf.String(), "Directories") {
		t.Error("colour/compact options not honoured")
	}
}

func TestTableNoAIHint(t *testing.T) {
	rep := sampleReport()
	rep.Commits = stats.Totals{Human: 3}
	rep.Commits.Finalize()
	var buf bytes.Buffer
	Table(&buf, rep, Options{})
	if !strings.Contains(buf.String(), "No AI disclosure found") {
		t.Error("expected hint when no AI commits")
	}
}

func TestMarkdown(t *testing.T) {
	var buf bytes.Buffer
	Markdown(&buf, sampleReport(), Options{})
	out := buf.String()
	for _, want := range []string{"## AI authorship · 70.6%", "| Commits | 7 |", "### Agents", "| Claude Code |", "### Contributors", "### Disclosure conventions", "`co-authored-by`", "### Timeline", "aiblame"} {
		if !strings.Contains(out, want) {
			t.Errorf("markdown missing %q\n%s", want, out)
		}
	}
}

func TestJSON(t *testing.T) {
	var buf bytes.Buffer
	if err := JSON(&buf, sampleReport()); err != nil {
		t.Fatal(err)
	}
	var m map[string]any
	if err := json.Unmarshal(buf.Bytes(), &m); err != nil {
		t.Fatal(err)
	}
	if m["headline_ai_share"].(float64) < 70 || m["tool"] != "aiblame" {
		t.Fatalf("json = %v", m)
	}
}

func TestBadge(t *testing.T) {
	svg := BadgeSVG("AI-written", "70.6%", "", "flat")
	for _, want := range []string{"<svg", "AI-written", "70.6%", "#7c3aed", "textLength", "</svg>"} {
		if !strings.Contains(svg, want) {
			t.Errorf("badge missing %q", want)
		}
	}
	if strings.Contains(BadgeSVG("a<b", "x&y", "red", "flat-square"), "a<b") {
		t.Error("label not escaped")
	}
	if ResolveColor("green", 0) != "97ca00" || ResolveColor("#ABC", 0) != "abc" || ResolveColor("nonsense", 0) != DefaultBadgeColor || ResolveColor("auto", 90) != "9333ea" || ResolveColor("auto", 0) != "9f9f9f" {
		t.Error("colour resolution wrong")
	}
	b, err := ShieldsJSON("AI-written", "70.6%", "auto", "flat")
	if err != nil {
		t.Fatal(err)
	}
	var e ShieldsEndpoint
	if err := json.Unmarshal(b, &e); err != nil || e.SchemaVersion != 1 || e.Message != "70.6%" || e.Label != "AI-written" {
		t.Fatalf("shields = %s err=%v", b, err)
	}
}

func TestSanitize(t *testing.T) {
	in := "title\x1b]0;pwned\x07 \x1b[31mred\x1b[0m\ttab\r\nnl nel ok"
	got := Sanitize(in)
	if strings.ContainsAny(got, "\x1b\x07\r\n") || !strings.Contains(got, "\ttab") || !strings.Contains(got, "ok") {
		t.Fatalf("Sanitize = %q", got)
	}
	if Sanitize("plain ünïcödé 🤖") != "plain ünïcödé 🤖" {
		t.Fatal("clean text changed")
	}
}

func TestHelpers(t *testing.T) {
	if commas(1234567) != "1,234,567" || commas(999) != "999" || commas(-1000) != "-1,000" || commas(0) != "0" {
		t.Error("commas wrong")
	}
	if Bar(50, 10) != "█████░░░░░" || Bar(200, 4) != "████" || Bar(-5, 4) != "░░░░" {
		t.Errorf("bar wrong: %q", Bar(50, 10))
	}
	if Pct(12.345) != "12.3%" {
		t.Error("pct wrong")
	}
}
