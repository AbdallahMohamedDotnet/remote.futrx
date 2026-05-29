package project

type ID string
type Status string
type ContainerState string

const (
	StatusUnknown      Status = ""
	StatusProvisioning Status = "provisioning"
	StatusRunning      Status = "running"
	StatusStopped      Status = "stopped"
	StatusError        Status = "error"
	StatusMissing      Status = "missing"
)

const (
	ContainerStateRunning ContainerState = "RUNNING"
	ContainerStateStopped ContainerState = "STOPPED"
	ContainerStateFrozen  ContainerState = "FROZEN"
	ContainerStateMissing ContainerState = "MISSING"
	ContainerStateUnknown ContainerState = "UNKNOWN"
)

type Meta struct {
	ID            ID     `json:"id"`
	Name          string `json:"name"`
	Slug          string `json:"slug"`
	Cwd           string `json:"cwd"`
	ContainerName string `json:"containerName"`
	Status        Status `json:"status"`
	ErrorMsg      string `json:"errorMsg,omitempty"`
	CreatedAt     int64  `json:"createdAt"`
	UpdatedAt     int64  `json:"updatedAt"`
}

type CreateInput struct {
	Name string `json:"name"`
}

type UpdateInput struct {
	Name *string `json:"name,omitempty"`
}

func ValidID(id ID) bool {
	if len(id) < 4 || len(id) > 32 {
		return false
	}
	for i := 0; i < len(id); i++ {
		c := id[i]
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f')) {
			return false
		}
	}
	return true
}
