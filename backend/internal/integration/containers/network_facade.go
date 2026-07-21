package containers

import "context"

func (c *Client) RepairNetwork(ctx context.Context, containerName string) error {
	return c.network.Repair(ctx, containerName)
}
