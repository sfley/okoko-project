package app

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http/httptest"
	"os"
	"testing"
	"time"
)

func rawBoiler(t *testing.T, a *App) {
	t.Helper()
	body, err := os.ReadFile("testdata/boiler-raw.json")
	if err != nil {
		t.Fatal(err)
	}
	decoder := json.NewDecoder(bytes.NewReader(body))
	decoder.UseNumber()
	if err = decoder.Decode(&a.live); err != nil {
		t.Fatal(err)
	}
	a.liveAt = time.Date(2026, 10, 23, 7, 44, 0, 0, time.UTC)
}

func TestRawBoilerTemperaturesNamesAndSentinels(t *testing.T) {
	a := testApp(t)
	rawBoiler(t, a)
	out := getJSON(t, a, "/api/v3/snapshot")
	boiler := out["boiler"].(map[string]any)
	if boiler["temperature"] != 39.7 || boiler["flame"] != 72.1 || boiler["pressure"] != 52.2 || out["outdoor"] != 13.1 {
		t.Fatalf("raw units: %v", out)
	}
	circuits := out["circuits"].(map[string]any)
	hk1 := circuits["hk1"].(map[string]any)
	hk2 := circuits["hk2"].(map[string]any)
	ecs := circuits["ww1"].(map[string]any)
	if hk1["name"] != "Chambres" || hk2["name"] != "Plancher" || hk1["temperature"] != 19.6 || hk2["temperature"] != 20.6 || hk1["flow"] != 27.7 {
		t.Fatalf("circuits: %v", circuits)
	}
	if hk1["setpoint"] != "19.6 °C" || hk2["setpoint"] != "20.2 °C" || ecs["minimum"] != float64(32) || ecs["maximum"] != float64(55) {
		t.Fatalf("setpoints: %v", circuits)
	}
	if temperature(a.live, "pe1", "L_ext_temp") != nil {
		t.Fatal("disconnected sensor must remain unknown")
	}
	if out["pellets"].(map[string]any)["stock"] != float64(1676) {
		t.Fatal("stock must not be scaled")
	}
}

func TestRawForecastFrenchDatesAndUnits(t *testing.T) {
	a := testApp(t)
	rawBoiler(t, a)
	out := getJSON(t, a, "/api/v3/weather")
	forecasts := out["forecast"].([]any)
	if len(forecasts) != 25 {
		t.Fatalf("lost forecast rows: %d", len(forecasts))
	}
	first := forecasts[0].(map[string]any)
	if first["date"] != "2026-10-23 09:44" || first["timestamp"] != "2026-10-23T09:44:00+02:00" || first["temp"] != "14" || first["wind"] != "57 km/h" || first["condition"] != "Couvert" {
		t.Fatalf("current weather: %v", first)
	}
	last := forecasts[24].(map[string]any)
	if last["day"] != "2026-10-26" || last["hour"] != float64(7) || last["timestamp"] != "2026-10-26T07:00:00+01:00" {
		t.Fatalf("DST/day 4: %v", last)
	}
	current := out["current"].(map[string]any)
	if current["Lever"] != "08:31" || current["Coucher"] != "19:04" || current["Durée du jour"] != "10h33" {
		t.Fatalf("daylight: %v", current)
	}
	// Both direct strings and metadata-wrapped strings are supported.
	a.live["forecast"].(map[string]any)["L_w_0"] = map[string]any{"val": "ven., 23 oct. 09:44|14|100|57|04d|804|C"}
	out = getJSON(t, a, "/api/v3/weather")
	if out["forecast"].([]any)[0].(map[string]any)["wind"] != "57 km/h" {
		t.Fatal("metadata weather lost")
	}
}

func TestForecastYearBoundaryAndInvalidDates(t *testing.T) {
	loc, _ := time.LoadLocation("Europe/Paris")
	reference := time.Date(2026, 12, 31, 10, 0, 0, 0, loc)
	parsed, ok := parseForecastTime("ven., 1 janv. 07:00", reference, loc)
	if !ok || parsed.Year() != 2027 || parsed.Day() != 1 {
		t.Fatalf("year boundary: %v", parsed)
	}
	for _, raw := range []string{"31 févr. 12:00", "1 janv. 25:00", "23 oct. 07:00", "demain"} {
		if _, ok := parseForecastTime(raw, reference, loc); ok {
			t.Fatalf("invented date: %s", raw)
		}
	}
}

func TestImportClockUTCConversionAndLegacyPreservation(t *testing.T) {
	a := testApp(t)
	for _, tc := range []struct{ input, expected string }{
		{"2026-10-23 07:50:00", "2026-10-23T09:50:00+02:00"},
		{"2026-12-23 07:50:00", "2026-12-23T08:50:00+01:00"},
	} {
		raw, ok := a.importTime(tc.input)
		instant, err := time.Parse(time.RFC3339, raw)
		if !ok || err != nil || instant.In(a.location()).Format(time.RFC3339) != tc.expected {
			t.Fatalf("import clock: %s -> %s", tc.input, raw)
		}
	}
	raw, known := a.importTime("date ancienne inconnue")
	if known || raw != "date ancienne inconnue" {
		t.Fatal("unknown source timestamp changed")
	}
	a.cfg.LegacyImportTimezone = "Europe/Paris"
	if value, _ := a.importTime("2026-10-23 07:50:00"); value != "2026-10-23T07:50:00+02:00" {
		t.Fatal("legacy timezone override ignored")
	}
	a.logRun("JSON test", 3, 5, nil)
	var at string
	if err := a.db.QueryRow("SELECT at FROM import_runs ORDER BY id DESC LIMIT 1").Scan(&at); err != nil {
		t.Fatal(err)
	}
	if _, err := time.Parse(time.RFC3339Nano, at); err != nil {
		t.Fatalf("new timestamp lacks zone: %s", at)
	}
}

func TestImportPaginationKeepsAllOlderRuns(t *testing.T) {
	a := testApp(t)
	for i := 1; i <= 25; i++ {
		a.logRun(fmt.Sprintf("CSV %02d", i), i, i, nil)
	}
	seen := map[float64]bool{}
	for page, count := range []int{10, 10, 5} {
		out := getJSON(t, a, fmt.Sprintf("/api/v3/system?page=%d", page+1))
		runs := out["runs"].([]any)
		p := out["pagination"].(map[string]any)
		if len(runs) != count || p["total"] != float64(25) || p["pages"] != float64(3) {
			t.Fatalf("pagination: %v", out)
		}
		for _, item := range runs {
			id := item.(map[string]any)["id"].(float64)
			if seen[id] {
				t.Fatal("duplicated page row")
			}
			seen[id] = true
		}
	}
	if len(seen) != 25 {
		t.Fatal("older imports inaccessible")
	}
	for _, page := range []string{"0", "-1", "bad"} {
		w := httptest.NewRecorder()
		a.Handler().ServeHTTP(w, httptest.NewRequest("GET", "/api/v3/system?page="+page, nil))
		if w.Code != 400 {
			t.Fatalf("invalid page %q: %d", page, w.Code)
		}
	}
	out := getJSON(t, a, "/api/v3/system?page=999")
	if out["pagination"].(map[string]any)["page"] != float64(3) {
		t.Fatal("out of range page not clamped")
	}
}
