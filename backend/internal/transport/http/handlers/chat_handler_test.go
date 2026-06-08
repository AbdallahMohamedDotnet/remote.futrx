package httphandlers

import "testing"

func TestResolveIDEOpenPath(t *testing.T) {
	workspaceRoot := "/var/lib/remote/projects/graphixy-ai/workspace"
	tests := []struct {
		name       string
		rawPath    string
		cwd        string
		wantFile   string
		wantRoot   string
		wantLine   int
		wantColumn int
	}{
		{
			name:     "container workspace file",
			rawPath:  "/workspace/app-graphixy-full-page-4k.png",
			cwd:      workspaceRoot,
			wantFile: workspaceRoot + "/app-graphixy-full-page-4k.png",
			wantRoot: workspaceRoot,
		},
		{
			name:     "container workspace from nested cwd",
			rawPath:  "/workspace/src/App.tsx",
			cwd:      workspaceRoot + "/src/components",
			wantFile: workspaceRoot + "/src/App.tsx",
			wantRoot: workspaceRoot,
		},
		{
			name:       "container workspace file with line",
			rawPath:    "/workspace/src/App.tsx:87",
			cwd:        workspaceRoot,
			wantFile:   workspaceRoot + "/src/App.tsx",
			wantRoot:   workspaceRoot,
			wantLine:   87,
			wantColumn: 0,
		},
		{
			name:       "container workspace file with line and column",
			rawPath:    "/workspace/src/App.tsx:87:5",
			cwd:        workspaceRoot,
			wantFile:   workspaceRoot + "/src/App.tsx",
			wantRoot:   workspaceRoot,
			wantLine:   87,
			wantColumn: 5,
		},
		{
			name:     "host workspace file",
			rawPath:  workspaceRoot + "/assets/logo.png",
			cwd:      workspaceRoot,
			wantFile: workspaceRoot + "/assets/logo.png",
			wantRoot: workspaceRoot,
		},
		{
			name:     "workspace root",
			rawPath:  "/workspace",
			cwd:      workspaceRoot,
			wantFile: workspaceRoot,
			wantRoot: workspaceRoot,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := resolveIDEOpenPath(tt.rawPath, tt.cwd)
			if err != nil {
				t.Fatalf("resolveIDEOpenPath() error = %v", err)
			}
			if got.FilePath != tt.wantFile || got.WorkspaceRoot != tt.wantRoot || got.Line != tt.wantLine || got.Column != tt.wantColumn {
				t.Fatalf("resolveIDEOpenPath() = %#v, want file=%q root=%q line=%d column=%d", got, tt.wantFile, tt.wantRoot, tt.wantLine, tt.wantColumn)
			}
		})
	}
}

func TestResolveIDEOpenPathRejectsUnsafePaths(t *testing.T) {
	workspaceRoot := "/var/lib/remote/projects/graphixy-ai/workspace"
	tests := []string{"", "relative/file.txt", "/workspace/../etc/passwd", "/var/lib/remote/projects/graphixy-ai/secret.txt", "/etc/passwd"}

	for _, rawPath := range tests {
		t.Run(rawPath, func(t *testing.T) {
			if got, err := resolveIDEOpenPath(rawPath, workspaceRoot); err == nil {
				t.Fatalf("resolveIDEOpenPath() = (%#v, nil), want error", got)
			}
		})
	}
}

func TestIDEOpenCommandTarget(t *testing.T) {
	tests := []struct {
		name   string
		target ideOpenTarget
		want   string
	}{
		{
			name:   "file only",
			target: ideOpenTarget{FilePath: "/var/lib/remote/projects/graphixy-ai/workspace/src/App.tsx"},
			want:   "/var/lib/remote/projects/graphixy-ai/workspace/src/App.tsx",
		},
		{
			name:   "file and line",
			target: ideOpenTarget{FilePath: "/var/lib/remote/projects/graphixy-ai/workspace/src/App.tsx", Line: 87},
			want:   "/var/lib/remote/projects/graphixy-ai/workspace/src/App.tsx:87",
		},
		{
			name:   "file line and column",
			target: ideOpenTarget{FilePath: "/var/lib/remote/projects/graphixy-ai/workspace/src/App.tsx", Line: 87, Column: 5},
			want:   "/var/lib/remote/projects/graphixy-ai/workspace/src/App.tsx:87:5",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ideOpenCommandTarget(tt.target); got != tt.want {
				t.Fatalf("ideOpenCommandTarget() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestIDEOpenFileURI(t *testing.T) {
	target := ideOpenTarget{
		FilePath: "/var/lib/remote/projects/graphixy-ai/workspace/src/App.tsx",
		Line:     87,
		Column:   5,
	}
	want := "file:///var/lib/remote/projects/graphixy-ai/workspace/src/App.tsx:87:5"
	if got := ideOpenFileURI(target); got != want {
		t.Fatalf("ideOpenFileURI() = %q, want %q", got, want)
	}
}

func TestIsBrowserMediaFile(t *testing.T) {
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
			if got := isBrowserMediaFile(path); got != want {
				t.Fatalf("isBrowserMediaFile() = %v, want %v", got, want)
			}
		})
	}
}
