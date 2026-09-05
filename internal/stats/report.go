// Package stats turns git history into AI-authorship statistics.
package stats

import (
	"sort"
	"time"

	"github.com/trinhbentre/aiblame/internal/attrib"
)

// Totals counts something (commits, lines) per authorship kind.
//
// AIShare is AI / (AI + Human) as a percentage. Non-AI bot automation
// (dependabot, renovate…) is reported but excluded from the denominator
// because it is authored by neither a person nor a coding agent.
type Totals struct {
	Human    int64   `json:"human"`
	Assisted int64   `json:"assisted"`
	Agent    int64   `json:"agent"`
	Bot      int64   `json:"bot"`
	Total    int64   `json:"total"`
	AI       int64   `json:"ai"`
	AIShare  float64 `json:"ai_share"`
}

// Add accumulates n into the bucket for k.
func (t *Totals) Add(k attrib.Kind, n int64) {
	switch k {
	case attrib.Human:
		t.Human += n
	case attrib.Assisted:
		t.Assisted += n
	case attrib.Agent:
		t.Agent += n
	case attrib.Bot:
		t.Bot += n
	}
}

// Finalize computes the derived fields.
func (t *Totals) Finalize() {
	t.AI = t.Assisted + t.Agent
	t.Total = t.Human + t.AI + t.Bot
	t.AIShare = share(t.AI, t.Human)
}

// Of returns the count for a kind.
func (t Totals) Of(k attrib.Kind) int64 {
	switch k {
	case attrib.Human:
		return t.Human
	case attrib.Assisted:
		return t.Assisted
	case attrib.Agent:
		return t.Agent
	case attrib.Bot:
		return t.Bot
	}
	return 0
}

func share(ai, human int64) float64 {
	den := ai + human
	if den == 0 {
		return 0
	}
	return float64(ai) * 100 / float64(den)
}

// AgentStat is the contribution of one coding agent.
type AgentStat struct {
	Name    string   `json:"name"`
	Vendor  string   `json:"vendor,omitempty"`
	Commits int64    `json:"commits"`
	Churn   int64    `json:"churn"`
	Lines   int64    `json:"lines"`
	Models  []string `json:"models,omitempty"`
}

// AuthorStat describes a human contributor and how much of their work was
// AI-assisted. Agent-authored and bot commits are not listed here.
type AuthorStat struct {
	Name      string  `json:"name"`
	Email     string  `json:"email"`
	Commits   int64   `json:"commits"`
	AICommits int64   `json:"ai_commits"`
	Churn     int64   `json:"churn"`
	AIChurn   int64   `json:"ai_churn"`
	Lines     int64   `json:"lines"`
	AILines   int64   `json:"ai_lines"`
	AIShare   float64 `json:"ai_share"` // by lines when available, else churn
}

// PathStat is the AI share of a directory or file.
type PathStat struct {
	Path     string  `json:"path"`
	Lines    int64   `json:"lines"`
	AILines  int64   `json:"ai_lines"`
	Human    int64   `json:"human_lines"`
	Bot      int64   `json:"bot_lines"`
	AIShare  float64 `json:"ai_share"`
	Assisted int64   `json:"assisted_lines"`
	Agent    int64   `json:"agent_lines"`
}

// MonthStat is the AI share of commits/churn in one calendar month.
type MonthStat struct {
	Month     string  `json:"month"` // YYYY-MM
	Commits   int64   `json:"commits"`
	AICommits int64   `json:"ai_commits"`
	Churn     int64   `json:"churn"`
	AIChurn   int64   `json:"ai_churn"`
	AIShare   float64 `json:"ai_share"` // by churn
}

// ConventionStat counts how commits disclosed AI involvement.
type ConventionStat struct {
	Name    string `json:"name"` // co-authored-by | assisted-by | … | agent-author | message-marker
	Commits int64  `json:"commits"`
}

// Report is the full analysis result. Field names are stable across 1.x.
type Report struct {
	Tool        string    `json:"tool"`
	Version     string    `json:"version"`
	SchemaVer   int       `json:"schema_version"`
	Repo        string    `json:"repo"`
	Remote      string    `json:"remote,omitempty"`
	RevName     string    `json:"rev"`
	Rev         string    `json:"rev_hash"`
	GeneratedAt time.Time `json:"generated_at"`

	// Metric names the headline: "lines" (surviving lines from blame) or
	// "churn" (lines added, when blame was skipped).
	Metric   string  `json:"metric"`
	Headline float64 `json:"headline_ai_share"`

	Commits Totals  `json:"commits"`
	Churn   Totals  `json:"churn"`
	Lines   *Totals `json:"lines,omitempty"`

	Agents      []AgentStat      `json:"agents"`
	Authors     []AuthorStat     `json:"authors"`
	Conventions []ConventionStat `json:"conventions"`
	Dirs        []PathStat       `json:"dirs"`
	Files       []PathStat       `json:"files"`
	Months      []MonthStat      `json:"months"`

	FileCount    int      `json:"file_count"`
	SkippedFiles int      `json:"skipped_files"`
	Since        string   `json:"since,omitempty"`
	Until        string   `json:"until,omitempty"`
	Excludes     []string `json:"excludes,omitempty"`
	Includes     []string `json:"includes,omitempty"`
	DurationMS   int64    `json:"duration_ms"`
	Warnings     []string `json:"warnings,omitempty"`
}

// SchemaVersion is bumped when the JSON shape changes incompatibly.
const SchemaVersion = 1

// HeadlineTotals returns the totals behind the headline metric.
func (r *Report) HeadlineTotals() Totals {
	if r.Lines != nil {
		return *r.Lines
	}
	return r.Churn
}

func sortAgents(a []AgentStat) {
	sort.Slice(a, func(i, j int) bool {
		if a[i].Lines != a[j].Lines {
			return a[i].Lines > a[j].Lines
		}
		if a[i].Churn != a[j].Churn {
			return a[i].Churn > a[j].Churn
		}
		if a[i].Commits != a[j].Commits {
			return a[i].Commits > a[j].Commits
		}
		return a[i].Name < a[j].Name
	})
}

func sortAuthors(a []AuthorStat) {
	sort.Slice(a, func(i, j int) bool {
		if a[i].Commits != a[j].Commits {
			return a[i].Commits > a[j].Commits
		}
		return a[i].Name < a[j].Name
	})
}

func sortPaths(p []PathStat) {
	sort.Slice(p, func(i, j int) bool {
		if p[i].AILines != p[j].AILines {
			return p[i].AILines > p[j].AILines
		}
		if p[i].Lines != p[j].Lines {
			return p[i].Lines > p[j].Lines
		}
		return p[i].Path < p[j].Path
	})
}
