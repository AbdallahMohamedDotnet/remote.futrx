package provisioning

import "time"

type InstallMode string

const (
	InstallWithNPM         InstallMode = "npm"
	InstallWithImageRepair InstallMode = "image-repair"
)

// CLISpec is the provider-owned description of a CLI installed in project
// containers. Container integrations own how the description is applied.
type CLISpec struct {
	Name               string
	ImageLabel         string
	Binary             string
	PackageName        string
	Version            string
	ReportVersion      bool
	CheckVersion       bool
	VerifyAfterInstall bool
	InstallMode        InstallMode
	InstallTimeout     time.Duration
	WaitTimeout        time.Duration
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

// Profile is the complete container-facing definition supplied by one agent.
// It contains policy only; no LXC or filesystem operation lives here.
type Profile struct {
	ID                  string
	CLI                 CLISpec
	Credentials         CredentialSpec
	Instructions        *InstructionTarget
	WorkspaceSkills     *WorkspaceSkills
	BrowserMCPTemplates []TemplateFile
}

func (p Profile) Clone() Profile {
	p.Credentials.Files = append([]CredentialFile(nil), p.Credentials.Files...)
	p.Credentials.LegacyDevices = append([]string(nil), p.Credentials.LegacyDevices...)
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
	p.BrowserMCPTemplates = append([]TemplateFile(nil), p.BrowserMCPTemplates...)
	for i := range p.BrowserMCPTemplates {
		p.BrowserMCPTemplates[i].Content = append([]byte(nil), p.BrowserMCPTemplates[i].Content...)
	}
	return p
}
