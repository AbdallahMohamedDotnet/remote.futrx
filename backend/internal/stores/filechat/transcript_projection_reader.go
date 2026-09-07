package filechat

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"sort"
	"strconv"

	configconstants "github.com/futrx-com/remote.futrx.com/internal/config/constants"
	servicechat "github.com/futrx-com/remote.futrx.com/internal/service/chat"
)

type projectedTranscriptItem struct {
	turnOrdinal  int64
	turnID       string
	turnStart    int64
	startSeq     int64
	endSeq       int64
	payload      []servicechat.Event
	payloadBytes int
}

// ReadTranscriptPage serves only complete materialized snapshots. Missing or
// stale legacy projections are rebuilt in the background and exposed as byte
// progress, so opening a large chat never performs a gigabyte request scan.
func (s *Store) ReadTranscriptPage(
	ctx context.Context,
	id servicechat.ID,
	query servicechat.TranscriptPageQuery,
) (servicechat.TranscriptPage, error) {
	if !servicechat.ValidID(id) {
		return servicechat.TranscriptPage{}, servicechat.ErrInvalidID
	}
	if err := s.index.availabilityError(); err != nil {
		return servicechat.TranscriptPage{}, fmt.Errorf(
			"%w: %v", servicechat.ErrTranscriptProjectionUnavailable, err,
		)
	}

	info, err := os.Stat(s.eventsPath(id))
	var totalBytes, mtimeNS int64
	if err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			return servicechat.TranscriptPage{}, err
		}
	} else {
		totalBytes = info.Size()
		mtimeNS = info.ModTime().UnixNano()
	}
	state, found, err := s.index.readState(ctx, id)
	if err != nil {
		return servicechat.TranscriptPage{}, err
	}
	ready := found && state.indexedBytes == totalBytes && state.fileMtimeNS == mtimeNS
	if !ready {
		s.startTranscriptIndex(id)
		indexedBytes := state.indexedBytes
		if indexedBytes < 0 || indexedBytes > totalBytes {
			indexedBytes = 0
		}
		lastSeq := state.lastSeq
		tailSeqKnown := totalBytes == 0
		if tailSeq, tailErr := lastStoredEventSeq(s.eventsPath(id), totalBytes); tailErr == nil && tailSeq > 0 {
			tailSeqKnown = true
			if tailSeq > lastSeq {
				lastSeq = tailSeq
			}
		}
		return servicechat.TranscriptPage{
			Turns:   []servicechat.TranscriptTurn{},
			LastSeq: lastSeq,
			Indexing: &servicechat.TranscriptIndexProgress{
				IndexedBytes: indexedBytes,
				TotalBytes:   totalBytes,
				TailSeqKnown: tailSeqKnown,
			},
		}, nil
	}
	return s.index.readProjectedTranscriptPage(ctx, id, state, query)
}

func lastStoredEventSeq(eventsPath string, fileSize int64) (int64, error) {
	if fileSize <= 0 {
		return 0, nil
	}
	window := int64(maxEventRecordBytes + 2)
	if window > fileSize {
		window = fileSize
	}
	file, err := os.Open(eventsPath)
	if err != nil {
		return 0, err
	}
	defer file.Close()
	raw := make([]byte, int(window))
	if _, err := file.ReadAt(raw, fileSize-window); err != nil && !errors.Is(err, io.EOF) {
		return 0, err
	}
	lines := bytes.Split(raw, []byte{'\n'})
	for index := len(lines) - 1; index >= 0; index-- {
		line := bytes.TrimSuffix(lines[index], []byte{'\r'})
		if len(line) == 0 {
			continue
		}
		event, err := decodeStoredEvent(line, 0)
		if err == nil && event.Seq > 0 {
			return event.Seq, nil
		}
	}
	return 0, errors.New("last stored event has no sequence")
}

func (s *Store) startTranscriptIndex(id servicechat.ID) {
	s.indexingMu.Lock()
	if _, exists := s.indexing[id]; exists {
		s.indexingMu.Unlock()
		return
	}
	s.indexing[id] = struct{}{}
	s.indexWG.Add(1)
	s.indexingMu.Unlock()

	go func() {
		defer s.indexWG.Done()
		defer func() {
			s.indexingMu.Lock()
			delete(s.indexing, id)
			s.indexingMu.Unlock()
		}()
		lock := s.lock(id)
		lock.Lock()
		defer lock.Unlock()
		if _, err := os.Stat(s.chatDir(id)); err != nil {
			return
		}
		if _, err := s.index.syncChat(s.indexContext, id, s.eventsPath(id)); err != nil &&
			!errors.Is(err, context.Canceled) {
			log.Printf("background transcript projection for chat %s failed: %v", id, err)
		}
	}()
}

func (index *chatEventIndex) readProjectedTranscriptPage(
	ctx context.Context,
	id servicechat.ID,
	state chatIndexState,
	query servicechat.TranscriptPageQuery,
) (servicechat.TranscriptPage, error) {
	turnLimit := query.Limit
	if turnLimit <= 0 {
		turnLimit = configconstants.DefaultChatTranscriptTurnLimit
	}
	if turnLimit > configconstants.MaxChatTranscriptTurnLimit {
		turnLimit = configconstants.MaxChatTranscriptTurnLimit
	}
	byteLimit := query.ByteLimit
	if byteLimit <= 0 {
		byteLimit = configconstants.DefaultChatTranscriptByteLimit
	}
	if byteLimit > configconstants.MaxChatTranscriptByteLimit {
		byteLimit = configconstants.MaxChatTranscriptByteLimit
	}

	rows, err := index.db.QueryContext(ctx, `
		SELECT i.turn_ordinal, t.source_turn_id, t.start_seq,
		       i.start_seq, i.end_seq, i.payload_json, i.payload_bytes
		FROM chat_transcript_items AS i
		JOIN chat_transcript_turns AS t
		  ON t.chat_id = i.chat_id AND t.turn_ordinal = i.turn_ordinal
		WHERE i.chat_id = ? AND (? <= 0 OR i.start_seq < ?)
		ORDER BY i.start_seq DESC, i.item_key DESC`,
		id, query.BeforeSeq, query.BeforeSeq,
	)
	if err != nil {
		return servicechat.TranscriptPage{}, err
	}
	defer rows.Close()

	items := make([]projectedTranscriptItem, 0, 64)
	turns := make(map[int64]struct{}, turnLimit)
	usedBytes := 0
	hasMore := false
	for rows.Next() {
		var item projectedTranscriptItem
		var payloadJSON []byte
		if err := rows.Scan(
			&item.turnOrdinal,
			&item.turnID,
			&item.turnStart,
			&item.startSeq,
			&item.endSeq,
			&payloadJSON,
			&item.payloadBytes,
		); err != nil {
			return servicechat.TranscriptPage{}, err
		}
		_, knownTurn := turns[item.turnOrdinal]
		if (!knownTurn && len(turns) == turnLimit) ||
			(len(items) > 0 && usedBytes+item.payloadBytes > byteLimit) {
			hasMore = true
			break
		}
		if err := json.Unmarshal(payloadJSON, &item.payload); err != nil {
			return servicechat.TranscriptPage{}, fmt.Errorf(
				"%w: decode projected item: %v", errInvalidChatEventIndex, err,
			)
		}
		items = append(items, item)
		turns[item.turnOrdinal] = struct{}{}
		usedBytes += item.payloadBytes
	}
	if err := rows.Err(); err != nil {
		return servicechat.TranscriptPage{}, err
	}

	for left, right := 0, len(items)-1; left < right; left, right = left+1, right-1 {
		items[left], items[right] = items[right], items[left]
	}
	projectedTurns := make([]servicechat.TranscriptTurn, 0, len(turns))
	var lastTurnOrdinal int64 = -1
	for _, item := range items {
		last := len(projectedTurns) - 1
		if last < 0 || lastTurnOrdinal != item.turnOrdinal {
			projectedTurns = append(projectedTurns, servicechat.TranscriptTurn{
				ID:       projectedTurnID(item),
				StartSeq: item.startSeq,
				EndSeq:   item.endSeq,
				Events:   append([]servicechat.Event(nil), item.payload...),
			})
			lastTurnOrdinal = item.turnOrdinal
			continue
		}
		projectedTurns[last].EndSeq = item.endSeq
		projectedTurns[last].Events = append(projectedTurns[last].Events, item.payload...)
	}
	for index := range projectedTurns {
		sort.SliceStable(projectedTurns[index].Events, func(left, right int) bool {
			return projectedTurns[index].Events[left].Seq < projectedTurns[index].Events[right].Seq
		})
	}

	var nextBefore int64
	if hasMore && len(items) > 0 {
		nextBefore = items[0].startSeq
	}
	return servicechat.TranscriptPage{
		Turns:      projectedTurns,
		NextBefore: nextBefore,
		LastSeq:    state.lastSeq,
		HasMore:    hasMore,
	}, nil
}

func projectedTurnID(item projectedTranscriptItem) string {
	if item.turnID != "" {
		return item.turnID
	}
	return "legacy-" + strconv.FormatInt(item.turnStart, 10)
}
