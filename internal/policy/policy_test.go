package policy

import (
	"strings"
	"testing"

	"github.com/trinhbentre/aiblame/internal/attrib"
)

func TestPresetsAreConsistent(t *testing.T) {
	if len(All()) < 5 {
		t.Fatal("expected several presets")
	}
	seen := map[string]bool{}
	for _, p := range All() {
		if p.Name == "" || p.Description == "" || !strings.HasPrefix(p.Source, "https://") {
			t.Errorf("preset %+v lacks name, description or https source", p)
		}
		if seen[p.Name] {
			t.Errorf("duplicate preset %q", p.Name)
		}
		seen[p.Name] = true
		if len(p.RequireTrailer) == 0 && len(p.ForbidTrailers) == 0 && !p.ForbidAgentAuthored && !p.ForbidAgentSignoff {
			t.Errorf("preset %q has no rules", p.Name)
		}
		for _, c := range append(append([]string{}, p.RequireTrailer...), p.ForbidTrailers...) {
			if !attrib.IsKnownConvention(c) {
				t.Errorf("preset %q uses unknown convention %q", p.Name, c)
			}
		}
		for _, r := range p.RequireTrailer {
			for _, f := range p.ForbidTrailers {
				if r == f {
					t.Errorf("preset %q both requires and forbids %q", p.Name, r)
				}
			}
		}
	}
	if Find("KERNEL") == nil || Find(" mesa ") == nil || Find("nope") != nil {
		t.Fatal("Find should be case- and space-insensitive")
	}
	names := Names()
	for i := 1; i < len(names); i++ {
		if names[i-1] >= names[i] {
			t.Fatalf("names not sorted: %v", names)
		}
	}
	// Find returns a copy: mutating it must not change the preset.
	p := Find("kernel")
	p.RequireTrailer[0] = "changed"
	if Find("kernel").RequireTrailer[0] == "changed" {
		t.Fatal("Find leaked the shared slice")
	}
}
