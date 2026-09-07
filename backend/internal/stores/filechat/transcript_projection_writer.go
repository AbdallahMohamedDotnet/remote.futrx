package filechat

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"sort"
	"strconv"
	"strings"
	"unicode/utf8"

	servicechat "github.com/futrx-com/remote.futrx.com/internal/service/chat"
)

const transcriptInlineFieldBytes = 32 * 1024

type transcriptContentRef struct {
	id           string
	fieldKind    string
	fieldKey     string
	sourceOffset int64
	sourceLength int64
	contentBytes int64
}

type pendingTranscriptItem struct {
	event       servicechat.Event
	itemKey     string
	startOffset int64
	endOffset   int64
	turnOrdinal int64
	displaySeq  int64
}

// transcriptProjectionWriter owns materialized items and replacement snapshots
// within one index checkpoint. The event writer owns the surrounding transaction.
type transcriptProjectionWriter struct {
	ctx     context.Context
	tx      *sql.Tx
	chatID  servicechat.ID
	pending map[string]pendingTranscriptItem
}

func newTranscriptProjectionWriter(
	ctx context.Context,
	tx *sql.Tx,
	chatID servicechat.ID,
) *transcriptProjectionWriter {
	return &transcriptProjectionWriter{
		ctx:     ctx,
		tx:      tx,
		chatID:  chatID,
		pending: make(map[string]pendingTranscriptItem),
	}
}

func (writer *transcriptProjectionWriter) indexItem(
	event servicechat.Event,
	eventOrdinal int64,
	turnOrdinal int64,
	startOffset int64,
	endOffset int64,
) error {
	itemKey, mode, visible := transcriptItemIdentity(event, eventOrdinal)
	if !visible {
		return nil
	}
	if mode == transcriptItemReplace {
		key := strconv.FormatInt(turnOrdinal, 10) + "\x00" + itemKey
		displaySeq := event.Seq
		if pending, exists := writer.pending[key]; exists {
			displaySeq = pending.displaySeq
		}
		writer.pending[key] = pendingTranscriptItem{
			event:       event,
			itemKey:     itemKey,
			startOffset: startOffset,
			endOffset:   endOffset,
			turnOrdinal: turnOrdinal,
			displaySeq:  displaySeq,
		}
		return nil
	}
	return writer.persistTranscriptItem(
		event, itemKey, mode, startOffset, endOffset, turnOrdinal, event.Seq,
	)
}

func (writer *transcriptProjectionWriter) flush() error {
	keys := make([]string, 0, len(writer.pending))
	for key := range writer.pending {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		item := writer.pending[key]
		if err := writer.persistTranscriptItem(
			item.event,
			item.itemKey,
			transcriptItemReplace,
			item.startOffset,
			item.endOffset,
			item.turnOrdinal,
			item.displaySeq,
		); err != nil {
			return err
		}
	}
	return nil
}

func (writer *transcriptProjectionWriter) persistTranscriptItem(
	event servicechat.Event,
	itemKey string,
	mode transcriptItemMode,
	startOffset int64,
	endOffset int64,
	turnOrdinal int64,
	displaySeq int64,
) error {
	existingStart, existingPayload, found, err := readTranscriptItem(
		writer.ctx,
		writer.tx,
		writer.chatID,
		turnOrdinal,
		itemKey,
		mode == transcriptItemAppend,
	)
	if err != nil {
		return err
	}

	projected, refs := writer.compactTranscriptEvent(
		event, itemKey, startOffset, endOffset-startOffset, turnOrdinal,
	)
	projected.Seq = displaySeq
	payload := []servicechat.Event{projected}
	startSeq := projected.Seq
	if found {
		startSeq = existingStart
		switch mode {
		case transcriptItemReplace:
			// Replacement items render at their first occurrence even though
			// their payload is the latest state snapshot.
			projected.Seq = existingStart
			payload[0] = projected
			if err := deleteTranscriptContentRefs(
				writer.ctx, writer.tx, writer.chatID, turnOrdinal, itemKey,
			); err != nil {
				return err
			}
		case transcriptItemAppend:
			if err := json.Unmarshal(existingPayload, &payload); err != nil {
				return err
			}
			payload = append(payload, projected)
		}
	}

	payloadJSON, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	if _, err := writer.tx.ExecContext(writer.ctx, `
		INSERT INTO chat_transcript_items
			(chat_id, turn_ordinal, item_key, start_seq, end_seq,
			 payload_json, payload_bytes)
		VALUES (?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(chat_id, turn_ordinal, item_key) DO UPDATE SET
			end_seq = excluded.end_seq,
			payload_json = excluded.payload_json,
			payload_bytes = excluded.payload_bytes`,
		writer.chatID,
		turnOrdinal,
		itemKey,
		startSeq,
		event.Seq,
		payloadJSON,
		len(payloadJSON),
	); err != nil {
		return err
	}
	for _, ref := range refs {
		if _, err := writer.tx.ExecContext(writer.ctx, `
			INSERT INTO chat_transcript_content_refs
				(chat_id, content_id, turn_ordinal, item_key, field_kind,
				 field_key, source_offset, source_length, content_bytes)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
			ON CONFLICT(chat_id, content_id) DO UPDATE SET
				source_offset = excluded.source_offset,
				source_length = excluded.source_length,
				content_bytes = excluded.content_bytes`,
			writer.chatID,
			ref.id,
			turnOrdinal,
			itemKey,
			ref.fieldKind,
			ref.fieldKey,
			ref.sourceOffset,
			ref.sourceLength,
			ref.contentBytes,
		); err != nil {
			return err
		}
	}
	return nil
}

type transcriptItemMode uint8

const (
	transcriptItemSingle transcriptItemMode = iota
	transcriptItemReplace
	transcriptItemAppend
)

func transcriptItemIdentity(event servicechat.Event, ordinal int64) (string, transcriptItemMode, bool) {
	switch event.Type {
	case "provider_event", "usage_update", "session", "sync", "system", "permission_request":
		return "", transcriptItemSingle, false
	case "collaboration":
		if event.Name == "wait" {
			return "", transcriptItemSingle, false
		}
		if event.ID != "" {
			return "collaboration:" + event.ID, transcriptItemReplace, true
		}
	case "turn_status":
		return "turn-status", transcriptItemReplace, true
	case "tool_use_start", "tool_use_end":
		if event.ID != "" {
			return "tool:" + event.ID, transcriptItemAppend, true
		}
	case "interaction_request", "interaction_resolved":
		if event.ID != "" {
			return "interaction:" + event.ID, transcriptItemAppend, true
		}
	}
	return "event:" + strconv.FormatInt(ordinal, 10), transcriptItemSingle, true
}

func readTranscriptItem(
	ctx context.Context,
	tx *sql.Tx,
	chatID servicechat.ID,
	turnOrdinal int64,
	itemKey string,
	includePayload bool,
) (int64, []byte, bool, error) {
	var startSeq int64
	var payload []byte
	if !includePayload {
		err := tx.QueryRowContext(ctx, `
			SELECT start_seq
			FROM chat_transcript_items
			WHERE chat_id = ? AND turn_ordinal = ? AND item_key = ?`,
			chatID, turnOrdinal, itemKey,
		).Scan(&startSeq)
		if errors.Is(err, sql.ErrNoRows) {
			return 0, nil, false, nil
		}
		return startSeq, nil, err == nil, err
	}
	err := tx.QueryRowContext(ctx, `
		SELECT start_seq, payload_json
		FROM chat_transcript_items
		WHERE chat_id = ? AND turn_ordinal = ? AND item_key = ?`,
		chatID, turnOrdinal, itemKey,
	).Scan(&startSeq, &payload)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, nil, false, nil
	}
	return startSeq, payload, err == nil, err
}

func deleteTranscriptContentRefs(
	ctx context.Context,
	tx *sql.Tx,
	chatID servicechat.ID,
	turnOrdinal int64,
	itemKey string,
) error {
	_, err := tx.ExecContext(ctx, `
		DELETE FROM chat_transcript_content_refs
		WHERE chat_id = ? AND turn_ordinal = ? AND item_key = ?`,
		chatID, turnOrdinal, itemKey,
	)
	return err
}

func (writer *transcriptProjectionWriter) compactTranscriptEvent(
	event servicechat.Event,
	itemKey string,
	sourceOffset int64,
	sourceLength int64,
	turnOrdinal int64,
) (servicechat.Event, []transcriptContentRef) {
	event.Native = nil
	makeRef := func(fieldKind, fieldKey string, contentBytes int) transcriptContentRef {
		sum := sha256.Sum256([]byte(strings.Join([]string{
			string(writer.chatID),
			strconv.FormatInt(turnOrdinal, 10),
			itemKey,
			fieldKind,
			fieldKey,
		}, "\x00")))
		return transcriptContentRef{
			id:           hex.EncodeToString(sum[:16]),
			fieldKind:    fieldKind,
			fieldKey:     fieldKey,
			sourceOffset: sourceOffset,
			sourceLength: sourceLength,
			contentBytes: int64(contentBytes),
		}
	}

	var refs []transcriptContentRef
	switch event.Type {
	case "tool_use_start":
		if len(event.Input) > transcriptInlineFieldBytes {
			ref := makeRef("tool_input", "", len(event.Input))
			refs = append(refs, ref)
			preview, _ := json.Marshal(map[string]any{
				"preview":    utf8Prefix(string(event.Input), transcriptInlineFieldBytes),
				"truncated":  true,
				"contentRef": ref.id,
			})
			event.Input = preview
		}
	case "tool_use_end":
		if len(event.Output) > transcriptInlineFieldBytes {
			fullBytes := len(event.Output)
			ref := makeRef("tool_output", "", fullBytes)
			refs = append(refs, ref)
			event.Output = utf8Prefix(event.Output, transcriptInlineFieldBytes)
			event.OutputRef = ref.id
			event.OutputBytes = int64(fullBytes)
			event.OutputTruncated = true
		}
	case "collaboration":
		event.Data, refs = compactCollaborationData(event.Data, makeRef)
	}
	return event, refs
}

func compactCollaborationData(
	raw json.RawMessage,
	makeRef func(string, string, int) transcriptContentRef,
) (json.RawMessage, []transcriptContentRef) {
	var data map[string]any
	if len(raw) == 0 || json.Unmarshal(raw, &data) != nil {
		return raw, nil
	}
	var refs []transcriptContentRef
	if tools, ok := data["tools"].([]any); ok {
		for index, value := range tools {
			tool, ok := value.(map[string]any)
			if !ok {
				continue
			}
			key := strconv.Itoa(index)
			if output, ok := tool["output"].(string); ok && len(output) > transcriptInlineFieldBytes {
				ref := makeRef("collaboration_tool_output", key, len(output))
				refs = append(refs, ref)
				tool["output"] = utf8Prefix(output, transcriptInlineFieldBytes)
				tool["outputRef"] = ref.id
				tool["outputBytes"] = len(output)
				tool["outputTruncated"] = true
			}
			if input, exists := tool["input"]; exists {
				encoded, err := json.Marshal(input)
				if err == nil && len(encoded) > transcriptInlineFieldBytes {
					ref := makeRef("collaboration_tool_input", key, len(encoded))
					refs = append(refs, ref)
					tool["input"] = map[string]any{
						"preview":    utf8Prefix(string(encoded), transcriptInlineFieldBytes),
						"truncated":  true,
						"contentRef": ref.id,
					}
				}
			}
		}
	}
	if states, ok := data["agentsStates"].(map[string]any); ok {
		for threadID, value := range states {
			state, ok := value.(map[string]any)
			if !ok {
				continue
			}
			if message, ok := state["message"].(string); ok && len(message) > transcriptInlineFieldBytes {
				ref := makeRef("collaboration_agent_message", threadID, len(message))
				refs = append(refs, ref)
				state["message"] = utf8Prefix(message, transcriptInlineFieldBytes)
				state["messageRef"] = ref.id
				state["messageBytes"] = len(message)
				state["messageTruncated"] = true
			}
		}
	}
	encoded, err := json.Marshal(data)
	if err != nil {
		return raw, nil
	}
	return encoded, refs
}

func utf8Prefix(value string, limit int) string {
	if limit <= 0 || len(value) <= limit {
		return value
	}
	end := limit
	for end > 0 && !utf8.RuneStart(value[end]) {
		end--
	}
	return value[:end]
}
