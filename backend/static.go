package remote

import "embed"

// PublicFS contains the Vite-built SPA bundle. The frontend build writes into
// backend/public before the Go binary is built.
//
//go:embed public
var PublicFS embed.FS
