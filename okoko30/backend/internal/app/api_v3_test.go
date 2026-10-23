package app

import (
	"archive/zip"
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func testApp(t *testing.T) *App {
	t.Helper()
	a, err := New(Config{Database: filepath.Join(t.TempDir(), "test.db"), PollSeconds: 60, ImportMinutes: 60})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(a.Close)
	return a
}
func getJSON(t *testing.T, a *App, path string) map[string]any {
	t.Helper()
	w := httptest.NewRecorder()
	a.Handler().ServeHTTP(w, httptest.NewRequest("GET", path, nil))
	if w.Code != 200 {
		t.Fatalf("%s: %d %s", path, w.Code, w.Body.String())
	}
	var out map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	return out
}
func TestSnapshotIsCachedAndDoesNotContactBoiler(t *testing.T) {
	a := testApp(t)
	var calls atomic.Int32
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		io.WriteString(w, `{"pe1":{"L_temp_act":{"val":642,"factor":0.1},"L_modulation":{"val":36}},"hk1":{"L_roomtemp_set":80,"L_statetext":"Mode confort actif","temp_heat":210}}`)
	}))
	defer s.Close()
	a.cfg.BoilerURL = s.URL + "/test-access/all"
	if err := a.collect(context.Background()); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 8; i++ {
		out := getJSON(t, a, "/api/v3/snapshot")
		getJSON(t, a, "/api/v3/weather")
		if out["boiler"].(map[string]any)["temperature"] != 64.2 {
			t.Fatalf("scaled temperature: %v", out)
		}
	}
	if calls.Load() != 1 {
		t.Fatalf("UI caused %d boiler requests", calls.Load())
	}
}
func TestMissingValuesRemainNullAndStaleDataIsMarked(t *testing.T) {
	a := testApp(t)
	out := getJSON(t, a, "/api/v3/snapshot")
	if out["boiler"].(map[string]any)["temperature"] != nil {
		t.Fatal("missing value became zero")
	}
	a.live = map[string]any{"pe1": map[string]any{"L_temp_act": json.Number("610")}}
	a.liveAt = time.Now().Add(-10 * time.Minute)
	out = getJSON(t, a, "/api/v3/snapshot")
	if out["stale"] != true || out["connected"] != false {
		t.Fatal("stale data marked live")
	}
	if out["boiler"].(map[string]any)["temperature"] != float64(61) {
		t.Fatal("last value lost")
	}
}
func TestConnectionSecretFileAndRedaction(t *testing.T) {
	dir := t.TempDir()
	config := filepath.Join(dir, "config.json")
	os.WriteFile(config, []byte(`{"database":"test.db","boiler_url":""}`), 0600)
	file := filepath.Join(dir, "connection")
	secret := fmt.Sprintf("generated-%d", time.Now().UnixNano())
	address := "http://127.0.0.1:1/" + secret + "/all"
	os.WriteFile(file, []byte(address), 0600)
	t.Setenv("OKOKO_BOILER_URL_FILE", file)
	c, err := LoadConfig(config)
	if err != nil || c.BoilerURL != address {
		t.Fatal("external credential not loaded")
	}
	a := testApp(t)
	a.cfg.BoilerURL = c.BoilerURL
	_ = a.collect(context.Background())
	w := httptest.NewRecorder()
	a.Handler().ServeHTTP(w, httptest.NewRequest("GET", "/api/v3/snapshot", nil))
	if strings.Contains(w.Body.String(), secret) {
		t.Fatal("secret exposed in API")
	}
	a.logRun("test", 0, 0, fmt.Errorf("GET %s/log1 failed", strings.TrimSuffix(address, "/all")))
	var problem string
	a.db.QueryRow("SELECT error FROM import_runs LIMIT 1").Scan(&problem)
	if strings.Contains(problem, secret) {
		t.Fatal("secret persisted in import log")
	}
}
func TestRestartPreservesMonthlyCorrectionsAndHistoricalRecharges(t *testing.T) {
	path := filepath.Join(t.TempDir(), "persistent.db")
	c := Config{Database: path}
	a, err := New(c)
	if err != nil {
		t.Fatal(err)
	}
	_, err = a.db.Exec(`UPDATE monthly_consumption SET kg=123.45 WHERE year=2024 AND month=1;INSERT INTO monthly_overrides(year,month,kg) VALUES(2024,1,122.5);UPDATE historical_recharges SET kg=1999 WHERE ts='2024-05-06T13:14:58'`)
	if err != nil {
		t.Fatal(err)
	}
	a.Close()
	a, err = New(c)
	if err != nil {
		t.Fatal(err)
	}
	defer a.Close()
	var monthly, recharge float64
	a.db.QueryRow(`SELECT kg FROM monthly_consumption WHERE year=2024 AND month=1`).Scan(&monthly)
	a.db.QueryRow(`SELECT kg FROM historical_recharges WHERE ts='2024-05-06T13:14:58'`).Scan(&recharge)
	if monthly != 123.45 || recharge != 1999 {
		t.Fatalf("existing values overwritten: %v %v", monthly, recharge)
	}
	out := getJSON(t, a, "/api/v3/pellets")
	for _, r := range out["monthly"].([]any) {
		m := r.(map[string]any)
		if m["year"] == float64(2024) && m["month"] == float64(1) && m["kg"] != 122.5 {
			t.Fatal("override lost in API")
		}
	}
}
func TestPelletCalculationRegression(t *testing.T) {
	a := testApp(t)
	_, e := a.db.Exec(`INSERT INTO measurements(ts,date,time,stock_kg) VALUES('2027-01-01T00:00:00','2027-01-01','00:00:00',1000),('2027-01-31T00:00:00','2027-01-31','00:00:00',600);INSERT INTO recharges(ts,kg,source) VALUES('2027-01-15T12:00:00',330,'manual')`)
	if e != nil {
		t.Fatal(e)
	}
	kg, ok, days, _ := a.monthlyStockMethod(2027, 1)
	if !ok || kg != 730 || days != 2 {
		t.Fatalf("monthly regression %v %v %d", kg, ok, days)
	}
	if e = a.recalculateYear(2027); e != nil {
		t.Fatal(e)
	}
	var count int
	a.db.QueryRow(`SELECT COUNT(*) FROM recharges WHERE source='manual'`).Scan(&count)
	if count != 1 {
		t.Fatal("recalculation modified refills")
	}
}
func TestCSVImportDeduplicatesAndPreservesRefills(t *testing.T) {
	a := testApp(t)
	row := make([]string, 67)
	for i := range row {
		row[i] = "0"
	}
	row[0] = "21.10.2026"
	row[1] = "12:05:00"
	row[11] = "20.5"
	row[44] = "850"
	file := filepath.Join(t.TempDir(), "touch_test.csv")
	data := strings.Repeat("header;", 66) + "header\n" + strings.Join(row, ";") + "\n"
	if e := os.WriteFile(file, []byte(data), 0600); e != nil {
		t.Fatal(e)
	}
	before := 0
	a.db.QueryRow(`SELECT COUNT(*) FROM historical_recharges`).Scan(&before)
	n, e := a.importOneCSV(file)
	if e != nil || n != 1 {
		t.Fatalf("first import %d %v", n, e)
	}
	n, e = a.importOneCSV(file)
	if e != nil || n != 0 {
		t.Fatalf("duplicate import %d %v", n, e)
	}
	var after int
	a.db.QueryRow(`SELECT COUNT(*) FROM historical_recharges`).Scan(&after)
	if before != after {
		t.Fatal("CSV changed refills")
	}
	var temp float64
	a.db.QueryRow(`SELECT hk1_room FROM measurements`).Scan(&temp)
	if temp != 20.5 {
		t.Fatalf("CSV mapping %v", temp)
	}
}
func TestUnavailableNASDoesNotBlockSnapshotOrSystem(t *testing.T) {
	a := testApp(t)
	a.cfg.CSVDir = "/mnt/okoko-definitely-unmounted"
	started := time.Now()
	files, n, err := a.ImportCSVDir()
	if err != nil || files != 0 || n != 0 {
		t.Fatalf("optional NAS import failed: %v", err)
	}
	for _, p := range []string{"/api/v3/snapshot", "/api/v3/system"} {
		getJSON(t, a, p)
	}
	if time.Since(started) > time.Second {
		t.Fatal("NAS unavailability blocked UI")
	}
}
func TestBackupIncludesCurrentWALAndPassesIntegrity(t *testing.T) {
	a := testApp(t)
	a.db.Exec(`INSERT INTO recharges(ts,kg,source) VALUES('2027-01-01T12:00:00',330,'manual')`)
	w := httptest.NewRecorder()
	a.Handler().ServeHTTP(w, httptest.NewRequest("GET", "/api/v3/backup", nil))
	if w.Code != 200 {
		t.Fatal(w.Body.String())
	}
	z, e := zip.NewReader(bytes.NewReader(w.Body.Bytes()), int64(w.Body.Len()))
	if e != nil {
		t.Fatal(e)
	}
	if len(z.File) != 1 || z.File[0].Name != "chaudiere.db" {
		t.Fatal("backup contains unexpected files")
	}
	r, e := z.File[0].Open()
	if e != nil {
		t.Fatal(e)
	}
	data, _ := io.ReadAll(r)
	r.Close()
	file := filepath.Join(t.TempDir(), "backup.db")
	os.WriteFile(file, data, 0600)
	db, e := sql.Open("sqlite", file)
	if e != nil {
		t.Fatal(e)
	}
	defer db.Close()
	var integrity string
	db.QueryRow("PRAGMA quick_check").Scan(&integrity)
	var kg float64
	db.QueryRow("SELECT kg FROM recharges WHERE source='manual'").Scan(&kg)
	if integrity != "ok" || kg != 330 {
		t.Fatalf("invalid WAL backup %s %v", integrity, kg)
	}
}
func TestMutationRejectsForeignOriginAndDuplicateRefills(t *testing.T) {
	a := testApp(t)
	post := func(origin string) int {
		r := httptest.NewRequest("POST", "http://okoko.test/api/v3/refills", strings.NewReader(`{"date":"2027-03-03","kg":330,"supplier":"Test","kind":"Sacs"}`))
		r.Header.Set("Content-Type", "application/json")
		r.Header.Set("Origin", origin)
		w := httptest.NewRecorder()
		a.Handler().ServeHTTP(w, r)
		return w.Code
	}
	if got := post("http://other.test"); got != 403 {
		t.Fatalf("origin: %d", got)
	}
	if got := post("http://okoko.test"); got != 201 {
		t.Fatalf("save: %d", got)
	}
	if got := post("http://okoko.test"); got != 409 {
		t.Fatalf("duplicate: %d", got)
	}
}
func TestHistoryNullAndExportAndSPARoutes(t *testing.T) {
	a := testApp(t)
	now := time.Now().Format("2006-01-02T15:04:05")
	_, err := a.db.Exec(`INSERT INTO measurements(ts,date,time,hk1_room) VALUES(?,?,?,?)`, now, now[:10], now[11:], 20.5)
	if err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{"/", "/chauffage", "/analyse", "/pellets", "/meteo", "/system", "/health", "/api/v3/export?type=measurements"} {
		w := httptest.NewRecorder()
		a.Handler().ServeHTTP(w, httptest.NewRequest("GET", path, nil))
		if w.Code != 200 {
			t.Fatalf("%s: %d", path, w.Code)
		}
	}
	w := httptest.NewRecorder()
	a.Handler().ServeHTTP(w, httptest.NewRequest("GET", "/api/v3/history?days=1", nil))
	var rows []map[string]any
	json.Unmarshal(w.Body.Bytes(), &rows)
	if len(rows) != 1 || rows[0]["ww1"] != nil || rows[0]["hk1"] != 20.5 {
		t.Fatalf("null/history: %v", rows)
	}
	w = httptest.NewRecorder()
	a.Handler().ServeHTTP(w, httptest.NewRequest("GET", "/api/not-found", nil))
	if w.Code != 404 {
		t.Fatal("API fallback must not serve HTML")
	}
}
func TestCancelledBoilerRequestDoesNotReachDevice(t *testing.T) {
	a := testApp(t)
	var calls atomic.Int32
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { calls.Add(1) }))
	defer s.Close()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	req, _ := http.NewRequestWithContext(ctx, "GET", s.URL, nil)
	if _, e := a.boilerRequest(req); e == nil {
		t.Fatal("expected cancellation")
	}
	if calls.Load() != 0 {
		t.Fatal("cancelled request reached boiler")
	}
}
