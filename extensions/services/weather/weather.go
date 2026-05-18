// Package weather is a Service that fetches a forecast from the free
// Open-Meteo API (https://open-meteo.com) and renders it through a
// caller-supplied text/template, the same pattern as the traintimes service.
//
// TOML usage (inside an action):
//
//	srv = { dst = "weather",
//	        args = { latitude = "57.78", longitude = "13.94", timezone = "Europe/Stockholm", name = "Reftele" },
//	        tmpl = """The forecast for {{.Place}}: currently {{.Current.Condition}},
//	        {{.Current.TempC}} degrees. Today's high {{.Today.HighC}}, low {{.Today.LowC}}.
//	        {{if .Today.RainSoon}}Rain expected around {{.Today.RainSoon}}.{{end}}""" }
//
// Required args: latitude, longitude (both as decimal-degree strings).
// Optional args: timezone (default "auto"), name (used as {{.Place}}; falls
// back to the coords).
package weather

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"text/template"
	"time"
)

const apiURL = "https://api.open-meteo.com/v1/forecast"

// Weather implements the services.Service interface.
type Weather struct {
	HTTP *http.Client // optional; defaults to http.DefaultClient with a short timeout
}

func (w *Weather) MaxInputLength() int { return 0 }

// Get pulls a forecast and renders the supplied template against the
// TemplateData shape. Returns the rendered text on success.
func (w *Weather) Get(_ string, tmpl string, args map[string]string) (string, error) {
	lat, ok := args["latitude"]
	if !ok || lat == "" {
		return "", fmt.Errorf("weather: missing 'latitude' arg")
	}
	lon, ok := args["longitude"]
	if !ok || lon == "" {
		return "", fmt.Errorf("weather: missing 'longitude' arg")
	}
	tz := args["timezone"]
	if tz == "" {
		tz = "auto"
	}

	resp, err := w.fetch(lat, lon, tz)
	if err != nil {
		return "", err
	}

	data := buildTemplateData(resp, args)

	t, err := template.New("weather").Parse(tmpl)
	if err != nil {
		return "", fmt.Errorf("weather: parse template: %w", err)
	}
	var buf bytes.Buffer
	if err := t.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("weather: render template: %w", err)
	}
	return buf.String(), nil
}

func (w *Weather) fetch(lat, lon, tz string) (*apiResponse, error) {
	client := w.HTTP
	if client == nil {
		client = &http.Client{Timeout: 10 * time.Second}
	}

	req, err := http.NewRequest("GET", apiURL, nil)
	if err != nil {
		return nil, err
	}
	q := req.URL.Query()
	q.Set("latitude", lat)
	q.Set("longitude", lon)
	q.Set("timezone", tz)
	// Note: keep this list aligned with the apiResponse struct below; if you
	// add fields to one, mirror in the other.
	q.Set("current", "temperature_2m,weather_code,relative_humidity_2m,wind_speed_10m,is_day")
	q.Set("daily", "temperature_2m_max,temperature_2m_min,weather_code,precipitation_sum,sunrise,sunset")
	q.Set("hourly", "temperature_2m,weather_code,precipitation_probability,precipitation")
	q.Set("forecast_days", "1")
	req.URL.RawQuery = q.Encode()

	httpResp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("weather: GET %s: %w", req.URL, err)
	}
	defer httpResp.Body.Close()
	if httpResp.StatusCode != 200 {
		body, _ := io.ReadAll(io.LimitReader(httpResp.Body, 512))
		return nil, fmt.Errorf("weather: open-meteo %d: %s", httpResp.StatusCode, string(body))
	}

	var out apiResponse
	if err := json.NewDecoder(httpResp.Body).Decode(&out); err != nil {
		return nil, fmt.Errorf("weather: decode: %w", err)
	}
	return &out, nil
}

// apiResponse mirrors the subset of the Open-Meteo forecast JSON we ask for.
type apiResponse struct {
	Timezone string `json:"timezone"`
	Current  struct {
		Time             string  `json:"time"`
		Temperature2m    float64 `json:"temperature_2m"`
		WeatherCode      int     `json:"weather_code"`
		RelativeHumidity float64 `json:"relative_humidity_2m"`
		WindSpeed10m     float64 `json:"wind_speed_10m"`
		IsDay            int     `json:"is_day"`
	} `json:"current"`
	Daily struct {
		Time              []string  `json:"time"`
		TemperatureMax    []float64 `json:"temperature_2m_max"`
		TemperatureMin    []float64 `json:"temperature_2m_min"`
		WeatherCode       []int     `json:"weather_code"`
		PrecipitationSum  []float64 `json:"precipitation_sum"`
		Sunrise           []string  `json:"sunrise"`
		Sunset            []string  `json:"sunset"`
	} `json:"daily"`
	Hourly struct {
		Time                     []string  `json:"time"`
		Temperature2m            []float64 `json:"temperature_2m"`
		WeatherCode              []int     `json:"weather_code"`
		PrecipitationProbability []int     `json:"precipitation_probability"`
		Precipitation            []float64 `json:"precipitation"`
	} `json:"hourly"`
}

// TemplateData is the value passed into the caller's text/template. The
// fields are chosen to be easy to talk about in TTS rather than maximally
// faithful to the API — e.g. temperatures are rounded to the nearest degree
// because "twenty point three degrees" sounds robotic in a phone IVR.
type TemplateData struct {
	Place   string
	Current Current
	Today   Today
	Hours   []HourPoint // 24 hours starting at midnight local time
}

type Current struct {
	TempC      int    // rounded
	Condition  string // english from WMO code, day/night aware
	HumidityPct int
	WindKmh    int
	IsDay      bool
}

type Today struct {
	HighC          int
	LowC           int
	Condition      string // worst-of-day or first-meaningful — see buildToday
	PrecipMm       int    // rounded total
	Sunrise        string // "HH:MM" local
	Sunset         string // "HH:MM" local
	// RainSoon is a friendly "Xpm" string for the next hour with
	// precipitation_probability >= 50, or empty if none in the rest of today.
	RainSoon       string
}

type HourPoint struct {
	Time       string // "HH:MM" local
	TempC      int
	Condition  string
	PrecipPct  int
	PrecipMm   float64
}

func buildTemplateData(r *apiResponse, args map[string]string) TemplateData {
	place := args["name"]
	if place == "" {
		place = fmt.Sprintf("%s, %s", args["latitude"], args["longitude"])
	}

	cur := Current{
		TempC:       roundDeg(r.Current.Temperature2m),
		Condition:   describeWMO(r.Current.WeatherCode, r.Current.IsDay == 1),
		HumidityPct: int(r.Current.RelativeHumidity + 0.5),
		WindKmh:     int(r.Current.WindSpeed10m + 0.5),
		IsDay:       r.Current.IsDay == 1,
	}

	var today Today
	if len(r.Daily.Time) > 0 {
		today = Today{
			HighC:     roundDeg(r.Daily.TemperatureMax[0]),
			LowC:      roundDeg(r.Daily.TemperatureMin[0]),
			Condition: describeWMO(r.Daily.WeatherCode[0], true),
			PrecipMm:  int(r.Daily.PrecipitationSum[0] + 0.5),
			Sunrise:   hhmm(r.Daily.Sunrise[0]),
			Sunset:    hhmm(r.Daily.Sunset[0]),
		}
	}

	hours := make([]HourPoint, 0, len(r.Hourly.Time))
	nowHourLocal := time.Now().Hour()
	for i, t := range r.Hourly.Time {
		if i >= len(r.Hourly.Temperature2m) || i >= len(r.Hourly.WeatherCode) {
			break
		}
		hp := HourPoint{
			Time:      hhmm(t),
			TempC:     roundDeg(r.Hourly.Temperature2m[i]),
			Condition: describeWMO(r.Hourly.WeatherCode[i], true),
		}
		if i < len(r.Hourly.PrecipitationProbability) {
			hp.PrecipPct = r.Hourly.PrecipitationProbability[i]
		}
		if i < len(r.Hourly.Precipitation) {
			hp.PrecipMm = r.Hourly.Precipitation[i]
		}
		hours = append(hours, hp)

		// First future hour with >= 50% precip chance → RainSoon string.
		if today.RainSoon == "" && i >= nowHourLocal && hp.PrecipPct >= 50 {
			today.RainSoon = hp.Time
		}
	}

	return TemplateData{
		Place:   place,
		Current: cur,
		Today:   today,
		Hours:   hours,
	}
}

func roundDeg(f float64) int {
	if f >= 0 {
		return int(f + 0.5)
	}
	return int(f - 0.5)
}

// hhmm pulls "HH:MM" out of an open-meteo ISO timestamp like
// "2026-05-17T18:00" without parsing the full time (saves a TZ round trip
// since open-meteo already returns local-zone wall-clock strings).
func hhmm(iso string) string {
	if len(iso) < 16 {
		return iso
	}
	return iso[11:16]
}

// describeWMO maps an Open-Meteo WMO weather code to a short English phrase
// suitable for being spoken aloud. Day/night affects only "clear sky" so we
// can say "clear night" instead of "sunny". See:
// https://open-meteo.com/en/docs#weathervariables
func describeWMO(code int, day bool) string {
	switch code {
	case 0:
		if day {
			return "sunny"
		}
		return "clear"
	case 1:
		return "mostly clear"
	case 2:
		return "partly cloudy"
	case 3:
		return "overcast"
	case 45, 48:
		return "foggy"
	case 51:
		return "light drizzle"
	case 53:
		return "drizzle"
	case 55:
		return "heavy drizzle"
	case 56, 57:
		return "freezing drizzle"
	case 61:
		return "light rain"
	case 63:
		return "rain"
	case 65:
		return "heavy rain"
	case 66, 67:
		return "freezing rain"
	case 71:
		return "light snow"
	case 73:
		return "snow"
	case 75:
		return "heavy snow"
	case 77:
		return "snow grains"
	case 80:
		return "light rain showers"
	case 81:
		return "rain showers"
	case 82:
		return "violent rain showers"
	case 85:
		return "light snow showers"
	case 86:
		return "heavy snow showers"
	case 95:
		return "thunderstorms"
	case 96, 99:
		return "thunderstorms with hail"
	default:
		return "unknown (code " + strconv.Itoa(code) + ")"
	}
}
