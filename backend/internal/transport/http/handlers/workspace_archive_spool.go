package httphandlers

import (
	"context"
	"io"
	"os"
	"sync"

	serviceworkspacefiles "github.com/futrx-com/remote.futrx.com/internal/service/workspacefiles"
)

const (
	maxWorkspaceArchiveBytes       int64 = 1 << 30 // 1 GiB per download.
	maxConcurrentWorkspaceArchives       = 2       // At most 2 GiB in aggregate temporary archives.
)

var errWorkspaceArchiveTooLarge = serviceworkspacefiles.ErrArchiveTooLarge

type archiveSpooler struct {
	slots    chan struct{}
	maxBytes int64
}

type spooledArchive struct {
	file    *os.File
	release func()
	once    sync.Once
}

var workspaceArchiveSpooler = newArchiveSpooler(
	maxConcurrentWorkspaceArchives,
	maxWorkspaceArchiveBytes,
)

func newArchiveSpooler(maxConcurrent int, maxBytes int64) *archiveSpooler {
	return &archiveSpooler{
		slots:    make(chan struct{}, maxConcurrent),
		maxBytes: maxBytes,
	}
}

func (s *archiveSpooler) prepare(
	ctx context.Context,
	writeArchive func(io.Writer) error,
) (*spooledArchive, error) {
	if err := s.acquire(ctx); err != nil {
		return nil, err
	}
	var temporary *os.File
	prepared := false
	defer func() {
		if prepared {
			return
		}
		if temporary != nil {
			_ = temporary.Close()
			_ = os.Remove(temporary.Name())
		}
		s.release()
	}()

	temporary, err := os.CreateTemp("", "remote-workspace-archive-*.zip")
	if err != nil {
		return nil, err
	}
	bounded := &boundedContextWriter{
		ctx:         ctx,
		destination: temporary,
		remaining:   s.maxBytes,
	}
	if err := writeArchive(bounded); err != nil {
		return nil, err
	}
	if bounded.writeErr != nil {
		return nil, bounded.writeErr
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if _, err := temporary.Seek(0, 0); err != nil {
		return nil, err
	}
	prepared = true
	return &spooledArchive{file: temporary, release: s.release}, nil
}

func (a *spooledArchive) close() {
	a.once.Do(func() {
		_ = a.file.Close()
		_ = os.Remove(a.file.Name())
		a.release()
	})
}

func (s *archiveSpooler) acquire(ctx context.Context) error {
	select {
	case s.slots <- struct{}{}:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (s *archiveSpooler) release() {
	<-s.slots
}

type boundedContextWriter struct {
	ctx         context.Context
	destination io.Writer
	remaining   int64
	writeErr    error
}

func (w *boundedContextWriter) Write(buffer []byte) (int, error) {
	if w.writeErr != nil {
		return 0, w.writeErr
	}
	if err := w.ctx.Err(); err != nil {
		w.writeErr = err
		return 0, err
	}

	allowed := buffer
	exceedsLimit := int64(len(buffer)) > w.remaining
	if exceedsLimit {
		allowed = buffer[:int(w.remaining)]
	}
	n, err := w.destination.Write(allowed)
	w.remaining -= int64(n)
	if err != nil {
		w.writeErr = err
		return n, err
	}
	if n != len(allowed) {
		w.writeErr = io.ErrShortWrite
		return n, io.ErrShortWrite
	}
	if exceedsLimit {
		w.writeErr = errWorkspaceArchiveTooLarge
		return n, errWorkspaceArchiveTooLarge
	}
	return n, nil
}
