//go:build !windows

package bench

import "time"

// processStart falls back to package init, which misses loading the binary
// and the runtime starting (a few milliseconds).
func processStart() time.Time { return initAt }
