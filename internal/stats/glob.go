package stats

import (
	"regexp"
	"strings"
)

// Matcher matches repo-relative paths against gitignore-style globs.
//
// Rules (a pragmatic subset of gitignore):
//   - "**" matches any number of path segments (including none)
//   - "*" matches within a single segment, "?" matches one character
//   - a pattern without "/" matches the basename in any directory
//   - a trailing "/" matches a directory and everything beneath it
//   - a leading "/" anchors to the repo root
type Matcher struct {
	res []*regexp.Regexp
	raw []string
}

// NewMatcher compiles patterns. Invalid or empty patterns are ignored.
func NewMatcher(patterns []string) *Matcher {
	m := &Matcher{}
	for _, p := range patterns {
		p = strings.TrimSpace(p)
		if p == "" || strings.HasPrefix(p, "#") {
			continue
		}
		if rx := compileGlob(p); rx != nil {
			m.res = append(m.res, rx)
			m.raw = append(m.raw, p)
		}
	}
	return m
}

// Empty reports whether the matcher has no patterns.
func (m *Matcher) Empty() bool { return m == nil || len(m.res) == 0 }

// Match reports whether path (forward slashes, no leading "./") matches any
// pattern.
func (m *Matcher) Match(path string) bool {
	if m == nil {
		return false
	}
	path = strings.TrimPrefix(path, "./")
	for _, rx := range m.res {
		if rx.MatchString(path) {
			return true
		}
	}
	return false
}

// Patterns returns the accepted raw patterns.
func (m *Matcher) Patterns() []string {
	if m == nil {
		return nil
	}
	return append([]string(nil), m.raw...)
}

func compileGlob(p string) *regexp.Regexp {
	dirOnly := strings.HasSuffix(p, "/")
	p = strings.TrimSuffix(p, "/")
	anchored := strings.HasPrefix(p, "/")
	p = strings.TrimPrefix(p, "/")
	if p == "" {
		return nil
	}
	hasSlash := strings.Contains(p, "/")

	var b strings.Builder
	b.WriteString("^")
	if !anchored && !hasSlash {
		// basename match anywhere
		b.WriteString(`(?:.*/)?`)
	} else if !anchored {
		// gitignore: a pattern with a slash in the middle is relative to root
		// unless it starts with **/; we treat it as root-relative.
	}
	i := 0
	for i < len(p) {
		c := p[i]
		switch {
		case strings.HasPrefix(p[i:], "**/"):
			b.WriteString(`(?:.*/)?`)
			i += 3
		case strings.HasPrefix(p[i:], "/**"):
			b.WriteString(`(?:/.*)?`)
			i += 3
		case strings.HasPrefix(p[i:], "**"):
			b.WriteString(`.*`)
			i += 2
		case c == '*':
			b.WriteString(`[^/]*`)
			i++
		case c == '?':
			b.WriteString(`[^/]`)
			i++
		case c == '[':
			// character class: copy through the closing bracket
			j := strings.IndexByte(p[i:], ']')
			if j < 0 {
				b.WriteString(regexp.QuoteMeta(string(c)))
				i++
			} else {
				class := p[i : i+j+1]
				if strings.HasPrefix(class, "[!") {
					class = "[^" + class[2:]
				}
				b.WriteString(class)
				i += j + 1
			}
		default:
			b.WriteString(regexp.QuoteMeta(string(c)))
			i++
		}
	}
	if dirOnly {
		b.WriteString(`/.*`)
	} else {
		// a pattern naming a directory should also match its contents
		b.WriteString(`(?:/.*)?`)
	}
	b.WriteString("$")
	rx, err := regexp.Compile(b.String())
	if err != nil {
		return nil
	}
	return rx
}

// DefaultExcludes are paths that inflate line counts without representing
// authored code: lockfiles, vendored trees, minified bundles and generated
// artifacts.
var DefaultExcludes = []string{
	// lockfiles
	"package-lock.json", "yarn.lock", "pnpm-lock.yaml", "bun.lockb", "bun.lock",
	"npm-shrinkwrap.json", "Cargo.lock", "go.sum", "poetry.lock", "Pipfile.lock",
	"uv.lock", "pdm.lock", "Gemfile.lock", "composer.lock", "mix.lock",
	"packages.lock.json", "pubspec.lock", "flake.lock", "Podfile.lock",
	"Package.resolved", "gradle.lockfile", "*.lockfile", "deno.lock",
	// vendored / dependency trees
	"vendor/", "node_modules/", "third_party/", "thirdparty/", "external/",
	".yarn/", "bower_components/", "jspm_packages/",
	// build output
	"dist/", "build/", "out/", "target/", ".next/", ".nuxt/", ".output/",
	"__pycache__/", "*.pyc",
	// minified / bundled
	"*.min.js", "*.min.css", "*.bundle.js", "*.map", "*.chunk.js",
	// generated code
	"*.pb.go", "*.pb.cc", "*.pb.h", "*_pb2.py", "*_pb2_grpc.py", "*.pb.ts",
	"*_generated.go", "*.generated.*", "*.gen.go", "*.g.dart", "*.freezed.dart",
	"*.g.cs", "*.designer.cs", "*.Designer.cs", "zz_generated*",
	"*_string.go", "*.mock.go", "mocks/", "__generated__/", "generated/",
	"*.snap", "__snapshots__/", "*.golden", "testdata/**/*.golden",
	"swagger.json", "openapi.json", "schema.graphql.json",
	// data / assets that are text but not code
	"*.svg", "*.csv", "*.tsv", "*.ipynb", "*.ndjson", "*.jsonl",
	"*.po", "*.pot", "*.mo", "*.xlf", "*.xliff", "*.resx",
	"*.sql.gz", "*.dump",
	// documentation sites often check in built HTML
	"docs/_build/", "site/", "public/build/",
}

// BinaryExtensions are skipped before blame even when git did not flag the
// file as binary in any numstat row.
var BinaryExtensions = map[string]bool{
	".png": true, ".jpg": true, ".jpeg": true, ".gif": true, ".webp": true, ".ico": true, ".bmp": true, ".tiff": true, ".tif": true, ".avif": true, ".heic": true, ".psd": true, ".ai": true,
	".pdf": true, ".doc": true, ".docx": true, ".xls": true, ".xlsx": true, ".ppt": true, ".pptx": true, ".odt": true,
	".zip": true, ".gz": true, ".tgz": true, ".bz2": true, ".xz": true, ".zst": true, ".7z": true, ".rar": true, ".tar": true, ".jar": true, ".war": true, ".whl": true, ".egg": true, ".nupkg": true,
	".woff": true, ".woff2": true, ".ttf": true, ".otf": true, ".eot": true,
	".mp3": true, ".mp4": true, ".wav": true, ".ogg": true, ".flac": true, ".mov": true, ".avi": true, ".mkv": true, ".webm": true, ".m4a": true,
	".exe": true, ".dll": true, ".so": true, ".dylib": true, ".a": true, ".o": true, ".obj": true, ".lib": true, ".bin": true, ".class": true, ".wasm": true, ".pyd": true,
	".sqlite": true, ".db": true, ".sqlite3": true, ".realm": true, ".parquet": true, ".arrow": true, ".feather": true, ".npy": true, ".npz": true, ".pkl": true, ".pickle": true, ".h5": true, ".hdf5": true, ".onnx": true, ".pt": true, ".pth": true, ".safetensors": true, ".gguf": true,
	".ds_store": true, ".icns": true, ".dmg": true, ".pkg": true, ".deb": true, ".rpm": true, ".apk": true, ".ipa": true, ".aab": true,
	".unitypackage": true, ".fbx": true, ".blend": true, ".glb": true, ".gltf": true, ".uasset": true, ".umap": true,
}
