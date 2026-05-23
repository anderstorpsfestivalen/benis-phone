package functions

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"text/template"
	"time"

	"github.com/itchyny/gojq"
)

// GenericJSON describes a node that fetches an HTTP(S) endpoint returning
// JSON and reads it out via TTS using a Go text/template. The response is
// decoded into untyped JSON (map[string]any / []any / primitives) and bound
// as `.Data` inside the template. `.Status` and `.Raw` are also available.
//
// Two ways to walk the JSON:
//
//   - text/template's built-in dot syntax + range, for simple shapes:
//     `{{range .Data.items}}{{.name}}: {{.value}}; {{end}}`
//
//   - The `jq` helper, which runs a full jq expression (via
//     itchyny/gojq) — use this for filter/select/map/transform:
//     `{{jq .Data ".[] | select(.name == \"Summalajnen\") | .temperature"}}`
//
// See genericJSONFuncs below for the full set of helpers.
//
// TOML example:
//
//	[[fn.actions]]
//	num = 1
//	[fn.actions.genericjson]
//	url = "https://api.example.com/sensor"
//	method = "GET"
//	tmpl = """The temperature is {{int .Data.temp}} celsius."""
type GenericJSON struct {
	// URL is the HTTP(S) endpoint to fetch.
	URL string `toml:"url"`

	// Method is the HTTP method (GET, POST, …). Defaults to GET when empty.
	Method string `toml:"method"`

	// Body is the request body sent with non-GET methods. Sent as
	// application/json unless overridden via Headers.
	Body string `toml:"body"`

	// Headers are extra request headers, keyed by header name.
	Headers map[string]string `toml:"headers"`

	// Template is a Go text/template rendered against the decoded JSON.
	// See genericJSONFuncs for available helpers.
	Template string `toml:"tmpl"`

	// TimeoutSeconds caps the HTTP request. Defaults to 10s when <=0.
	TimeoutSeconds int `toml:"timeout_seconds"`

	// TTS overrides the voice/lang/engine/provider for the rendered output.
	// Empty fields fall back to the definition defaults via SetDefault.
	TTS TTS `toml:"tts"`
}

// httpClient is shared across all GenericJSON nodes so connection pooling
// and DNS caching work as expected.
var httpClient = &http.Client{Timeout: 30 * time.Second}

// FetchAndRender fetches the configured endpoint, decodes the JSON, and
// renders the template against it. The rendered text is returned ready to
// hand to the TTS engine.
func (g *GenericJSON) FetchAndRender() (string, error) {
	if strings.TrimSpace(g.URL) == "" {
		return "", fmt.Errorf("genericjson: missing url")
	}
	if strings.TrimSpace(g.Template) == "" {
		return "", fmt.Errorf("genericjson: missing template")
	}

	method := strings.ToUpper(strings.TrimSpace(g.Method))
	if method == "" {
		method = "GET"
	}

	var body io.Reader
	if g.Body != "" {
		body = strings.NewReader(g.Body)
	}

	req, err := http.NewRequest(method, g.URL, body)
	if err != nil {
		return "", fmt.Errorf("genericjson: build request: %w", err)
	}
	if body != nil && req.Header.Get("Content-Type") == "" {
		req.Header.Set("Content-Type", "application/json")
	}
	req.Header.Set("Accept", "application/json")
	for k, v := range g.Headers {
		req.Header.Set(k, v)
	}

	client := httpClient
	if g.TimeoutSeconds > 0 {
		client = &http.Client{Timeout: time.Duration(g.TimeoutSeconds) * time.Second}
	}

	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("genericjson: %s %s: %w", method, g.URL, err)
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(io.LimitReader(resp.Body, 8<<20)) // 8 MiB cap
	if err != nil {
		return "", fmt.Errorf("genericjson: read body: %w", err)
	}
	if resp.StatusCode >= 400 {
		return "", fmt.Errorf("genericjson: %s %s: HTTP %d: %s", method, g.URL, resp.StatusCode, truncateForError(raw))
	}

	var data any
	if len(bytes.TrimSpace(raw)) > 0 {
		// Plain decode (no UseNumber): gojq operates on map[string]any /
		// []any / float64 / string / bool / nil, so we have to feed it
		// those exact types. Large integers (>2^53) lose precision here,
		// which is fine for IVR readouts but worth noting.
		if err := json.Unmarshal(raw, &data); err != nil {
			return "", fmt.Errorf("genericjson: decode JSON: %w", err)
		}
	}

	tmpl, err := template.New("genericjson").Funcs(genericJSONFuncs).Parse(g.Template)
	if err != nil {
		return "", fmt.Errorf("genericjson: parse template: %w", err)
	}

	var out bytes.Buffer
	if err := tmpl.Execute(&out, map[string]any{
		"Data":   data,
		"Status": resp.StatusCode,
		"Raw":    string(raw),
	}); err != nil {
		return "", fmt.Errorf("genericjson: render template: %w", err)
	}
	return out.String(), nil
}

func truncateForError(b []byte) string {
	const max = 256
	if len(b) <= max {
		return string(b)
	}
	return string(b[:max]) + "…"
}

// genericJSONFuncs are the helpers exposed inside the template. The star
// of the show is `jq`, which runs a real jq expression (via itchyny/gojq)
// against the parsed JSON tree. That covers filter/select/map/iterate —
// the whole jq vocabulary — so callers can write things like:
//
//	{{jq .Data ".[] | select(.name == \"Summalajnen\") | .temperature"}}
//
// `jq` returns the first result and ignores the rest; `jqAll` collects
// every result into a slice for `{{range}}`.
//
// The other helpers (int, default, add, …) are quality-of-life
// converters/formatters that make raw JSON values nicer to speak aloud.
var genericJSONFuncs = template.FuncMap{
	"int":     toInt,
	"float":   toFloat,
	"round":   toInt, // alias; rounds-half-toward-zero via toInt
	"num":     toFloat,
	"str":     toString,
	"default": defaultFn,
	"jq":      jqFirst,
	"jqAll":   jqAll,
	"jqall":   jqAll, // case-insensitive alias
	"first":   firstFn,
	"last":    lastFn,
	"join":    joinFn,
	"add":     addFn,
	"sub":     subFn,
	"mul":     mulFn,
	"div":     divFn,
	"keys":    keysFn,
	"length":  lengthFn,
}

// toFloat coerces any of (json.Number, float*, int*, string, bool) into a
// float64. Returns 0 for nil/unknown.
func toFloat(v any) float64 {
	switch t := v.(type) {
	case nil:
		return 0
	case json.Number:
		f, _ := t.Float64()
		return f
	case float64:
		return t
	case float32:
		return float64(t)
	case int:
		return float64(t)
	case int64:
		return float64(t)
	case int32:
		return float64(t)
	case uint64:
		return float64(t)
	case bool:
		if t {
			return 1
		}
		return 0
	case string:
		f, _ := strconv.ParseFloat(strings.TrimSpace(t), 64)
		return f
	}
	return 0
}

// toInt rounds toFloat to the nearest int.
func toInt(v any) int {
	f := toFloat(v)
	if f >= 0 {
		return int(f + 0.5)
	}
	return int(f - 0.5)
}

func toString(v any) string {
	switch t := v.(type) {
	case nil:
		return ""
	case string:
		return t
	case json.Number:
		return t.String()
	case bool:
		return strconv.FormatBool(t)
	case float64:
		return strconv.FormatFloat(t, 'f', -1, 64)
	}
	return fmt.Sprintf("%v", v)
}

// defaultFn returns the fallback when value is "empty" (nil, "", 0, false,
// empty slice/map). Useful for "no data" branches that should still speak.
func defaultFn(fallback any, value any) any {
	if isEmpty(value) {
		return fallback
	}
	return value
}

func isEmpty(v any) bool {
	switch t := v.(type) {
	case nil:
		return true
	case string:
		return t == ""
	case bool:
		return !t
	case json.Number:
		return t.String() == "" || t.String() == "0"
	case float64:
		return t == 0
	case []any:
		return len(t) == 0
	case map[string]any:
		return len(t) == 0
	}
	return false
}

// jqFirst runs a jq expression against the given input and returns the
// first result (or nil if there are no results / the query errors). This
// is the workhorse template helper — it accepts the full jq language:
// `select`, `map`, `|`, comparisons, arithmetic, string interpolation,
// builtins like length / keys / sort_by, etc.
//
// Example: filter an array by a field and read another:
//
//	{{jq .Data ".[] | select(.name == \"Summalajnen\") | .temperature"}}
//
// Errors and "no result" both collapse to nil so templates can use the
// output inside `{{if}}` and `default` without panicking.
func jqFirst(input any, query string) any {
	results, err := runJQ(input, query)
	if err != nil || len(results) == 0 {
		return nil
	}
	return results[0]
}

// jqAll returns every value produced by the query, so callers can
// `{{range jqAll .Data ".[] | select(.active)"}}{{.name}}{{end}}`.
func jqAll(input any, query string) []any {
	results, err := runJQ(input, query)
	if err != nil {
		return nil
	}
	return results
}

func runJQ(input any, query string) ([]any, error) {
	q, err := gojq.Parse(query)
	if err != nil {
		return nil, fmt.Errorf("genericjson: jq parse %q: %w", query, err)
	}
	iter := q.Run(input)
	var out []any
	for {
		v, ok := iter.Next()
		if !ok {
			break
		}
		if e, isErr := v.(error); isErr {
			return out, e
		}
		out = append(out, v)
	}
	return out, nil
}

func firstFn(v any) any {
	if arr, ok := v.([]any); ok && len(arr) > 0 {
		return arr[0]
	}
	return nil
}

func lastFn(v any) any {
	if arr, ok := v.([]any); ok && len(arr) > 0 {
		return arr[len(arr)-1]
	}
	return nil
}

// joinFn glues a slice with a separator. Slice elements are coerced to
// strings via toString so mixed-type arrays still print sensibly.
func joinFn(sep string, v any) string {
	arr, ok := v.([]any)
	if !ok {
		return ""
	}
	parts := make([]string, 0, len(arr))
	for _, e := range arr {
		parts = append(parts, toString(e))
	}
	return strings.Join(parts, sep)
}

func addFn(a, b any) float64 { return toFloat(a) + toFloat(b) }
func subFn(a, b any) float64 { return toFloat(a) - toFloat(b) }
func mulFn(a, b any) float64 { return toFloat(a) * toFloat(b) }
func divFn(a, b any) float64 {
	bv := toFloat(b)
	if bv == 0 {
		return 0
	}
	return toFloat(a) / bv
}

func keysFn(v any) []string {
	m, ok := v.(map[string]any)
	if !ok {
		return nil
	}
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}

func lengthFn(v any) int {
	switch t := v.(type) {
	case []any:
		return len(t)
	case map[string]any:
		return len(t)
	case string:
		return len(t)
	}
	return 0
}
