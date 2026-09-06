// Package cli implements the aiblame command line.
package cli

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/trinhbentre/aiblame/internal/attrib"
	"github.com/trinhbentre/aiblame/internal/config"
	"github.com/trinhbentre/aiblame/internal/gitx"
	"github.com/trinhbentre/aiblame/internal/stats"
)

// Build metadata, overridden with -ldflags at release time.
var (
	Version = "dev"
	Commit  = ""
	Date    = ""
)

// Exit codes.
const (
	ExitOK     = 0
	ExitFailed = 1 // a check/policy failed
	ExitUsage  = 2
	ExitError  = 3
)

// Env bundles the process environment so Main is testable.
type Env struct {
	Stdout io.Writer
	Stderr io.Writer
	Getenv func(string) string
	// Cwd is used to resolve relative paths (default: process cwd).
	Cwd string
}

// Main runs aiblame with args (without the program name) and returns an exit
// code.
func Main(ctx context.Context, args []string, env Env) int {
	if env.Getenv == nil {
		env.Getenv = os.Getenv
	}
	if env.Stdout == nil {
		env.Stdout = os.Stdout
	}
	if env.Stderr == nil {
		env.Stderr = os.Stderr
	}
	if len(args) == 0 {
		return cmdStats(ctx, nil, env)
	}
	name, rest := args[0], args[1:]
	switch name {
	case "stats", "report":
		return cmdStats(ctx, rest, env)
	case "blame":
		return cmdBlame(ctx, rest, env)
	case "log":
		return cmdLog(ctx, rest, env)
	case "badge":
		return cmdBadge(ctx, rest, env)
	case "check":
		return cmdCheck(ctx, rest, env)
	case "hook":
		return cmdHook(ctx, rest, env)
	case "agents":
		return cmdAgents(ctx, rest, env)
	case "init":
		return cmdInit(ctx, rest, env)
	case "version", "--version", "-v", "-V":
		return cmdVersion(rest, env)
	case "help", "--help", "-h":
		if len(rest) > 0 {
			return Main(ctx, []string{rest[0], "--help"}, env)
		}
		fmt.Fprint(env.Stdout, usage)
		return ExitOK
	}
	// `aiblame --json .`, `aiblame ./path`, `aiblame owner/repo`: implicit stats.
	return cmdStats(ctx, args, env)
}

const usage = `aiblame — how much of this repo did AI write?

Zero-setup AI authorship statistics for any git repository, computed from the
attribution that coding agents already leave behind: Co-authored-by trailers
(Claude Code, Codex, Cursor, Copilot, aider), Assisted-by trailers (Linux
kernel, Mesa, Fedora), Generated-by trailers, agent author identities (Devin,
Copilot SWE agent, Jules) and "Generated with …" message markers.

Usage:
  aiblame [stats] [PATH|URL|owner/repo] [flags]   Full report (default command)
                                                  (also host/owner/repo for GitLab, Codeberg…)
  aiblame blame FILE [flags]                      Per-line AI/human view of a file
  aiblame log [flags]                             Commits with their AI classification
  aiblame badge [flags]                           SVG badge / shields.io endpoint JSON
  aiblame check [flags]                           CI gate: thresholds and disclosure policy
  aiblame hook install|uninstall|status|print|detect
                                                  prepare-commit-msg hook that adds trailers
                                                  when committing from an agent session
  aiblame agents                                  List recognised agents and bots
  aiblame init                                    Write a starter .aiblame.toml
  aiblame version

Common flags (stats, badge, check, log):
  --rev REV              Revision to analyse (default HEAD). A range BASE..HEAD
                         reports only what the range introduced (a PR)
  --since DATE           Only count commits after DATE (YYYY-MM-DD or "6 months ago")
  --until DATE           Only count commits before DATE
  --no-blame             Skip git blame; use lines added instead of surviving lines
  -j, --jobs N           Parallel blame workers (default: number of CPUs)
  --include GLOB         Only count matching paths (repeatable)
  --exclude GLOB         Skip matching paths (repeatable)
  --no-default-excludes  Count lockfiles, vendor/, minified and generated files too
  -w, --ignore-whitespace  Pass -w to git blame
  --no-ignore-revs       Do not honour .git-blame-ignore-revs
  --include-merges       Count merge commits
  --no-provenance        Ignore git-ai notes, Entire checkpoints and other sidecar data
  --no-authors           Leave out the per-contributor table
  --top N                Rows per section (default 10)
  --depth N              Directory depth for the per-directory table (default 1)
  --max-file-size BYTES  Skip larger blobs (default 1 MiB)
  --config FILE          Config file (default <repo>/.aiblame.toml)
  -f, --format FMT       table | json | md
  --json                 Shorthand for --format json
  -o, --output FILE      Write to FILE instead of stdout
  --no-color             Disable ANSI colours (also honours NO_COLOR)
  -q, --quiet            No progress output

Exit codes: 0 ok · 1 check failed · 2 usage error · 3 error

Docs: https://github.com/trinhbentre/aiblame
`

// stringList is a repeatable string flag.
type stringList []string

func (s *stringList) String() string     { return strings.Join(*s, ",") }
func (s *stringList) Set(v string) error { *s = append(*s, v); return nil }

// common holds flags shared by the analysis commands.
type common struct {
	rev, since, until string
	noBlame           bool
	jobs              int
	includes          stringList
	excludes          stringList
	noDefaultExcludes bool
	ignoreWS          bool
	noIgnoreRevs      bool
	includeMerges     bool
	noProvenance      bool
	noAuthors         bool
	top               int
	depth             int
	maxFileSize       int64
	configPath        string
	noColor           bool
	quiet             bool
	format            string
	jsonFlag          bool
	output            string
}

func (c *common) bind(fs *flag.FlagSet) {
	fs.StringVar(&c.rev, "rev", "HEAD", "revision to analyse")
	fs.StringVar(&c.since, "since", "", "only count commits after this date")
	fs.StringVar(&c.until, "until", "", "only count commits before this date")
	fs.BoolVar(&c.noBlame, "no-blame", false, "skip git blame (use lines added)")
	fs.IntVar(&c.jobs, "jobs", 0, "parallel blame workers")
	fs.IntVar(&c.jobs, "j", 0, "parallel blame workers")
	fs.Var(&c.includes, "include", "only count matching paths (repeatable)")
	fs.Var(&c.excludes, "exclude", "skip matching paths (repeatable)")
	fs.BoolVar(&c.noDefaultExcludes, "no-default-excludes", false, "count lockfiles, vendor and generated files")
	fs.BoolVar(&c.ignoreWS, "ignore-whitespace", false, "pass -w to git blame")
	fs.BoolVar(&c.ignoreWS, "w", false, "pass -w to git blame")
	fs.BoolVar(&c.noIgnoreRevs, "no-ignore-revs", false, "do not honour .git-blame-ignore-revs")
	fs.BoolVar(&c.includeMerges, "include-merges", false, "count merge commits")
	fs.BoolVar(&c.noProvenance, "no-provenance", false, "ignore sidecar attribution data")
	fs.BoolVar(&c.noAuthors, "no-authors", false, "omit the contributor table")
	fs.IntVar(&c.top, "top", 10, "rows per section")
	fs.IntVar(&c.depth, "depth", 1, "directory depth")
	fs.Int64Var(&c.maxFileSize, "max-file-size", 0, "skip blobs larger than this many bytes")
	fs.StringVar(&c.configPath, "config", "", "config file path")
	fs.BoolVar(&c.noColor, "no-color", false, "disable colours")
	fs.BoolVar(&c.quiet, "quiet", false, "no progress output")
	fs.BoolVar(&c.quiet, "q", false, "no progress output")
	fs.StringVar(&c.format, "format", "", "output format: table, json, md")
	fs.StringVar(&c.format, "f", "", "output format")
	fs.BoolVar(&c.jsonFlag, "json", false, "output JSON")
	fs.StringVar(&c.output, "output", "", "write output to file")
	fs.StringVar(&c.output, "o", "", "write output to file")
}

func (c *common) resolvedFormat(def string) (string, error) {
	f := strings.ToLower(strings.TrimSpace(c.format))
	if c.jsonFlag {
		f = "json"
	}
	switch f {
	case "":
		return def, nil
	case "table", "text", "txt":
		return "table", nil
	case "json":
		return "json", nil
	case "md", "markdown":
		return "md", nil
	}
	return "", fmt.Errorf("unknown format %q (want table, json or md)", c.format)
}

// newFlagSet creates a flag set that prints its own usage.
func newFlagSet(name string, env Env, help string) *flag.FlagSet {
	fs := flag.NewFlagSet(name, flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	fs.Usage = func() { fmt.Fprint(env.Stderr, help) }
	return fs
}

// parseInterspersed parses flags allowing positional arguments anywhere.
func parseInterspersed(fs *flag.FlagSet, args []string) ([]string, error) {
	var pos []string
	for {
		if err := fs.Parse(args); err != nil {
			return nil, err
		}
		if fs.NArg() == 0 {
			return pos, nil
		}
		pos = append(pos, fs.Arg(0))
		args = fs.Args()[1:]
	}
}

func usageErr(env Env, fs *flag.FlagSet, err error) int {
	if errors.Is(err, flag.ErrHelp) {
		fs.Usage()
		return ExitOK
	}
	fmt.Fprintf(env.Stderr, "aiblame: %v\n", err)
	fs.Usage()
	return ExitUsage
}

func fail(env Env, err error) int {
	fmt.Fprintf(env.Stderr, "aiblame: %v\n", err)
	if errors.Is(err, gitx.ErrNotRepository) {
		fmt.Fprintln(env.Stderr, "hint: run inside a git repository, pass a path, or pass a URL / owner/repo to clone")
	}
	return ExitError
}

// useColor decides whether to emit ANSI colours.
func useColor(w io.Writer, noColor bool, getenv func(string) string) bool {
	if noColor || getenv("NO_COLOR") != "" || getenv("TERM") == "dumb" {
		return false
	}
	if getenv("FORCE_COLOR") != "" {
		return true
	}
	return isTTY(w)
}

func isTTY(w io.Writer) bool {
	f, ok := w.(*os.File)
	if !ok {
		return false
	}
	st, err := f.Stat()
	return err == nil && st.Mode()&os.ModeCharDevice != 0
}

// openTarget resolves PATH / URL / owner/repo into an opened repository. The
// returned warnings (e.g. "using cached history") belong in the report so
// that JSON consumers see them too.
func openTarget(ctx context.Context, target string, env Env, quiet bool) (*gitx.Runner, []string, error) {
	if target == "" {
		target = "."
	}
	if !filepath.IsAbs(target) && env.Cwd != "" {
		if st, err := os.Stat(filepath.Join(env.Cwd, target)); err == nil && st.IsDir() {
			target = filepath.Join(env.Cwd, target)
		}
	}
	if st, err := os.Stat(target); err == nil && st.IsDir() {
		r, err := gitx.Open(ctx, target)
		return r, nil, err
	}
	url, ok := remoteURL(target)
	if !ok {
		if _, err := os.Stat(target); err != nil {
			return nil, nil, fmt.Errorf("%s: not a directory, URL or owner/repo", target)
		}
		r, err := gitx.Open(ctx, filepath.Dir(target))
		return r, nil, err
	}
	dir, warns, err := cloneToCache(ctx, url, env, quiet)
	if err != nil {
		return nil, nil, err
	}
	r, err := gitx.Open(ctx, dir)
	return r, warns, err
}

// remoteURL recognises git URLs and GitHub owner/repo shorthand. Hosts that
// start with '-' are rejected so nothing can reach git or ssh as an option.
func remoteURL(s string) (string, bool) {
	for _, p := range []string{"https://", "http://", "git@", "ssh://", "git://"} {
		if strings.HasPrefix(s, p) {
			rest := strings.TrimPrefix(s, p)
			if rest == "" || strings.HasPrefix(rest, "-") || strings.ContainsAny(rest, " \t\n") {
				return "", false
			}
			return s, true
		}
	}
	parts := strings.Split(strings.TrimSuffix(s, ".git"), "/")
	if strings.ContainsAny(s, " \\:") {
		return "", false
	}
	for _, p := range parts {
		if p == "" || p == "." || p == ".." || strings.HasPrefix(p, "-") {
			return "", false
		}
	}
	switch {
	case len(parts) == 2:
		return "https://github.com/" + parts[0] + "/" + parts[1] + ".git", true
	case len(parts) == 3 && strings.Contains(parts[0], ".") && !strings.HasPrefix(parts[0], "."):
		// host/owner/repo for GitLab, Codeberg, Gitea, Forgejo and friends.
		return "https://" + parts[0] + "/" + parts[1] + "/" + parts[2] + ".git", true
	}
	return "", false
}

// sidecarRefspecs fetch the attribution refs that other tools keep outside
// branches: git-ai notes and authorship refs, Entire checkpoints, Exceeds
// Ink and Claudit notes. Servers without them return nothing, which is fine.
var sidecarRefspecs = []string{
	"+refs/notes/*:refs/notes/*",
	"+refs/ai/*:refs/ai/*",
	"+refs/entire/*:refs/entire/*",
	"+refs/heads/entire/checkpoints/*:refs/remotes/origin/entire/checkpoints/*",
}

func fetchSidecars(ctx context.Context, r *gitx.Runner) {
	args := append([]string{"fetch", "--quiet", "origin"}, sidecarRefspecs...)
	_, _ = r.Run(ctx, args...) // best effort: a report never fails on this
}

// cloneToCache clones url into the user cache (or refreshes an existing
// clone). Refresh problems do not abort the run; they are returned as
// warnings so the caller can surface them in the report.
func cloneToCache(ctx context.Context, url string, env Env, quiet bool) (string, []string, error) {
	base := env.Getenv("AIBLAME_CACHE_DIR")
	if base == "" {
		c, err := os.UserCacheDir()
		if err != nil {
			c = os.TempDir()
		}
		base = filepath.Join(c, "aiblame", "repos")
	}
	dir := filepath.Join(base, cacheName(url))
	if err := os.MkdirAll(base, 0o755); err != nil {
		return "", nil, err
	}
	// Serialise concurrent invocations against the same cache entry.
	unlock, err := lockPath(ctx, dir+".lock")
	if err != nil {
		return "", nil, err
	}
	defer unlock()
	if _, err := os.Stat(filepath.Join(dir, ".git")); err == nil {
		if !quiet {
			fmt.Fprintf(env.Stderr, "updating cached clone %s\n", dir)
		}
		r := &gitx.Runner{Dir: dir}
		if _, err := r.Run(ctx, "fetch", "--quiet", "--prune", "origin"); err != nil {
			return dir, []string{fmt.Sprintf("fetch of %s failed (%v); results use the cached history in %s", url, shortErr(err), dir)}, nil
		}
		if out, err := r.Run(ctx, "symbolic-ref", "-q", "refs/remotes/origin/HEAD"); err == nil {
			ref := strings.TrimSpace(string(out))
			// The cache is a throwaway mirror; move the checkout to the remote head.
			if _, err := r.Run(ctx, "reset", "--hard", "--quiet", ref); err != nil {
				return dir, []string{fmt.Sprintf("could not move cached clone to %s (%v); results may be stale", ref, shortErr(err))}, nil
			}
		}
		fetchSidecars(ctx, r)
		return dir, nil, nil
	}
	if !quiet {
		fmt.Fprintf(env.Stderr, "cloning %s into %s …\n", url, dir)
	}
	r := &gitx.Runner{Dir: base}
	// "--" keeps a URL that starts with '-' from being read as an option, and
	// the ext:: transport (arbitrary command execution) is disabled outright.
	if _, err := r.Run(ctx, "-c", "protocol.ext.allow=never", "clone", "--quiet", "--", url, dir); err != nil {
		return "", nil, err
	}
	fetchSidecars(ctx, &gitx.Runner{Dir: dir})
	return dir, nil, nil
}

// cacheName turns a URL into a single safe path segment: a readable slug
// plus a short hash so distinct URLs never collide or escape the cache dir.
func cacheName(url string) string {
	slug := strings.NewReplacer("https://", "", "http://", "", "ssh://", "", "git://", "", "git@", "").Replace(url)
	slug = strings.TrimSuffix(slug, ".git")
	var b strings.Builder
	for _, r := range slug {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '.', r == '-':
			b.WriteRune(r)
		default:
			b.WriteByte('_')
		}
	}
	s := strings.Trim(b.String(), "_.")
	if len(s) > 80 {
		s = s[:80]
	}
	sum := sha256.Sum256([]byte(url))
	return s + "-" + hex.EncodeToString(sum[:4])
}

// lockPath acquires an exclusive lock file with O_EXCL, waiting up to a
// minute for a concurrent holder and reclaiming locks older than 15 minutes
// (a crashed process). It works the same on every OS, unlike flock.
func lockPath(ctx context.Context, path string) (func(), error) {
	deadline := time.Now().Add(60 * time.Second)
	for {
		f, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o644)
		if err == nil {
			fmt.Fprintf(f, "%d\n", os.Getpid())
			f.Close()
			return func() { os.Remove(path) }, nil
		}
		if !errors.Is(err, os.ErrExist) {
			return nil, err
		}
		if st, serr := os.Stat(path); serr == nil && time.Since(st.ModTime()) > 15*time.Minute {
			os.Remove(path)
			continue
		}
		if time.Now().After(deadline) {
			return nil, fmt.Errorf("another aiblame is updating %s (remove %s if that is not the case)", strings.TrimSuffix(path, ".lock"), path)
		}
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(200 * time.Millisecond):
		}
	}
}

func shortErr(err error) string {
	s := err.Error()
	if i := strings.IndexByte(s, '\n'); i > 0 {
		s = s[:i]
	}
	return s
}

// buildAnalyzer wires config + flags into a stats.Analyzer.
func buildAnalyzer(c *common, r *gitx.Runner, env Env, blame bool) (*stats.Analyzer, *config.Config, error) {
	cfgPath := c.configPath
	if cfgPath == "" {
		cfgPath = filepath.Join(r.Dir, config.FileName)
	}
	cfg, err := config.Load(cfgPath)
	if err != nil {
		return nil, nil, err
	}
	opts := stats.Options{
		Rev:              c.rev,
		Since:            c.since,
		Until:            c.until,
		Blame:            blame,
		IgnoreWhitespace: c.ignoreWS,
		NoIgnoreRevs:     c.noIgnoreRevs,
		Include:          append(append([]string{}, cfg.Paths.Include...), c.includes...),
		Exclude:          append(append([]string{}, cfg.Paths.Exclude...), c.excludes...),
		DefaultExcludes:  !c.noDefaultExcludes && cfg.Paths.DefaultExcludesEnabled(),
		MaxFileSize:      c.maxFileSize,
		Jobs:             c.jobs,
		IncludeMerges:    c.includeMerges,
		Provenance:       !c.noProvenance && cfg.Detect.ProvenanceEnabled(),
		NoAuthors:        c.noAuthors,
		Top:              c.top,
		PathDepth:        c.depth,
		Version:          Version,
	}
	if opts.MaxFileSize == 0 {
		opts.MaxFileSize = cfg.Paths.MaxFileSize
	}
	if blame && !c.quiet && isTTY(env.Stderr) {
		opts.Progress = progressPrinter(env.Stderr)
	}
	copts := cfg.ClassifierOptions()
	return &stats.Analyzer{
		Runner:      r,
		Classifier:  attrib.NewClassifier(copts),
		Opts:        opts,
		ExtraAgents: copts.ExtraAgents,
	}, cfg, nil
}

func progressPrinter(w io.Writer) func(done, total int) {
	last := -1
	return func(done, total int) {
		step := total / 100
		if step < 1 {
			step = 1
		}
		if done != total && done/step == last {
			return
		}
		last = done / step
		if done == total {
			fmt.Fprint(w, "\r\x1b[K")
			return
		}
		fmt.Fprintf(w, "\r\x1b[K  blaming %d/%d files…", done, total)
	}
}

// openOutput returns the writer for -o.
func openOutput(path string, env Env) (io.Writer, func() error, error) {
	if path == "" || path == "-" {
		return env.Stdout, func() error { return nil }, nil
	}
	if dir := filepath.Dir(path); dir != "" {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return nil, nil, err
		}
	}
	f, err := os.Create(path)
	if err != nil {
		return nil, nil, err
	}
	return f, f.Close, nil
}
