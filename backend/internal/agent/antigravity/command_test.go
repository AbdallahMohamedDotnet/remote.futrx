package antigravity

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/futrx-com/remote.futrx.com/internal/agent"
	"github.com/futrx-com/remote.futrx.com/internal/agent/provisioning"
)

func TestArgsComposition(t *testing.T) {
	p := &Provider{profile: Profile()}

	base := p.args(agent.RunRequest{Prompt: "do the thing"})
	joined := strings.Join(base, " ")
	if !strings.Contains(joined, "--print do the thing") {
		t.Fatalf("args missing print prompt: %v", base)
	}
	if !strings.Contains(joined, "--dangerously-skip-permissions") {
		t.Fatalf("headless run must auto-approve tools: %v", base)
	}
	if !strings.Contains(joined, "--print-timeout") {
		t.Fatalf("args missing print timeout: %v", base)
	}
	if strings.Contains(joined, "--conversation") || strings.Contains(joined, "--model") {
		t.Fatalf("unexpected optional flags in %v", base)
	}

	full := p.args(agent.RunRequest{
		Prompt:      "next",
		Model:       "gemini-3-pro",
		Mode:        agent.RunModePlan,
		ResumeID:    "abc-123",
		Preferences: agent.RunPreferences{ReasoningEffort: "xhigh"},
	})
	joined = strings.Join(full, " ")
	for _, want := range []string{
		"--model gemini-3-pro",
		"--mode plan",
		"--conversation abc-123",
		"--effort high",
	} {
		if !strings.Contains(joined, want) {
			t.Fatalf("args missing %q: %v", want, full)
		}
	}
	if strings.Contains(joined, "--dangerously-skip-permissions") {
		t.Fatalf("Plan mode must not bypass Antigravity permissions: %v", full)
	}
}

func TestEffortFlagClamping(t *testing.T) {
	tests := map[agent.ReasoningEffort]string{
		"":        "",
		"none":    "low",
		"minimal": "low",
		"low":     "low",
		"medium":  "medium",
		"high":    "high",
		"xhigh":   "high",
		"ultra":   "ultra",
		"bad;arg": "",
	}
	for effort, want := range tests {
		if got := effortFlag(effort); got != want {
			t.Fatalf("effortFlag(%q) = %q, want %q", effort, got, want)
		}
	}
}

func TestInstallScriptPinsVersionedRelease(t *testing.T) {
	script := Profile().CLI.InstallScript
	version := Profile().CLI.Version
	for _, want := range []string{
		releaseBaseURL + "/" + version + "/${asset}",
		`asset="agy_cli_linux_x64.tar.gz"`,
		`asset="agy_cli_linux_arm64.tar.gz"`,
		provisioning.MustPin("ANTIGRAVITY_LINUX_X64_SHA512"),
		provisioning.MustPin("ANTIGRAVITY_LINUX_ARM64_SHA512"),
	} {
		if !strings.Contains(script, want) {
			t.Fatalf("install script does not contain pinned release value %q", want)
		}
	}
	if !strings.Contains(script, "sha512sum -c") {
		t.Fatal("install script must verify the pinned checksum")
	}
	if !strings.Contains(script, "/usr/local/bin/agy") {
		t.Fatal("install script must install the agy binary")
	}
	if strings.Contains(script, "/manifests/") {
		t.Fatal("install script must not consult the moving latest manifest")
	}
}

func TestParserEmitsTextDeltas(t *testing.T) {
	parser := NewParser(agent.RunRequest{ConversationID: "c1"})
	events, err := parser.ParseLine([]byte("hello world"))
	if err != nil || len(events) != 1 {
		t.Fatalf("ParseLine = (%v, %v)", events, err)
	}
	if events[0].Type != agent.EventAssistantTextDelta || events[0].Text != "hello world\n" {
		t.Fatalf("unexpected event: %#v", events[0])
	}
}

func TestConversationDiscovery(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	brain := filepath.Join(home, stateDirUnderHome, "brain")
	if err := os.MkdirAll(filepath.Join(brain, "0aa89f21-1111-4222-8333-abcdef012345"), 0o755); err != nil {
		t.Fatal(err)
	}

	store := conversationStore{}
	before := store.list(context.Background())
	if len(before) != 1 {
		t.Fatalf("expected 1 existing conversation, got %d", len(before))
	}

	// No new conversation yet -> ambiguous/none.
	if id := store.newConversation(context.Background(), before); id != "" {
		t.Fatalf("expected no new conversation, got %q", id)
	}

	fresh := "1bb99a32-2222-4333-9444-bcdef0123456"
	if err := os.MkdirAll(filepath.Join(brain, fresh), 0o755); err != nil {
		t.Fatal(err)
	}
	if id := store.newConversation(context.Background(), before); id != fresh {
		t.Fatalf("newConversation = %q, want %q", id, fresh)
	}

	// Junk entries are ignored.
	if err := os.MkdirAll(filepath.Join(brain, "not a conversation!"), 0o755); err != nil {
		t.Fatal(err)
	}
	if id := store.newConversation(context.Background(), before); id != fresh {
		t.Fatalf("junk entry changed discovery: %q", id)
	}
}
