package containers

import "context"

func (c *Client) EnsureAgentInstructions(ctx context.Context, containerName string) error {
	return c.workspace.EnsureAgentInstructions(ctx, containerName)
}

func (c *Client) EnsureWorkspaceSkillLinks(ctx context.Context, containerName string) error {
	return c.workspace.EnsureSkillLinks(ctx, containerName)
}
