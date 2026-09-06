package hook

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/trinhbentre/aiblame/internal/gitx"
	"github.com/trinhbentre/aiblame/internal/testrepo"
)

func env(m map[string]string) func(string) string {
	return func(k string) string { return m[k] }
}

func TestDetect(t *testing.T) {
	if _, ok := Detect(env(nil)); ok {
		t.Fatal("empty env should not detect")
	}
	d, ok := Detect(env(map[string]string{"CLAUDECODE": "1", "AIBLAME_MODEL": "claude-opus-4-6"}))
	if !ok || d.Agent != "Claude Code" || d.Model != "claude-opus-4-6" || d.Var != "CLAUDECODE" {
		t.Fatalf("claude: %+v %v", d, ok)
	}
	d, ok = Detect(env(map[string]string{"CODEX_SANDBOX_NETWORK_DISABLED": "1"}))
	if !ok || d.Agent != "Codex" {
		t.Fatalf("codex: %+v", d)
	}
	d, ok = Detect(env(map[string]string{"AIBLAME_AGENT": "Acme Bot", "CLAUDECODE": "1"}))
	if !ok || d.Agent != "Acme Bot" {
		t.Fatalf("override: %+v", d)
	}
	if _, ok := Detect(env(map[string]string{"CLAUDECODE": "1", "AIBLAME_DISABLE": "1"})); ok {
		t.Fatal("disable ignored")
	}
	d, ok = Detect(env(map[string]string{"AI_AGENT": "claude-code_2-1-261_agent"}))
	if !ok || d.Agent != "claude code" {
		t.Fatalf("generic: %+v", d)
	}
}

func TestTrailer(t *testing.T) {
	d := Detected{Agent: "Claude Code", Email: "noreply@anthropic.com", Model: "m1"}
	if got := Trailer(d, AssistedBy); got != "Assisted-by: Claude Code (m1)" {
		t.Error(got)
	}
	if got := Trailer(d, CoAuthoredBy); got != "Co-authored-by: Claude Code <noreply@anthropic.com>" {
		t.Error(got)
	}
	if got := Trailer(d, GeneratedBy); got != "Generated-by: Claude Code (model: m1)" {
		t.Error(got)
	}
	if got := Trailer(Detected{Agent: "X"}, CoAuthoredBy); got != "Assisted-by: X" {
		t.Errorf("no e-mail should fall back: %s", got)
	}
	// Kernel 7.3+ drops tool and model names; Zephyr / kernel 7.0 use Agent:model.
	if got := Trailer(d, Kernel); got != "Assisted-by: LLM" {
		t.Error(got)
	}
	if got := Trailer(d, AgentModel); got != "Assisted-by: Claude Code:m1" {
		t.Error(got)
	}
	if got := Trailer(Detected{Agent: "Codex"}, AgentModel); got != "Assisted-by: Codex" {
		t.Error(got)
	}
	if st, err := ParseStyle("zephyr"); err != nil || st != AgentModel {
		t.Errorf("zephyr alias: %v %v", st, err)
	}
	if _, err := ParseStyle("bogus"); err == nil {
		t.Error("expected style error")
	}
	for _, st := range Styles {
		if !strings.Contains(Script(st), "# Style: "+string(st)+".") {
			t.Errorf("script for %s lacks its style marker", st)
		}
	}
	if !strings.Contains(Script(Kernel), `trailer="Assisted-by: LLM"`) {
		t.Error("kernel script should write the generic LLM tag")
	}
	if !strings.Contains(Script(AssistedBy), "claude-session|amp-thread-id|agent-logs-url") {
		t.Error("script should skip commits that already carry a session trailer")
	}
}

func TestInstallUninstallAndScriptRuns(t *testing.T) {
	if _, err := exec.LookPath("sh"); err != nil {
		t.Skip("sh not available")
	}
	repo := testrepo.Standard(t)
	ctx := context.Background()
	r, err := gitx.Open(ctx, repo.Dir)
	if err != nil {
		t.Fatal(err)
	}
	st, _ := Inspect(ctx, r)
	if st.Installed {
		t.Fatal("hook should not exist yet")
	}
	// foreign hook is protected
	p, _ := Path(ctx, r)
	os.MkdirAll(filepath.Dir(p), 0o755)
	os.WriteFile(p, []byte("#!/bin/sh\necho custom\n"), 0o755)
	if _, err := Install(ctx, r, AssistedBy, false); err != ErrExists {
		t.Fatalf("expected ErrExists, got %v", err)
	}
	if _, _, err := Uninstall(ctx, r); err == nil {
		t.Fatal("should refuse to remove foreign hook")
	}
	if _, err := Install(ctx, r, AssistedBy, true); err != nil {
		t.Fatal(err)
	}
	st, _ = Inspect(ctx, r)
	if !st.Installed || !st.Managed || st.Style != AssistedBy {
		t.Fatalf("status = %+v", st)
	}

	// Simulate a commit from a Claude Code session and check the trailer lands.
	repo.Write("src/hooked.go", "package x\n")
	cmd := exec.Command("git", "add", "-A")
	cmd.Dir = repo.Dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("%v %s", err, out)
	}
	cmd = exec.Command("git", "commit", "-q", "-m", "Add hooked file\n\nSome body.")
	cmd.Dir = repo.Dir
	cmd.Env = append(os.Environ(), "CLAUDECODE=1", "AIBLAME_MODEL=claude-test")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("%v %s", err, out)
	}
	msg := repo.Git("log", "-1", "--format=%B")
	if !strings.Contains(msg, "Assisted-by: Claude Code (claude-test)") {
		t.Fatalf("trailer missing:\n%s", msg)
	}
	// A second commit outside any agent session gets no trailer.
	repo.Write("src/human2.go", "package y\n")
	cmd = exec.Command("git", "add", "-A")
	cmd.Dir = repo.Dir
	cmd.Run()
	cmd = exec.Command("git", "commit", "-q", "-m", "Human change")
	cmd.Dir = repo.Dir
	cmd.Env = scrubbed()
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("%v %s", err, out)
	}
	if msg := repo.Git("log", "-1", "--format=%B"); strings.Contains(msg, "Assisted-by") {
		t.Fatalf("unexpected trailer:\n%s", msg)
	}
	// Agent that already disclosed itself is not double-tagged.
	repo.Write("src/h3.go", "package z\n")
	exec.Command("git", "-C", repo.Dir, "add", "-A").Run()
	cmd = exec.Command("git", "commit", "-q", "-m", "Already disclosed\n\nCo-authored-by: Claude <noreply@anthropic.com>")
	cmd.Dir = repo.Dir
	cmd.Env = append(os.Environ(), "CLAUDECODE=1")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("%v %s", err, out)
	}
	if msg := repo.Git("log", "-1", "--format=%B"); strings.Contains(msg, "Assisted-by") {
		t.Fatalf("double tagged:\n%s", msg)
	}

	removed, _, err := Uninstall(ctx, r)
	if err != nil || !removed {
		t.Fatalf("uninstall: %v %v", removed, err)
	}
}

// scrubbed returns the environment without any agent variables so the hook
// sees a plain human shell.
func scrubbed() []string {
	var out []string
	for _, kv := range os.Environ() {
		k, _, _ := strings.Cut(kv, "=")
		switch {
		case k == "AI_AGENT", strings.HasPrefix(k, "CLAUDE"), strings.HasPrefix(k, "CODEX"), strings.HasPrefix(k, "CURSOR"), k == "GEMINI_CLI", k == "OPENCODE", strings.HasPrefix(k, "AIBLAME"):
			continue
		}
		out = append(out, kv)
	}
	return out
}
