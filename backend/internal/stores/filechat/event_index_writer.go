package filechat

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"

	servicechat "github.com/futrx-com/remote.futrx.com/internal/service/chat"
)

const (
	indexPrefixHashOffset64 = uint64(14695981039346656037)
	indexPrefixHashPrime64  = uint64(1099511628211)
)

func newChatIndexState() chatIndexState {
	return chatIndexState{
		prefixHash:   indexPrefixHashOffset64,
		tailComplete: true,
	}
}

// buildChatIndexTail extends state while the Store's per-chat lock is held.
// The reader is capped at observedSize so a concurrent out-of-band append is
// picked up by the next stat/sync rather than half-indexed here.
func buildChatIndexTail(
	ctx context.Context,
	eventsPath string,
	observedSize int64,
	state chatIndexState,
) (chatIndexState, error) {
	next := state
	if observedSize == next.indexedBytes {
		return next, nil
	}
	file, err := os.Open(eventsPath)
	if err != nil {
		return chatIndexState{}, err
	}
	defer file.Close()
	if _, err := file.Seek(next.indexedBytes, io.SeekStart); err != nil {
		return chatIndexState{}, err
	}

	remaining := observedSize - next.indexedBytes
	reader := bufio.NewReader(io.LimitReader(file, remaining))
	offset := next.indexedBytes
	next.tailComplete = true
	for offset < observedSize {
		if err := ctx.Err(); err != nil {
			return chatIndexState{}, err
		}
		raw, readErr := reader.ReadBytes('\n')
		if len(raw) > 0 {
			lineOffset := offset
			offset += int64(len(raw))
			next.tailComplete = raw[len(raw)-1] == '\n'
			next.prefixHash = updateIndexPrefixHash(next.prefixHash, raw)
			if err := indexEventRecord(&next, raw, lineOffset); err != nil {
				return chatIndexState{}, err
			}
		}
		if errors.Is(readErr, io.EOF) {
			break
		}
		if readErr != nil {
			return chatIndexState{}, readErr
		}
	}
	if offset != observedSize {
		return chatIndexState{}, io.ErrUnexpectedEOF
	}
	return next, nil
}

func updateIndexPrefixHash(value uint64, data []byte) uint64 {
	for _, item := range data {
		value ^= uint64(item)
		value *= indexPrefixHashPrime64
	}
	return value
}

func chatIndexPrefixMatches(
	ctx context.Context,
	eventsPath string,
	state chatIndexState,
) (bool, error) {
	file, err := os.Open(eventsPath)
	if err != nil {
		return false, err
	}
	defer file.Close()

	value := indexPrefixHashOffset64
	remaining := state.indexedBytes
	buffer := make([]byte, 32*1024)
	for remaining > 0 {
		if err := ctx.Err(); err != nil {
			return false, err
		}
		readSize := int64(len(buffer))
		if remaining < readSize {
			readSize = remaining
		}
		count, readErr := io.ReadFull(file, buffer[:int(readSize)])
		if count > 0 {
			value = updateIndexPrefixHash(value, buffer[:count])
			remaining -= int64(count)
		}
		if readErr != nil {
			if errors.Is(readErr, io.EOF) || errors.Is(readErr, io.ErrUnexpectedEOF) {
				return false, nil
			}
			return false, readErr
		}
	}
	return value == state.prefixHash, nil
}

func indexEventRecord(state *chatIndexState, raw []byte, offset int64) error {
	line := bytes.TrimSuffix(raw, []byte{'\n'})
	line = bytes.TrimSuffix(line, []byte{'\r'})
	if len(line) == 0 {
		return nil
	}

	state.eventOrdinal++
	if len(line) > maxEventRecordBytes {
		return fmt.Errorf("event record exceeds %d bytes", maxEventRecordBytes)
	}
	var record eventRecord
	if err := json.Unmarshal(line, &record); err != nil {
		return nil
	}
	event := record.toDomain()
	if event.Seq == 0 {
		event.Seq = state.eventOrdinal
	}
	if event.Seq > state.lastSeq {
		state.lastSeq = event.Seq
	}

	eventIndex := len(state.events)
	state.events = append(state.events, indexedEventLocation{
		offset: offset,
		length: int64(len(raw)),
		seq:    event.Seq,
	})
	startsNewTurn := len(state.turns) == 0 || servicechat.EventStartsTranscriptTurn(
		state.currentTurnID,
		state.currentTurnHasUser,
		true,
		event,
	)
	if startsNewTurn {
		state.turns = append(state.turns, indexedTurnRange{
			startEvent: eventIndex,
			endEvent:   eventIndex + 1,
			startSeq:   event.Seq,
		})
		state.currentTurnID = event.TurnID
		state.currentTurnHasUser = event.Type == "user"
		return nil
	}

	state.turns[len(state.turns)-1].endEvent = eventIndex + 1
	if state.currentTurnID == "" && event.TurnID != "" {
		state.currentTurnID = event.TurnID
	}
	state.currentTurnHasUser = state.currentTurnHasUser || event.Type == "user"
	return nil
}
