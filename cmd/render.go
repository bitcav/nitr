package cmd

import (
	"fmt"
	"io"
	"os"
	"reflect"
	"strconv"
	"strings"
	"text/tabwriter"
	"unicode"

	"github.com/mattn/go-isatty"
)

// isTTY reports whether w is a terminal. Used to decide whether to emit
// color codes and whether --watch can clear the screen between ticks --
// both are wrong to do into a pipe or redirect.
func isTTY(w io.Writer) bool {
	f, ok := w.(*os.File)
	if !ok {
		return false
	}
	return isatty.IsTerminal(f.Fd()) || isatty.IsCygwinTerminal(f.Fd())
}

// colorEnabled reports whether ANSI color should be applied to w: only when
// w is a terminal and NO_COLOR (https://no-color.org/) is unset. Piped or
// redirected output (the `--json` consumer's usual `| jq` case, or a plain
// redirect) never gets escape codes mixed into it.
func colorEnabled(w io.Writer) bool {
	if os.Getenv("NO_COLOR") != "" {
		return false
	}
	return isTTY(w)
}

const (
	ansiBold  = "\033[1m"
	ansiCyan  = "\033[36m"
	ansiReset = "\033[0m"
)

func colorize(enabled bool, code, s string) string {
	if !enabled || s == "" {
		return s
	}
	return code + s + ansiReset
}

// renderValue writes v to w as human-readable text: a struct becomes an
// aligned key/value listing, a slice of structs becomes a table, and
// anything else falls back to fmt's default formatting.
func renderValue(w io.Writer, v any) error {
	color := colorEnabled(w)
	rv := reflect.ValueOf(v)
	for rv.Kind() == reflect.Pointer {
		if rv.IsNil() {
			_, err := fmt.Fprintln(w, "(none)")
			return err
		}
		rv = rv.Elem()
	}

	switch rv.Kind() {
	case reflect.Slice, reflect.Array:
		return renderTable(w, rv, color)
	case reflect.Struct:
		tw := tabwriter.NewWriter(w, 0, 2, 2, ' ', 0)
		renderObject(tw, rv, 0, color)
		return tw.Flush()
	default:
		_, err := fmt.Fprintln(w, v)
		return err
	}
}

// renderObject prints one "Label:\tvalue" line per exported field of rv onto
// tw, recursing (with deeper indent) into nested structs such as
// overview.Overview's embedded HostInfo and RAM. Write errors are dropped
// here because tw retains the first one and the caller's Flush reports it.
func renderObject(tw *tabwriter.Writer, rv reflect.Value, indent int, color bool) {
	prefix := strings.Repeat("  ", indent)
	t := rv.Type()
	for i := 0; i < rv.NumField(); i++ {
		f := t.Field(i)
		if f.PkgPath != "" { // unexported
			continue
		}
		fv := rv.Field(i)
		label := colorize(color, ansiBold+ansiCyan, fieldLabel(f))
		if fv.Kind() == reflect.Struct {
			_, _ = fmt.Fprintf(tw, "%s%s:\n", prefix, label)
			renderObject(tw, fv, indent+1, color)
			continue
		}
		_, _ = fmt.Fprintf(tw, "%s%s:\t%s\n", prefix, label, formatScalar(fv))
	}
}

// renderTable prints rv (a slice) as a table: one column per exported field
// of its element type, one row per element. A slice of anything other than
// structs (or pointers to structs) falls back to one value per line.
func renderTable(w io.Writer, rv reflect.Value, color bool) error {
	if rv.Len() == 0 {
		_, err := fmt.Fprintln(w, "(none)")
		return err
	}

	elemType := rv.Type().Elem()
	for elemType.Kind() == reflect.Pointer {
		elemType = elemType.Elem()
	}
	if elemType.Kind() != reflect.Struct {
		tw := tabwriter.NewWriter(w, 0, 2, 2, ' ', 0)
		for i := 0; i < rv.Len(); i++ {
			_, _ = fmt.Fprintln(tw, formatScalar(rv.Index(i)))
		}
		return tw.Flush()
	}

	var headers []string
	var fieldIdx []int
	for i := 0; i < elemType.NumField(); i++ {
		f := elemType.Field(i)
		if f.PkgPath != "" {
			continue
		}
		headers = append(headers, fieldLabel(f))
		fieldIdx = append(fieldIdx, i)
	}

	tw := tabwriter.NewWriter(w, 0, 2, 2, ' ', 0)
	headerCells := make([]string, len(headers))
	for i, h := range headers {
		headerCells[i] = colorize(color, ansiBold+ansiCyan, h)
	}
	_, _ = fmt.Fprintln(tw, strings.Join(headerCells, "\t"))

	for i := 0; i < rv.Len(); i++ {
		row := rv.Index(i)
		for row.Kind() == reflect.Pointer {
			row = row.Elem()
		}
		cells := make([]string, len(fieldIdx))
		for j, fi := range fieldIdx {
			cells[j] = formatScalar(row.Field(fi))
		}
		_, _ = fmt.Fprintln(tw, strings.Join(cells, "\t"))
	}
	return tw.Flush()
}

// formatScalar renders a single field value as CLI-friendly text: floats
// are fixed to 2 decimals instead of Go's default %v precision, slices and
// structs (e.g. a network device's []address or process' nested fields)
// collapse to a compact inline summary instead of a Go-syntax dump.
func formatScalar(fv reflect.Value) string {
	switch fv.Kind() {
	case reflect.Slice, reflect.Array:
		if fv.Len() == 0 {
			return ""
		}
		parts := make([]string, fv.Len())
		for i := 0; i < fv.Len(); i++ {
			parts[i] = formatScalar(fv.Index(i))
		}
		return strings.Join(parts, ", ")
	case reflect.Pointer:
		if fv.IsNil() {
			return ""
		}
		return formatScalar(fv.Elem())
	case reflect.Struct:
		if s, ok := fv.Interface().(fmt.Stringer); ok {
			return s.String()
		}
		t := fv.Type()
		var parts []string
		for i := 0; i < fv.NumField(); i++ {
			if t.Field(i).PkgPath != "" {
				continue
			}
			parts = append(parts, formatScalar(fv.Field(i)))
		}
		return strings.Join(parts, " ")
	case reflect.Float32, reflect.Float64:
		return strconv.FormatFloat(fv.Float(), 'f', 2, 64)
	case reflect.Bool:
		return strconv.FormatBool(fv.Bool())
	default:
		return fmt.Sprint(fv.Interface())
	}
}

// fieldLabel turns a struct field into a human label: it prefers the json
// tag (so labels track the API's field names, not Go's) and expands
// camelCase into space-separated title case ("clockSpeed" -> "Clock Speed").
func fieldLabel(f reflect.StructField) string {
	name := f.Name
	if tag := f.Tag.Get("json"); tag != "" && tag != "-" {
		if base, _, _ := strings.Cut(tag, ","); base != "" {
			name = base
		}
	}
	return camelToTitle(name)
}

func camelToTitle(s string) string {
	var b strings.Builder
	runes := []rune(s)
	for i, r := range runes {
		if i > 0 && unicode.IsUpper(r) && !unicode.IsUpper(runes[i-1]) {
			b.WriteByte(' ')
		}
		if i == 0 {
			b.WriteRune(unicode.ToUpper(r))
		} else {
			b.WriteRune(r)
		}
	}
	return b.String()
}
