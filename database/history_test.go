package database

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	bolt "go.etcd.io/bbolt"
)

// seedHistory writes n samples, step apart starting at base, into the cpu
// metric bucket, each a distinct JSON payload {"i":<k>}.
func seedHistory(t *testing.T, base time.Time, step time.Duration, n int) time.Time {
	t.Helper()
	var ts time.Time
	for i := 0; i < n; i++ {
		ts = base.Add(time.Duration(i) * step)
		payload := []byte(fmt.Sprintf(`{"i":%d}`, i))
		require.NoError(t, PutHistoryBatch(ts, map[string][]byte{MetricCPU: payload}, ts.Add(-time.Hour)))
	}
	return ts
}

func TestPutAndQueryHistoryRoundTrip(t *testing.T) {
	cdTemp(t)
	t.Cleanup(func() { _ = Close() })
	require.NoError(t, SetupDB())

	base := time.Unix(1785316800, 0).UTC() // 2026-07-29T10:00:00Z
	last := seedHistory(t, base, 10*time.Second, 3)

	got, err := QueryHistory(MetricCPU, base, last)
	require.NoError(t, err)
	require.Len(t, got, 3)
	assert.Equal(t, `{"i":0}`, string(got[0].Data))
	assert.Equal(t, `{"i":2}`, string(got[2].Data))
	// oldest first, timestamps preserved at second precision
	assert.True(t, got[0].Timestamp.Equal(base), "got %v want %v", got[0].Timestamp, base)
	assert.True(t, got[1].Timestamp.After(got[0].Timestamp))
}

func TestQueryHistoryRangeBounds(t *testing.T) {
	cdTemp(t)
	t.Cleanup(func() { _ = Close() })
	require.NoError(t, SetupDB())

	base := time.Unix(1785316800, 0).UTC()
	seedHistory(t, base, 10*time.Second, 5) // t0..t40

	// interior window, inclusive on both ends
	got, err := QueryHistory(MetricCPU, base.Add(10*time.Second), base.Add(30*time.Second))
	require.NoError(t, err)
	require.Len(t, got, 3)
	assert.Equal(t, `{"i":1}`, string(got[0].Data))
	assert.Equal(t, `{"i":3}`, string(got[2].Data))

	// zero from scans from the oldest sample
	got, err = QueryHistory(MetricCPU, time.Time{}, base.Add(10*time.Second))
	require.NoError(t, err)
	require.Len(t, got, 2)

	// a range outside retention is empty but NOT nil ([] marshals as [], not null)
	got, err = QueryHistory(MetricCPU, base.Add(time.Hour), base.Add(2*time.Hour))
	require.NoError(t, err)
	assert.NotNil(t, got)
	assert.Len(t, got, 0)
}

func TestQueryHistoryUnknownMetricIsEmptyNotError(t *testing.T) {
	cdTemp(t)
	t.Cleanup(func() { _ = Close() })
	require.NoError(t, SetupDB())

	got, err := QueryHistory("nosuchmetric", time.Time{}, time.Time{})
	require.NoError(t, err)
	assert.NotNil(t, got)
	assert.Len(t, got, 0)
}

// TestPutHistoryBatchPrunesExpired proves retention actually evicts: samples
// older than the cutoff disappear from the same transaction that writes the
// new one, which is what bounds the DB at retention/interval keys per metric.
func TestPutHistoryBatchPrunesExpired(t *testing.T) {
	cdTemp(t)
	t.Cleanup(func() { _ = Close() })
	require.NoError(t, SetupDB())

	old := time.Unix(1785316800, 0).UTC()
	// two old samples, kept (cutoff far in the past)
	require.NoError(t, PutHistoryBatch(old, map[string][]byte{MetricCPU: []byte(`{"old":0}`)}, old.Add(-time.Hour)))
	require.NoError(t, PutHistoryBatch(old.Add(10*time.Second), map[string][]byte{MetricCPU: []byte(`{"old":1}`)}, old.Add(-time.Hour)))

	// a new write 2h later with a 24h-style cutoff that evicts the first one
	now := old.Add(2 * time.Hour)
	cutoff := now.Add(-time.Hour) // keeps only samples newer than old+1h
	require.NoError(t, PutHistoryBatch(now, map[string][]byte{MetricCPU: []byte(`{"new":true}`)}, cutoff))

	got, err := QueryHistory(MetricCPU, time.Time{}, time.Time{})
	require.NoError(t, err)
	require.Len(t, got, 1, "expired samples must be pruned by the batch write")
	assert.Equal(t, `{"new":true}`, string(got[0].Data))

	// PruneHistory standalone: shorten the window again and the rest goes
	// (pruning is strictly "older than cutoff", so the cutoff must pass the
	// newest key).
	require.NoError(t, PruneHistory(now.Add(time.Second)))
	got, err = QueryHistory(MetricCPU, time.Time{}, time.Time{})
	require.NoError(t, err)
	assert.Len(t, got, 0)
}

// TestPutHistoryBatchKeepsMetricsInSeparateBuckets proves the metric ->
// bucket layout: a batch can carry several metrics under one timestamp and
// each is queryable on its own.
func TestPutHistoryBatchKeepsMetricsInSeparateBuckets(t *testing.T) {
	cdTemp(t)
	t.Cleanup(func() { _ = Close() })
	require.NoError(t, SetupDB())

	ts := time.Unix(1785316800, 0).UTC()
	require.NoError(t, PutHistoryBatch(ts, map[string][]byte{
		MetricCPU:       []byte(`{"usage":42}`),
		MetricRAM:       []byte(`{"total":1}`),
		MetricDisks:     []byte(`[]`),
		MetricBandwidth: []byte(`[]`),
	}, ts.Add(-time.Hour)))

	for metric, want := range map[string]string{
		MetricCPU:       `{"usage":42}`,
		MetricRAM:       `{"total":1}`,
		MetricDisks:     `[]`,
		MetricBandwidth: `[]`,
	} {
		got, err := QueryHistory(metric, ts, ts)
		require.NoError(t, err)
		require.Len(t, got, 1, "metric %s", metric)
		assert.Equal(t, want, string(got[0].Data), "metric %s", metric)
	}
}

// TestSetupDBCreatesNoHistoryBucket is the disabled-by-default guarantee:
// nothing but PutHistoryBatch may create the history bucket, so a default
// (retention-off) install's nitr.db stays byte-compatible with before.
func TestSetupDBCreatesNoHistoryBucket(t *testing.T) {
	cdTemp(t)
	t.Cleanup(func() { _ = Close() })
	require.NoError(t, SetupDB())

	db, err := open()
	require.NoError(t, err)
	err = db.View(func(tx *bolt.Tx) error {
		assert.Nil(t, tx.Bucket([]byte(historyBucket)), "history bucket must not exist until the sampler writes")
		return nil
	})
	require.NoError(t, err)
}
