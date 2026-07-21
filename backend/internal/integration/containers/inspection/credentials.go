package inspection

import (
	"context"
	"os"
	"strconv"
	"strings"

	serviceprofiles "github.com/Kings-Of-The-Web/remote.futrx.dev/internal/service/container/profiles"
	serviceproject "github.com/Kings-Of-The-Web/remote.futrx.dev/internal/service/project"
)

// containerCredentialInspector compares each configured credential file's host
// and in-container timestamps.
type containerCredentialInspector struct {
	commands *quickCommandRunner
	profiles serviceprofiles.Source
}

func (i *containerCredentialInspector) inspect(ctx context.Context, containerName string, state serviceproject.ContainerState) []serviceproject.AuthBundleStatus {
	profiles := i.profiles.Snapshot()
	bundles := make([]serviceproject.AuthBundleStatus, 0, len(profiles))
	for _, profile := range profiles {
		credentials := profile.Credentials
		if len(credentials.Files) == 0 {
			continue
		}
		status := serviceproject.AuthBundleStatus{Name: credentials.Name}
		for _, file := range credentials.Files {
			fileStatus := serviceproject.AuthBundleFileStatus{
				HostPath:      file.HostPath,
				ContainerPath: file.ContainerPath,
			}
			if info, err := os.Stat(file.HostPath); err == nil {
				fileStatus.HostExists = true
				fileStatus.HostMTime = info.ModTime().Unix()
			}
			if state == serviceproject.ContainerStateRunning {
				if raw, err := i.commands.run(ctx,
					"exec", containerName, "--", "stat", "-c", "%Y", file.ContainerPath); err == nil {
					if modifiedAt, parseErr := strconv.ParseInt(strings.TrimSpace(raw), 10, 64); parseErr == nil {
						fileStatus.ContainerExists = true
						fileStatus.ContainerMTime = modifiedAt
					}
				}
			}
			switch {
			case fileStatus.HostExists && fileStatus.ContainerExists:
				fileStatus.HostNewer = fileStatus.HostMTime > fileStatus.ContainerMTime
				fileStatus.ContainerNewer = fileStatus.ContainerMTime > fileStatus.HostMTime
			case fileStatus.HostExists && !fileStatus.ContainerExists && state == serviceproject.ContainerStateRunning:
				fileStatus.HostNewer = true
			case !fileStatus.HostExists && fileStatus.ContainerExists:
				fileStatus.ContainerNewer = true
			}
			status.Files = append(status.Files, fileStatus)
		}
		bundles = append(bundles, status)
	}
	return bundles
}
