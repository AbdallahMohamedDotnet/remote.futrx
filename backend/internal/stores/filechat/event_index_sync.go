package filechat

import (
	"context"
	"errors"
	"os"

	servicechat "github.com/futrx-com/remote.futrx.com/internal/service/chat"
)

func (index *chatEventIndex) lastEventSeq(
	ctx context.Context,
	id servicechat.ID,
	eventsPath string,
) (int64, error) {
	state, err := index.syncChat(ctx, id, eventsPath)
	return state.lastSeq, err
}

func (index *chatEventIndex) refreshAfterAppend(
	ctx context.Context,
	id servicechat.ID,
	eventsPath string,
) error {
	_, err := index.syncChatWithGrowth(ctx, id, eventsPath, true)
	return err
}

func (index *chatEventIndex) refreshAfterFallback(
	ctx context.Context,
	id servicechat.ID,
	eventsPath string,
) error {
	_, err := index.syncChat(ctx, id, eventsPath)
	return err
}

// syncChat validates the process-local snapshot against the authoritative
// JSONL file. Appends extend the cached state; rewrites and truncations rebuild
// it. A scan is published only after the complete observed file range succeeds.
func (index *chatEventIndex) syncChat(
	ctx context.Context,
	id servicechat.ID,
	eventsPath string,
) (chatIndexState, error) {
	return index.syncChatWithGrowth(ctx, id, eventsPath, false)
}

func (index *chatEventIndex) syncChatWithGrowth(
	ctx context.Context,
	id servicechat.ID,
	eventsPath string,
	trustedAppend bool,
) (chatIndexState, error) {
	if err := ctx.Err(); err != nil {
		return chatIndexState{}, err
	}
	info, err := os.Stat(eventsPath)
	var fileSize int64
	var fileMtimeNS int64
	if err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			return chatIndexState{}, err
		}
	} else {
		fileSize = info.Size()
		fileMtimeNS = info.ModTime().UnixNano()
	}

	state, found := index.snapshot(id)
	rebuild := !found || state.indexedBytes > fileSize ||
		(state.indexedBytes == fileSize && state.fileMtimeNS != fileMtimeNS) ||
		(state.indexedBytes < fileSize && !state.tailComplete)
	if !rebuild && !trustedAppend && state.indexedBytes < fileSize {
		matches, matchErr := chatIndexPrefixMatches(ctx, eventsPath, state)
		if matchErr != nil {
			return chatIndexState{}, matchErr
		}
		rebuild = !matches
	}
	if !rebuild && state.indexedBytes == fileSize {
		return state, nil
	}
	if rebuild {
		state = newChatIndexState()
	}

	next, err := buildChatIndexTail(ctx, eventsPath, fileSize, state)
	if err != nil {
		// Tail builds extend cached slice storage in place. Discard the entry if
		// a partial scan fails so no mutated prefix can be observed or reused.
		index.invalidate(id)
		return chatIndexState{}, err
	}
	next.indexedBytes = fileSize
	next.fileMtimeNS = fileMtimeNS
	index.publish(id, next)
	return next, nil
}

func (index *chatEventIndex) rebuildChat(
	ctx context.Context,
	id servicechat.ID,
	eventsPath string,
) error {
	if err := index.forget(ctx, id); err != nil {
		return err
	}
	_, err := index.syncChat(ctx, id, eventsPath)
	return err
}
