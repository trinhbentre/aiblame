// Package policy holds named presets for `aiblame check` that mirror the
// AI-contribution rules open source projects have published. Each preset is
// a plain set of check flags; the Source URL is the document it was read
// from, last checked 2026-09-06. Projects change their minds — Kubernetes
// went from "use Assisted-by" to "no trailers at all" in 2026 — so treat
// these as starting points and pass explicit flags when a project's page
// says otherwise.
package policy

import (
	"sort"
	"strings"
)

// Policy is a preset of check rules.
type Policy struct {
	Name        string
	Description string
	Source      string
	// RequireTrailer: an AI-assisted commit must carry at least one of these
	// conventions.
	RequireTrailer []string
	// ForbidTrailers: an AI-touched commit must not use these conventions.
	ForbidTrailers []string
	// ForbidAgentAuthored: the commit author must be a person.
	ForbidAgentAuthored bool
	// ForbidAgentSignoff: no Signed-off-by may name an AI identity (DCO).
	ForbidAgentSignoff bool
}

var presets = []Policy{
	{
		Name:                "kernel",
		Description:         "Linux kernel: disclose with Assisted-by (7.3+: \"Assisted-by: LLM [tools]\"); Co-authored-by is for people; an AI must never author or sign off a commit",
		Source:              "https://www.kernel.org/doc/html/latest/process/coding-assistants.html",
		RequireTrailer:      []string{"assisted-by"},
		ForbidTrailers:      []string{"co-authored-by", "co-developed-by"},
		ForbidAgentAuthored: true,
		ForbidAgentSignoff:  true,
	},
	{
		Name:                "zephyr",
		Description:         "Zephyr: same rules as the kernel with the \"Assisted-by: Agent:model [tools]\" form",
		Source:              "https://docs.zephyrproject.org/latest/contribute/guidelines.html",
		RequireTrailer:      []string{"assisted-by"},
		ForbidTrailers:      []string{"co-authored-by", "co-developed-by"},
		ForbidAgentAuthored: true,
		ForbidAgentSignoff:  true,
	},
	{
		Name:                "mesa",
		Description:         "Mesa: Assisted-by (AI helped) or Generated-by (almost all generated); Co-authored-by is reserved for human co-authors; no autonomous tools",
		Source:              "https://docs.mesa3d.org/submittingpatches.html",
		RequireTrailer:      []string{"assisted-by", "generated-by"},
		ForbidTrailers:      []string{"co-authored-by"},
		ForbidAgentAuthored: true,
		ForbidAgentSignoff:  true,
	},
	{
		Name:           "openinfra",
		Description:    "OpenInfra Foundation projects: Assisted-by or Generated-by",
		Source:         "https://openinfra.org/legal/ai-policy",
		RequireTrailer: []string{"assisted-by", "generated-by"},
		ForbidTrailers: []string{"co-authored-by"},
	},
	{
		Name:                "llvm",
		Description:         "LLVM: label tool-generated content with an Assisted-by trailer; a human must be in the loop",
		Source:              "https://llvm.org/docs/AIToolPolicy.html",
		RequireTrailer:      []string{"assisted-by"},
		ForbidAgentAuthored: true,
	},
	{
		Name:           "asf",
		Description:    "Apache Software Foundation: mark generated content with Generated-by",
		Source:         "https://www.apache.org/legal/generative-tooling.html",
		RequireTrailer: []string{"generated-by"},
	},
	{
		Name:           "fedora",
		Description:    "Fedora: Assisted-by: <name of code assistant>",
		Source:         "https://docs.fedoraproject.org/en-US/council/policy/ai-assisted-contributions/",
		RequireTrailer: []string{"assisted-by"},
	},
	{
		Name:           "artsy",
		Description:    "Artsy RFC (June 2026): Assisted-by rather than Co-authored-by, so the tool gets no avatar",
		Source:         "https://github.com/artsy/README/issues",
		RequireTrailer: []string{"assisted-by"},
		ForbidTrailers: []string{"co-authored-by"},
	},
	{
		Name:                "kubernetes",
		Description:         "Kubernetes (June 2026): disclose in the PR description; no AI co-author, Assisted-by, Co-developed-by or Generated-by trailers; an AI cannot sign the CLA",
		Source:              "https://www.kubernetes.dev/blog/2026/06/26/open-source-maintainership-in-the-age-of-ai/",
		ForbidTrailers:      []string{"assisted-by", "co-authored-by", "co-developed-by", "generated-by"},
		ForbidAgentAuthored: true,
		ForbidAgentSignoff:  true,
	},
}

// clone returns an independent copy so callers can append to the rule slices
// without mutating the shared preset.
func (p Policy) clone() Policy {
	p.RequireTrailer = append([]string(nil), p.RequireTrailer...)
	p.ForbidTrailers = append([]string(nil), p.ForbidTrailers...)
	return p
}

// All returns every preset, sorted by name.
func All() []Policy {
	out := make([]Policy, 0, len(presets))
	for _, p := range presets {
		out = append(out, p.clone())
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

// Find returns the preset with the given name (case-insensitive), or nil.
func Find(name string) *Policy {
	name = strings.ToLower(strings.TrimSpace(name))
	for i := range presets {
		if presets[i].Name == name {
			p := presets[i].clone()
			return &p
		}
	}
	return nil
}

// Names lists the preset names, sorted.
func Names() []string {
	var names []string
	for _, p := range All() {
		names = append(names, p.Name)
	}
	return names
}
