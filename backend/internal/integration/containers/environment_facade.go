package containers

import "context"

func (c *Client) ApplyContainerEnvDiff(
	ctx context.Context,
	container string,
	set map[string]string,
	unset []string,
) error {
	return c.environment.ApplyDiff(ctx, container, set, unset)
}
