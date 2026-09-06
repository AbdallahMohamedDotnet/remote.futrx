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

func (index *chatEventIndex) readEventPage(
	ctx context.Context,
	id servicechat.ID,
	eventsPath string,
	beforeSeq int64,
	limit int,
) (servicechat.EventPage, error) {
	state, err := index.syncChat(ctx, id, eventsPath)
	if err != nil {
		return servicechat.EventPage{}, err
	}
	locations, hasMore, err := eventPageLocations(ctx, state, beforeSeq, limit)
	if err != nil {
		return servicechat.EventPage{}, err
	}
	events, err := readIndexedEvents(ctx, eventsPath, locations)
	if err != nil {
		return servicechat.EventPage{}, err
	}
	var nextBefore int64
	if hasMore && len(events) > 0 {
		nextBefore = events[0].Seq
	}
	return servicechat.EventPage{
		Events:     events,
		NextBefore: nextBefore,
		LastSeq:    state.lastSeq,
		HasMore:    hasMore,
	}, nil
}

func (index *chatEventIndex) readEventsAfter(
	ctx context.Context,
	id servicechat.ID,
	eventsPath string,
	afterSeq int64,
) ([]servicechat.Event, error) {
	state, err := index.syncChat(ctx, id, eventsPath)
	if err != nil {
		return nil, err
	}
	locations, err := eventLocationsAfter(ctx, state, afterSeq)
	if err != nil {
		return nil, err
	}
	return readIndexedEvents(ctx, eventsPath, locations)
}

func (index *chatEventIndex) readTranscriptWindow(
	ctx context.Context,
	id servicechat.ID,
	eventsPath string,
	beforeSeq int64,
	turnLimit int,
) (servicechat.TranscriptEventWindow, error) {
	state, err := index.syncChat(ctx, id, eventsPath)
	if err != nil {
		return servicechat.TranscriptEventWindow{}, err
	}
	locations, err := transcriptLocations(ctx, state, beforeSeq, turnLimit)
	if err != nil {
		return servicechat.TranscriptEventWindow{}, err
	}
	events, err := readIndexedEvents(ctx, eventsPath, locations)
	if err != nil {
		return servicechat.TranscriptEventWindow{}, err
	}
	return servicechat.TranscriptEventWindow{
		Events:  events,
		LastSeq: state.lastSeq,
	}, nil
}

func transcriptLocations(
	ctx context.Context,
	state chatIndexState,
	beforeSeq int64,
	turnLimit int,
) ([]indexedEventLocation, error) {
	if turnLimit <= 0 {
		turnLimit = 1
	}
	firstEvent := -1
	lastEvent := 0
	selected := 0
	for i := len(state.turns) - 1; i >= 0 && selected < turnLimit+1; i-- {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		turn := state.turns[i]
		if beforeSeq > 0 && turn.startSeq >= beforeSeq {
			continue
		}
		if firstEvent < 0 || turn.startEvent < firstEvent {
			firstEvent = turn.startEvent
		}
		if turn.endEvent > lastEvent {
			lastEvent = turn.endEvent
		}
		selected++
	}
	if firstEvent < 0 {
		return nil, nil
	}
	if firstEvent > len(state.events) || lastEvent > len(state.events) || firstEvent > lastEvent {
		return nil, errors.New("indexed transcript turn range is invalid")
	}
	return state.events[firstEvent:lastEvent], nil
}

func eventPageLocations(
	ctx context.Context,
	state chatIndexState,
	beforeSeq int64,
	limit int,
) ([]indexedEventLocation, bool, error) {
	locations := make([]indexedEventLocation, 0, limit+1)
	for i := len(state.events) - 1; i >= 0 && len(locations) <= limit; i-- {
		if err := ctx.Err(); err != nil {
			return nil, false, err
		}
		location := state.events[i]
		if beforeSeq > 0 && location.seq >= beforeSeq {
			continue
		}
		locations = append(locations, location)
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

func eventLocationsAfter(
	ctx context.Context,
	state chatIndexState,
	afterSeq int64,
) ([]indexedEventLocation, error) {
	locations := make([]indexedEventLocation, 0, 32)
	for _, location := range state.events {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if location.seq > afterSeq {
			locations = append(locations, location)
		}
	}
	return locations, nil
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
