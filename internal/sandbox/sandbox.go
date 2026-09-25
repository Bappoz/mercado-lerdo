// Package sandbox defines the platform's view of a running sandbox. The
// container runtime lives behind it (internal/sandbox/docker is the only
// implementation; see docs/architecture/sandbox.md §3).
package sandbox

import "time"

// ResourceStats is one resource sample of one container, read from the host.
// Counters are cumulative since the container started.
type ResourceStats struct {
	Component string // e.g. "postgres", "app"
	Replica   int    // 1-based
	At        time.Time

	CPUUsageSeconds  float64 // counter
	CPUQuotaCores    float64 // limit in cores; 0 = unlimited
	CPUPeriods       uint64  // counter: CFS periods with runnable tasks
	ThrottledPeriods uint64  // counter: periods in which the quota ran out
	ThrottledSeconds float64 // counter: time spent throttled

	MemoryBytes      uint64 // working set (usage minus inactive file cache)
	MemoryLimitBytes uint64 // 0 = unlimited
	Pids             uint64
	NetRxBytes       uint64 // counter
	NetTxBytes       uint64 // counter
}
