package gitx

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
)

// RefExists reports whether a fully qualified ref (refs/notes/ai,
// refs/heads/x) exists in the repository.
func (r *Runner) RefExists(ctx context.Context, ref string) bool {
	if ref == "" || strings.HasPrefix(ref, "-") {
		return false
	}
	_, err := r.Run(ctx, "rev-parse", "--verify", "--quiet", "--end-of-options", ref)
	return err == nil
}

// NotesList returns commit -> note object id for the notes ref (the short
// form, e.g. "ai" for refs/notes/ai). A missing ref yields an empty map.
func (r *Runner) NotesList(ctx context.Context, ref string) (map[string]string, error) {
	if ref == "" || strings.HasPrefix(ref, "-") {
		return nil, fmt.Errorf("gitx: invalid notes ref %q", ref)
	}
	if !r.RefExists(ctx, "refs/notes/"+ref) {
		return map[string]string{}, nil
	}
	out, err := r.Run(ctx, "notes", "--ref="+ref, "list")
	if err != nil {
		return nil, err
	}
	notes := map[string]string{}
	sc := bufio.NewScanner(bytes.NewReader(out))
	for sc.Scan() {
		f := strings.Fields(sc.Text())
		if len(f) == 2 {
			notes[f[1]] = f[0]
		}
	}
	return notes, sc.Err()
}

// Ref is one entry from for-each-ref.
type Ref struct {
	Name   string // full ref name
	Object string // object id
	Type   string // commit | blob | tree | tag
}

// ForEachRef lists refs under the given prefixes (e.g. "refs/entire/",
// "refs/ai/authorship/").
func (r *Runner) ForEachRef(ctx context.Context, prefixes ...string) ([]Ref, error) {
	args := []string{"for-each-ref", "--format=%(objectname) %(objecttype) %(refname)"}
	for _, p := range prefixes {
		if p == "" || strings.HasPrefix(p, "-") {
			return nil, fmt.Errorf("gitx: invalid ref prefix %q", p)
		}
		args = append(args, p)
	}
	out, err := r.Run(ctx, args...)
	if err != nil {
		return nil, err
	}
	var refs []Ref
	sc := bufio.NewScanner(bytes.NewReader(out))
	for sc.Scan() {
		f := strings.Fields(sc.Text())
		if len(f) == 3 {
			refs = append(refs, Ref{Object: f[0], Type: f[1], Name: f[2]})
		}
	}
	return refs, sc.Err()
}

// CatFileBatch reads many objects in one git process. specs may be object
// ids or revision expressions such as "refs/notes/ai:path" or
// "<commit>:dir/file". Missing objects are absent from the result rather
// than an error, so callers can probe several candidate locations.
func (r *Runner) CatFileBatch(ctx context.Context, specs []string) (map[string][]byte, error) {
	res := make(map[string][]byte, len(specs))
	if len(specs) == 0 {
		return res, nil
	}
	git := r.Git
	if git == "" {
		git = "git"
	}
	cmd := exec.CommandContext(ctx, git, "cat-file", "--batch")
	cmd.Dir = r.Dir
	cmd.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0", "LC_ALL=C")
	var in bytes.Buffer
	for _, s := range specs {
		if s == "" || strings.HasPrefix(s, "-") || strings.ContainsAny(s, "\n\r") {
			return nil, fmt.Errorf("gitx: invalid object spec %q", s)
		}
		in.WriteString(s)
		in.WriteByte('\n')
	}
	cmd.Stdin = &in
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		msg := strings.TrimSpace(stderr.String())
		if msg == "" {
			msg = err.Error()
		}
		return nil, &Error{Args: []string{"cat-file", "--batch"}, Msg: msg, Err: err}
	}
	// Output: "<oid> <type> <size>\n<size bytes>\n" per object, or
	// "<spec> missing\n". Records come back in input order, so results are
	// keyed by the spec that requested them.
	data := stdout.Bytes()
	pos := 0
	for _, spec := range specs {
		nl := bytes.IndexByte(data[pos:], '\n')
		if nl < 0 {
			break
		}
		header := string(data[pos : pos+nl])
		pos += nl + 1
		f := strings.Fields(header)
		if len(f) < 3 {
			// "<spec> missing" (or "ambiguous")
			continue
		}
		size, err := strconv.Atoi(f[2])
		if err != nil || pos+size > len(data) {
			return nil, fmt.Errorf("gitx: malformed cat-file record %q", header)
		}
		res[spec] = append([]byte(nil), data[pos:pos+size]...)
		pos += size + 1 // trailing newline
	}
	return res, nil
}
