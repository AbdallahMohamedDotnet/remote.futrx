package httphandlers

import "testing"

func TestResolveIDEOpenPath(t *testing.T) {
	workspaceRoot := "/var/lib/remote/projects/graphixy-ai/workspace"
	tests := []struct {
		name     string
		rawPath  string
		cwd      string
		wantFile string
		wantRoot string
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
			gotFile, gotRoot, err := resolveIDEOpenPath(tt.rawPath, tt.cwd)
			if err != nil {
				t.Fatalf("resolveIDEOpenPath() error = %v", err)
			}
			if gotFile != tt.wantFile || gotRoot != tt.wantRoot {
				t.Fatalf("resolveIDEOpenPath() = (%q, %q), want (%q, %q)", gotFile, gotRoot, tt.wantFile, tt.wantRoot)
			}
		})
	}
}

func TestResolveIDEOpenPathRejectsUnsafePaths(t *testing.T) {
	workspaceRoot := "/var/lib/remote/projects/graphixy-ai/workspace"
	tests := []string{"", "relative/file.txt", "/workspace/../etc/passwd", "/var/lib/remote/projects/graphixy-ai/secret.txt", "/etc/passwd"}

	for _, rawPath := range tests {
		t.Run(rawPath, func(t *testing.T) {
			if gotFile, gotRoot, err := resolveIDEOpenPath(rawPath, workspaceRoot); err == nil {
				t.Fatalf("resolveIDEOpenPath() = (%q, %q, nil), want error", gotFile, gotRoot)
			}
		})
	}
}
