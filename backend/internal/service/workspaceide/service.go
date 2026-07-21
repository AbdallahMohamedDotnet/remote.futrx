package workspaceide

import (
	"net/url"

	"github.com/futrx-com/remote.futrx.com/internal/shared/workspacepath"
)

type Service struct {
	baseURL      string
	projectsRoot string
}

func New(baseURL, projectsRoot string) *Service {
	return &Service{baseURL: baseURL, projectsRoot: projectsRoot}
}

func (s *Service) OpenURL(cwd, rawPath string) (string, error) {
	target, err := workspacepath.ResolveFile(rawPath, cwd)
	if err != nil {
		return "", err
	}
	return s.redirectURL(target), nil
}

func (s *Service) redirectURL(target workspacepath.Target) string {
	base := s.baseURL
	folder := target.WorkspaceRoot
	file := target.FilePath
	if slug, containerRoot, ok := workspacepath.ContainerPath(target.WorkspaceRoot, s.projectsRoot); ok {
		base = s.baseURL + slug + "/"
		folder = containerRoot
		if _, containerFile, fileIsInContainer := workspacepath.ContainerPath(target.FilePath, s.projectsRoot); fileIsInContainer {
			file = containerFile
		}
	}
	redirect, err := url.Parse(base)
	if err != nil {
		return base
	}
	query := redirect.Query()
	query.Set("folder", folder)
	if file != "" && file != folder {
		query.Set("file", file)
	}
	redirect.RawQuery = query.Encode()
	return redirect.String()
}
