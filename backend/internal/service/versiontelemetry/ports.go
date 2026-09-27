package versiontelemetry

import "context"

// Reporter is the outbound capability used to publish one pseudonymous
// running-version heartbeat. Implementations must bound their own I/O.
type Reporter interface {
	ReportVersion(ctx context.Context, version string) error
}
