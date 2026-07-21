package workspaceide

import "testing"

func TestOpenURLMapsProjectPathsIntoContainer(t *testing.T) {
	service := New("https://code.remote.futrx.com/", "/var/lib/remote/projects")
	got, err := service.OpenURL(
		"/var/lib/remote/projects/graphixy-ai/workspace",
		"/workspace/src/App.tsx:87:5",
	)
	if err != nil {
		t.Fatal(err)
	}
	want := "https://code.remote.futrx.com/graphixy-ai/?file=%2Fworkspace%2Fsrc%2FApp.tsx&folder=%2Fworkspace"
	if got != want {
		t.Fatalf("OpenURL() = %q, want %q", got, want)
	}
}
