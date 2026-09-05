package stats

import (
	"context"
	"fmt"
	"path"
	"runtime"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/trinhbentre/aiblame/internal/attrib"
	"github.com/trinhbentre/aiblame/internal/gitx"
)

// Options controls an analysis run.
type Options struct {
	Rev              string
	Since            string
	Until            string
	Blame            bool // compute surviving lines via git blame
	IgnoreWhitespace bool
	NoIgnoreRevs     bool // do not honour .git-blame-ignore-revs
	Include          []string
	Exclude          []string
	DefaultExcludes  bool
	MaxFileSize      int64 // 0 = 1 MiB
	Jobs             int   // 0 = GOMAXPROCS
	IncludeMerges    bool
	Top              int // rows in agents/authors/dirs/files (0 = 10)
	PathDepth        int // directory depth for Dirs (0 = 1)
	Version          string
	// Progress, when set, is called from the blame phase.
	Progress func(done, total int)
}

// ClassifiedCommit pairs a commit with its attribution.
type ClassifiedCommit struct {
	gitx.Commit
	Attr attrib.Attribution
}

// Analyzer runs the analysis.
type Analyzer struct {
	Runner     *gitx.Runner
	Classifier *attrib.Classifier
	Opts       Options
}

// Commits returns every commit reachable from Opts.Rev with its attribution,
// newest first. Since/Until are NOT applied here so that blame can resolve
// every hash; callers filter with InWindow.
func (a *Analyzer) Commits(ctx context.Context) ([]ClassifiedCommit, error) {
	rev := a.Opts.Rev
	if rev == "" {
		rev = "HEAD"
	}
	commits, err := a.Runner.Log(ctx, gitx.LogOptions{Rev: rev, NumStat: true})
	if err != nil {
		return nil, err
	}
	out := make([]ClassifiedCommit, 0, len(commits))
	for _, c := range commits {
		att := a.Classifier.Classify(attrib.Commit{
			Hash:           c.Hash,
			AuthorName:     c.AuthorName,
			AuthorEmail:    c.AuthorEmail,
			CommitterName:  c.CommitterName,
			CommitterEmail: c.CommitterEmail,
			Message:        c.Message,
		})
		out = append(out, ClassifiedCommit{Commit: c, Attr: att})
	}
	return out, nil
}

// window is the parsed Since/Until bound, resolved through git so that
// expressions like "6 months ago" work.
type window struct {
	since, until time.Time
	hasSince     bool
	hasUntil     bool
}

func (w window) contains(t time.Time) bool {
	if w.hasSince && t.Before(w.since) {
		return false
	}
	if w.hasUntil && t.After(w.until) {
		return false
	}
	return true
}

func (a *Analyzer) resolveWindow(ctx context.Context) (window, error) {
	var w window
	if a.Opts.Since != "" {
		t, err := a.resolveDate(ctx, a.Opts.Since)
		if err != nil {
			return w, fmt.Errorf("--since: %w", err)
		}
		w.since, w.hasSince = t, true
	}
	if a.Opts.Until != "" {
		t, err := a.resolveDate(ctx, a.Opts.Until)
		if err != nil {
			return w, fmt.Errorf("--until: %w", err)
		}
		w.until, w.hasUntil = t, true
	}
	return w, nil
}

// resolveDate uses git's approxidate parser via `git rev-parse`'s date
// handling is not exposed; instead we accept RFC3339/YYYY-MM-DD directly and
// fall back to `git log --since` semantics by asking git for the oldest
// commit in range. Simpler: parse common layouts here.
func (a *Analyzer) resolveDate(_ context.Context, s string) (time.Time, error) {
	s = strings.TrimSpace(s)
	for _, layout := range []string{time.RFC3339, "2006-01-02", "2006-01-02T15:04:05", "2006-01", "2006"} {
		if t, err := time.Parse(layout, s); err == nil {
			return t.UTC(), nil
		}
	}
	// relative: "N days|weeks|months|years ago"
	f := strings.Fields(strings.ToLower(s))
	if len(f) == 3 && f[2] == "ago" {
		var n int
		if _, err := fmt.Sscanf(f[0], "%d", &n); err == nil {
			now := time.Now().UTC()
			switch strings.TrimSuffix(f[1], "s") {
			case "day":
				return now.AddDate(0, 0, -n), nil
			case "week":
				return now.AddDate(0, 0, -7*n), nil
			case "month":
				return now.AddDate(0, -n, 0), nil
			case "year":
				return now.AddDate(-n, 0, 0), nil
			}
		}
	}
	return time.Time{}, fmt.Errorf("unrecognised date %q (use YYYY-MM-DD, RFC3339 or \"N months ago\")", s)
}

// Run performs the full analysis.
func (a *Analyzer) Run(ctx context.Context) (*Report, error) {
	start := time.Now()
	opts := a.Opts
	if opts.Top <= 0 {
		opts.Top = 10
	}
	if opts.PathDepth <= 0 {
		opts.PathDepth = 1
	}
	if opts.MaxFileSize <= 0 {
		opts.MaxFileSize = 1 << 20
	}
	if opts.Jobs <= 0 {
		opts.Jobs = runtime.GOMAXPROCS(0)
	}
	rev := opts.Rev
	if rev == "" {
		rev = "HEAD"
	}
	hash, err := a.Runner.ResolveRev(ctx, rev)
	if err != nil {
		if rev == "HEAD" && !a.Runner.HasCommits(ctx) {
			return nil, fmt.Errorf("repository %s has no commits yet", a.Runner.Dir)
		}
		return nil, err
	}
	win, err := a.resolveWindow(ctx)
	if err != nil {
		return nil, err
	}

	rep := &Report{
		Tool:        "aiblame",
		Version:     opts.Version,
		SchemaVer:   SchemaVersion,
		Repo:        a.Runner.Dir,
		Remote:      a.Runner.RemoteURL(ctx),
		RevName:     rev,
		Rev:         hash,
		GeneratedAt: time.Now().UTC(),
		Since:       opts.Since,
		Until:       opts.Until,
		Includes:    opts.Include,
	}
	if a.Runner.IsShallow(ctx) {
		rep.Warnings = append(rep.Warnings, "shallow clone: history is incomplete, so commit and line counts are understated (run `git fetch --unshallow`, or use fetch-depth: 0 in CI)")
	}

	// ---- file filter ------------------------------------------------------
	var excl []string
	if opts.DefaultExcludes {
		excl = append(excl, DefaultExcludes...)
	}
	excl = append(excl, opts.Exclude...)
	exclude := NewMatcher(excl)
	include := NewMatcher(opts.Include)
	rep.Excludes = exclude.Patterns()
	counted := func(p string) bool {
		if !include.Empty() && !include.Match(p) {
			return false
		}
		return !exclude.Match(p)
	}

	// ---- commits ----------------------------------------------------------
	commits, err := a.Commits(ctx)
	if err != nil {
		return nil, err
	}
	byHash := make(map[string]*ClassifiedCommit, len(commits))
	binaryPaths := map[string]bool{}
	agents := map[string]*AgentStat{}
	authors := map[string]*AuthorStat{}
	months := map[string]*MonthStat{}
	convs := map[string]int64{}
	churnByPath := map[string]*PathStat{}
	agentModels := map[string]map[string]bool{}

	for i := range commits {
		c := &commits[i]
		byHash[c.Hash] = c
		for _, f := range c.Files {
			if f.Binary {
				binaryPaths[f.Path] = true
			}
		}
		if c.IsMerge() && !opts.IncludeMerges {
			continue
		}
		if !win.contains(c.AuthorTime) {
			continue
		}
		k := c.Attr.Kind
		rep.Commits.Add(k, 1)

		var added int64
		for _, f := range c.Files {
			if f.Binary || !counted(f.Path) {
				continue
			}
			added += int64(f.Added)
			ps := churnByPath[f.Path]
			if ps == nil {
				ps = &PathStat{Path: f.Path}
				churnByPath[f.Path] = ps
			}
			ps.Lines += int64(f.Added)
			addPathKind(ps, k, int64(f.Added))
		}
		rep.Churn.Add(k, added)

		m := c.AuthorTime.Format("2006-01")
		ms := months[m]
		if ms == nil {
			ms = &MonthStat{Month: m}
			months[m] = ms
		}
		ms.Commits++
		ms.Churn += added
		if k.IsAI() {
			ms.AICommits++
			ms.AIChurn += added
		}

		for _, name := range c.Attr.Agents {
			as := agents[name]
			if as == nil {
				as = &AgentStat{Name: name, Vendor: vendorOf(name)}
				agents[name] = as
			}
			as.Commits++
			as.Churn += added
			for _, mdl := range c.Attr.Models {
				if agentModels[name] == nil {
					agentModels[name] = map[string]bool{}
				}
				agentModels[name][mdl] = true
			}
		}
		for _, cv := range c.Attr.Conventions {
			convs[cv]++
		}
		if len(c.Attr.Conventions) == 0 {
			switch {
			case k == attrib.Agent:
				convs["agent-author"]++
			case k == attrib.Assisted:
				convs["message-marker"]++
			}
		}

		if k == attrib.Human || k == attrib.Assisted {
			key := strings.ToLower(c.AuthorEmail)
			if key == "" {
				key = strings.ToLower(c.AuthorName)
			}
			au := authors[key]
			if au == nil {
				au = &AuthorStat{Name: c.AuthorName, Email: c.AuthorEmail}
				authors[key] = au
			}
			au.Commits++
			au.Churn += added
			if k == attrib.Assisted {
				au.AICommits++
				au.AIChurn += added
			}
		}
	}
	rep.Commits.Finalize()
	rep.Churn.Finalize()

	// ---- blame ------------------------------------------------------------
	if opts.Blame {
		sizes, err := a.Runner.FileSizes(ctx, hash)
		if err != nil {
			return nil, err
		}
		var files []string
		for p, sz := range sizes {
			if !counted(p) {
				continue
			}
			if binaryPaths[p] || BinaryExtensions[strings.ToLower(path.Ext(p))] || sz > opts.MaxFileSize || sz == 0 {
				rep.SkippedFiles++
				continue
			}
			files = append(files, p)
		}
		sort.Strings(files)
		rep.FileCount = len(files)

		bopts := gitx.BlameOptions{Rev: hash, IgnoreWhitespace: opts.IgnoreWhitespace}
		if !opts.NoIgnoreRevs {
			bopts.IgnoreRevsFile = a.Runner.IgnoreRevsFile()
		}

		lines := &Totals{}
		linesByPath := make(map[string]*PathStat, len(files))
		authorLines := map[string][2]int64{} // email -> [lines, aiLines]
		agentLines := map[string]int64{}
		var mu sync.Mutex
		var wg sync.WaitGroup
		sem := make(chan struct{}, opts.Jobs)
		done := 0
		blameFailed := 0
		var firstErr error
		ctx, cancel := context.WithCancel(ctx)
		defer cancel()

		for _, p := range files {
			p := p
			wg.Add(1)
			sem <- struct{}{}
			go func() {
				defer wg.Done()
				defer func() { <-sem }()
				if ctx.Err() != nil {
					return
				}
				res, err := a.Runner.Blame(ctx, p, bopts)
				mu.Lock()
				defer mu.Unlock()
				done++
				if opts.Progress != nil {
					opts.Progress(done, len(files))
				}
				if err != nil {
					if ctx.Err() != nil {
						return
					}
					// Skip unreadable files (e.g. submodule gitlinks) but keep going;
					// the total is reported below and Run fails if nothing worked.
					rep.SkippedFiles++
					blameFailed++
					if len(rep.Warnings) < 20 {
						rep.Warnings = append(rep.Warnings, fmt.Sprintf("blame %s: %v", p, shortErr(err)))
					}
					if firstErr == nil {
						firstErr = err
					}
					return
				}
				ps := &PathStat{Path: p}
				linesByPath[p] = ps
				for h, n := range res.Counts {
					k := attrib.Human
					var agentNames []string
					if cc, ok := byHash[h]; ok {
						k = cc.Attr.Kind
						agentNames = cc.Attr.Agents
						if k == attrib.Human || k == attrib.Assisted {
							key := strings.ToLower(cc.AuthorEmail)
							if key == "" {
								key = strings.ToLower(cc.AuthorName)
							}
							v := authorLines[key]
							v[0] += int64(n)
							if k == attrib.Assisted {
								v[1] += int64(n)
							}
							authorLines[key] = v
						}
					}
					lines.Add(k, int64(n))
					ps.Lines += int64(n)
					addPathKind(ps, k, int64(n))
					for _, an := range agentNames {
						agentLines[an] += int64(n)
					}
				}
			}()
		}
		wg.Wait()
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if blameFailed > 0 {
			if blameFailed == len(files) {
				return nil, fmt.Errorf("git blame failed for all %d files (first error: %w)", len(files), firstErr)
			}
			rep.Warnings = append(rep.Warnings, fmt.Sprintf("%d of %d files could not be blamed and were skipped", blameFailed, len(files)))
		}
		lines.Finalize()
		rep.Lines = lines
		for name, n := range agentLines {
			if as := agents[name]; as != nil {
				as.Lines = n
			} else {
				agents[name] = &AgentStat{Name: name, Vendor: vendorOf(name), Lines: n}
			}
		}
		for key, v := range authorLines {
			if au := authors[key]; au != nil {
				au.Lines, au.AILines = v[0], v[1]
			}
		}
		rep.Dirs, rep.Files = pathStats(linesByPath, opts.PathDepth, opts.Top)
		rep.Metric = "lines"
		rep.Headline = lines.AIShare
	} else {
		rep.Dirs, rep.Files = pathStats(churnByPath, opts.PathDepth, opts.Top)
		rep.Metric = "churn"
		rep.Headline = rep.Churn.AIShare
	}

	// ---- finalize lists ---------------------------------------------------
	for name, as := range agents {
		if mm := agentModels[name]; len(mm) > 0 {
			for m := range mm {
				as.Models = append(as.Models, m)
			}
			sort.Strings(as.Models)
		}
		rep.Agents = append(rep.Agents, *as)
	}
	sortAgents(rep.Agents)

	for _, au := range authors {
		if rep.Lines != nil && au.Lines > 0 {
			au.AIShare = share(au.AILines, au.Lines-au.AILines)
		} else {
			au.AIShare = share(au.AIChurn, au.Churn-au.AIChurn)
		}
		rep.Authors = append(rep.Authors, *au)
	}
	sortAuthors(rep.Authors)
	if len(rep.Authors) > opts.Top {
		rep.Authors = rep.Authors[:opts.Top]
	}

	for name, n := range convs {
		rep.Conventions = append(rep.Conventions, ConventionStat{Name: name, Commits: n})
	}
	sort.Slice(rep.Conventions, func(i, j int) bool {
		if rep.Conventions[i].Commits != rep.Conventions[j].Commits {
			return rep.Conventions[i].Commits > rep.Conventions[j].Commits
		}
		return rep.Conventions[i].Name < rep.Conventions[j].Name
	})

	for _, ms := range months {
		ms.AIShare = share(ms.AIChurn, ms.Churn-ms.AIChurn)
		rep.Months = append(rep.Months, *ms)
	}
	sort.Slice(rep.Months, func(i, j int) bool { return rep.Months[i].Month < rep.Months[j].Month })

	if rep.Agents == nil {
		rep.Agents = []AgentStat{}
	}
	if rep.Authors == nil {
		rep.Authors = []AuthorStat{}
	}
	if rep.Conventions == nil {
		rep.Conventions = []ConventionStat{}
	}
	if rep.Months == nil {
		rep.Months = []MonthStat{}
	}
	if rep.Dirs == nil {
		rep.Dirs = []PathStat{}
	}
	if rep.Files == nil {
		rep.Files = []PathStat{}
	}
	rep.DurationMS = time.Since(start).Milliseconds()
	return rep, nil
}

func addPathKind(ps *PathStat, k attrib.Kind, n int64) {
	switch k {
	case attrib.Human:
		ps.Human += n
	case attrib.Assisted:
		ps.Assisted += n
		ps.AILines += n
	case attrib.Agent:
		ps.Agent += n
		ps.AILines += n
	case attrib.Bot:
		ps.Bot += n
	}
}

// pathStats rolls per-file stats up into directories of the given depth and
// returns the top N directories and files by AI lines.
func pathStats(files map[string]*PathStat, depth, top int) (dirs []PathStat, topFiles []PathStat) {
	dirMap := map[string]*PathStat{}
	for p, ps := range files {
		ps.AIShare = share(ps.AILines, ps.Human)
		d := dirKey(p, depth)
		ds := dirMap[d]
		if ds == nil {
			ds = &PathStat{Path: d}
			dirMap[d] = ds
		}
		ds.Lines += ps.Lines
		ds.AILines += ps.AILines
		ds.Human += ps.Human
		ds.Bot += ps.Bot
		ds.Assisted += ps.Assisted
		ds.Agent += ps.Agent
		topFiles = append(topFiles, *ps)
	}
	for _, ds := range dirMap {
		ds.AIShare = share(ds.AILines, ds.Human)
		dirs = append(dirs, *ds)
	}
	sortPaths(dirs)
	sortPaths(topFiles)
	if len(dirs) > top {
		dirs = dirs[:top]
	}
	if len(topFiles) > top {
		topFiles = topFiles[:top]
	}
	return dirs, topFiles
}

func dirKey(p string, depth int) string {
	parts := strings.Split(p, "/")
	if len(parts) <= 1 {
		return "(root)"
	}
	if depth > len(parts)-1 {
		depth = len(parts) - 1
	}
	return strings.Join(parts[:depth], "/") + "/"
}

func vendorOf(agent string) string {
	for _, id := range attrib.KnownAgents {
		if id.Name == agent {
			return id.Vendor
		}
	}
	return ""
}

func shortErr(err error) string {
	s := err.Error()
	if i := strings.IndexByte(s, '\n'); i > 0 {
		s = s[:i]
	}
	if len(s) > 160 {
		s = s[:160] + "…"
	}
	return s
}

// InWindow reports whether the commit falls inside Since/Until, using the
// same parser as Run.
func (a *Analyzer) InWindow(ctx context.Context, c gitx.Commit) (bool, error) {
	w, err := a.resolveWindow(ctx)
	if err != nil {
		return false, err
	}
	return w.contains(c.AuthorTime), nil
}
