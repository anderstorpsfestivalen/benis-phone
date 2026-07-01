package functions

import (
	"bytes"
	"context"
	"fmt"
	"strings"
	"text/template"
)

// ListMenu is a dynamic menu built at call time from a fetched JSON array —
// the one thing a static Fn/Action menu can't express, since the number of
// options and the key→value mapping depend on runtime data. It fetches an
// endpoint (like GenericJSON, with the same .Vars templating of url/body/
// headers), selects an array via a jq expression, speaks a numbered menu
// ("Tryck 1 för …, Tryck 2 för …"), and on selection stores the chosen item
// into a flow variable and advances to Dst.
//
// TOML example:
//
//	listmenu = {
//	  url = "https://beer.anderstorpsfestivalen.se/api/members",
//	  label = "{{.name}}",
//	  store = "member",
//	  dst = "beer_bac",
//	}
type ListMenu struct {
	// Fetch config — identical semantics to GenericJSON (url/body/header
	// values are rendered over .Vars before the request).
	URL            string            `toml:"url"`
	Method         string            `toml:"method"`
	Body           string            `toml:"body"`
	Headers        map[string]string `toml:"headers"`
	TimeoutSeconds int               `toml:"timeout_seconds"`

	// List is a jq expression (rendered over .Vars first) selecting the array
	// to build options from. Defaults to "." (the whole response). A single
	// array result is used as the items; multiple results are each an item.
	List string `toml:"list"`

	// Label is a Go template rendered per item (the item is the root dot) to
	// produce the spoken option name, e.g. "{{.name}}".
	Label string `toml:"label"`

	// Intro is spoken before the options (rendered over .Vars). Optional.
	Intro string `toml:"intro"`

	// Option is the per-option phrase template. Context: {Num, Label}.
	// Defaults to Swedish "Tryck {{.Num}} för {{.Label}}. ".
	Option string `toml:"option"`

	// Store is the flow-variable name the selected item is saved under.
	Store string `toml:"store"`

	// Dst is the fn entered after a selection.
	Dst string `toml:"dst"`

	// Max caps how many options are offered (DTMF 1-9). Defaults to 9.
	Max int `toml:"max"`

	// TTS overrides the voice/lang/engine/provider for the spoken menu.
	TTS TTS `toml:"tts"`
}

const defaultOptionTemplate = "Tryck {{.Num}} för {{.Label}}. "

// Build fetches the list, renders per-item labels, and returns the items
// (capped to Max, so index i maps to DTMF key i+1) plus the fully-rendered
// spoken prompt. vars is the current flow state, exposed as .Vars to the
// url/body/header/list/intro expressions.
func (m *ListMenu) Build(ctx context.Context, vars map[string]any) (items []any, prompt string, err error) {
	data, _, _, err := fetchJSON(ctx, fetchSpec{
		URL:            m.URL,
		Method:         m.Method,
		Body:           m.Body,
		Headers:        m.Headers,
		TimeoutSeconds: m.TimeoutSeconds,
	}, vars)
	if err != nil {
		return nil, "", err
	}

	listExpr := strings.TrimSpace(m.List)
	if listExpr == "" {
		listExpr = "."
	}
	renderedExpr, err := renderTemplateString(listExpr, vars)
	if err != nil {
		return nil, "", fmt.Errorf("listmenu: render list expr: %w", err)
	}
	results, err := runJQ(data, renderedExpr)
	if err != nil {
		return nil, "", fmt.Errorf("listmenu: list jq %q: %w", renderedExpr, err)
	}
	// A single []any result is the array; otherwise each result is an item.
	if len(results) == 1 {
		if arr, ok := results[0].([]any); ok {
			items = arr
		} else {
			items = results
		}
	} else {
		items = results
	}

	max := m.Max
	if max <= 0 || max > 9 {
		max = 9
	}
	if len(items) > max {
		items = items[:max]
	}

	labelTmpl, err := template.New("label").Funcs(genericJSONFuncs).Parse(m.Label)
	if err != nil {
		return nil, "", fmt.Errorf("listmenu: parse label: %w", err)
	}
	optStr := m.Option
	if strings.TrimSpace(optStr) == "" {
		optStr = defaultOptionTemplate
	}
	optTmpl, err := template.New("option").Funcs(genericJSONFuncs).Parse(optStr)
	if err != nil {
		return nil, "", fmt.Errorf("listmenu: parse option: %w", err)
	}

	var b strings.Builder
	if strings.TrimSpace(m.Intro) != "" {
		intro, ierr := renderTemplateString(m.Intro, vars)
		if ierr != nil {
			return nil, "", fmt.Errorf("listmenu: render intro: %w", ierr)
		}
		b.WriteString(intro)
		b.WriteString(" ")
	}
	for i, item := range items {
		var lb bytes.Buffer
		if err := labelTmpl.Execute(&lb, item); err != nil {
			return nil, "", fmt.Errorf("listmenu: render label[%d]: %w", i, err)
		}
		var ob bytes.Buffer
		if err := optTmpl.Execute(&ob, map[string]any{"Num": i + 1, "Label": lb.String()}); err != nil {
			return nil, "", fmt.Errorf("listmenu: render option[%d]: %w", i, err)
		}
		b.WriteString(ob.String())
	}
	return items, b.String(), nil
}
