package wstransport

import (
	"encoding/json"
	"testing"

	servicechat "github.com/futrx-com/remote.futrx.com/internal/service/chat"
	serviceprompt "github.com/futrx-com/remote.futrx.com/internal/service/prompt"
)

func TestExpectedExecutionPreferencesAllowsOnlyCompleteLegacyOmission(t *testing.T) {
	if got := expectedExecutionPreferences("", ""); got != nil {
		t.Fatalf("legacy omission = %#v, want nil", got)
	}
	if got := expectedExecutionPreferences(servicechat.ProviderCodex, ""); got == nil {
		t.Fatal("partial expectation must remain explicit and fail closed")
	}
	got := expectedExecutionPreferences(servicechat.ProviderCodex, "default")
	if got == nil || got.Provider != servicechat.ProviderCodex || got.Mode != "default" {
		t.Fatalf("expectation = %#v", got)
	}
}

func TestPromptAckEventAccepted(t *testing.T) {
	ev := promptAckEvent("q-1", nil)
	if ev.Type != "system" || ev.Subtype != "prompt_accepted" {
		t.Fatalf("unexpected ack event: %#v", ev)
	}
	if ev.Seq != 0 {
		t.Fatalf("ack must be transient (no seq), got %d", ev.Seq)
	}
	var data struct {
		ClientID  string `json:"clientId"`
		Retryable bool   `json:"retryable"`
		Reason    string `json:"reason"`
	}
	if err := json.Unmarshal(ev.Data, &data); err != nil {
		t.Fatalf("ack data: %v", err)
	}
	if data.ClientID != "q-1" || data.Retryable || data.Reason != "accepted" {
		t.Fatalf("ack data = %#v", data)
	}
}

func TestPromptAckEventRejected(t *testing.T) {
	ev := promptAckEvent("q-2", serviceprompt.ErrPromptAlreadyRunning)
	if ev.Type != "system" || ev.Subtype != "prompt_rejected" {
		t.Fatalf("unexpected ack event: %#v", ev)
	}
	var data struct {
		ClientID  string `json:"clientId"`
		Retryable bool   `json:"retryable"`
		Reason    string `json:"reason"`
	}
	if err := json.Unmarshal(ev.Data, &data); err != nil {
		t.Fatalf("ack data: %v", err)
	}
	if data.ClientID != "q-2" || !data.Retryable || data.Reason != "busy" {
		t.Fatalf("ack data = %#v", data)
	}
}

func TestPromptAckEventMarksPreferenceMismatchNonRetryable(t *testing.T) {
	ev := promptAckEvent("q-3", serviceprompt.ErrExecutionPreferencesChanged)
	var data struct {
		Retryable bool   `json:"retryable"`
		Reason    string `json:"reason"`
	}
	if err := json.Unmarshal(ev.Data, &data); err != nil {
		t.Fatalf("ack data: %v", err)
	}
	if data.Retryable || data.Reason != "preferences_changed" {
		t.Fatalf("ack data = %#v", data)
	}
}
