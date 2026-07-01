// Package beer implements the "Tryck 4 för öl" interactive IVR flow against the
// public beer-roll API (https://beer.anderstorpsfestivalen.se). It:
//
//  1. GET /api/members            → build a "Tryck N för <name>" menu (id asc)
//  2. GET /api/public/bac         → speak the selected member's promille
//  3. POST /api/public/roll       → roll a beer for that member, speak it
//  4. POST /api/public/roll/:id/{accept,veto} → resolve the caller's choice
//
// It is registered into interactive.Registry from init() and blank-imported by
// the controller.
//
// TOML usage (inside an action):
//
//	interactive = { dst = "beer", args = { base_url = "https://beer.anderstorpsfestivalen.se" } }
package beer

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/anderstorpsfestivalen/benis-phone/extensions/interactive"
)

const (
	defaultBaseURL = "https://beer.anderstorpsfestivalen.se"
	// maxMenu caps the member menu at the number of single-digit keys we can
	// map (1-9); 0 is reserved for "go back".
	maxMenu = 9
)

var httpClient = &http.Client{Timeout: 10 * time.Second}

func init() {
	interactive.Register("beer", &Beer{})
}

// Beer is the interactive.Handler for the beer-roll flow.
type Beer struct{}

// --- API response shapes (only the fields we use) ---

type member struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

type bacMember struct {
	UserID   int     `json:"userId"`
	Username string  `json:"username"`
	Promille float64 `json:"promille"`
}

type bacResp struct {
	Members []bacMember `json:"members"`
}

type rollResp struct {
	ID              int     `json:"id"`
	Username        string  `json:"username"`
	ProductNameBold string  `json:"productNameBold"`
	ProductNameThin string  `json:"productNameThin"`
	ProducerName    string  `json:"producerName"`
	AlcoholPercent  float64 `json:"alcoholPercent"`
}

// Run drives the whole dialogue. It returns nil for any normal exit (caller
// pressed 0, made an invalid choice, or walked away); a non-nil error is
// reserved for genuine failures (used only for logging by the controller).
func (b *Beer) Run(ctx context.Context, io interactive.IO, args map[string]string) error {
	base := strings.TrimRight(args["base_url"], "/")
	if base == "" {
		base = defaultBaseURL
	}

	// 1. Members → menu.
	var members []member
	if err := doRequest(ctx, http.MethodGet, base+"/api/members", nil, &members); err != nil {
		return b.fail(ctx, io, "Kunde inte hämta medlemslistan.", err)
	}
	if len(members) == 0 {
		return io.Speak(ctx, "Det finns inga medlemmar att välja bland.")
	}
	sort.Slice(members, func(i, j int) bool { return members[i].ID < members[j].ID })
	if len(members) > maxMenu {
		members = members[:maxMenu]
	}

	if err := io.Speak(ctx, buildMenu(members)); err != nil {
		return err
	}
	key, err := io.NextKey(ctx)
	if err != nil {
		return nil // hangup or timeout: fall back to the menu silently
	}
	idx, ok := selection(key, len(members))
	if !ok {
		return nil // 0 or invalid: back to the menu
	}
	chosen := members[idx]

	// 2. BAC for the chosen member + roll offer.
	var bac bacResp
	if err := doRequest(ctx, http.MethodGet, base+"/api/public/bac", nil, &bac); err != nil {
		return b.fail(ctx, io, "Kunde inte hämta promillenivåerna.", err)
	}
	promille, found := promilleFor(bac, chosen.ID)
	var offer string
	if found {
		offer = fmt.Sprintf("%s, din alkoholhalt är %s promille. ", chosen.Name, formatSV(promille))
	} else {
		offer = fmt.Sprintf("%s, din alkoholhalt är okänd. ", chosen.Name)
	}
	offer += "Tryck 1 för att rulla en öl. Tryck 0 för att gå tillbaka."
	if err := io.Speak(ctx, offer); err != nil {
		return err
	}
	key, err = io.NextKey(ctx)
	if err != nil || key != "1" {
		return nil
	}

	// 3. Roll a beer for the chosen member.
	body, _ := json.Marshal(map[string]int{"userId": chosen.ID})
	var roll rollResp
	if err := doRequest(ctx, http.MethodPost, base+"/api/public/roll", body, &roll); err != nil {
		return b.fail(ctx, io, "Kunde inte rulla en öl just nu.", err)
	}
	if err := io.Speak(ctx, describeRoll(roll)); err != nil {
		return err
	}

	// 4. Accept or veto.
	key, err = io.NextKey(ctx)
	if err != nil {
		return nil
	}
	switch key {
	case "1":
		url := fmt.Sprintf("%s/api/public/roll/%d/accept", base, roll.ID)
		if err := doRequest(ctx, http.MethodPost, url, []byte("{}"), nil); err != nil {
			return b.fail(ctx, io, "Kunde inte acceptera ölen.", err)
		}
		return io.Speak(ctx, "Accepterad. Skål!")
	case "2":
		url := fmt.Sprintf("%s/api/public/roll/%d/veto", base, roll.ID)
		if err := doRequest(ctx, http.MethodPost, url, []byte("{}"), nil); err != nil {
			return b.fail(ctx, io, "Kunde inte avvisa ölen.", err)
		}
		return io.Speak(ctx, "Avvisad.")
	default:
		return nil
	}
}

// fail speaks a friendly Swedish message and returns the underlying error so
// the controller can log it. Speak errors (e.g. hangup mid-message) shadow the
// original since the call is already gone.
func (b *Beer) fail(ctx context.Context, io interactive.IO, msg string, cause error) error {
	_ = io.Speak(ctx, msg)
	return cause
}

func buildMenu(members []member) string {
	var b strings.Builder
	b.WriteString("Välkommen till ölmenyn. ")
	for i, m := range members {
		fmt.Fprintf(&b, "Tryck %d för %s. ", i+1, m.Name)
	}
	b.WriteString("Tryck 0 för att gå tillbaka.")
	return b.String()
}

// selection maps a DTMF key to a 0-based index into the (already capped)
// member slice. Returns ok=false for "0", non-digits, or out-of-range.
func selection(key string, n int) (int, bool) {
	d, err := strconv.Atoi(key)
	if err != nil || d < 1 || d > n {
		return 0, false
	}
	return d - 1, true
}

func promilleFor(bac bacResp, userID int) (float64, bool) {
	for _, m := range bac.Members {
		if m.UserID == userID {
			return m.Promille, true
		}
	}
	return 0, false
}

func describeRoll(r rollResp) string {
	name := r.ProductNameBold
	if name == "" {
		name = "en öl"
	}
	return fmt.Sprintf(
		"Du rullade %s, %s procent. Tryck 1 för att acceptera. Tryck 2 för att avvisa.",
		name, formatSV(r.AlcoholPercent),
	)
}

// formatSV renders a number for Swedish TTS: shortest decimal form with a
// comma decimal separator (5 → "5", 5.5 → "5,5", 0.054 → "0,054").
func formatSV(f float64) string {
	s := strconv.FormatFloat(f, 'f', -1, 64)
	return strings.Replace(s, ".", ",", 1)
}

func doRequest(ctx context.Context, method, url string, body []byte, out any) error {
	var r io.Reader
	if body != nil {
		r = bytes.NewReader(body)
	}
	req, err := http.NewRequestWithContext(ctx, method, url, r)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/json")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("beer: %s %s: %w", method, url, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		snippet, _ := io.ReadAll(io.LimitReader(resp.Body, 256))
		return fmt.Errorf("beer: %s %s: HTTP %d: %s", method, url, resp.StatusCode, strings.TrimSpace(string(snippet)))
	}
	if out != nil {
		if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
			return fmt.Errorf("beer: decode %s: %w", url, err)
		}
	}
	return nil
}
