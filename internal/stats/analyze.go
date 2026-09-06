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
	"github.com/trinhbentre/aiblame/internal/provenance"
)

// Options controls an analysis run.
type Options struct {
	// Rev is the revision to analyse (default HEAD). A range "BASE..HEAD"
	// restricts commits and churn to the range and surviving lines to those
	// introduced by it — the shape of a pull request.
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
	// Provenance reads attribution left by other tools (git-ai notes,
	// Entire checkpoints) in addition to commit messages.
	Provenance bool
	// NoAuthors leaves the per-contributor table empty (privacy mode for
	// shared reports).
	NoAuthors bool
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
	// ExtraAgents are user-defined identities, also used to canonicalise
	// tool names found in sidecar data.
	ExtraAgents []attrib.Identity

	provOnce sync.Once
	prov     *provenance.Store
	provErr  error
}

// splitRange interprets Rev. "A..B" and "A...B" are ranges: commits come
// from `git log A..B`, blame runs at B (HEAD when B is empty).
func splitRange(rev string) (spec, head, base string, isRange bool) {
	if rev == "" {
		return "HEAD", "HEAD", "", false
	}
	i := strings.Index(rev, "..")
	if i < 0 {
		return rev, rev, "", false
	}
	base = rev[:i]
	head = strings.TrimLeft(rev[i:], ".")
	if head == "" {
		head = "HEAD"
	}
	if base == "" {
		base = "HEAD"
	}
	return rev, head, base, true
}

// Commits returns every commit reachable from Opts.Rev with its attribution,
// newest first. Since/Until are NOT applied here so that blame can resolve
// every hash; callers filter with InWindow. When Opts.Provenance is set,
// evidence from git-ai notes and Entire checkpoints is merged in.
func (a *Analyzer) Commits(ctx context.Context) ([]ClassifiedCommit, error) {
	spec, _, _, _ := splitRange(a.Opts.Rev)
	commits, err := a.Runner.Log(ctx, gitx.LogOptions{Rev: spec, NumStat: true})
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
	if a.Opts.Provenance {
		store, err := a.provenanceStore(ctx, out)
		if err != nil {
			return nil, err
		}
		for i := range out {
			applyProvenance(&out[i].Attr, out[i].Hash, store)
		}
	}
	return out, nil
}

// provenanceStore loads the sidecar data once per Analyzer.
func (a *Analyzer) provenanceStore(ctx context.Context, commits []ClassifiedCommit) (*provenance.Store, error) {
	a.provOnce.Do(func() {
		var ids []string
		for _, c := range commits {
			for _, ev := range c.Attr.Evidence {
				if ev.Source == provenance.EntireTrailerSource {
					ids = append(ids, ev.Value)
				}
			}
		}
		a.prov, a.provErr = provenance.Load(ctx, a.Runner, provenance.Options{ExtraAgents: a.ExtraAgents, CheckpointIDs: ids})
	})
	return a.prov, a.provErr
}

// applyProvenance merges sidecar evidence into a commit's attribution.
func applyProvenance(att *attrib.Attribution, hash string, store *provenance.Store) {
	if store == nil {
		return
	}
	if rec := store.ForCommit(hash); rec != nil {
		for _, ev := range rec.Evidence {
			att.Merge(ev, rec.Source)
		}
	}
	for _, ev := range att.Evidence {
		if ev.Source != provenance.EntireTrailerSource {
			continue
		}
		if cp := store.Checkpoint(ev.Value); cp != nil && cp.Agent != "" {
			att.SetAgent(provenance.EntireTrailerSource, cp.Agent, cp.Model)
			break
		}
	}
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

func (w window) active() bool { return w.hasSince || w.hasUntil }

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

// resolveDate accepts RFC3339 / YYYY-MM-DD style layouts and the relative
// form "N days|weeks|months|years ago".
func (a *Analyzer) resolveDate(_ context.Context, s string) (time.Time, error) {
	s = strings.TrimSpace(s)
	for _, layout := range []string{time.RFC3339, "2006-01-02", "2006-01-02T15:04:05", "2006-01", "2006"} {
		if t, err := time.Parse(layout, s); err == nil {
			return t.UTC(), nil
		}
	}
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
	spec, headRev, baseRev, isRange := splitRange(opts.Rev)
	hash, err := a.Runner.ResolveRev(ctx, headRev)
	if err != nil {
		if headRev == "HEAD" && !a.Runner.HasCommits(ctx) {
			return nil, fmt.Errorf("repository %s has no commits yet", a.Runner.Dir)
		}
		return nil, err
	}
	if isRange {
		if _, err := a.Runner.ResolveRev(ctx, baseRev); err != nil {
			return nil, err
		}
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
		RevName:     spec,
		Rev:         hash,
		GeneratedAt: time.Now().UTC(),
		Since:       opts.Since,
		Until:       opts.Until,
		Includes:    opts.Include,
	}
	if isRange {
		rep.BaseRev = baseRev
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
	touched := map[string]bool{} // paths changed by in-range commits
	agents := map[string]*AgentStat{}
	authors := map[string]*AuthorStat{}
	months := map[string]*MonthStat{}
	convs := map[string]int64{}
	churnByPath := map[string]*PathStat{}
	agentModels := map[string]map[string]bool{}
	unrecognised := map[string]int64{}

	for i := range commits {
		c := &commits[i]
		byHash[c.Hash] = c
		for _, f := range c.Files {
			if f.Binary {
				binaryPaths[f.Path] = true
			}
			touched[f.Path] = true
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
		for _, ig := range c.Attr.Ignored {
			if ig.Reason == attrib.ReasonUnrecognisedTool {
				name := attrib.ParseAgentRef(ig.Value).Tool
				if name == "" {
					name = ig.Value
				}
				if attrib.IsToolLikeName(name) {
					unrecognised[name]++
				}
			}
		}

		if !opts.NoAuthors && (k == attrib.Human || k == attrib.Assisted) {
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

	// ---- provenance summary ----------------------------------------------
	store := a.prov
	if opts.Provenance && store != nil {
		for name, n := range store.Sources {
			rep.Provenance = append(rep.Provenance, NameCount{Name: name, Commits: int64(n)})
		}
		rep.LineLevelCommits = store.LineLevelCommits()
		rep.Warnings = append(rep.Warnings, store.Warnings...)
	}
	lineLevel := opts.Provenance && store.HasLineLevel()

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
			if isRange && !touched[p] {
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

		bopts := gitx.BlameOptions{Rev: hash, IgnoreWhitespace: opts.IgnoreWhitespace, Detail: lineLevel}
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

		// account credits n surviving lines of a file to commit h. When
		// override is set the line's kind comes from a line-level authorship
		// log rather than from the commit. The caller holds mu.
		account := func(ps *PathStat, h string, n int64, override *attrib.Kind) {
			cc, ok := byHash[h]
			if !ok && isRange {
				return // a line older than the range: not part of this change
			}
			k := attrib.Human
			var agentNames []string
			if ok {
				k = cc.Attr.Kind
				agentNames = cc.Attr.Agents
				if override != nil {
					k = *override
					if !k.IsAI() {
						agentNames = nil
					}
				}
				if !opts.NoAuthors && (k == attrib.Human || k == attrib.Assisted) {
					key := strings.ToLower(cc.AuthorEmail)
					if key == "" {
						key = strings.ToLower(cc.AuthorName)
					}
					v := authorLines[key]
					v[0] += n
					if k == attrib.Assisted {
						v[1] += n
					}
					authorLines[key] = v
				}
			}
			lines.Add(k, n)
			ps.Lines += n
			addPathKind(ps, k, n)
			for _, an := range agentNames {
				agentLines[an] += n
			}
		}

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
				// Aggregate per (commit, kind override) outside the lock; byHash
				// and store are read-only during the blame phase.
				type group struct {
					hash     string
					override int // -1 = inherit the commit's kind
				}
				local := map[group]int64{}
				if err == nil {
					if lineLevel && len(res.Lines) > 0 {
						for _, l := range res.Lines {
							g := group{hash: l.Hash, override: -1}
							if k := lineKind(store, byHash[l.Hash], l, p); k != nil {
								g.override = int(*k)
							}
							local[g]++
						}
					} else {
						for h, n := range res.Counts {
							local[group{hash: h, override: -1}] = int64(n)
						}
					}
				}
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
				for g, n := range local {
					var override *attrib.Kind
					if g.override >= 0 {
						k := attrib.Kind(g.override)
						override = &k
					}
					account(ps, g.hash, n, override)
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
		if !win.active() {
			rep.Survival = &Survival{
				AI:    survival(lines.AI, rep.Churn.AI),
				Human: survival(lines.Human, rep.Churn.Human),
				All:   survival(lines.AI+lines.Human, rep.Churn.AI+rep.Churn.Human),
			}
			for _, as := range agents {
				if as.Churn > 0 && as.Lines > 0 {
					s := survival(as.Lines, as.Churn)
					as.Survival = &s
				}
			}
		}
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
	for name, n := range unrecognised {
		rep.Unrecognised = append(rep.Unrecognised, NameCount{Name: name, Commits: n})
	}
	sortNameCounts(rep.Unrecognised)
	if len(rep.Unrecognised) > opts.Top {
		rep.Unrecognised = rep.Unrecognised[:opts.Top]
	}
	sortNameCounts(rep.Provenance)

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
	if rep.Provenance == nil {
		rep.Provenance = []NameCount{}
	}
	if rep.Unrecognised == nil {
		rep.Unrecognised = []NameCount{}
	}
	rep.DurationMS = time.Since(start).Milliseconds()
	return rep, nil
}

// lineKind decides the kind of one surviving line when a line-level
// authorship log exists for its commit. It returns nil to inherit the
// commit's kind. Lines the log does not mention are "untracked": they stay
// human unless the commit has other AI evidence (a trailer, an agent
// author), in which case the commit's kind applies.
func lineKind(store *provenance.Store, cc *ClassifiedCommit, l gitx.BlameLine, currentPath string) *attrib.Kind {
	if cc == nil {
		return nil
	}
	rec := store.ForCommit(l.Hash)
	if !rec.LineLevel() {
		return nil
	}
	fa := rec.Files[l.OrigPath]
	if fa == nil {
		fa = rec.Files[currentPath]
	}
	// A file the log does not mention is untracked as a whole.
	ai, known := fa.Kind(l.OrigLine)
	switch {
	case known && ai:
		k := attrib.Assisted
		if cc.Attr.Kind == attrib.Agent {
			k = attrib.Agent
		}
		return &k
	case known:
		k := attrib.Human
		return &k
	}
	if onlySidecarEvidence(cc.Attr) {
		k := attrib.Human
		return &k
	}
	return nil
}

// onlySidecarEvidence reports whether every piece of AI evidence on the
// commit came from an authorship log (no trailer, marker or identity).
func onlySidecarEvidence(att attrib.Attribution) bool {
	if !att.Kind.IsAI() {
		return false
	}
	for _, ev := range att.Evidence {
		if !strings.HasPrefix(ev.Source, "notes:") && ev.Source != "refs/ai/authorship" {
			return false
		}
	}
	return true
}

// survival is surviving/added as a percentage, capped at 100 (renames and
// ignore-revs can make blame attribute more lines than numstat counted).
func survival(surviving, added int64) float64 {
	if added <= 0 {
		return 0
	}
	s := float64(surviving) * 100 / float64(added)
	if s > 100 {
		s = 100
	}
	return s
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
	if id := attrib.AgentByName(agent); id != nil {
		return id.Vendor
	}
	return ""
}

func sortNameCounts(s []NameCount) {
	sort.Slice(s, func(i, j int) bool {
		if s[i].Commits != s[j].Commits {
			return s[i].Commits > s[j].Commits
		}
		return s[i].Name < s[j].Name
	})
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

// Provenance returns the sidecar store loaded by Commits (nil before Commits
// has run or when Opts.Provenance is off).
func (a *Analyzer) Provenance() *provenance.Store { return a.prov }

// LineKind exposes the line-level rule used by Run for `aiblame blame`.
func LineKind(store *provenance.Store, cc *ClassifiedCommit, l gitx.BlameLine, currentPath string) *attrib.Kind {
	if store == nil {
		return nil
	}
	return lineKind(store, cc, l, currentPath)
}

// HeadOfRange returns the revision blame runs at: the right side of a
// "BASE..HEAD" range, or rev itself.
func HeadOfRange(rev string) string {
	_, head, _, _ := splitRange(rev)
	return head
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
