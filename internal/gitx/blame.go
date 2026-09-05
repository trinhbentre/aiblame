package gitx

import (
	"bufio"
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// BlameLine is one line of a blamed file.
type BlameLine struct {
	Hash    string
	LineNo  int    // final line number (1-based)
	Content string // only populated when BlameOptions.Content is true
}

// BlameResult is the outcome of blaming one file.
type BlameResult struct {
	Path  string
	Lines []BlameLine
	// Counts is the number of lines attributed to each commit hash.
	Counts map[string]int
}

// BlameOptions tunes Blame.
type BlameOptions struct {
	Rev              string
	IgnoreWhitespace bool
	// IgnoreRevsFile is passed as --ignore-revs-file when non-empty.
	IgnoreRevsFile string
	// Content keeps the text of each line (needed for `aiblame blame`).
	Content bool
}

// Blame runs git blame --porcelain on one file.
func (r *Runner) Blame(ctx context.Context, path string, opts BlameOptions) (*BlameResult, error) {
	rev := opts.Rev
	if rev == "" {
		rev = "HEAD"
	}
	args := []string{"blame", "--porcelain"}
	if opts.IgnoreWhitespace {
		args = append(args, "-w")
	}
	if opts.IgnoreRevsFile != "" {
		args = append(args, "--ignore-revs-file", opts.IgnoreRevsFile)
	}
	args = append(args, rev, "--", path)
	out, err := r.Run(ctx, args...)
	if err != nil {
		return nil, err
	}
	res := ParseBlamePorcelain(out, opts.Content)
	res.Path = path
	return res, nil
}

// ParseBlamePorcelain parses `git blame --porcelain` output.
//
// Each line group starts with "<sha> <orig> <final> [<n>]"; header key/value
// lines follow on first occurrence of a commit; the content line starts with
// a tab.
func ParseBlamePorcelain(out []byte, keepContent bool) *BlameResult {
	res := &BlameResult{Counts: map[string]int{}}
	sc := bufio.NewScanner(bytes.NewReader(out))
	sc.Buffer(make([]byte, 1024*1024), 64*1024*1024)
	var cur string
	var curLine int
	expectContent := false
	for sc.Scan() {
		line := sc.Text()
		if expectContent {
			if strings.HasPrefix(line, "\t") {
				bl := BlameLine{Hash: cur, LineNo: curLine}
				if keepContent {
					bl.Content = line[1:]
				}
				res.Lines = append(res.Lines, bl)
				res.Counts[cur]++
				expectContent = false
				continue
			}
			// header key/value line (author, summary, filename, boundary…)
			continue
		}
		if h, n, ok := parseBlameHeader(line); ok {
			cur, curLine = h, n
			expectContent = true
		}
	}
	return res
}

func parseBlameHeader(line string) (hash string, final int, ok bool) {
	if len(line) < 44 || line[40] != ' ' {
		return "", 0, false
	}
	for i := 0; i < 40; i++ {
		c := line[i]
		if !(c >= '0' && c <= '9' || c >= 'a' && c <= 'f') {
			return "", 0, false
		}
	}
	f := strings.Fields(line[41:])
	if len(f) < 2 || len(f) > 3 {
		return "", 0, false
	}
	n, err := strconv.Atoi(f[1])
	if err != nil {
		return "", 0, false
	}
	return line[:40], n, true
}

// IgnoreRevsFile returns the path of .git-blame-ignore-revs at the top level
// if it exists, else "".
func (r *Runner) IgnoreRevsFile() string {
	p := filepath.Join(r.Dir, ".git-blame-ignore-revs")
	if st, err := os.Stat(p); err == nil && !st.IsDir() {
		return p
	}
	return ""
}
