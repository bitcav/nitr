package cmd

import (
	"bytes"
	"reflect"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type renderStruct struct {
	Vendor     string  `json:"vendor"`
	ClockSpeed float64 `json:"clockSpeed"`
	UsageEach  []float64
}

func TestRenderValueStruct(t *testing.T) {
	var buf bytes.Buffer
	require.NoError(t, renderValue(&buf, renderStruct{Vendor: "Intel", ClockSpeed: 4200, UsageEach: []float64{1.5, 2.25}}))
	out := buf.String()
	assert.Contains(t, out, "Vendor:")
	assert.Contains(t, out, "Intel")
	// "clockSpeed" json tag must expand to "Clock Speed", not the Go field name.
	assert.Contains(t, out, "Clock Speed:")
	assert.Contains(t, out, "4200.00")
	assert.Contains(t, out, "1.50, 2.25")
}

type renderRow struct {
	Name    string `json:"name"`
	Percent float64
}

func TestRenderValueSliceOfStructsIsATable(t *testing.T) {
	var buf bytes.Buffer
	rows := []renderRow{{Name: "sda", Percent: 12.5}, {Name: "sdb", Percent: 99}}
	require.NoError(t, renderValue(&buf, rows))
	lines := strings.Split(strings.TrimRight(buf.String(), "\n"), "\n")
	require.Len(t, lines, 3) // header + 2 rows
	assert.Contains(t, lines[0], "Name")
	assert.Contains(t, lines[0], "Percent")
	assert.Contains(t, lines[1], "sda")
	assert.Contains(t, lines[1], "12.50")
	assert.Contains(t, lines[2], "sdb")
}

func TestRenderValueEmptySlice(t *testing.T) {
	var buf bytes.Buffer
	require.NoError(t, renderValue(&buf, []renderRow{}))
	assert.Equal(t, "(none)\n", buf.String())
}

func TestRenderValueNestedStruct(t *testing.T) {
	type inner struct {
		Name string `json:"name"`
	}
	type outer struct {
		Inner inner `json:"inner"`
		Count int   `json:"count"`
	}
	var buf bytes.Buffer
	require.NoError(t, renderValue(&buf, outer{Inner: inner{Name: "eth0"}, Count: 3}))
	out := buf.String()
	assert.Contains(t, out, "Inner:")
	assert.Contains(t, out, "Name:")
	assert.Contains(t, out, "eth0")
	assert.Contains(t, out, "Count:")
	assert.Contains(t, out, "3")
}

func TestFieldLabelPrefersJSONTagAndExpandsCamelCase(t *testing.T) {
	rt := reflect.TypeFor[renderStruct]()
	f, ok := rt.FieldByName("ClockSpeed")
	require.True(t, ok)
	assert.Equal(t, "Clock Speed", fieldLabel(f))

	f, ok = rt.FieldByName("UsageEach")
	require.True(t, ok)
	// no json tag on this field -> falls back to the Go field name, still humanized
	assert.Equal(t, "Usage Each", fieldLabel(f))
}

func TestCamelToTitle(t *testing.T) {
	cases := map[string]string{
		"cpuUsage":   "Cpu Usage",
		"vendor":     "Vendor",
		"mountPoint": "Mount Point",
		"":           "",
	}
	for in, want := range cases {
		assert.Equal(t, want, camelToTitle(in), in)
	}
}

func TestColorizeNoopWhenDisabled(t *testing.T) {
	assert.Equal(t, "text", colorize(false, ansiBold, "text"))
	assert.Equal(t, ansiBold+"text"+ansiReset, colorize(true, ansiBold, "text"))
	assert.Equal(t, "", colorize(true, ansiBold, ""))
}

// TestColorEnabledFalseForNonFileWriter proves color never leaks into a
// buffer/pipe writer that isn't *os.File (the shape every test's output
// capture uses), regardless of NO_COLOR.
func TestColorEnabledFalseForNonFileWriter(t *testing.T) {
	var buf bytes.Buffer
	assert.False(t, colorEnabled(&buf))
}

func TestIsTTYFalseForNonFileWriter(t *testing.T) {
	var buf bytes.Buffer
	assert.False(t, isTTY(&buf))
}
