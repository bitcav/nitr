package database

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"time"

	"github.com/bitcav/nitr/models"
	bolt "go.etcd.io/bbolt"
)

// historyBucket is the root bucket for retained metric samples. It holds one
// nested bucket per metric, each keyed by big-endian uint64 Unix-nanosecond
// timestamps so cursor scans come out in chronological order and a time
// range is a Seek + iterate. The bucket is created lazily by
// PutHistoryBatch: with retention disabled (the default) nothing calls it,
// so no history bucket ever appears in nitr.db.
const historyBucket = "history"

// Metric names, used both as the nested bucket names under historyBucket and
// to select the bucket on range queries. They deliberately match the API
// paths (/api/v1/cpu etc.) so route -> bucket needs no translation table.
const (
	MetricCPU       = "cpu"
	MetricRAM       = "ram"
	MetricDisks     = "disks"
	MetricBandwidth = "bandwidth"
)

// historyKey encodes a timestamp as an 8-byte big-endian Unix-nanosecond
// key. Pre-1970 nanoseconds are clamped to 0: a negative int64 reinterpreted
// as uint64 would sort after every real sample instead of before them. Zero
// time (year 1) is out of UnixNano's representable range and is handled by
// QueryHistory's nil-seek fast path before reaching here.
func historyKey(ts time.Time) []byte {
	ns := ts.UnixNano()
	if ns < 0 {
		ns = 0
	}
	k := make([]byte, 8)
	binary.BigEndian.PutUint64(k, uint64(ns))
	return k
}

// PutHistoryBatch writes one timestamped sample per metric in a single
// transaction — one fsync per tick for all four metrics, the pattern that
// matters on the flash/SD-backed hosts this retention is opt-in for — and
// then prunes every sample older than cutoff in the same transaction, so
// retention enforcement rides along with the write instead of needing a
// second sweep.
//
// Steady state: once the retention window is full, each tick adds one key
// per metric and prunes roughly one, so the live-key count per metric
// stabilizes at retention/interval (defaults 24h / 10s = 8640). bbolt does
// not shrink its file — freed pages go to the freelist and are reused — so
// nitr.db plateaus at that high-water size rather than growing without
// bound.
func PutHistoryBatch(ts time.Time, samples map[string][]byte, cutoff time.Time) error {
	db, err := open()
	if err != nil {
		return err
	}
	return db.Update(func(tx *bolt.Tx) error {
		root, err := tx.CreateBucketIfNotExists([]byte(historyBucket))
		if err != nil {
			return fmt.Errorf("could not create history bucket: %w", err)
		}
		key := historyKey(ts)
		for metric, payload := range samples {
			b, err := root.CreateBucketIfNotExists([]byte(metric))
			if err != nil {
				return fmt.Errorf("could not create history bucket %q: %w", metric, err)
			}
			if err := b.Put(key, payload); err != nil {
				return fmt.Errorf("could not store %s history sample: %w", metric, err)
			}
		}
		return pruneHistoryTx(root, cutoff)
	})
}

// PruneHistory deletes every retained sample older than cutoff across all
// metric buckets. Retention normally rides along inside PutHistoryBatch;
// this exists for one-off cleanup (e.g. after the retention window is
// shortened in config) and for tests.
func PruneHistory(cutoff time.Time) error {
	db, err := open()
	if err != nil {
		return err
	}
	return db.Update(func(tx *bolt.Tx) error {
		root := tx.Bucket([]byte(historyBucket))
		if root == nil {
			return nil
		}
		return pruneHistoryTx(root, cutoff)
	})
}

// pruneHistoryTx deletes all keys older than cutoff from every metric bucket
// under root. Bucket names are collected before any deletion: mutating a
// bucket while ForEach iterates it is not safe in bbolt.
func pruneHistoryTx(root *bolt.Bucket, cutoff time.Time) error {
	var names [][]byte
	if err := root.ForEach(func(name, _ []byte) error {
		names = append(names, append([]byte(nil), name...))
		return nil
	}); err != nil {
		return err
	}
	cutoffKey := historyKey(cutoff)
	for _, name := range names {
		b := root.Bucket(name)
		if b == nil {
			continue // a plain value, not a nested metric bucket: nothing to prune
		}
		// Collect expired keys before deleting: removing the key under a
		// cursor shifts the remaining keys and the next Next() skips one,
		// so delete-while-iterating leaves every other expired key behind.
		var expired [][]byte
		c := b.Cursor()
		for k, _ := c.First(); k != nil && bytes.Compare(k, cutoffKey) < 0; k, _ = c.Next() {
			expired = append(expired, append([]byte(nil), k...))
		}
		for _, k := range expired {
			if err := b.Delete(k); err != nil {
				return fmt.Errorf("could not prune history bucket %q: %w", name, err)
			}
		}
	}
	return nil
}

// QueryHistory returns the retained samples for metric with from <= ts <= to,
// oldest first. A zero from scans from the oldest sample; a zero to means
// now. The result is always non-nil so an empty range marshals as [] rather
// than null.
func QueryHistory(metric string, from, to time.Time) ([]models.HistorySample, error) {
	samples := make([]models.HistorySample, 0)
	db, err := open()
	if err != nil {
		return samples, err
	}
	if to.IsZero() {
		to = time.Now()
	}
	max := historyKey(to)
	err = db.View(func(tx *bolt.Tx) error {
		root := tx.Bucket([]byte(historyBucket))
		if root == nil {
			return nil // retention never ran: empty range, not an error
		}
		b := root.Bucket([]byte(metric))
		if b == nil {
			return nil
		}
		c := b.Cursor()
		k, v := c.First()
		if !from.IsZero() {
			k, v = c.Seek(historyKey(from))
		}
		for ; k != nil && bytes.Compare(k, max) <= 0; k, v = c.Next() {
			ts := time.Unix(0, int64(binary.BigEndian.Uint64(k))).UTC()
			samples = append(samples, models.HistorySample{
				Timestamp: ts,
				Data:      append([]byte(nil), v...),
			})
		}
		return nil
	})
	if err != nil {
		return samples, fmt.Errorf("could not query %s history: %w", metric, err)
	}
	return samples, nil
}
