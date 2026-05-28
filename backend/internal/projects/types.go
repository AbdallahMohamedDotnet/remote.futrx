package projects

// ProjectStatus tracks the lifecycle of a project's LXC container.
// The container itself doesn't exist in this commit — we only track the
// data model. The Provisioning/Running/Stopped/Missing transitions become
// meaningful once the container.go wrapper lands in the next commit.
type ProjectStatus string

const (
	StatusUnknown      ProjectStatus = ""
	StatusProvisioning ProjectStatus = "provisioning" // create requested, container being launched
	StatusRunning      ProjectStatus = "running"      // container is up
	StatusStopped      ProjectStatus = "stopped"      // container exists but is not running
	StatusError        ProjectStatus = "error"        // last operation failed; see ErrorMsg
	StatusMissing      ProjectStatus = "missing"      // meta exists but no container — needs reprovision
)

// ProjectMeta lives at data/projects/{id}/meta.json. The same pattern as
// ChatMeta: small file, frequent reads (sidebar listing), occasional writes
// (status updates, rename).
type ProjectMeta struct {
	ID            string        `json:"id"`            // random hex id
	Name          string        `json:"name"`          // user-facing display name
	Slug          string        `json:"slug"`          // url-safe identifier; container name = "proj-" + Slug
	Cwd           string        `json:"cwd"`           // workspace dir on the host (bind-mounted into /workspace)
	ContainerName string        `json:"containerName"` // LXC name on the host
	Status        ProjectStatus `json:"status"`
	ErrorMsg      string        `json:"errorMsg,omitempty"` // populated on StatusError
	CreatedAt     int64         `json:"createdAt"`          // unix ms
	UpdatedAt     int64         `json:"updatedAt"`          // unix ms
}
