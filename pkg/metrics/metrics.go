package metrics

import "sync/atomic"

// Counters are in-process stubs for MVP. Wire to Prometheus in a later slice.
type Counters struct {
	TracesBuilt    atomic.Int64
	TracesEligible atomic.Int64
	DeletePlans    atomic.Int64
	Deletes        atomic.Int64
	DryRuns        atomic.Int64
	Orphans        atomic.Int64
}

// Global is the process-wide stub registry.
var Global Counters

// Snapshot is a plain copy for status / logging.
type Snapshot struct {
	TracesBuilt    int64 `json:"tracesBuilt"`
	TracesEligible int64 `json:"tracesEligible"`
	DeletePlans    int64 `json:"deletePlans"`
	Deletes        int64 `json:"deletes"`
	DryRuns        int64 `json:"dryRuns"`
	Orphans        int64 `json:"orphans"`
}

// Read returns a snapshot of Global.
func Read() Snapshot {
	return Snapshot{
		TracesBuilt:    Global.TracesBuilt.Load(),
		TracesEligible: Global.TracesEligible.Load(),
		DeletePlans:    Global.DeletePlans.Load(),
		Deletes:        Global.Deletes.Load(),
		DryRuns:        Global.DryRuns.Load(),
		Orphans:        Global.Orphans.Load(),
	}
}
