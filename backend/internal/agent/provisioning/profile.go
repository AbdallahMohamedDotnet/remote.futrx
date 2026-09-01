package provisioning

import (
	"errors"
	"path"
	"strings"
	"time"
)

type InstallMode string

const (
	InstallWithNPM         InstallMode = "npm"
	InstallWithImageRepair InstallMode = "image-repair"
	// InstallWithScript runs the spec's InstallScript in the target execution
	// environment. It supports CLIs that ship as standalone binaries rather
	// than npm packages.
	InstallWithScript InstallMode = "script"
)

// CLISpec is the provider-owned description of a CLI installed on the host
// and/or in project containers. Each execution-environment integration owns
// how the shared policy is applied.
type CLISpec struct {
	Name       string
	ImageLabel string
	Binary     string
	// VersionArgs are passed to Binary when reporting or checking its
	// installed version. Output must contain a parseable semver; each execution
	// environment owns whether readiness requires the exact pin or a compatible
	// version at least as new as that pin.
	VersionArgs        []string
	PackageName        string
	Version            string
	ReportVersion      bool
	CheckVersion       bool
	VerifyAfterInstall bool
	InstallMode        InstallMode
	// InstallScript is the bash program used by InstallWithScript. It must be
	// self-contained, idempotent, and pinned to Version.
	InstallScript  string
	InstallTimeout time.Duration
	WaitTimeout    time.Duration
}

func (s CLISpec) NPMPackage() string {
	if s.Version == "" {
		return s.PackageName
	}
	return s.PackageName + "@" + s.Version
}

type CredentialFile struct {
	HostPath      string
	ContainerPath string
	Mode          string
	PushRequired  bool
	PullRequired  bool
}

// CredentialDirectory describes a dynamic directory of credential files.
// It supports providers that rotate an unknown set of files rather than one
// fixed credential document.
type CredentialDirectory struct {
	HostPath                 string
	ContainerPath            string
	ContainerDirs            []string
	AllowContainerOnly       bool
	MissingErrorFormat       string
	SyncOnlyWhenHostHasFiles bool
	SyncUnavailableIsNoop    bool
}

// CredentialSpec is provider policy for credential placement. The container
// integration performs the file transfer without knowing which provider owns
// the paths.
type CredentialSpec struct {
	Name          string
	HostDir       string
	ContainerDir  string
	Files         []CredentialFile
	LegacyDevices []string
	Directory     *CredentialDirectory
	SeedOnLaunch  bool
}

func (s CredentialSpec) Empty() bool {
	return len(s.Files) == 0 && s.Directory == nil
}

type InstructionTarget struct {
	Path     string
	HashPath string
}

type WorkspaceSkills struct {
	WorkspaceHome string
	HomeSkillsDir string
}

type TemplateFile struct {
	Content       []byte
	Path          string
	HashPath      string
	Mode          string
	Directory     string
	DirectoryMode string
}

// Validate rejects template targets that could escape provider-owned durable
// homes or the project workspace. Hash markers share the declared directory so
// publication never depends on an unprepared parent path.
func (t TemplateFile) Validate() error {
	if !validTemplatePath(t.Path) {
		return errors.New("template path must be a clean path below /root or /workspace")
	}
	if !validTemplatePath(t.HashPath) || t.HashPath == t.Path {
		return errors.New("template hash path must be a distinct clean path below /root or /workspace")
	}
	directory := t.Directory
	if directory == "" {
		directory = path.Dir(t.Path)
	}
	if !validTemplatePath(directory) {
		return errors.New("template directory must be a clean path below /root or /workspace")
	}
	if !pathWithin(directory, t.Path) || !pathWithin(directory, t.HashPath) {
		return errors.New("template and hash paths must be inside the template directory")
	}
	if t.Mode != "" && !validTemplateMode(t.Mode) {
		return errors.New("invalid template file mode")
	}
	if t.DirectoryMode != "" && !validTemplateMode(t.DirectoryMode) {
		return errors.New("invalid template directory mode")
	}
	return nil
}

func validTemplatePath(value string) bool {
	if !path.IsAbs(value) || path.Clean(value) != value || strings.ContainsRune(value, '\x00') {
		return false
	}
	return pathWithin("/root", value) || pathWithin("/workspace", value)
}

func pathWithin(root, target string) bool {
	return target != root && strings.HasPrefix(target, root+"/")
}

func validTemplateMode(mode string) bool {
	if len(mode) != 3 && len(mode) != 4 {
		return false
	}
	for _, character := range mode {
		if character < '0' || character > '7' {
			return false
		}
	}
	return true
}

// PersistentDirectory declares provider state that must survive project
// container replacement. HostDirectory is a single directory name below the
// project's private agent-home root; ContainerPath is the absolute location
// used by the provider CLI. Device must be a stable LXD disk-device name.
type PersistentDirectory struct {
	Device        string
	HostDirectory string
	ContainerPath string
}

func (d PersistentDirectory) Validate() error {
	if !validPersistentName(d.Device) || d.Device == "workspace" {
		return errors.New("invalid or reserved device name")
	}
	if !validPersistentName(d.HostDirectory) {
		return errors.New("invalid host directory name")
	}
	if !path.IsAbs(d.ContainerPath) || path.Clean(d.ContainerPath) != d.ContainerPath ||
		d.ContainerPath == "/root" || !strings.HasPrefix(d.ContainerPath, "/root/") {
		return errors.New("container path must be a clean path below /root")
	}
	return nil
}

func validPersistentName(value string) bool {
	if value == "" || value != strings.TrimSpace(value) || value[0] < 'a' || value[0] > 'z' {
		return false
	}
	for index := 1; index < len(value); index++ {
		char := value[index]
		if (char >= 'a' && char <= 'z') || (char >= '0' && char <= '9') || char == '-' {
			continue
		}
		return false
	}
	return true
}

// Profile is the provisioning policy supplied by one agent. CLI policy may be
// consumed for host and project execution; credential, mount, instruction,
// skill, and browser policy applies to project containers. No LXC, process,
// or filesystem operation lives here.
type Profile struct {
	ID                  string
	CLI                 CLISpec
	Credentials         CredentialSpec
	PersistentState     []PersistentDirectory
	Instructions        *InstructionTarget
	WorkspaceSkills     *WorkspaceSkills
	RuntimeTemplates    []TemplateFile
	BrowserMCPTemplates []TemplateFile
}

func (p Profile) Clone() Profile {
	p.CLI.VersionArgs = append([]string(nil), p.CLI.VersionArgs...)
	p.Credentials.Files = append([]CredentialFile(nil), p.Credentials.Files...)
	p.Credentials.LegacyDevices = append([]string(nil), p.Credentials.LegacyDevices...)
	p.PersistentState = append([]PersistentDirectory(nil), p.PersistentState...)
	if p.Credentials.Directory != nil {
		directory := *p.Credentials.Directory
		directory.ContainerDirs = append([]string(nil), directory.ContainerDirs...)
		p.Credentials.Directory = &directory
	}
	if p.Instructions != nil {
		instructions := *p.Instructions
		p.Instructions = &instructions
	}
	if p.WorkspaceSkills != nil {
		skills := *p.WorkspaceSkills
		p.WorkspaceSkills = &skills
	}
	p.RuntimeTemplates = cloneTemplateFiles(p.RuntimeTemplates)
	p.BrowserMCPTemplates = cloneTemplateFiles(p.BrowserMCPTemplates)
	return p
}

func cloneTemplateFiles(templates []TemplateFile) []TemplateFile {
	cloned := append([]TemplateFile(nil), templates...)
	for i := range cloned {
		cloned[i].Content = append([]byte(nil), cloned[i].Content...)
	}
	return cloned
}
