package usersettings

import "context"

type Repository interface {
	Get(ctx context.Context, key Key) (Settings, error)
	Save(ctx context.Context, key Key, settings Settings) (Settings, error)
}
