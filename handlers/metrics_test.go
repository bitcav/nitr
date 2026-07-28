package handlers

import (
	"net/http/httptest"
	"testing"

	"github.com/prometheus/common/expfmt"
	"github.com/prometheus/common/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestMetricsExpositionFormat hits /metrics behind the x-api-key middleware,
// parses the body with the same expfmt parser Prometheus tooling uses, and
// checks the expected metric families are present with valid help/type
// metadata. Asserting on parsed families (rather than raw strings) catches
// malformed exposition that scrapers reject with confusing errors.
func TestMetricsExpositionFormat(t *testing.T) {
	setupEnv(t)
	app := newTestApp()
	app.Get("/metrics", AuthAPI, Metrics)

	req := httptest.NewRequest("GET", "/metrics", nil)
	req.Header.Set("x-api-key", "testapikey")
	resp, err := app.Test(req, 30000)
	require.NoError(t, err)
	require.Equal(t, 200, resp.StatusCode)

	parser := expfmt.NewTextParser(model.LegacyValidation)
	families, err := (&parser).TextToMetricFamilies(resp.Body)
	require.NoError(t, err, "body must parse as Prometheus exposition format")

	type expected struct {
		name   string
		mtype  string // Prometheus MetricType.String()
		labels []string
	}
	for _, e := range []expected{
		{"nitr_cpu_seconds_total", "COUNTER", []string{"cpu", "mode"}},
		{"nitr_ram_total_bytes", "GAUGE", nil},
		{"nitr_ram_free_bytes", "GAUGE", nil},
		{"nitr_ram_used_bytes", "GAUGE", nil},
		{"nitr_disk_free_bytes", "GAUGE", []string{"mountpoint"}},
		{"nitr_disk_size_bytes", "GAUGE", []string{"mountpoint"}},
		{"nitr_disk_used_bytes", "GAUGE", []string{"mountpoint"}},
	} {
		mf, ok := families[e.name]
		require.Truef(t, ok, "expected metric family %q", e.name)
		assert.Equal(t, e.mtype, mf.GetType().String(), "metric %q type", e.name)
		assert.NotEmpty(t, mf.GetHelp(), "metric %q must carry a HELP line", e.name)

		want := map[string]bool{}
		for _, l := range e.labels {
			want[l] = true
		}
		require.NotEmpty(t, mf.GetMetric(), "metric %q emitted no samples on this host", e.name)
		for _, m := range mf.GetMetric() {
			got := map[string]bool{}
			for _, lp := range m.GetLabel() {
				got[lp.GetName()] = true
			}
			for l := range want {
				assert.Truef(t, got[l], "metric %q missing label %q", e.name, l)
			}
		}
	}
}

func TestMetricsRequiresAPIKey(t *testing.T) {
	setupEnv(t)
	app := newTestApp()
	app.Get("/metrics", AuthAPI, Metrics)

	resp := get(t, app, "/metrics")
	assert.Equal(t, 401, resp.StatusCode)
	_ = body(t, resp)
}
