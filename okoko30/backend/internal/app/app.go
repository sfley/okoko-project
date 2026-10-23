package app

import (
	"archive/zip"
	"bytes"
	"context"
	"database/sql"
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"math"
	_ "modernc.org/sqlite"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode/utf8"
)

const Version = "3.0.1"

type Config struct {
	Timezone             string `json:"timezone"`
	LegacyImportTimezone string `json:"legacy_import_timezone,omitempty"`
	Listen               string `json:"listen"`
	BoilerURL            string `json:"boiler_url"`
	Database             string `json:"database"`
	PollSeconds          int    `json:"poll_seconds"`
	RetentionDays        int    `json:"retention_days"`
	CSVDir               string `json:"csv_dir"`
	ImportMinutes        int    `json:"import_minutes"`
	BackupDir            string `json:"backup_dir"`
}
type App struct {
	cfg               Config
	db                *sql.DB
	client            *http.Client
	mu                sync.RWMutex
	live              map[string]any
	liveAt            time.Time
	liveErr           string
	importMu          sync.Mutex
	importActive      bool
	importStateMu     sync.RWMutex
	boilerMu          sync.Mutex
	lastBoilerRequest time.Time
}

func LoadConfig(path string) (Config, error) {
	c := Config{Listen: ":8888", Database: "./chaudiere.db", PollSeconds: 60, RetentionDays: 730, CSVDir: "/mnt/nas/googledrive/chaudiere_csv", ImportMinutes: 60, BackupDir: "", Timezone: "Europe/Paris"}
	b, e := os.ReadFile(path)
	if e != nil {
		return c, e
	}
	e = json.Unmarshal(b, &c)
	if value := os.Getenv("OKOKO_BOILER_URL"); value != "" {
		c.BoilerURL = value
	}
	if name := os.Getenv("OKOKO_BOILER_URL_FILE"); name != "" {
		secret, err := os.ReadFile(name)
		if err != nil {
			return c, fmt.Errorf("lecture du fichier de connexion impossible")
		}
		c.BoilerURL = strings.TrimSpace(string(secret))
	}
	return c, e
}

func New(c Config) (*App, error) {
	if c.Timezone != "" {
		if _, err := time.LoadLocation(c.Timezone); err != nil {
			return nil, fmt.Errorf("fuseau horaire inconnu: %s", c.Timezone)
		}
	}
	if c.LegacyImportTimezone != "" {
		if _, err := time.LoadLocation(c.LegacyImportTimezone); err != nil {
			return nil, fmt.Errorf("ancien fuseau horaire inconnu: %s", c.LegacyImportTimezone)
		}
	}
	db, e := sql.Open("sqlite", c.Database)
	if e != nil {
		return nil, e
	}
	db.SetMaxOpenConns(1)
	a := &App{cfg: c, db: db, client: &http.Client{Timeout: 8 * time.Second}}
	if e = a.schema(); e != nil {
		return nil, e
	}
	if e = a.initTransitions(); e != nil {
		return nil, e
	}
	if e = a.seedHistoricalConsumption(); e != nil {
		return nil, e
	}
	return a, nil
}

func (a *App) Close() { a.db.Close() }

func (a *App) schema() error {
	_, e := a.db.Exec(`PRAGMA journal_mode=WAL; PRAGMA busy_timeout=5000; PRAGMA synchronous=NORMAL;
CREATE TABLE IF NOT EXISTS import_runs(id INTEGER PRIMARY KEY AUTOINCREMENT, at TEXT NOT NULL, source TEXT NOT NULL, added INTEGER NOT NULL, error TEXT NOT NULL DEFAULT '', read_count INTEGER NOT NULL DEFAULT 0, duplicate_count INTEGER NOT NULL DEFAULT 0);
CREATE TABLE IF NOT EXISTS samples(ts TEXT PRIMARY KEY, ext REAL,hk1 REAL,hk2 REAL,ww1 REAL,modulation REAL,stock_kg REAL,stock_max REAL,raw_json TEXT);
CREATE TABLE IF NOT EXISTS measurements(
 ts TEXT PRIMARY KEY, date TEXT NOT NULL, time TEXT NOT NULL,
 ext REAL, ext_actual REAL, boiler REAL, boiler_set REAL,
 hk1_flow REAL, hk1_flow_set REAL, hk1_room REAL, hk1_room_set REAL,
 hk2_flow REAL, hk2_flow_set REAL, hk2_room REAL, hk2_room_set REAL,
 dhw_in REAL, dhw_out REAL, dhw_set REAL,
 modulation REAL, flame REAL, flame_set REAL,
 stock_kg REAL, stock_zwb_kg REAL, boiler_status INTEGER,
 burner INTEGER, hk1_pump INTEGER, hk2_pump INTEGER, dhw_pump INTEGER,
 error1 TEXT, error2 TEXT, error3 TEXT, source_file TEXT
);
CREATE INDEX IF NOT EXISTS idx_measurements_date ON measurements(date);
CREATE INDEX IF NOT EXISTS idx_measurements_stock ON measurements(ts,stock_kg);
CREATE TABLE IF NOT EXISTS imported_files(path TEXT PRIMARY KEY,size INTEGER NOT NULL,mtime INTEGER NOT NULL,rows_added INTEGER NOT NULL,imported_at TEXT NOT NULL);
CREATE TABLE IF NOT EXISTS recharges(ts TEXT PRIMARY KEY,kg REAL NOT NULL,source TEXT NOT NULL DEFAULT 'auto',note TEXT NOT NULL DEFAULT '',stock_before REAL,stock_after REAL);
CREATE TABLE IF NOT EXISTS monthly_consumption(year INTEGER,month INTEGER,kg REAL,PRIMARY KEY(year,month));
CREATE TABLE IF NOT EXISTS legacy_monthly_consumption(year INTEGER,month INTEGER,kg REAL,PRIMARY KEY(year,month));
CREATE TABLE IF NOT EXISTS historical_recharges(ts TEXT PRIMARY KEY,kg REAL NOT NULL,fournisseur TEXT NOT NULL,conditionnement TEXT NOT NULL,origine TEXT NOT NULL);
CREATE TABLE IF NOT EXISTS recharge_tombstones(ts TEXT PRIMARY KEY);
CREATE TABLE IF NOT EXISTS monthly_overrides(year INTEGER,month INTEGER,kg REAL NOT NULL,original_kg REAL,PRIMARY KEY(year,month));
CREATE TABLE IF NOT EXISTS daily_consumption(date TEXT PRIMARY KEY,kg REAL NOT NULL, anomalies INTEGER NOT NULL DEFAULT 0, samples INTEGER NOT NULL DEFAULT 0);`)
	if e != nil {
		return e
	}
	for _, q := range []string{
		`ALTER TABLE import_runs ADD COLUMN read_count INTEGER NOT NULL DEFAULT 0`,
		`ALTER TABLE import_runs ADD COLUMN duplicate_count INTEGER NOT NULL DEFAULT 0`,
		`ALTER TABLE recharges ADD COLUMN source TEXT NOT NULL DEFAULT 'auto'`,
		`ALTER TABLE recharges ADD COLUMN note TEXT NOT NULL DEFAULT ''`,
		`ALTER TABLE recharges ADD COLUMN stock_before REAL`,
		`ALTER TABLE recharges ADD COLUMN stock_after REAL`,
	} {
		_, _ = a.db.Exec(q)
	}
	for _, x := range []struct {
		ts                                    string
		kg                                    float64
		fournisseur, conditionnement, origine string
	}{
		{"2023-07-06T00:00:09", 2000, "Sybellis Durepaire", "Vrac", "dump MariaDB"},
		{"2024-05-06T13:14:58", 2000, "Sybellis Durepaire", "Vrac", "dump MariaDB"},
		{"2025-06-11T00:00:02", 2000, "Sybellis Durepaire", "Vrac", "dump MariaDB"},
		{"2026-03-03T00:00:00", 330, "Brico Dépôt", "Sacs", "saisie manuelle (heure inconnue)"},
		{"2026-04-02T00:00:27", 2000, "Sybellis Durepaire", "Vrac", "dump MariaDB"},
	} {
		if _, err := a.db.Exec(`INSERT OR IGNORE INTO historical_recharges(ts,kg,fournisseur,conditionnement,origine) SELECT ?,?,?,?,? WHERE NOT EXISTS(SELECT 1 FROM recharge_tombstones WHERE ts=?)`, x.ts, x.kg, x.fournisseur, x.conditionnement, x.origine, x.ts); err != nil {
			return err
		}
	}
	return nil
}

func num(v any) (float64, bool) {
	switch x := v.(type) {
	case float64:
		return x, true
	case json.Number:
		f, e := x.Float64()
		return f, e == nil
	case string:
		f, e := strconv.ParseFloat(strings.ReplaceAll(x, ",", "."), 64)
		return f, e == nil
	}
	return 0, false
}

func leaf(root map[string]any, cat, key string) (float64, bool) {
	c, ok := root[cat].(map[string]any)
	if !ok {
		return 0, false
	}
	v, ok := c[key]
	if !ok {
		return 0, false
	}
	if m, ok := v.(map[string]any); ok {
		n, ok := num(m["val"])
		if !ok {
			return 0, false
		}
		if f, ok := num(m["factor"]); ok {
			n *= f
		}
		return n, true
	}
	return num(v)
}

func first(root map[string]any, pairs ...[2]string) float64 {
	for _, p := range pairs {
		if v, ok := leaf(root, p[0], p[1]); ok {
			return v
		}
	}
	return 0
}

// The boiler sometimes emits Latin-1 bytes inside otherwise UTF-8 JSON.

func decodeBoilerText(b []byte) string {
	if utf8.Valid(b) {
		return string(b)
	}
	var out strings.Builder
	for len(b) > 0 {
		r, size := utf8.DecodeRune(b)
		if r == utf8.RuneError && size == 1 {
			out.WriteRune(rune(b[0]))
			b = b[1:]
			continue
		}
		out.WriteRune(r)
		b = b[size:]
	}
	return out.String()
}

// boilerRequest serializes all boiler requests and enforces a minimum 2.5s
// between starts. A canceled request never reaches the boiler.

func (a *App) boilerRequest(req *http.Request) (*http.Response, error) {
	a.boilerMu.Lock()
	defer a.boilerMu.Unlock()
	wait := 2500*time.Millisecond - time.Since(a.lastBoilerRequest)
	if !a.lastBoilerRequest.IsZero() && wait > 0 {
		timer := time.NewTimer(wait)
		select {
		case <-req.Context().Done():
			if !timer.Stop() {
				<-timer.C
			}
			return nil, req.Context().Err()
		case <-timer.C:
		}
	}
	if err := req.Context().Err(); err != nil {
		return nil, err
	}
	a.lastBoilerRequest = time.Now()
	return a.client.Do(req)
}

func (a *App) fetch(ctx context.Context) (map[string]any, []byte, error) {
	req, e := http.NewRequestWithContext(ctx, "GET", a.cfg.BoilerURL, nil)
	if e != nil {
		return nil, nil, e
	}
	r, e := a.boilerRequest(req)
	if e != nil {
		return nil, nil, e
	}
	defer r.Body.Close()
	if r.StatusCode/100 != 2 {
		return nil, nil, fmt.Errorf("chaudière HTTP %s", r.Status)
	}
	b, e := io.ReadAll(io.LimitReader(r.Body, 4<<20))
	if e != nil {
		return nil, nil, e
	}
	var m map[string]any
	d := json.NewDecoder(strings.NewReader(decodeBoilerText(b)))
	d.UseNumber()
	if e = d.Decode(&m); e != nil {
		return nil, b, e
	}
	return m, b, nil
}

func (a *App) collect(ctx context.Context) error {
	m, b, e := a.fetch(ctx)
	a.mu.Lock()
	if e != nil {
		a.liveErr = "Lecture chaudière indisponible"
		a.mu.Unlock()
		return e
	}
	a.live = m
	a.liveAt = time.Now()
	at := a.liveAt
	a.liveErr = ""
	a.mu.Unlock()
	a.recordTransitions(m, at)
	_ = b // temps réel volontairement non historisé : les CSV sont la source de vérité
	return nil
}

func (a *App) RunCollector(ctx context.Context) {
	_ = a.collect(ctx)
	t := time.NewTicker(time.Duration(max(10, a.cfg.PollSeconds)) * time.Second)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			_ = a.collect(ctx)
			if a.cfg.RetentionDays > 0 {
				cut := time.Now().AddDate(0, 0, -a.cfg.RetentionDays).Format(time.RFC3339)
				_, _ = a.db.Exec("DELETE FROM samples WHERE ts < ?", cut)
			}
		}
	}
}

func isNFSMounted(path string) bool {
	data, err := os.ReadFile("/proc/mounts")
	if err != nil {
		return false
	}
	for _, line := range strings.Split(string(data), "\n") {
		parts := strings.Fields(line)
		if len(parts) >= 3 && strings.HasPrefix(parts[2], "nfs") && (path == parts[1] || strings.HasPrefix(path, strings.TrimRight(parts[1], "/")+"/")) {
			return true
		}
	}
	return false
}

func (a *App) rechargeRows() ([]refillRow, error) {
	rows, e := a.db.Query(`SELECT ts,kg,fournisseur,conditionnement,origine FROM historical_recharges UNION ALL SELECT ts,kg,'','',note FROM recharges WHERE source='manual' ORDER BY ts DESC`)
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	var out []refillRow
	for rows.Next() {
		var v refillRow
		if e = rows.Scan(&v.ts, &v.kg, &v.supplier, &v.kind, &v.note); e != nil {
			return nil, e
		}
		v.manual = v.supplier == "" && v.kind == ""
		if v.manual {
			v.supplier = "Manuel"
			v.kind = "Vrac"
		}
		out = append(out, v)
	}
	filtered := []refillRow{}
	for i, v := range out {
		duplicate := false
		for j, w := range out {
			if i == j || v.manual == w.manual || math.Abs(v.kg-w.kg) > .01 {
				continue
			}
			d1, e1 := time.Parse("2006-01-02", v.ts[:10])
			d2, e2 := time.Parse("2006-01-02", w.ts[:10])
			if e1 == nil && e2 == nil && math.Abs(d1.Sub(d2).Hours()) <= 48 && v.manual {
				duplicate = true
				break
			}
		}
		if !duplicate {
			filtered = append(filtered, v)
		}
	}
	return filtered, rows.Err()
}

type refillRow struct {
	ts, supplier, kind, note string
	kg                       float64
	manual                   bool
}

func validRefillDate(d string) bool {
	if len(d) != 10 {
		return false
	}
	t, e := time.Parse("2006-01-02", d)
	return e == nil && t.Format("2006-01-02") == d
}

func (a *App) monthlyStockMethod(year, month int) (float64, bool, int, int) {
	start := fmt.Sprintf("%04d-%02d-01", year, month)
	next := time.Date(year, time.Month(month+1), 1, 0, 0, 0, 0, time.Local)
	end := next.Format("2006-01-02")
	daysExpected := next.AddDate(0, 0, -1).Day()
	var first, last sql.NullFloat64
	var days int
	err := a.db.QueryRow(`SELECT
        (SELECT stock_kg FROM measurements WHERE date>=? AND date<? AND stock_kg IS NOT NULL ORDER BY ts ASC LIMIT 1),
        (SELECT stock_kg FROM measurements WHERE date>=? AND date<? AND stock_kg IS NOT NULL ORDER BY ts DESC LIMIT 1),
        (SELECT COUNT(DISTINCT date) FROM measurements WHERE date>=? AND date<? AND stock_kg IS NOT NULL)`,
		start, end, start, end, start, end).Scan(&first, &last, &days)
	if err != nil || !first.Valid || !last.Valid || days < 2 {
		return 0, false, days, daysExpected
	}
	var refill float64
	err = a.db.QueryRow(`SELECT COALESCE(SUM(kg),0) FROM recharges WHERE ts>=? AND ts<? AND source='manual'`, start, end).Scan(&refill)
	if err != nil {
		return 0, false, days, daysExpected
	}
	return first.Float64 - last.Float64 + refill, true, days, daysExpected
}

func (a *App) recalculateYear(year int) error {
	start := fmt.Sprintf("%04d-01-01", year)
	end := fmt.Sprintf("%04d-01-01", year+1)
	rows, err := a.db.Query(`SELECT date,stock_kg FROM measurements WHERE date>=? AND date<? AND stock_kg IS NOT NULL ORDER BY ts`, start, end)
	if err != nil {
		return err
	}
	type day struct {
		kg                 float64
		anomalies, samples int
	}
	days := map[string]day{}
	var prev float64
	hasPrev := false
	for rows.Next() {
		var date string
		var stock float64
		if err = rows.Scan(&date, &stock); err != nil {
			break
		}
		d := days[date]
		d.samples++
		if hasPrev {
			diff := prev - stock
			if diff > 0 && diff <= 30 {
				d.kg += diff
			} else if diff > 30 {
				d.anomalies++
			}
			// positive stock jump is a refill or reset; do not add it to consumption
			if diff < -30 {
				d.anomalies++
			}
		}
		days[date] = d
		prev, hasPrev = stock, true
	}
	if err == nil {
		err = rows.Err()
	}
	rows.Close()
	if err != nil {
		return err
	}
	// Compute monthly figures BEFORE starting a write transaction.
	// monthlyStockMethod queries a.db; doing so while a write transaction is
	// open can deadlock when the SQLite connection pool has one connection.
	months := map[int]float64{}
	for date := range days {
		month, _ := strconv.Atoi(date[5:7])
		if _, done := months[month]; done {
			continue
		}
		kg, valid, _, _ := a.monthlyStockMethod(year, month)
		if valid && kg >= 0 {
			months[month] = kg
		}
	}
	tx, err := a.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err = tx.Exec(`DELETE FROM daily_consumption WHERE date>=? AND date<?`, start, end); err != nil {
		return err
	}
	if _, err = tx.Exec(`DELETE FROM monthly_consumption WHERE year=? AND month NOT IN (SELECT month FROM legacy_monthly_consumption WHERE year=?)`, year, year); err != nil {
		return err
	}
	for date, d := range days {
		if _, err = tx.Exec(`INSERT INTO daily_consumption(date,kg,anomalies,samples) VALUES(?,?,?,?)`, date, d.kg, d.anomalies, d.samples); err != nil {
			return err
		}
	}
	for month, kg := range months {
		if _, err = tx.Exec(`INSERT INTO monthly_consumption(year,month,kg) SELECT ?,?,? WHERE NOT EXISTS (SELECT 1 FROM legacy_monthly_consumption WHERE year=? AND month=?)`, year, month, kg, year, month); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func csvFloat(s string) (any, error) {
	s = strings.TrimSpace(strings.ReplaceAll(s, ",", "."))
	if s == "" {
		return nil, nil
	}
	v, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return nil, err
	}
	return v, nil
}

func csvInt(s string) any {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}
	v, err := strconv.Atoi(s)
	if err != nil {
		return nil
	}
	return v
}

func field(row []string, i int) string {
	if i >= 0 && i < len(row) {
		return strings.TrimSpace(row[i])
	}
	return ""
}

func (a *App) importOneCSV(path string) (int, error) {
	f, err := os.Open(path)
	if err != nil {
		return 0, err
	}
	defer f.Close()
	cr := csv.NewReader(f)
	cr.Comma = ';'
	cr.FieldsPerRecord = -1
	cr.ReuseRecord = false
	if _, err = cr.Read(); err != nil {
		return 0, err
	}
	tx, err := a.db.Begin()
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()
	stmt, err := tx.Prepare(`INSERT OR IGNORE INTO measurements(ts,date,time,ext,ext_actual,boiler,boiler_set,hk1_flow,hk1_flow_set,hk1_room,hk1_room_set,hk2_flow,hk2_flow_set,hk2_room,hk2_room_set,dhw_in,dhw_out,dhw_set,modulation,flame,flame_set,stock_kg,stock_zwb_kg,boiler_status,burner,hk1_pump,hk2_pump,dhw_pump,error1,error2,error3,source_file) VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`)
	if err != nil {
		return 0, err
	}
	defer stmt.Close()
	added := 0
	for {
		row, e := cr.Read()
		if errors.Is(e, io.EOF) {
			break
		}
		if e != nil {
			continue
		}
		if len(row) < 67 {
			continue
		}
		t, e := time.ParseInLocation("02.01.2006 15:04:05", field(row, 0)+" "+field(row, 1), time.Local)
		if e != nil {
			continue
		}
		vals := make([]any, 0, 32)
		vals = append(vals, t.Format("2006-01-02T15:04:05"), t.Format("2006-01-02"), t.Format("15:04:05"))
		for _, i := range []int{2, 3, 4, 5, 9, 10, 11, 12, 17, 18, 19, 20, 25, 26, 27, 34, 35, 36, 44, 45} {
			v, _ := csvFloat(field(row, i))
			vals = append(vals, v)
		}
		vals = append(vals, csvInt(field(row, 46)), csvInt(field(row, 6)), csvInt(field(row, 13)), csvInt(field(row, 21)), csvInt(field(row, 28)), field(row, 64), field(row, 65), field(row, 66), filepath.Base(path))
		res, e := stmt.Exec(vals...)
		if e != nil {
			return 0, fmt.Errorf("%s: %w", filepath.Base(path), e)
		}
		n, _ := res.RowsAffected()
		added += int(n)
	}
	if err = tx.Commit(); err != nil {
		return 0, err
	}
	return added, nil
}

func (a *App) ImportCSVDir() (int, int, error) {
	if a.cfg.CSVDir == "" {
		return 0, 0, nil
	}
	if !isNFSMounted(a.cfg.CSVDir) {
		return 0, 0, nil
	}
	files, err := filepath.Glob(filepath.Join(a.cfg.CSVDir, "touch_*.csv"))
	if err != nil {
		return 0, 0, err
	}
	sort.Strings(files)
	filesDone, rowsAdded := 0, 0
	for _, p := range files {
		st, e := os.Stat(p)
		if e != nil {
			continue
		}
		var size, mtime int64
		e = a.db.QueryRow(`SELECT size,mtime FROM imported_files WHERE path=?`, p).Scan(&size, &mtime)
		if e == nil && size == st.Size() && mtime == st.ModTime().Unix() {
			continue
		}
		n, e := a.importOneCSV(p)
		if e != nil {
			return filesDone, rowsAdded, e
		}
		_, e = a.db.Exec(`INSERT INTO imported_files(path,size,mtime,rows_added,imported_at) VALUES(?,?,?,?,?) ON CONFLICT(path) DO UPDATE SET size=excluded.size,mtime=excluded.mtime,rows_added=excluded.rows_added,imported_at=excluded.imported_at`, p, st.Size(), st.ModTime().Unix(), n, time.Now().Format(time.RFC3339))
		if e != nil {
			return filesDone, rowsAdded, e
		}
		filesDone++
		rowsAdded += n
	}
	// OKOKO9: no refill history is ever deleted by CSV import.
	years, err := a.db.Query(`SELECT DISTINCT CAST(substr(date,1,4) AS INTEGER) FROM measurements ORDER BY 1`)
	if err == nil {
		var ys []int
		for years.Next() {
			var y int
			if years.Scan(&y) == nil {
				ys = append(ys, y)
			}
		}
		years.Close()
		for _, y := range ys {
			_ = a.recalculateYear(y)
		}
	}
	return filesDone, rowsAdded, nil
}

func (a *App) RunCSVImporter(ctx context.Context) {
	run := func() {
		a.withImportLock(func() {
			f, n, e := a.ImportCSVDir()
			if e != nil {
				a.logRun("CSV · NAS", n, n, e)
				log.Printf("Import CSV: %v", e)
			} else if f > 0 {
				a.logRun("CSV · NAS", n, n, nil)
				log.Printf("Import CSV: %d fichier(s), %d nouvelle(s) ligne(s)", f, n)
			}
		})
	}
	// Premier import NAS différé pour ne pas retarder le démarrage.
	mins := a.cfg.ImportMinutes
	if mins <= 0 {
		mins = 60
	}
	t := time.NewTicker(time.Duration(mins) * time.Minute)
	go run()
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			run()
		}
	}
}

func (a *App) exportCSV(w http.ResponseWriter, r *http.Request) {
	typ := r.URL.Query().Get("type")
	var query, filename string
	switch typ {
	case "measurements":
		query = `SELECT ts,ext,hk1_room,hk2_room,dhw_in,modulation,stock_kg FROM measurements ORDER BY ts`
		filename = "okoko-mesures.csv"
	case "daily":
		query = `SELECT date,kg,anomalies,samples FROM daily_consumption ORDER BY date`
		filename = "okoko-journalier.csv"
	case "monthly":
		query = `SELECT m.year,m.month,COALESCE(o.kg,m.kg) FROM monthly_consumption m LEFT JOIN monthly_overrides o ON m.year=o.year AND m.month=o.month ORDER BY m.year,m.month`
		filename = "okoko-mensuel.csv"
	case "refills":
		query = `SELECT ts,kg,note FROM recharges WHERE source='manual' ORDER BY ts`
		filename = "okoko-remplissages.csv"
	default:
		http.Error(w, "Type d'export inconnu", 400)
		return
	}
	rows, err := a.db.Query(query)
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	defer rows.Close()
	cols, _ := rows.Columns()
	buf := new(bytes.Buffer)
	buf.WriteString("\xef\xbb\xbf")
	writer := csv.NewWriter(buf)
	writer.Comma = ';'
	_ = writer.Write(cols)
	for rows.Next() {
		values := make([]any, len(cols))
		dest := make([]any, len(cols))
		for i := range values {
			dest[i] = &values[i]
		}
		if err = rows.Scan(dest...); err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
		out := make([]string, len(cols))
		for i, v := range values {
			if v != nil {
				out[i] = strings.ReplaceAll(fmt.Sprint(v), ".", ",")
			}
		}
		_ = writer.Write(out)
	}
	if err = rows.Err(); err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	writer.Flush()
	if err = writer.Error(); err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	w.Header().Set("Content-Disposition", `attachment; filename="`+filename+`"`)
	_, _ = w.Write(buf.Bytes())
}

func (a *App) backup() error {
	if a.cfg.BackupDir == "" || !isNFSMounted(a.cfg.BackupDir) {
		return fmt.Errorf("NAS NFS non monté : sauvegarde impossible")
	}
	if err := os.MkdirAll(a.cfg.BackupDir, 0750); err != nil {
		return err
	}
	path := filepath.Join(a.cfg.BackupDir, "okoko-"+time.Now().Format("20060102-150405")+".db")
	// VACUUM INTO creates a consistent SQLite snapshot, including WAL changes.
	escaped := strings.ReplaceAll(path, "'", "''")
	_, err := a.db.Exec("VACUUM INTO '" + escaped + "'")
	if err != nil {
		return err
	}
	checkDB, err := sql.Open("sqlite", path)
	if err != nil {
		return err
	}
	defer checkDB.Close()
	var result string
	if err = checkDB.QueryRow("PRAGMA quick_check").Scan(&result); err != nil || result != "ok" {
		return fmt.Errorf("sauvegarde créée mais contrôle d'intégrité échoué: %s (%v)", result, err)
	}
	return nil
}

func (a *App) downloadBackup(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Méthode interdite", 405)
		return
	}
	f, e := os.CreateTemp("", "okoko12-backup-*.db")
	if e != nil {
		http.Error(w, e.Error(), 500)
		return
	}
	name := f.Name()
	f.Close()
	os.Remove(name)
	defer os.Remove(name)
	escaped := strings.ReplaceAll(name, "'", "''")
	if _, e = a.db.Exec("VACUUM INTO '" + escaped + "'"); e != nil {
		http.Error(w, e.Error(), 500)
		return
	}
	check, e := sql.Open("sqlite", name)
	if e != nil {
		http.Error(w, e.Error(), 500)
		return
	}
	var status string
	e = check.QueryRow("PRAGMA quick_check").Scan(&status)
	check.Close()
	if e != nil || status != "ok" {
		http.Error(w, "Sauvegarde invalide", 500)
		return
	}
	src, e := os.Open(name)
	if e != nil {
		http.Error(w, e.Error(), 500)
		return
	}
	defer src.Close()
	w.Header().Set("Content-Type", "application/zip")
	w.Header().Set("Content-Disposition", `attachment; filename="okoko12-`+time.Now().Format("20060102-150405")+`.zip"`)
	z := zip.NewWriter(w)
	out, e := z.Create("chaudiere.db")
	if e == nil {
		_, e = io.Copy(out, src)
	}
	if e != nil {
		log.Printf("Téléchargement sauvegarde: %v", e)
	}
	if e = z.Close(); e != nil {
		log.Printf("Fermeture sauvegarde: %v", e)
	}
}

func (a *App) Serve(ctx context.Context) error {
	s := &http.Server{Addr: a.cfg.Listen, Handler: a.Handler(), ReadHeaderTimeout: 5 * time.Second}
	go func() {
		<-ctx.Done()
		c, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = s.Shutdown(c)
	}()
	log.Printf("Chaudière Go sur %s", a.cfg.Listen)
	e := s.ListenAndServe()
	if errors.Is(e, http.ErrServerClosed) {
		return nil
	}
	return e
}

func cleanText(v string) string {
	v = strings.ToValidUTF8(v, "°")
	// The ÖkoFEN HTTP JSON can contain Latin-1 degree bytes.
	v = strings.ReplaceAll(v, "�C", "°C")
	v = strings.ReplaceAll(v, "�", "°")
	return v
}

func (a *App) addRecharge(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodPost {
		d := r.FormValue("date")
		kg, e := strconv.ParseFloat(strings.ReplaceAll(r.FormValue("kg"), ",", "."), 64)
		if !validRefillDate(d) || e != nil || kg <= 0 || kg > 10000 {
			http.Error(w, "Date ou quantité invalide", 400)
			return
		}
		supplier := strings.TrimSpace(r.FormValue("fournisseur"))
		kind := strings.TrimSpace(r.FormValue("type"))
		note := strings.TrimSpace(r.FormValue("note"))
		if len(supplier) > 100 || len(kind) > 60 || len(note) > 500 {
			http.Error(w, "Texte trop long", 400)
			return
		}
		existing, _ := a.rechargeRows()
		day, _ := time.Parse("2006-01-02", d)
		for _, v := range existing {
			vd, _ := time.Parse("2006-01-02", v.ts[:10])
			if math.Abs(day.Sub(vd).Hours()) <= 48 && math.Abs(kg-v.kg) < .01 && r.FormValue("confirm") != "yes" {
				http.Error(w, "Doublon possible (même quantité à deux jours près). Vérifier l'historique avant l'ajout.", 409)
				return
			}
		}
		ts := d + "T12:00:00"
		if supplier == "" {
			supplier = "Manuel"
		}
		if kind == "" {
			kind = "Vrac"
		}
		_, e = a.db.Exec(`INSERT INTO historical_recharges(ts,kg,fournisseur,conditionnement,origine) VALUES(?,?,?,?,?)`, ts, kg, supplier, kind, note)
		if e != nil {
			http.Error(w, e.Error(), 409)
			return
		}
		http.Redirect(w, r, "/recharge", 303)
		return
	}
}
