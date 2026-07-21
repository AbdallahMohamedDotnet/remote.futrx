package workspacefiles

import "testing"

func TestSupportedMediaType(t *testing.T) {
	tests := map[string]bool{
		"/workspace/screenshot.PNG": true,
		"/workspace/demo.mp4":       true,
		"/workspace/audio.m4a":      true,
		"/workspace/report.pdf":     true,
		"/workspace/app.tsx":        false,
		"/workspace/archive.zip":    false,
	}

	for path, want := range tests {
		t.Run(path, func(t *testing.T) {
			_, got := supportedMediaType(path)
			if got != want {
				t.Fatalf("supportedMediaType() supported = %v, want %v", got, want)
			}
		})
	}
}
