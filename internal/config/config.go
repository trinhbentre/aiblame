// Package config loads the optional .aiblame.toml file.
package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"

	"github.com/BurntSushi/toml"

	"github.com/trinhbentre/aiblame/internal/attrib"
)

// FileName is the config file looked up at the repository root.
const FileName = ".aiblame.toml"

// Config is the parsed configuration.
type Config struct {
	Paths  Paths  `toml:"paths"`
	Detect Detect `toml:"detect"`
	Check  Check  `toml:"check"`
	Badge  Badge  `toml:"badge"`
}

// Paths controls which files are counted.
type Paths struct {
	// Include restricts analysis to matching globs (empty = everything).
	Include []string `toml:"include"`
	// Exclude removes matching globs.
	Exclude []string `toml:"exclude"`
	// UseDefaultExcludes keeps the built-in lockfile/vendor/minified list
	// (default true; set to false to count everything).
	UseDefaultExcludes *bool `toml:"use_default_excludes"`
	// MaxFileSize skips blobs larger than this many bytes (default 1 MiB).
	MaxFileSize int64 `toml:"max_file_size"`
}

// Detect extends the built-in identity tables.
type Detect struct {
	Agents           []AgentDef `toml:"agents"`
	Bots             []AgentDef `toml:"bots"`
	ExtraTrailerKeys []string   `toml:"extra_trailer_keys"`
	MessageMarkers   []string   `toml:"message_markers"`
	// DisableMessageMarkers turns off free-text detection ("Generated with…").
	DisableMessageMarkers bool `toml:"disable_message_markers"`
	// CommitterEvidence treats an AI committer as evidence even when the
	// author is human.
	CommitterEvidence bool `toml:"committer_evidence"`
}

// AgentDef is a user-defined identity.
type AgentDef struct {
	Name          string   `toml:"name"`
	Vendor        string   `toml:"vendor"`
	Emails        []string `toml:"emails"`
	EmailSuffixes []string `toml:"email_suffixes"`
	Names         []string `toml:"names"`
	NamePattern   string   `toml:"name_pattern"`
	TrailerEmail  string   `toml:"trailer_email"`
}

// Check holds default thresholds for `aiblame check`.
type Check struct {
	Max                 *float64 `toml:"max"`
	Min                 *float64 `toml:"min"`
	Metric              string   `toml:"metric"`
	ForbidAgentAuthored bool     `toml:"forbid_agent_authored"`
	RequireTrailer      string   `toml:"require_trailer"`
}

// Badge holds defaults for `aiblame badge`.
type Badge struct {
	Label string `toml:"label"`
	Color string `toml:"color"`
	Style string `toml:"style"`
}

// Load reads the config file at path. A missing file yields an empty config
// and no error.
func Load(path string) (*Config, error) {
	var cfg Config
	b, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return &cfg, nil
		}
		return nil, err
	}
	md, err := toml.Decode(string(b), &cfg)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	if undec := md.Undecoded(); len(undec) > 0 {
		return nil, fmt.Errorf("%s: unknown key %q", path, undec[0].String())
	}
	if err := cfg.validate(); err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	return &cfg, nil
}

// LoadFromRepo loads <repoRoot>/.aiblame.toml.
func LoadFromRepo(repoRoot string) (*Config, error) {
	return Load(filepath.Join(repoRoot, FileName))
}

func (c *Config) validate() error {
	for _, a := range append(append([]AgentDef{}, c.Detect.Agents...), c.Detect.Bots...) {
		if a.Name == "" {
			return errors.New("detect.agents/bots entries need a name")
		}
		if a.NamePattern != "" {
			if _, err := regexp.Compile(a.NamePattern); err != nil {
				return fmt.Errorf("name_pattern for %q: %w", a.Name, err)
			}
		}
	}
	for _, m := range c.Detect.MessageMarkers {
		if _, err := regexp.Compile(m); err != nil {
			return fmt.Errorf("message_markers %q: %w", m, err)
		}
	}
	switch c.Check.Metric {
	case "", "lines", "churn", "commits":
	default:
		return fmt.Errorf("check.metric must be lines, churn or commits (got %q)", c.Check.Metric)
	}
	switch c.Check.RequireTrailer {
	case "", "assisted-by", "co-authored-by", "generated-by", "coding-agent":
	default:
		return fmt.Errorf("check.require_trailer %q is not a known convention", c.Check.RequireTrailer)
	}
	return nil
}

// DefaultExcludesEnabled resolves the tri-state use_default_excludes option.
func (p Paths) DefaultExcludesEnabled() bool {
	return p.UseDefaultExcludes == nil || *p.UseDefaultExcludes
}

// ClassifierOptions converts the config into attrib.Options.
func (c *Config) ClassifierOptions() attrib.Options {
	opts := attrib.Options{
		ExtraTrailerKeys:         c.Detect.ExtraTrailerKeys,
		MessageMarkers:           c.Detect.MessageMarkers,
		DisableMessageMarkers:    c.Detect.DisableMessageMarkers,
		TreatCommitterAsEvidence: c.Detect.CommitterEvidence,
	}
	for _, a := range c.Detect.Agents {
		opts.ExtraAgents = append(opts.ExtraAgents, toIdentity(a, false))
	}
	for _, b := range c.Detect.Bots {
		opts.ExtraBots = append(opts.ExtraBots, toIdentity(b, true))
	}
	return opts
}

func toIdentity(a AgentDef, bot bool) attrib.Identity {
	id := attrib.Identity{
		Name:          a.Name,
		Vendor:        a.Vendor,
		Emails:        a.Emails,
		EmailSuffixes: a.EmailSuffixes,
		Names:         a.Names,
		TrailerEmail:  a.TrailerEmail,
		IsBot:         bot,
	}
	if a.NamePattern != "" {
		id.NamePattern = regexp.MustCompile(a.NamePattern) // validated in Load
	}
	return id
}

// Example is a documented starter config written by `aiblame init`.
const Example = `# aiblame configuration — https://github.com/trinhbentre/aiblame
# Every key is optional. Delete what you do not need.

[paths]
# Only count these globs (gitignore-style; empty = whole repo).
include = []
# Skip these globs in addition to the built-in defaults
# (lockfiles, vendor/, node_modules/, minified and generated files).
exclude = ["docs/generated/**"]
# Set to false to count lockfiles, vendored and generated code too.
use_default_excludes = true
# Skip blobs bigger than this (bytes). Default 1 MiB.
max_file_size = 1048576

[detect]
# Extra trailer keys that mean "an AI helped", e.g. an in-house convention.
extra_trailer_keys = []
# Extra regexes matched against the whole commit message. A named group
# (?P<agent>…) becomes the agent name.
message_markers = []
# Turn off free-text detection ("Generated with Claude Code" etc.).
disable_message_markers = false
# Count an AI *committer* as evidence even when the author is human.
committer_evidence = false

# In-house agents. Matched case-insensitively against author and trailer
# identities.
# [[detect.agents]]
# name = "Acme Copilot"
# emails = ["acme-agent@acme.example"]
# names = ["acme-agent"]
# trailer_email = "acme-agent@acme.example"

# Non-AI automation you want reported as "bot" instead of "human".
# [[detect.bots]]
# name = "Acme CI"
# emails = ["ci@acme.example"]

[check]
# Fail ` + "`aiblame check`" + ` when the AI share exceeds this percentage.
# max = 60
# metric = "lines"   # lines | churn | commits
# forbid_agent_authored = false
# require_trailer = "assisted-by"   # enforce one convention

[badge]
label = "AI-written"
color = "7c3aed"
style = "flat"
`
