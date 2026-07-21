package containers

import "context"

func (c *Client) EnsureCodeServer(ctx context.Context, containerName, displayName string) error {
	return c.codeServer.Ensure(ctx, containerName, displayName)
}
