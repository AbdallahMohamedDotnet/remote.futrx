package wstransport

import (
	"encoding/json"
	"testing"
)

func TestPromptAckEventAccepted(t *testing.T) {
	ev := promptAckEvent("q-1", true)
	if ev.Type != "system" || ev.Subtype != "prompt_accepted" {
		t.Fatalf("unexpected ack event: %#v", ev)
	}
	if ev.Seq != 0 {
		t.Fatalf("ack must be transient (no seq), got %d", ev.Seq)
	}
	var data struct {
		ClientID string `json:"clientId"`
	}
	if err := json.Unmarshal(ev.Data, &data); err != nil {
		t.Fatalf("ack data: %v", err)
	}
	if data.ClientID != "q-1" {
		t.Fatalf("ack clientId = %q, want %q", data.ClientID, "q-1")
	}
}

func TestPromptAckEventRejected(t *testing.T) {
	ev := promptAckEvent("q-2", false)
	if ev.Type != "system" || ev.Subtype != "prompt_rejected" {
		t.Fatalf("unexpected ack event: %#v", ev)
	}
	var data struct {
		ClientID string `json:"clientId"`
	}
	if err := json.Unmarshal(ev.Data, &data); err != nil {
		t.Fatalf("ack data: %v", err)
	}
	if data.ClientID != "q-2" {
		t.Fatalf("ack clientId = %q, want %q", data.ClientID, "q-2")
	}
}
