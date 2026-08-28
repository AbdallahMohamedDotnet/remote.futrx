package chat

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"strings"
)

var errPromptReceiptConflict = errors.New("prompt delivery receipt conflicts with accepted history")

// PromptReceiptStatus describes whether a client-generated prompt ID is new,
// matches the text already accepted for that ID, or conflicts with it.
type PromptReceiptStatus uint8

const (
	PromptReceiptMissing PromptReceiptStatus = iota
	PromptReceiptAccepted
	PromptReceiptConflict
)

// PromptReceiptLedger is the durable, hidden record of accepted interactive
// prompts. Its entries are private and every mutation returns a copy so cached
// chat metadata cannot be changed before the repository commits it.
type PromptReceiptLedger struct {
	entries map[string]string
}

// NewPromptReceiptLedger restores a ledger from its persistence shape without
// retaining the caller's mutable map.
func NewPromptReceiptLedger(entries map[string]string) PromptReceiptLedger {
	return PromptReceiptLedger{entries: clonePromptReceiptEntries(entries)}
}

func (ledger PromptReceiptLedger) Len() int {
	return len(ledger.entries)
}

// Status compares a delivery ID and its exact prompt text with the durable
// acceptance record. Client IDs are trimmed; prompt text is intentionally not.
func (ledger PromptReceiptLedger) Status(clientID, prompt string) PromptReceiptStatus {
	key, promptHash := promptReceiptHashes(clientID, prompt)
	acceptedHash, found := ledger.entries[key]
	if !found {
		return PromptReceiptMissing
	}
	if acceptedHash == promptHash {
		return PromptReceiptAccepted
	}
	return PromptReceiptConflict
}

// WithAcceptedPrompt returns a ledger containing the accepted delivery. It is
// copy-on-write even when replacing an existing entry.
func (ledger PromptReceiptLedger) WithAcceptedPrompt(clientID, prompt string) PromptReceiptLedger {
	entries := clonePromptReceiptEntries(ledger.entries)
	if entries == nil {
		entries = make(map[string]string, 1)
	}
	key, promptHash := promptReceiptHashes(clientID, prompt)
	entries[key] = promptHash
	return PromptReceiptLedger{entries: entries}
}

// withAcceptedEvents promotes prompt receipts from the portion of visible
// history a rewind is about to remove. A conflict fails closed so history is
// never truncated without an unambiguous durable acceptance record.
func (ledger PromptReceiptLedger) withAcceptedEvents(
	events []Event,
	beforeT int64,
) (PromptReceiptLedger, bool, error) {
	entries := clonePromptReceiptEntries(ledger.entries)
	changed := false
	for _, event := range events {
		if event.Type != "user" || event.T < beforeT {
			continue
		}
		clientID := strings.TrimSpace(event.PromptClientID())
		if clientID == "" {
			continue
		}
		key, promptHash := promptReceiptHashes(clientID, event.Text)
		if acceptedHash, found := entries[key]; found {
			if acceptedHash != promptHash {
				return PromptReceiptLedger{}, false, errPromptReceiptConflict
			}
			continue
		}
		if entries == nil {
			entries = make(map[string]string)
		}
		entries[key] = promptHash
		changed = true
	}
	if !changed {
		return ledger, false, nil
	}
	return PromptReceiptLedger{entries: entries}, true, nil
}

// Snapshot returns the persistence representation without exposing the
// ledger's internal map for mutation.
func (ledger PromptReceiptLedger) Snapshot() map[string]string {
	return clonePromptReceiptEntries(ledger.entries)
}

type promptEventData struct {
	ClientID string `json:"clientId"`
}

// SetPromptClientID attaches the interactive delivery ID to a user event using
// the existing event-data wire shape.
func (event *Event) SetPromptClientID(clientID string) {
	if event == nil {
		return
	}
	event.Data, _ = json.Marshal(promptEventData{ClientID: clientID})
}

// PromptClientID reads the typed delivery metadata. Missing or malformed event
// data has no delivery ID, matching legacy event behavior.
func (event Event) PromptClientID() string {
	if len(event.Data) == 0 {
		return ""
	}
	var data promptEventData
	if json.Unmarshal(event.Data, &data) != nil {
		return ""
	}
	return data.ClientID
}

func promptReceiptHashes(clientID, prompt string) (string, string) {
	clientSum := sha256.Sum256([]byte(strings.TrimSpace(clientID)))
	promptSum := sha256.Sum256([]byte(prompt))
	return hex.EncodeToString(clientSum[:]), hex.EncodeToString(promptSum[:])
}

func clonePromptReceiptEntries(entries map[string]string) map[string]string {
	if len(entries) == 0 {
		return nil
	}
	cloned := make(map[string]string, len(entries))
	for key, promptHash := range entries {
		cloned[key] = promptHash
	}
	return cloned
}
