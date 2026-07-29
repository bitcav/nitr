package models

import (
	"encoding/json"
	"time"
)

// HistorySample is one retained metric sample, as stored in nitr.db and as
// returned by the range-query form of the metric endpoints
// (/api/v1/cpu?from=&to= etc.). Data carries the exact JSON payload the
// instantaneous form of the endpoint returns, so a range response is a
// series of ordinary endpoint payloads keyed by time rather than a new
// shape per metric.
type HistorySample struct {
	Timestamp time.Time       `json:"timestamp"`
	Data      json.RawMessage `json:"data"`
}
