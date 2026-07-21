package containers

import "context"

const (
	BaseImageAlias       = containerImageAlias
	BaseImageSourceImage = containerImageSource
)

func (c *Client) BuildBaseImage(ctx context.Context, alias string) error {
	return c.images.Build(ctx, alias)
}
