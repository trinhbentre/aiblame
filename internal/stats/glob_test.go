package stats

import "testing"

func TestMatcher(t *testing.T) {
	m := NewMatcher([]string{"vendor/", "*.min.js", "docs/**/*.md", "/README.md", "**/generated/**", "src/*.go", "*.pb.go", "[!a]bc.txt"})
	yes := []string{
		"vendor/x.go", "vendor/a/b/c.js", "lib/app.min.js", "app.min.js",
		"docs/a.md", "docs/a/b/c.md", "README.md", "generated/x", "a/generated/b/c",
		"src/main.go", "pkg/api/v1/api.pb.go", "xbc.txt",
	}
	no := []string{
		"vendorx/x.go", "app.js", "src/README.md", "docs/a.txt", "src/sub/main.go", "abc.txt", "myvendor/x",
	}
	for _, p := range yes {
		if !m.Match(p) {
			t.Errorf("expected match: %s", p)
		}
	}
	for _, p := range no {
		if m.Match(p) {
			t.Errorf("unexpected match: %s", p)
		}
	}
	if NewMatcher(nil).Match("anything") {
		t.Error("empty matcher matched")
	}
	var nilM *Matcher
	if nilM.Match("x") || !nilM.Empty() {
		t.Error("nil matcher misbehaved")
	}
}

func TestDefaultExcludesCoverLockfiles(t *testing.T) {
	m := NewMatcher(DefaultExcludes)
	for _, p := range []string{"package-lock.json", "web/package-lock.json", "go.sum", "node_modules/a/b.js", "dist/app.js", "api/v1/x.pb.go", "a/b/c.min.css", "vendor/github.com/x/y.go"} {
		if !m.Match(p) {
			t.Errorf("default excludes should match %s", p)
		}
	}
	for _, p := range []string{"go.mod", "main.go", "src/app.css", "README.md", "Makefile", "internal/stats/glob.go"} {
		if m.Match(p) {
			t.Errorf("default excludes should not match %s", p)
		}
	}
}
