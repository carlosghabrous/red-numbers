// Package platform holds cross-domain infrastructure shared by every handler.
package platform

import "log/slog"

// Dependencies bundles the infrastructure every domain handler needs.
// New cross-cutting concerns (metrics, tracing, config) can be added here
// without changing any handler constructor's signature.
type Dependencies struct {
	Logger *slog.Logger
}
