package httphandlers

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	servicechat "github.com/futrx-com/remote.futrx.com/internal/service/chat"
	serviceworkspacefiles "github.com/futrx-com/remote.futrx.com/internal/service/workspacefiles"
)

func TestFolderDownloadReportsArchiveFailureBeforeWritingResponse(t *testing.T) {
	wantErr := errors.New("archive failed")
	store := &archiveStoreStub{writeErr: wantErr}
	handler := &ChatHandler{files: serviceworkspacefiles.New(store)}
	request := httptest.NewRequest(http.MethodGet, "/?path=src", nil)
	response := httptest.NewRecorder()

	handler.handleFilesDownloadFolder(response, request, servicechat.Meta{Cwd: "/workspace"})

	if response.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusInternalServerError)
	}
	if contentType := response.Header().Get("Content-Type"); contentType == "application/zip" {
		t.Fatal("failed archive was advertised as a zip download")
	}
}

func TestFolderDownloadRejectsArchiveAboveSpoolLimit(t *testing.T) {
	previousSpooler := workspaceArchiveSpooler
	workspaceArchiveSpooler = newArchiveSpooler(1, 4)
	t.Cleanup(func() { workspaceArchiveSpooler = previousSpooler })
	store := &archiveStoreStub{payload: []byte("12345")}
	handler := &ChatHandler{files: serviceworkspacefiles.New(store)}
	request := httptest.NewRequest(http.MethodGet, "/?path=src", nil)
	response := httptest.NewRecorder()

	handler.handleFilesDownloadFolder(response, request, servicechat.Meta{Cwd: "/workspace"})

	if response.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusRequestEntityTooLarge)
	}
	if contentType := response.Header().Get("Content-Type"); contentType == "application/zip" {
		t.Fatal("oversized archive was advertised as a zip download")
	}
}

func TestFolderDownloadStopsWhileWaitingForSpoolSlotWhenRequestIsCanceled(t *testing.T) {
	previousSpooler := workspaceArchiveSpooler
	workspaceArchiveSpooler = newArchiveSpooler(1, 16)
	t.Cleanup(func() { workspaceArchiveSpooler = previousSpooler })
	occupied, err := workspaceArchiveSpooler.prepare(context.Background(), func(destination io.Writer) error {
		_, err := destination.Write([]byte("occupied"))
		return err
	})
	if err != nil {
		t.Fatalf("occupy spool slot: %v", err)
	}
	defer occupied.close()

	store := &archiveStoreStub{payload: []byte("zip")}
	handler := &ChatHandler{files: serviceworkspacefiles.New(store)}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	request := httptest.NewRequest(http.MethodGet, "/?path=src", nil).WithContext(ctx)
	response := httptest.NewRecorder()

	handler.handleFilesDownloadFolder(response, request, servicechat.Meta{Cwd: "/workspace"})

	if store.writeCalls != 0 {
		t.Fatalf("archive writer called %d times after cancellation", store.writeCalls)
	}
	if response.Body.Len() != 0 {
		t.Fatalf("response body after cancellation = %q", response.Body.String())
	}
}

type archiveStoreStub struct {
	serviceworkspacefiles.Store
	writeErr   error
	payload    []byte
	writeCalls int
}

func (s *archiveStoreStub) DirectoryExists(string, string) bool {
	return true
}

func (s *archiveStoreStub) WriteArchive(_ context.Context, _, _ string, destination io.Writer) error {
	s.writeCalls++
	if s.writeErr != nil {
		return s.writeErr
	}
	_, err := io.Copy(destination, bytes.NewReader(s.payload))
	return err
}
