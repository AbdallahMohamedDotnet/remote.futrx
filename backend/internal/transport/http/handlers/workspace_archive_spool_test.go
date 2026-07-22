package httphandlers

import (
	"context"
	"errors"
	"io"
	"os"
	"testing"
)

func TestArchiveSpoolerCapsEachTemporaryFile(t *testing.T) {
	spooler := newArchiveSpooler(1, 4)
	spooled, err := spooler.prepare(context.Background(), func(destination io.Writer) error {
		_, _ = destination.Write([]byte("12345"))
		return nil
	})
	if spooled != nil {
		spooled.close()
		t.Fatal("oversized archive returned a spool file")
	}
	if !errors.Is(err, errWorkspaceArchiveTooLarge) {
		t.Fatalf("prepare error = %v, want %v", err, errWorkspaceArchiveTooLarge)
	}
	if len(spooler.slots) != 0 {
		t.Fatal("oversized archive did not release its spool slot")
	}
}

func TestArchiveSpoolerBoundsConcurrencyAndHonorsCancellation(t *testing.T) {
	spooler := newArchiveSpooler(1, 16)
	first, err := spooler.prepare(context.Background(), writeSpoolPayload("first"))
	if err != nil {
		t.Fatalf("prepare first archive: %v", err)
	}
	firstPath := first.file.Name()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	second, err := spooler.prepare(ctx, writeSpoolPayload("second"))
	if second != nil {
		second.close()
		t.Fatal("canceled waiter acquired a spool slot")
	}
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("prepare canceled error = %v, want %v", err, context.Canceled)
	}

	first.close()
	if _, err := os.Stat(firstPath); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("spool file still exists after close: %v", err)
	}
	third, err := spooler.prepare(context.Background(), writeSpoolPayload("third"))
	if err != nil {
		t.Fatalf("prepare after release: %v", err)
	}
	third.close()
}

func writeSpoolPayload(payload string) func(io.Writer) error {
	return func(destination io.Writer) error {
		_, err := io.WriteString(destination, payload)
		return err
	}
}
