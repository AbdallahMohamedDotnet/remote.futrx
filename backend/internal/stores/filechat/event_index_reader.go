package filechat

import (
	"bufio"
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"

	servicechat "github.com/futrx-com/remote.futrx.com/internal/service/chat"
)

type indexedEventLocation struct {
	offset int64
	length int64
	seq    int64
}

func (index *chatEventIndex) transcriptLocations(
	ctx context.Context,
	id servicechat.ID,
	beforeSeq int64,
	turnLimit int,
) ([]indexedEventLocation, error) {
	if turnLimit <= 0 {
		turnLimit = 1
	}
	rows, err := index.db.QueryContext(ctx, `
		SELECT start_offset, end_offset
		FROM chat_transcript_turns
		WHERE chat_id = ? AND (? <= 0 OR start_seq < ?)
		ORDER BY turn_ordinal DESC
		LIMIT ?`, id, beforeSeq, beforeSeq, turnLimit+1,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var firstOffset int64 = -1
	var lastOffset int64
	for rows.Next() {
		var startOffset, endOffset int64
		if err := rows.Scan(&startOffset, &endOffset); err != nil {
			return nil, err
		}
		if firstOffset < 0 || startOffset < firstOffset {
			firstOffset = startOffset
		}
		if endOffset > lastOffset {
			lastOffset = endOffset
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if firstOffset < 0 {
		return nil, nil
	}

	return index.eventLocationsInRange(ctx, id, firstOffset, lastOffset)
}

func (index *chatEventIndex) eventPageLocations(
	ctx context.Context,
	id servicechat.ID,
	beforeSeq int64,
	limit int,
) ([]indexedEventLocation, bool, error) {
	rows, err := index.db.QueryContext(ctx, `
		SELECT byte_offset, byte_length, event_seq
		FROM chat_event_offsets
		WHERE chat_id = ? AND (? <= 0 OR event_seq < ?)
		ORDER BY event_ordinal DESC
		LIMIT ?`, id, beforeSeq, beforeSeq, limit+1,
	)
	if err != nil {
		return nil, false, err
	}
	locations, err := scanEventLocations(rows)
	if err != nil {
		return nil, false, err
	}
	hasMore := len(locations) > limit
	if hasMore {
		locations = locations[:limit]
	}
	for left, right := 0, len(locations)-1; left < right; left, right = left+1, right-1 {
		locations[left], locations[right] = locations[right], locations[left]
	}
	return locations, hasMore, nil
}

func (index *chatEventIndex) eventLocationsAfter(
	ctx context.Context,
	id servicechat.ID,
	afterSeq int64,
) ([]indexedEventLocation, error) {
	rows, err := index.db.QueryContext(ctx, `
		SELECT byte_offset, byte_length, event_seq
		FROM chat_event_offsets
		WHERE chat_id = ? AND event_seq > ?
		ORDER BY event_ordinal`, id, afterSeq,
	)
	if err != nil {
		return nil, err
	}
	return scanEventLocations(rows)
}

func (index *chatEventIndex) eventLocationsInRange(
	ctx context.Context,
	id servicechat.ID,
	startOffset int64,
	endOffset int64,
) ([]indexedEventLocation, error) {
	rows, err := index.db.QueryContext(ctx, `
		SELECT byte_offset, byte_length, event_seq
		FROM chat_event_offsets
		WHERE chat_id = ? AND byte_offset >= ? AND byte_offset < ?
		ORDER BY event_ordinal`, id, startOffset, endOffset,
	)
	if err != nil {
		return nil, err
	}
	return scanEventLocations(rows)
}

func scanEventLocations(rows *sql.Rows) ([]indexedEventLocation, error) {
	defer rows.Close()
	locations := make([]indexedEventLocation, 0, 64)
	for rows.Next() {
		var location indexedEventLocation
		if err := rows.Scan(&location.offset, &location.length, &location.seq); err != nil {
			return nil, err
		}
		locations = append(locations, location)
	}
	return locations, rows.Err()
}

func readIndexedEvents(
	ctx context.Context,
	eventsPath string,
	locations []indexedEventLocation,
) ([]servicechat.Event, error) {
	if len(locations) == 0 {
		return nil, nil
	}
	file, err := os.Open(eventsPath)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	if _, err := file.Seek(locations[0].offset, io.SeekStart); err != nil {
		return nil, err
	}
	reader := bufio.NewReader(file)
	position := locations[0].offset

	events := make([]servicechat.Event, 0, len(locations))
	for _, location := range locations {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if location.length <= 0 || location.length > maxEventRecordBytes+2 {
			return nil, fmt.Errorf("invalid indexed event length %d", location.length)
		}
		if location.offset < position {
			return nil, errors.New("indexed event offsets are out of order")
		}
		if gap := location.offset - position; gap > 0 {
			if _, err := io.CopyN(io.Discard, reader, gap); err != nil {
				return nil, err
			}
			position += gap
		}
		line := make([]byte, int(location.length))
		if _, err := io.ReadFull(reader, line); err != nil {
			return nil, err
		}
		position += location.length
		line = bytes.TrimSuffix(line, []byte{'\n'})
		line = bytes.TrimSuffix(line, []byte{'\r'})
		var record eventRecord
		if err := json.Unmarshal(line, &record); err != nil {
			continue
		}
		event := record.toDomain()
		if event.Seq == 0 {
			event.Seq = location.seq
		}
		events = append(events, event)
	}
	return events, nil
}
