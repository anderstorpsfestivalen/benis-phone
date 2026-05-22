// Package saunatemp is a Service that fetches the current sauna temperatures
// from bastu-bot (https://bastu-bot.wberg.workers.dev/temperatures) and renders
// a single sauna's reading through a caller-supplied text/template.
//
// TOML usage (inside an action):
//
//	srv = { dst = "saunatemp",
//	        args = { target = "Summalajnen" },
//	        tmpl = """{{.Name}} is {{.TemperatureRounded}} celsius""" }
//
// Required args: target (case-insensitive sauna name).
// Optional args: none.
package saunatemp

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"text/template"
	"time"
)

const apiURL = "https://bastu-bot.wberg.workers.dev/temperatures"

type Saunatemp struct {
	HTTP *http.Client
}

func (s *Saunatemp) MaxInputLength() int { return 0 }

func (s *Saunatemp) Get(_ string, tmpl string, args map[string]string) (string, error) {
	target := strings.TrimSpace(args["target"])
	if target == "" {
		return "", fmt.Errorf("saunatemp: missing 'target' arg")
	}

	readings, err := s.fetch()
	if err != nil {
		return "", err
	}

	match, ok := findSauna(readings, target)
	if !ok {
		return "", fmt.Errorf("saunatemp: no sauna named %q in response", target)
	}

	t, err := template.New("saunatemp").Parse(tmpl)
	if err != nil {
		return "", fmt.Errorf("saunatemp: parse template: %w", err)
	}
	var buf bytes.Buffer
	if err := t.Execute(&buf, buildTemplateData(match)); err != nil {
		return "", fmt.Errorf("saunatemp: render template: %w", err)
	}
	return buf.String(), nil
}

func (s *Saunatemp) fetch() ([]reading, error) {
	client := s.HTTP
	if client == nil {
		client = &http.Client{Timeout: 10 * time.Second}
	}
	resp, err := client.Get(apiURL)
	if err != nil {
		return nil, fmt.Errorf("saunatemp: GET %s: %w", apiURL, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return nil, fmt.Errorf("saunatemp: bastu-bot %d: %s", resp.StatusCode, string(body))
	}
	var out []reading
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, fmt.Errorf("saunatemp: decode: %w", err)
	}
	return out, nil
}

func findSauna(readings []reading, target string) (reading, bool) {
	for _, r := range readings {
		if strings.EqualFold(r.Name, target) {
			return r, true
		}
	}
	return reading{}, false
}

type reading struct {
	Name        string  `json:"name"`
	Temperature float64 `json:"temperature"`
}

// TemplateData is the value passed into the caller's text/template.
type TemplateData struct {
	Name               string
	Temperature        float64
	TemperatureRounded int
}

func buildTemplateData(r reading) TemplateData {
	return TemplateData{
		Name:               r.Name,
		Temperature:        r.Temperature,
		TemperatureRounded: roundDeg(r.Temperature),
	}
}

func roundDeg(f float64) int {
	if f >= 0 {
		return int(f + 0.5)
	}
	return int(f - 0.5)
}
