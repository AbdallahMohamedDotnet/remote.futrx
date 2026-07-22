package httphandlers

import (
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

type archiveStoreStub struct {
	serviceworkspacefiles.Store
	writeErr error
}

func (s *archiveStoreStub) DirectoryExists(string, string) bool {
	return true
}

func (s *archiveStoreStub) WriteArchive(string, string, io.Writer) error {
	return s.writeErr
}
