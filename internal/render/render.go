// Package render formats a stats.Report for terminals, Markdown, JSON and
// badges.
package render

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"unicode/utf8"

	"github.com/trinhbentre/aiblame/internal/attrib"
	"github.com/trinhbentre/aiblame/internal/stats"
)

// Options tunes text output.
type Options struct {
	Color bool
	// Top caps rows per section (0 = as in the report).
	Top int
	// Compact hides the per-directory, per-file and timeline sections.
	Compact bool
}

// ANSI helpers.
type palette struct {
	bold, dim, reset, ai, human, bot, accent, warn string
}

func newPalette(color bool) palette {
	if !color {
		return palette{}
	}
	return palette{
		bold:   "\x1b[1m",
		dim:    "\x1b[2m",
		reset:  "\x1b[0m",
		ai:     "\x1b[35m", // magenta
		human:  "\x1b[36m", // cyan
		bot:    "\x1b[33m", // yellow
		accent: "\x1b[1;35m",
		warn:   "\x1b[33m",
	}
}

func (p palette) wrap(code, s string) string {
	if code == "" {
		return s
	}
	return code + s + p.reset
}

// Sanitize neutralises terminal control characters in untrusted text (commit
// subjects, author names, file paths, blamed source lines) before it is
// printed to a terminal. Tabs are kept; every other C0/C1 control character
// and DEL becomes '?'. JSON output does not need this.
func Sanitize(s string) string {
	clean := true
	for _, r := range s {
		if r < 0x20 && r != '\t' || r == 0x7f || r >= 0x80 && r <= 0x9f {
			clean = false
			break
		}
	}
	if clean {
		return s
	}
	var b strings.Builder
	for _, r := range s {
		switch {
		case r == '\t':
			b.WriteRune(r)
		case r < 0x20, r == 0x7f, r >= 0x80 && r <= 0x9f:
			b.WriteByte('?')
		default:
			b.WriteRune(r)
		}
	}
	return b.String()
}

// JSON writes the report as indented JSON.
func JSON(w io.Writer, rep *stats.Report) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(rep)
}

// Pct formats a percentage with one decimal.
func Pct(f float64) string { return fmt.Sprintf("%.1f%%", f) }

// Bar renders a fixed-width progress bar.
func Bar(pct float64, width int) string {
	if width <= 0 {
		width = 20
	}
	if pct < 0 {
		pct = 0
	}
	if pct > 100 {
		pct = 100
	}
	filled := int(pct/100*float64(width) + 0.5)
	return strings.Repeat("█", filled) + strings.Repeat("░", width-filled)
}

// Table writes the human-readable terminal report.
func Table(w io.Writer, rep *stats.Report, o Options) {
	p := newPalette(o.Color)
	top := o.Top
	if top <= 0 {
		top = 1 << 30
	}
	tot := rep.HeadlineTotals()
	unit := "surviving lines"
	if rep.Metric == "churn" {
		unit = "lines added"
	}
	short := rep.Rev
	if len(short) > 7 {
		short = short[:7]
	}
	fmt.Fprintf(w, "%s %s @ %s (%s)\n\n", p.wrap(p.bold, "aiblame ·"), rep.Repo, rep.RevName, short)

	fmt.Fprintf(w, "  %s  %s  %s  of %s %s\n",
		p.wrap(p.accent, "AI-written"),
		p.wrap(p.ai, Bar(rep.Headline, 20)),
		p.wrap(p.bold, Pct(rep.Headline)),
		commas(tot.AI+tot.Human), unit)
	fmt.Fprintf(w, "  %s\n\n", p.wrap(p.dim, fmt.Sprintf("%s AI (%s assisted + %s agent-authored) · %s human · %s bot",
		commas(tot.AI), commas(tot.Assisted), commas(tot.Agent), commas(tot.Human), commas(tot.Bot))))

	rows := [][]string{
		{"", "total", "AI", "assisted", "agent", "human", "bot"},
		{"Commits", commas(rep.Commits.Total), fmt.Sprintf("%s (%s)", commas(rep.Commits.AI), Pct(rep.Commits.AIShare)), commas(rep.Commits.Assisted), commas(rep.Commits.Agent), commas(rep.Commits.Human), commas(rep.Commits.Bot)},
		{"Lines added", commas(rep.Churn.Total), fmt.Sprintf("%s (%s)", commas(rep.Churn.AI), Pct(rep.Churn.AIShare)), commas(rep.Churn.Assisted), commas(rep.Churn.Agent), commas(rep.Churn.Human), commas(rep.Churn.Bot)},
	}
	if rep.Lines != nil {
		rows = append(rows, []string{"Surviving lines", commas(rep.Lines.Total), fmt.Sprintf("%s (%s)", commas(rep.Lines.AI), Pct(rep.Lines.AIShare)), commas(rep.Lines.Assisted), commas(rep.Lines.Agent), commas(rep.Lines.Human), commas(rep.Lines.Bot)})
	}
	writeAligned(w, rows, "  ", p, []int{0})
	fmt.Fprintln(w)

	if len(rep.Agents) > 0 {
		fmt.Fprintln(w, p.wrap(p.bold, "Agents"))
		rows = [][]string{{"", metricHeader(rep), "share", "commits", "models"}}
		for i, a := range rep.Agents {
			if i >= top {
				break
			}
			n := a.Lines
			if rep.Metric == "churn" {
				n = a.Churn
			}
			models := Sanitize(strings.Join(a.Models, ", "))
			if utf8.RuneCountInString(models) > 40 {
				models = string([]rune(models)[:37]) + "…"
			}
			rows = append(rows, []string{"  " + Sanitize(a.Name), commas(n), Pct(pct(n, tot.AI+tot.Human)), commas(a.Commits), models})
		}
		writeAligned(w, rows, "", p, []int{0, 4})
		fmt.Fprintln(w)
	}

	if len(rep.Authors) > 0 {
		fmt.Fprintln(w, p.wrap(p.bold, "Contributors"))
		rows = [][]string{{"", "commits", "AI-assisted", metricHeader(rep), "AI share"}}
		for i, a := range rep.Authors {
			if i >= top {
				break
			}
			n, ai := a.Lines, a.AILines
			if rep.Metric == "churn" {
				n, ai = a.Churn, a.AIChurn
			}
			_ = ai
			rows = append(rows, []string{"  " + Sanitize(a.Name), commas(a.Commits), fmt.Sprintf("%s (%s)", commas(a.AICommits), Pct(pct(a.AICommits, a.Commits))), commas(n), Pct(a.AIShare)})
		}
		writeAligned(w, rows, "", p, []int{0})
		fmt.Fprintln(w)
	}

	if len(rep.Conventions) > 0 {
		fmt.Fprintln(w, p.wrap(p.bold, "Disclosure conventions"))
		rows = nil
		for _, c := range rep.Conventions {
			rows = append(rows, []string{"  " + c.Name, commas(c.Commits) + " commits"})
		}
		writeAligned(w, rows, "", p, []int{0, 1})
		fmt.Fprintln(w)
	}

	if !o.Compact {
		if len(rep.Dirs) > 0 {
			fmt.Fprintln(w, p.wrap(p.bold, "Directories"))
			rows = [][]string{{"", metricHeader(rep), "AI", "share"}}
			for i, d := range rep.Dirs {
				if i >= top {
					break
				}
				rows = append(rows, []string{"  " + Sanitize(d.Path), commas(d.Lines), commas(d.AILines), Pct(d.AIShare)})
			}
			writeAligned(w, rows, "", p, []int{0})
			fmt.Fprintln(w)
		}
		if len(rep.Files) > 0 {
			fmt.Fprintln(w, p.wrap(p.bold, "Most AI-written files"))
			rows = [][]string{{"", metricHeader(rep), "AI", "share"}}
			for i, f := range rep.Files {
				if i >= top {
					break
				}
				rows = append(rows, []string{"  " + Sanitize(f.Path), commas(f.Lines), commas(f.AILines), Pct(f.AIShare)})
			}
			writeAligned(w, rows, "", p, []int{0})
			fmt.Fprintln(w)
		}
		if len(rep.Months) > 1 {
			fmt.Fprintln(w, p.wrap(p.bold, "Timeline (AI share of lines added)"))
			for _, m := range tail(rep.Months, top) {
				fmt.Fprintf(w, "  %s  %s %6s  %s/%s commits\n", m.Month, p.wrap(p.ai, Bar(m.AIShare, 20)), Pct(m.AIShare), commas(m.AICommits), commas(m.Commits))
			}
			fmt.Fprintln(w)
		}
	}

	var foot []string
	if rep.Lines != nil {
		foot = append(foot, fmt.Sprintf("%d files blamed", rep.FileCount))
		if rep.SkippedFiles > 0 {
			foot = append(foot, fmt.Sprintf("%d skipped (binary/oversized/unreadable)", rep.SkippedFiles))
		}
	} else {
		foot = append(foot, "blame skipped (--no-blame): shares are by lines added")
	}
	if rep.Since != "" || rep.Until != "" {
		foot = append(foot, fmt.Sprintf("window %s..%s", orDash(rep.Since), orDash(rep.Until)))
	}
	if len(rep.Excludes) > 0 {
		foot = append(foot, fmt.Sprintf("%d exclude patterns", len(rep.Excludes)))
	}
	foot = append(foot, fmt.Sprintf("%.1fs", float64(rep.DurationMS)/1000))
	fmt.Fprintln(w, p.wrap(p.dim, strings.Join(foot, " · ")))
	for _, warn := range rep.Warnings {
		fmt.Fprintln(w, p.wrap(p.warn, "warning: "+Sanitize(warn)))
	}
	if rep.Commits.AI == 0 {
		fmt.Fprintln(w, p.wrap(p.dim, "\nNo AI disclosure found. aiblame only counts commits that disclose AI involvement\n(Co-authored-by / Assisted-by trailers, agent author identities, \"Generated with …\" markers).\nRun `aiblame hook install` to add trailers automatically from agent sessions."))
	}
}

// Markdown writes a GitHub-flavoured Markdown report (suitable for
// $GITHUB_STEP_SUMMARY or a PR comment).
func Markdown(w io.Writer, rep *stats.Report, o Options) {
	top := o.Top
	if top <= 0 {
		top = 1 << 30
	}
	tot := rep.HeadlineTotals()
	unit := "surviving lines"
	if rep.Metric == "churn" {
		unit = "lines added"
	}
	short := rep.Rev
	if len(short) > 7 {
		short = short[:7]
	}
	fmt.Fprintf(w, "## AI authorship · %s\n\n", Pct(rep.Headline))
	fmt.Fprintf(w, "**%s** of %s %s were written with AI (%s assisted, %s agent-authored, %s human, %s bot). Revision `%s`.\n\n",
		Pct(rep.Headline), commas(tot.AI+tot.Human), unit, commas(tot.Assisted), commas(tot.Agent), commas(tot.Human), commas(tot.Bot), short)

	fmt.Fprintln(w, "| | Total | AI | Assisted | Agent | Human | Bot |")
	fmt.Fprintln(w, "|---|---:|---:|---:|---:|---:|---:|")
	mdTotals(w, "Commits", rep.Commits)
	mdTotals(w, "Lines added", rep.Churn)
	if rep.Lines != nil {
		mdTotals(w, "Surviving lines", *rep.Lines)
	}
	fmt.Fprintln(w)

	if len(rep.Agents) > 0 {
		fmt.Fprintf(w, "### Agents\n\n| Agent | %s | Share | Commits | Models |\n|---|---:|---:|---:|---|\n", metricHeader(rep))
		for i, a := range rep.Agents {
			if i >= top {
				break
			}
			n := a.Lines
			if rep.Metric == "churn" {
				n = a.Churn
			}
			fmt.Fprintf(w, "| %s | %s | %s | %s | %s |\n", esc(a.Name), commas(n), Pct(pct(n, tot.AI+tot.Human)), commas(a.Commits), esc(strings.Join(a.Models, ", ")))
		}
		fmt.Fprintln(w)
	}
	if len(rep.Authors) > 0 {
		fmt.Fprintf(w, "### Contributors\n\n| Contributor | Commits | AI-assisted | %s | AI share |\n|---|---:|---:|---:|---:|\n", metricHeader(rep))
		for i, a := range rep.Authors {
			if i >= top {
				break
			}
			n := a.Lines
			if rep.Metric == "churn" {
				n = a.Churn
			}
			fmt.Fprintf(w, "| %s | %s | %s (%s) | %s | %s |\n", esc(a.Name), commas(a.Commits), commas(a.AICommits), Pct(pct(a.AICommits, a.Commits)), commas(n), Pct(a.AIShare))
		}
		fmt.Fprintln(w)
	}
	if len(rep.Conventions) > 0 {
		fmt.Fprintln(w, "### Disclosure conventions\n\n| Convention | Commits |\n|---|---:|")
		for _, c := range rep.Conventions {
			fmt.Fprintf(w, "| `%s` | %s |\n", c.Name, commas(c.Commits))
		}
		fmt.Fprintln(w)
	}
	if !o.Compact && len(rep.Dirs) > 0 {
		fmt.Fprintf(w, "### Directories\n\n| Path | %s | AI | Share |\n|---|---:|---:|---:|\n", metricHeader(rep))
		for i, d := range rep.Dirs {
			if i >= top {
				break
			}
			fmt.Fprintf(w, "| `%s` | %s | %s | %s |\n", d.Path, commas(d.Lines), commas(d.AILines), Pct(d.AIShare))
		}
		fmt.Fprintln(w)
	}
	if !o.Compact && len(rep.Months) > 1 {
		fmt.Fprintln(w, "### Timeline\n\n| Month | Commits | AI commits | AI share of lines added |\n|---|---:|---:|---:|")
		for _, m := range tail(rep.Months, top) {
			fmt.Fprintf(w, "| %s | %s | %s | %s |\n", m.Month, commas(m.Commits), commas(m.AICommits), Pct(m.AIShare))
		}
		fmt.Fprintln(w)
	}
	fmt.Fprintf(w, "<sub>Generated by <a href=\"https://github.com/trinhbentre/aiblame\">aiblame</a> %s · counts disclosed AI involvement only (trailers, agent identities, message markers)</sub>\n", rep.Version)
}

func mdTotals(w io.Writer, label string, t stats.Totals) {
	fmt.Fprintf(w, "| %s | %s | %s (%s) | %s | %s | %s | %s |\n", label, commas(t.Total), commas(t.AI), Pct(t.AIShare), commas(t.Assisted), commas(t.Agent), commas(t.Human), commas(t.Bot))
}

func esc(s string) string {
	return strings.NewReplacer("|", "\\|", "<", "&lt;", ">", "&gt;").Replace(Sanitize(s))
}

func metricHeader(rep *stats.Report) string {
	if rep.Metric == "churn" {
		return "lines added"
	}
	return "lines"
}

func pct(n, den int64) float64 {
	if den == 0 {
		return 0
	}
	return float64(n) * 100 / float64(den)
}

func orDash(s string) string {
	if s == "" {
		return "-"
	}
	return s
}

func tail[T any](s []T, n int) []T {
	if n <= 0 || len(s) <= n {
		return s
	}
	return s[len(s)-n:]
}

func commas(n int64) string {
	neg := n < 0
	if neg {
		n = -n
	}
	s := fmt.Sprintf("%d", n)
	if len(s) <= 3 {
		if neg {
			return "-" + s
		}
		return s
	}
	var b strings.Builder
	pre := len(s) % 3
	if pre > 0 {
		b.WriteString(s[:pre])
	}
	for i := pre; i < len(s); i += 3 {
		if b.Len() > 0 {
			b.WriteByte(',')
		}
		b.WriteString(s[i : i+3])
	}
	if neg {
		return "-" + b.String()
	}
	return b.String()
}

// writeAligned prints rows as aligned columns. Columns listed in leftAlign
// are left-aligned; the rest are right-aligned. The first row is treated as a
// header when it exists and is dimmed.
func writeAligned(w io.Writer, rows [][]string, indent string, p palette, leftAlign []int) {
	if len(rows) == 0 {
		return
	}
	left := map[int]bool{}
	for _, i := range leftAlign {
		left[i] = true
	}
	ncol := 0
	for _, r := range rows {
		if len(r) > ncol {
			ncol = len(r)
		}
	}
	widths := make([]int, ncol)
	for _, r := range rows {
		for i, c := range r {
			if n := utf8.RuneCountInString(c); n > widths[i] {
				widths[i] = n
			}
		}
	}
	for ri, r := range rows {
		var b strings.Builder
		b.WriteString(indent)
		for i := 0; i < ncol; i++ {
			c := ""
			if i < len(r) {
				c = r[i]
			}
			pad := widths[i] - utf8.RuneCountInString(c)
			if left[i] {
				b.WriteString(c)
				if i < ncol-1 {
					b.WriteString(strings.Repeat(" ", pad))
				}
			} else {
				b.WriteString(strings.Repeat(" ", pad))
				b.WriteString(c)
			}
			if i < ncol-1 {
				b.WriteString("  ")
			}
		}
		line := strings.TrimRight(b.String(), " ")
		if ri == 0 && len(rows) > 1 && isHeader(r) {
			line = p.wrap(p.dim, line)
		}
		fmt.Fprintln(w, line)
	}
}

func isHeader(r []string) bool {
	return len(r) > 0 && r[0] == ""
}

// KindColor returns the ANSI colour for a kind (used by `aiblame blame`).
func KindColor(p palette, k attrib.Kind) string {
	switch k {
	case attrib.Assisted, attrib.Agent:
		return p.ai
	case attrib.Bot:
		return p.bot
	default:
		return p.human
	}
}
