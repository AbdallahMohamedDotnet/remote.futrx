package httphandlers

import (
	"io"
	"os"
)

type archiveSpooler struct{}

type spooledArchive struct {
	file *os.File
}

var workspaceArchiveSpooler archiveSpooler

func (archiveSpooler) prepare(writeArchive func(io.Writer) error) (*spooledArchive, error) {
	temporary, err := os.CreateTemp("", "remote-workspace-archive-*.zip")
	if err != nil {
		return nil, err
	}
	prepared := false
	defer func() {
		if prepared {
			return
		}
		_ = temporary.Close()
		_ = os.Remove(temporary.Name())
	}()

	if err := writeArchive(temporary); err != nil {
		return nil, err
	}
	if _, err := temporary.Seek(0, 0); err != nil {
		return nil, err
	}
	prepared = true
	return &spooledArchive{file: temporary}, nil
}

func (a *spooledArchive) close() {
	_ = a.file.Close()
	_ = os.Remove(a.file.Name())
}
