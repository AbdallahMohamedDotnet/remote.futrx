package workspace

import (
	"strings"
	"testing"

	"github.com/futrx-com/remote.futrx.com/internal/agent/provisioning"
)

func TestWorkspaceSkillLinksScriptUsesConfiguredProfiles(t *testing.T) {
	profiles := []provisioning.Profile{
		{WorkspaceSkills: &provisioning.WorkspaceSkills{WorkspaceHome: "/workspace/.alpha"}},
		{WorkspaceSkills: &provisioning.WorkspaceSkills{WorkspaceHome: "/workspace/.beta", HomeSkillsDir: "/root/.beta/skills"}},
	}
	script := workspaceSkillLinksScript(profiles)

	for _, want := range []string{
		"migrate_skills_dir '/workspace/.alpha/skills'",
		"migrate_skills_dir '/workspace/.beta/skills'",
		"link_skills_dir '/workspace/.alpha' '../.agents/skills'",
		"link_skills_dir '/workspace/.beta' '../.agents/skills'",
		"mirror_home_skills '/root/.beta/skills'",
	} {
		if !strings.Contains(script, want) {
			t.Fatalf("workspace skills script is missing %q", want)
		}
	}
}
