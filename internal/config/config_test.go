package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadMissingIsEmpty(t *testing.T) {
	cfg, err := Load(filepath.Join(t.TempDir(), FileName))
	if err != nil || cfg == nil {
		t.Fatalf("err=%v cfg=%v", err, cfg)
	}
	if !cfg.Paths.DefaultExcludesEnabled() {
		t.Fatal("default excludes should be on")
	}
}

func TestLoadExampleParses(t *testing.T) {
	p := filepath.Join(t.TempDir(), FileName)
	if err := os.WriteFile(p, []byte(Example), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(p)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Badge.Label != "AI-written" || len(cfg.Paths.Exclude) != 1 {
		t.Fatalf("cfg = %+v", cfg)
	}
}

func TestLoadRejectsUnknownKeyAndBadRegex(t *testing.T) {
	p := filepath.Join(t.TempDir(), FileName)
	os.WriteFile(p, []byte("[paths]\nfoo = 1\n"), 0o644)
	if _, err := Load(p); err == nil || !strings.Contains(err.Error(), "unknown key") {
		t.Fatalf("expected unknown key error, got %v", err)
	}
	os.WriteFile(p, []byte("[detect]\nmessage_markers = [\"(\"]\n"), 0o644)
	if _, err := Load(p); err == nil {
		t.Fatal("expected regex error")
	}
	os.WriteFile(p, []byte("[check]\nmetric = \"bytes\"\n"), 0o644)
	if _, err := Load(p); err == nil {
		t.Fatal("expected metric error")
	}
}

func TestClassifierOptions(t *testing.T) {
	p := filepath.Join(t.TempDir(), FileName)
	os.WriteFile(p, []byte(`
[detect]
extra_trailer_keys = ["X-AI"]
[[detect.agents]]
name = "Acme"
emails = ["a@acme"]
name_pattern = "^acme"
[[detect.bots]]
name = "CI"
emails = ["ci@acme"]
`), 0o644)
	cfg, err := Load(p)
	if err != nil {
		t.Fatal(err)
	}
	o := cfg.ClassifierOptions()
	if len(o.ExtraAgents) != 1 || o.ExtraAgents[0].NamePattern == nil || len(o.ExtraBots) != 1 || !o.ExtraBots[0].IsBot || o.ExtraTrailerKeys[0] != "X-AI" {
		t.Fatalf("opts = %+v", o)
	}
}
