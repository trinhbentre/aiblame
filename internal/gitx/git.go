// Package gitx is a thin, dependency-free wrapper around the git CLI. It
// shells out rather than re-implementing git so that every optimisation in
// the user's git (commit-graph, packfile bitmaps, ignore-revs) applies.
package gitx

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// Runner executes git commands inside a repository.
type Runner struct {
	// Dir is the working directory for git. It should be the repository
	// top-level so that paths are repo-relative.
	Dir string
	// Git is the git executable (default "git").
	Git string
}

// ErrNotRepository is returned when the directory is not inside a git work tree.
var ErrNotRepository = errors.New("not a git repository")

// Open locates the repository containing dir and returns a Runner rooted at
// its top level.
func Open(ctx context.Context, dir string) (*Runner, error) {
	if dir == "" {
		dir = "."
	}
	abs, err := filepath.Abs(dir)
	if err != nil {
		return nil, err
	}
	r := &Runner{Dir: abs, Git: "git"}
	out, err := r.Run(ctx, "rev-parse", "--show-toplevel")
	if err != nil {
		if strings.Contains(err.Error(), "not a git repository") {
			return nil, fmt.Errorf("%w: %s", ErrNotRepository, abs)
		}
		return nil, err
	}
	top := strings.TrimSpace(string(out))
	if top == "" {
		// Bare repository: --show-toplevel prints nothing. Use git dir.
		out, err = r.Run(ctx, "rev-parse", "--git-dir")
		if err != nil {
			return nil, err
		}
		top = strings.TrimSpace(string(out))
		if !filepath.IsAbs(top) {
			top = filepath.Join(abs, top)
		}
	}
	r.Dir = top
	return r, nil
}

// Run executes git with args and returns stdout. Stderr is folded into the
// error message on failure.
func (r *Runner) Run(ctx context.Context, args ...string) ([]byte, error) {
	git := r.Git
	if git == "" {
		git = "git"
	}
	cmd := exec.CommandContext(ctx, git, args...)
	cmd.Dir = r.Dir
	cmd.Env = append(os.Environ(),
		"GIT_TERMINAL_PROMPT=0",
		"LC_ALL=C",
		"GIT_PAGER=cat",
		"PAGER=cat",
	)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		msg := strings.TrimSpace(stderr.String())
		if msg == "" {
			msg = err.Error()
		}
		return stdout.Bytes(), &Error{Args: args, Msg: msg, Err: err}
	}
	return stdout.Bytes(), nil
}

// Error describes a failed git invocation.
type Error struct {
	Args []string
	Msg  string
	Err  error
}

func (e *Error) Error() string {
	return fmt.Sprintf("git %s: %s", strings.Join(e.Args, " "), e.Msg)
}

func (e *Error) Unwrap() error { return e.Err }

// Version returns the git version string, e.g. "2.50.1".
func (r *Runner) Version(ctx context.Context) (string, error) {
	out, err := r.Run(ctx, "version")
	if err != nil {
		return "", err
	}
	s := strings.TrimSpace(string(out))
	s = strings.TrimPrefix(s, "git version ")
	if i := strings.IndexByte(s, ' '); i > 0 {
		s = s[:i]
	}
	return s, nil
}

// ResolveRev returns the full hash for a revision expression.
func (r *Runner) ResolveRev(ctx context.Context, rev string) (string, error) {
	out, err := r.Run(ctx, "rev-parse", "--verify", "--quiet", rev+"^{commit}")
	if err != nil {
		return "", fmt.Errorf("cannot resolve revision %q: %w", rev, err)
	}
	return strings.TrimSpace(string(out)), nil
}

// HasCommits reports whether the repository has at least one commit.
func (r *Runner) HasCommits(ctx context.Context) bool {
	_, err := r.Run(ctx, "rev-parse", "--verify", "--quiet", "HEAD^{commit}")
	return err == nil
}

// FileStat is one --numstat row.
type FileStat struct {
	Path    string
	Added   int
	Deleted int
	Binary  bool
}

// Commit is one entry from git log.
type Commit struct {
	Hash           string
	Parents        []string
	AuthorName     string
	AuthorEmail    string
	AuthorTime     time.Time
	CommitterName  string
	CommitterEmail string
	CommitTime     time.Time
	Message        string
	Files          []FileStat
}

// IsMerge reports whether the commit has more than one parent.
func (c Commit) IsMerge() bool { return len(c.Parents) > 1 }

// Subject is the first line of the message.
func (c Commit) Subject() string {
	s, _, _ := strings.Cut(c.Message, "\n")
	return strings.TrimSpace(s)
}

// LogOptions controls Log.
type LogOptions struct {
	// Rev is the starting revision (default HEAD).
	Rev string
	// Since limits commits by author date ("2025-01-01", "6 months ago"…).
	Since string
	// Until limits commits by author date.
	Until string
	// NumStat requests per-file insertion/deletion counts.
	NumStat bool
	// Paths restricts history to these paths (pathspecs).
	Paths []string
	// FirstParent follows only the first parent of merges.
	FirstParent bool
	// MaxCount limits the number of commits (0 = unlimited).
	MaxCount int
}

const (
	// recordSep / fieldSep are the bytes git emits for %x1e / %x00. They
	// cannot be passed literally in argv, so the format string uses the
	// escaped forms below and we split on the raw bytes afterwards.
	recordSep    = "\x1e"
	fieldSep     = "\x00"
	recordSepFmt = "%x1e"
	fieldSepFmt  = "%x00"
)

// Log returns commits reachable from opts.Rev, newest first.
func (r *Runner) Log(ctx context.Context, opts LogOptions) ([]Commit, error) {
	rev := opts.Rev
	if rev == "" {
		rev = "HEAD"
	}
	// %aN/%aE/%cN/%cE honour .mailmap so contributors with several
	// addresses collapse into one row.
	format := recordSepFmt + strings.Join([]string{"%H", "%P", "%aN", "%aE", "%at", "%cN", "%cE", "%ct", "%B"}, fieldSepFmt) + fieldSepFmt
	args := []string{"log", "--format=" + format, "--date-order"}
	if opts.NumStat {
		args = append(args, "--numstat", "-M", "-C")
	}
	if opts.Since != "" {
		args = append(args, "--since="+opts.Since)
	}
	if opts.Until != "" {
		args = append(args, "--until="+opts.Until)
	}
	if opts.FirstParent {
		args = append(args, "--first-parent")
	}
	if opts.MaxCount > 0 {
		args = append(args, "-n", strconv.Itoa(opts.MaxCount))
	}
	args = append(args, rev)
	if len(opts.Paths) > 0 {
		args = append(args, "--")
		args = append(args, opts.Paths...)
	}
	out, err := r.Run(ctx, args...)
	if err != nil {
		return nil, err
	}
	return ParseLog(out)
}

// ParseLog parses the output produced by Log's format string.
func ParseLog(out []byte) ([]Commit, error) {
	var commits []Commit
	for _, rec := range strings.Split(string(out), recordSep) {
		if strings.TrimSpace(rec) == "" {
			continue
		}
		fields := strings.SplitN(rec, fieldSep, 10)
		if len(fields) < 9 {
			return nil, fmt.Errorf("gitx: malformed log record (%d fields)", len(fields))
		}
		at, _ := strconv.ParseInt(fields[4], 10, 64)
		ct, _ := strconv.ParseInt(fields[7], 10, 64)
		c := Commit{
			Hash:           fields[0],
			Parents:        strings.Fields(fields[1]),
			AuthorName:     fields[2],
			AuthorEmail:    fields[3],
			AuthorTime:     time.Unix(at, 0).UTC(),
			CommitterName:  fields[5],
			CommitterEmail: fields[6],
			CommitTime:     time.Unix(ct, 0).UTC(),
			Message:        strings.TrimRight(fields[8], "\n"),
		}
		if len(fields) >= 10 {
			c.Files = ParseNumStat(fields[9])
		}
		commits = append(commits, c)
	}
	return commits, nil
}

// ParseNumStat parses "--numstat" rows: "added\tdeleted\tpath". Binary files
// show "-\t-". Renames appear as "old => new" or "dir/{old => new}/file".
func ParseNumStat(s string) []FileStat {
	var out []FileStat
	for _, line := range strings.Split(s, "\n") {
		line = strings.TrimRight(line, "\r")
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, "\t", 3)
		if len(parts) != 3 {
			continue
		}
		fs := FileStat{Path: NormalizeRenamePath(parts[2])}
		if parts[0] == "-" || parts[1] == "-" {
			fs.Binary = true
		} else {
			fs.Added, _ = strconv.Atoi(parts[0])
			fs.Deleted, _ = strconv.Atoi(parts[1])
		}
		out = append(out, fs)
	}
	return out
}

// NormalizeRenamePath converts git's rename notation to the new path.
//
//	"a/{b => c}/d.go" -> "a/c/d.go"
//	"old.go => new.go" -> "new.go"
//	"{old => new}.go" -> "new.go"
func NormalizeRenamePath(p string) string {
	if !strings.Contains(p, " => ") {
		return unquotePath(p)
	}
	if i := strings.IndexByte(p, '{'); i >= 0 {
		if j := strings.IndexByte(p[i:], '}'); j >= 0 {
			inner := p[i+1 : i+j]
			_, newPart, _ := strings.Cut(inner, " => ")
			res := p[:i] + newPart + p[i+j+1:]
			res = strings.ReplaceAll(res, "//", "/")
			return unquotePath(res)
		}
	}
	_, newPart, _ := strings.Cut(p, " => ")
	return unquotePath(strings.TrimSpace(newPart))
}

// unquotePath handles git's C-style quoting for unusual file names.
func unquotePath(p string) string {
	if len(p) >= 2 && p[0] == '"' && p[len(p)-1] == '"' {
		if s, err := strconv.Unquote(p); err == nil {
			return s
		}
	}
	return p
}

// ListFiles returns the tracked paths at rev (blobs only, repo-relative,
// forward slashes).
func (r *Runner) ListFiles(ctx context.Context, rev string) ([]string, error) {
	if rev == "" {
		rev = "HEAD"
	}
	out, err := r.Run(ctx, "ls-tree", "-r", "-z", "--name-only", rev)
	if err != nil {
		return nil, err
	}
	var files []string
	for _, p := range bytes.Split(out, []byte{0}) {
		if len(p) == 0 {
			continue
		}
		files = append(files, string(p))
	}
	return files, nil
}

// FileSizes returns the blob size of each tracked file at rev.
func (r *Runner) FileSizes(ctx context.Context, rev string) (map[string]int64, error) {
	if rev == "" {
		rev = "HEAD"
	}
	out, err := r.Run(ctx, "ls-tree", "-r", "-l", "-z", rev)
	if err != nil {
		return nil, err
	}
	sizes := map[string]int64{}
	for _, rec := range bytes.Split(out, []byte{0}) {
		if len(rec) == 0 {
			continue
		}
		// "<mode> <type> <object> <size>\t<path>"
		meta, path, ok := strings.Cut(string(rec), "\t")
		if !ok {
			continue
		}
		f := strings.Fields(meta)
		if len(f) < 4 || f[1] != "blob" {
			continue
		}
		n, err := strconv.ParseInt(f[3], 10, 64)
		if err != nil {
			continue
		}
		sizes[path] = n
	}
	return sizes, nil
}

// GitPath resolves a path inside the git directory (e.g. "hooks"), honouring
// core.hooksPath for the hooks directory.
func (r *Runner) GitPath(ctx context.Context, name string) (string, error) {
	out, err := r.Run(ctx, "rev-parse", "--git-path", name)
	if err != nil {
		return "", err
	}
	p := strings.TrimSpace(string(out))
	if !filepath.IsAbs(p) {
		p = filepath.Join(r.Dir, p)
	}
	return p, nil
}

// HooksDir returns the effective hooks directory (core.hooksPath aware).
func (r *Runner) HooksDir(ctx context.Context) (string, error) {
	out, err := r.Run(ctx, "config", "--get", "core.hooksPath")
	if err == nil {
		p := strings.TrimSpace(string(out))
		if p != "" {
			if strings.HasPrefix(p, "~") {
				if home, herr := os.UserHomeDir(); herr == nil {
					p = filepath.Join(home, strings.TrimPrefix(p, "~"))
				}
			}
			if !filepath.IsAbs(p) {
				p = filepath.Join(r.Dir, p)
			}
			return p, nil
		}
	}
	return r.GitPath(ctx, "hooks")
}

// RemoteURL returns the URL of origin (or "" when absent).
func (r *Runner) RemoteURL(ctx context.Context) string {
	out, err := r.Run(ctx, "remote", "get-url", "origin")
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}
