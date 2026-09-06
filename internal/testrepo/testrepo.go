// Package testrepo builds throwaway git repositories for tests.
package testrepo

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// Repo is a scripted git repository in a temp dir.
type Repo struct {
	T    testing.TB
	Dir  string
	tick int
}

// New initialises an empty repository with deterministic identity settings.
func New(t testing.TB) *Repo {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not installed")
	}
	dir := t.TempDir()
	r := &Repo{T: t, Dir: dir}
	r.Git("init", "-q", "-b", "main")
	r.Git("config", "user.name", "Test Human")
	r.Git("config", "user.email", "human@example.com")
	r.Git("config", "commit.gpgsign", "false")
	r.Git("config", "core.autocrlf", "false")
	return r
}

// Git runs a git command in the repo and fails the test on error.
func (r *Repo) Git(args ...string) string {
	r.T.Helper()
	return r.git(nil, args...)
}

func (r *Repo) git(extraEnv []string, args ...string) string {
	r.T.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = r.Dir
	cmd.Env = append(os.Environ(),
		"GIT_COMMITTER_NAME=Test Human",
		"GIT_COMMITTER_EMAIL=human@example.com",
	)
	cmd.Env = append(cmd.Env, extraEnv...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		r.T.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
	}
	return string(out)
}

// GitStdin runs a git command with stdin and returns its stdout.
func (r *Repo) GitStdin(stdin string, args ...string) string {
	r.T.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = r.Dir
	cmd.Env = append(os.Environ(),
		"GIT_COMMITTER_NAME=Test Human",
		"GIT_COMMITTER_EMAIL=human@example.com",
		"GIT_AUTHOR_NAME=Test Human",
		"GIT_AUTHOR_EMAIL=human@example.com",
	)
	cmd.Stdin = strings.NewReader(stdin)
	out, err := cmd.CombinedOutput()
	if err != nil {
		r.T.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
	}
	return string(out)
}

// Blob writes content as a blob object and returns its id.
func (r *Repo) Blob(content string) string {
	r.T.Helper()
	return strings.TrimSpace(r.GitStdin(content, "hash-object", "-w", "--stdin"))
}

// Tree writes a tree from repo-relative paths (forward slashes) to file
// contents, creating sub-trees as needed, and returns the tree id.
func (r *Repo) Tree(files map[string]string) string {
	r.T.Helper()
	type node struct {
		files map[string]string
		dirs  map[string]*node
	}
	root := &node{files: map[string]string{}, dirs: map[string]*node{}}
	for p, content := range files {
		parts := strings.Split(p, "/")
		n := root
		for _, d := range parts[:len(parts)-1] {
			child := n.dirs[d]
			if child == nil {
				child = &node{files: map[string]string{}, dirs: map[string]*node{}}
				n.dirs[d] = child
			}
			n = child
		}
		n.files[parts[len(parts)-1]] = content
	}
	var write func(n *node) string
	write = func(n *node) string {
		var b strings.Builder
		for name, content := range n.files {
			b.WriteString("100644 blob " + r.Blob(content) + "\t" + name + "\n")
		}
		for name, child := range n.dirs {
			b.WriteString("040000 tree " + write(child) + "\t" + name + "\n")
		}
		return strings.TrimSpace(r.GitStdin(b.String(), "mktree"))
	}
	return write(root)
}

// CommitTree creates a root commit from a tree and returns its id. It does
// not move any branch; pair it with update-ref.
func (r *Repo) CommitTree(tree, message string) string {
	r.T.Helper()
	r.tick++
	ts := CommitTime(r.tick).Format(time.RFC3339)
	cmd := exec.Command("git", "commit-tree", tree, "-m", message)
	cmd.Dir = r.Dir
	cmd.Env = append(os.Environ(),
		"GIT_COMMITTER_NAME=Test Human", "GIT_COMMITTER_EMAIL=human@example.com",
		"GIT_AUTHOR_NAME=Test Human", "GIT_AUTHOR_EMAIL=human@example.com",
		"GIT_AUTHOR_DATE="+ts, "GIT_COMMITTER_DATE="+ts,
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		r.T.Fatalf("git commit-tree: %v\n%s", err, out)
	}
	return strings.TrimSpace(string(out))
}

// CommitTime returns the deterministic timestamp of the n-th commit (1-based):
// 2026-01-01T00:00Z plus n hours.
func CommitTime(n int) time.Time {
	return time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC).Add(time.Duration(n) * time.Hour)
}

// Write writes a file (creating directories) relative to the repo root.
func (r *Repo) Write(rel, content string) {
	r.T.Helper()
	p := filepath.Join(r.Dir, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		r.T.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		r.T.Fatal(err)
	}
}

// CommitOpts customises a commit.
type CommitOpts struct {
	AuthorName  string
	AuthorEmail string
	Message     string
}

// Commit stages everything and commits with the given options. It returns
// the full hash.
func (r *Repo) Commit(o CommitOpts) string {
	r.T.Helper()
	r.Git("add", "-A")
	args := []string{"commit", "-q", "--allow-empty", "-m", o.Message}
	if o.AuthorName != "" || o.AuthorEmail != "" {
		name := o.AuthorName
		if name == "" {
			name = "Test Human"
		}
		email := o.AuthorEmail
		if email == "" {
			email = "human@example.com"
		}
		args = append(args, "--author="+name+" <"+email+">")
	}
	r.tick++
	ts := CommitTime(r.tick).Format(time.RFC3339)
	r.git([]string{"GIT_AUTHOR_DATE=" + ts, "GIT_COMMITTER_DATE=" + ts}, args...)
	return strings.TrimSpace(r.Git("rev-parse", "HEAD"))
}

// Lines builds n numbered lines with a prefix.
func Lines(prefix string, n int) string {
	var b strings.Builder
	for i := 1; i <= n; i++ {
		b.WriteString(prefix)
		b.WriteString(" line ")
		b.WriteString(itoa(i))
		b.WriteString("\n")
	}
	return b.String()
}

func itoa(i int) string {
	if i == 0 {
		return "0"
	}
	var buf [20]byte
	pos := len(buf)
	for i > 0 {
		pos--
		buf[pos] = byte('0' + i%10)
		i /= 10
	}
	return string(buf[pos:])
}

// Standard creates a repository with a representative mix of commits and
// returns it. The layout is used by several packages' tests:
//
//	human   : README.md (5 lines), src/human.go (20 lines)
//	claude  : src/ai.go (30 lines) via Co-Authored-By trailer
//	codex   : src/codex.go (10 lines) via Co-authored-by trailer
//	kernel  : src/kernel.c (8 lines) via Assisted-by trailer
//	devin   : bot author, src/devin.py (12 lines)
//	depbot  : dependabot[bot] bumps package-lock.json (100 lines)
//	binary  : assets/logo.png (binary)
func Standard(t testing.TB) *Repo {
	t.Helper()
	r := New(t)
	r.Write("README.md", Lines("readme", 5))
	r.Write("src/human.go", Lines("human", 20))
	r.Commit(CommitOpts{Message: "Initial human commit"})

	r.Write("src/ai.go", Lines("ai", 30))
	r.Commit(CommitOpts{Message: "Add AI module\n\n🤖 Generated with [Claude Code](https://claude.com/claude-code)\n\nCo-Authored-By: Claude <noreply@anthropic.com>"})

	r.Write("src/codex.go", Lines("codex", 10))
	r.Commit(CommitOpts{Message: "Add codex module\n\nCo-authored-by: Codex <noreply@openai.com>"})

	r.Write("src/kernel.c", Lines("kernel", 8))
	r.Commit(CommitOpts{Message: "kernel: add driver\n\nAssisted-by: Claude Code (claude-opus-4-6)\nSigned-off-by: Test Human <human@example.com>"})

	r.Write("src/devin.py", Lines("devin", 12))
	r.Commit(CommitOpts{AuthorName: "devin-ai-integration[bot]", AuthorEmail: "158243242+devin-ai-integration[bot]@users.noreply.github.com", Message: "Implement feature"})

	r.Write("package-lock.json", Lines("lock", 100))
	r.Commit(CommitOpts{AuthorName: "dependabot[bot]", AuthorEmail: "49699333+dependabot[bot]@users.noreply.github.com", Message: "Bump deps"})

	r.Write("assets/logo.png", "\x89PNG\r\n\x1a\n\x00\x00binary\x00data")
	r.Commit(CommitOpts{Message: "Add logo"})
	return r
}
