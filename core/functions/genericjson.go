package functions

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"text/template"
	"time"

	"github.com/itchyny/gojq"
	log "github.com/sirupsen/logrus"
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

	// Body is the request body sent with the HTTP request. When non-empty
	// AND Headers does not already set Content-Type, the request goes out
	// as application/json. Empty body sends no payload and no
	// Content-Type header.
	Body string `toml:"body"`

	// Headers are extra request headers, keyed by header name.
	Headers map[string]string `toml:"headers"`

	// Template is a Go text/template rendered against the decoded JSON.
	// See genericJSONFuncs for available helpers.
	Template string `toml:"tmpl"`

	// Store extracts values from the decoded response into per-call flow
	// variables, keyed by variable name → jq expression. The jq string is
	// first rendered as a Go template over `.Vars` (so it can reference
	// prior state, e.g. select(.userId == {{.Vars.member.id}})), then run
	// against the decoded JSON. A single jq result is stored as-is; zero
	// results store nil; multiple results store a slice. The controller
	// merges the result into the call's `.Vars`, which later nodes read via
	// their URL / body / template. This is what lets genericjson nodes be
	// chained into a stateful flow.
	Store map[string]string `toml:"store"`

	// TimeoutSeconds caps the HTTP request. Defaults to defaultTimeout
	// (10 s) when <= 0. Cancelled early if the parent context fires
	// (e.g. caller hangs up).
	TimeoutSeconds int `toml:"timeout_seconds"`

	// TTS overrides the voice/lang/engine/provider for the rendered output.
	// Empty fields fall back to the definition defaults via SetDefault.
	TTS TTS `toml:"tts"`
}

const (
	// defaultTimeout caps a GenericJSON HTTP request when the node leaves
	// TimeoutSeconds unset. Picked to stay well under typical IVR caller
	// patience while still letting slow upstream APIs finish.
	defaultTimeout = 10 * time.Second

	// maxBodyBytes caps how much of the response we'll read into memory.
	// Anything larger almost certainly isn't intended as an IVR readout
	// source; we surface a clear error instead of OOMing the process.
	maxBodyBytes = 8 << 20 // 8 MiB

	// errorBodyPeek is the byte-count we surface in *log* output (never
	// in error strings the caller hears) when a response is non-2xx.
	// Capped tight to make accidental token/PII exposure in logs less
	// painful — the full body still lives in upstream observability if
	// needed.
	errorBodyPeek = 256
)

// httpClient is shared across all GenericJSON nodes so connection pooling
// and DNS caching work. Per-call timeouts are layered on via
// context.WithTimeout rather than swapping the Timeout field, which would
// race when two calls fire simultaneously.
var httpClient = &http.Client{}

// FetchAndRender fetches the configured endpoint, decodes the JSON, renders
// the template against it, and extracts any Store variables. It returns the
// rendered text (ready for TTS) and the map of variables to merge into the
// call's flow state. `vars` is the current per-call state, exposed to the
// URL / body / header / template / store expressions as `.Vars`. ctx scopes
// the fetch + JQ work to the current call so a hangup cancels the request
// immediately instead of waiting on the upstream timeout.
func (g *GenericJSON) FetchAndRender(ctx context.Context, vars map[string]any) (rendered string, stored map[string]any, err error) {
	if strings.TrimSpace(g.Template) == "" {
		return "", nil, fmt.Errorf("genericjson: missing template")
	}

	data, status, raw, err := fetchJSON(ctx, fetchSpec{
		URL:            g.URL,
		Method:         g.Method,
		Body:           g.Body,
		Headers:        g.Headers,
		TimeoutSeconds: g.TimeoutSeconds,
	}, vars)
	if err != nil {
		return "", nil, err
	}

	tmpl, err := template.New("genericjson").Funcs(genericJSONFuncs).Parse(g.Template)
	if err != nil {
		return "", nil, fmt.Errorf("genericjson: parse template: %w", err)
	}
	var out bytes.Buffer
	if err := tmpl.Execute(&out, map[string]any{
		"Data":   data,
		"Status": status,
		"Raw":    raw,
		"Vars":   vars,
	}); err != nil {
		return "", nil, fmt.Errorf("genericjson: render template: %w", err)
	}

	stored, err = extractStore(g.Store, data, vars)
	if err != nil {
		return "", nil, err
	}
	return out.String(), stored, nil
}

// fetchSpec is the subset of a node's config needed to perform an HTTP JSON
// fetch, with .Vars templating of url/body/headers and shared
// decode/limits/redaction. (Scripts use DoScriptRequest instead — no
// templating and 4xx/5xx returned rather than errored.)
type fetchSpec struct {
	URL            string
	Method         string
	Body           string
	Headers        map[string]string
	TimeoutSeconds int
}

// fetchJSON renders url/body/header-values against `.Vars`, performs the HTTP
// request, and returns the decoded JSON (untyped), the status, and the raw
// body. Errors mirror the original GenericJSON behavior (status-only on 4xx/5xx
// so tokens in the body don't leak into TTS/logs).
func fetchJSON(ctx context.Context, spec fetchSpec, vars map[string]any) (data any, status int, raw string, err error) {
	urlStr, err := renderTemplateString(spec.URL, vars)
	if err != nil {
		return nil, 0, "", fmt.Errorf("genericjson: render url: %w", err)
	}
	if strings.TrimSpace(urlStr) == "" {
		return nil, 0, "", fmt.Errorf("genericjson: missing url")
	}

	method := strings.ToUpper(strings.TrimSpace(spec.Method))
	if method == "" {
		method = "GET"
	}

	timeout := defaultTimeout
	if spec.TimeoutSeconds > 0 {
		timeout = time.Duration(spec.TimeoutSeconds) * time.Second
	}
	reqCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	bodyStr, err := renderTemplateString(spec.Body, vars)
	if err != nil {
		return nil, 0, "", fmt.Errorf("genericjson: render body: %w", err)
	}
	var body io.Reader
	if bodyStr != "" {
		body = strings.NewReader(bodyStr)
	}

	req, err := http.NewRequestWithContext(reqCtx, method, urlStr, body)
	if err != nil {
		return nil, 0, "", fmt.Errorf("genericjson: build request: %w", err)
	}
	if body != nil && req.Header.Get("Content-Type") == "" {
		req.Header.Set("Content-Type", "application/json")
	}
	req.Header.Set("Accept", "application/json")
	for k, v := range spec.Headers {
		hv, herr := renderTemplateString(v, vars)
		if herr != nil {
			return nil, 0, "", fmt.Errorf("genericjson: render header %q: %w", k, herr)
		}
		req.Header.Set(k, hv)
	}

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, 0, "", fmt.Errorf("genericjson: %s %s: %w", method, redactURL(urlStr), err)
	}
	defer resp.Body.Close()

	rawBytes, err := io.ReadAll(io.LimitReader(resp.Body, maxBodyBytes))
	if err != nil {
		return nil, 0, "", fmt.Errorf("genericjson: read body: %w", err)
	}
	if resp.StatusCode >= 400 {
		// Surface only status + host in the error (which checkError will
		// log AND speak). The body often contains auth tokens, OAuth
		// error_description, request IDs — we capture it at Trace level
		// for debugging instead of leaking it into TTS audio / logs.
		log.WithFields(log.Fields{
			"status": resp.StatusCode,
			"url":    redactURL(urlStr),
			"body":   truncateBody(string(rawBytes), errorBodyPeek),
		}).Trace("genericjson: non-2xx response")
		return nil, resp.StatusCode, "", fmt.Errorf("genericjson: %s %s: HTTP %d", method, redactURL(urlStr), resp.StatusCode)
	}

	if len(bytes.TrimSpace(rawBytes)) > 0 {
		// Plain decode (no UseNumber): gojq operates on map[string]any /
		// []any / float64 / string / bool / nil, so we have to feed it
		// those exact types. Large integers (>2^53) lose precision here,
		// which is fine for IVR readouts but worth noting.
		if err := json.Unmarshal(rawBytes, &data); err != nil {
			return nil, resp.StatusCode, "", fmt.Errorf("genericjson: decode JSON: %w", err)
		}
	}
	return data, resp.StatusCode, string(rawBytes), nil
}

// DoScriptRequest performs an HTTP request on behalf of a Script node and
// returns the decoded JSON (nil if the body isn't valid JSON), the status
// code, and the raw body. Unlike fetchJSON it does NOT template the inputs
// (the JS caller already built the final url/body/headers) and does NOT treat
// 4xx/5xx as an error — it hands the status back so the script can branch on
// it. A transport-level failure (DNS, connection, timeout) still returns err.
// The shared httpClient + maxBodyBytes cap are reused.
func DoScriptRequest(ctx context.Context, method, urlStr, body string, headers map[string]string, timeout time.Duration) (data any, status int, raw string, err error) {
	if strings.TrimSpace(urlStr) == "" {
		return nil, 0, "", fmt.Errorf("script http: missing url")
	}
	method = strings.ToUpper(strings.TrimSpace(method))
	if method == "" {
		method = "GET"
	}
	if timeout <= 0 {
		timeout = defaultTimeout
	}
	reqCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	var bodyR io.Reader
	if body != "" {
		bodyR = strings.NewReader(body)
	}
	req, err := http.NewRequestWithContext(reqCtx, method, urlStr, bodyR)
	if err != nil {
		return nil, 0, "", fmt.Errorf("script http: build request: %w", err)
	}
	if bodyR != nil && req.Header.Get("Content-Type") == "" {
		req.Header.Set("Content-Type", "application/json")
	}
	req.Header.Set("Accept", "application/json")
	for k, v := range headers {
		req.Header.Set(k, v)
	}

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, 0, "", fmt.Errorf("script http: %s %s: %w", method, redactURL(urlStr), err)
	}
	defer resp.Body.Close()

	rawBytes, err := io.ReadAll(io.LimitReader(resp.Body, maxBodyBytes))
	if err != nil {
		return nil, resp.StatusCode, "", fmt.Errorf("script http: read body: %w", err)
	}
	if len(bytes.TrimSpace(rawBytes)) > 0 {
		// Best-effort decode; a non-JSON body leaves data nil and the caller
		// falls back to the raw text.
		_ = json.Unmarshal(rawBytes, &data)
	}
	return data, resp.StatusCode, string(rawBytes), nil
}

// renderTemplateString renders a Go template string over `.Vars`, reusing the
// genericjson helper funcs. Strings with no template actions are returned
// unchanged (fast path), so plain URLs/bodies pay nothing.
func renderTemplateString(s string, vars map[string]any) (string, error) {
	if !strings.Contains(s, "{{") {
		return s, nil
	}
	t, err := template.New("s").Funcs(genericJSONFuncs).Parse(s)
	if err != nil {
		return "", err
	}
	var b bytes.Buffer
	if err := t.Execute(&b, map[string]any{"Vars": vars}); err != nil {
		return "", err
	}
	return b.String(), nil
}

// extractStore evaluates each Store jq expression (first rendered as a template
// over `.Vars`) against the decoded data and returns the resulting variable
// map. One jq result → the value; zero → nil; many → a slice.
func extractStore(store map[string]string, data any, vars map[string]any) (map[string]any, error) {
	if len(store) == 0 {
		return nil, nil
	}
	out := make(map[string]any, len(store))
	for name, expr := range store {
		rendered, err := renderTemplateString(expr, vars)
		if err != nil {
			return nil, fmt.Errorf("genericjson: store %q render: %w", name, err)
		}
		results, err := runJQ(data, rendered)
		if err != nil {
			return nil, fmt.Errorf("genericjson: store %q jq %q: %w", name, rendered, err)
		}
		switch len(results) {
		case 0:
			out[name] = nil
		case 1:
			out[name] = results[0]
		default:
			out[name] = results
		}
	}
	return out, nil
}

// redactURL strips the query string and any userinfo from a URL before
// it lands in an error/log. The path stays — usually informative — but
// query params and basic-auth credentials are common token-carrying
// surfaces we don't want speaking through the TTS or showing up in logs.
func redactURL(raw string) string {
	u, err := url.Parse(raw)
	if err != nil {
		return raw
	}
	u.User = nil
	u.RawQuery = ""
	u.Fragment = ""
	return u.String()
}

func truncateBody(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
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

// toFloat coerces a value into a float64. We feed JSON through plain
// json.Unmarshal (no UseNumber) so numbers always arrive as float64
// here — the other cases cover hand-constructed inputs from Go code
// and the string/bool conversions templates frequently need.
func toFloat(v any) float64 {
	switch t := v.(type) {
	case nil:
		return 0
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
