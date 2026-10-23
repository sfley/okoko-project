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
	"html/template"
	"io"
	"log"
	"math"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	_ "modernc.org/sqlite"
)

const Version = "1.7"

type Config struct {
	Listen        string `json:"listen"`
	BoilerURL     string `json:"boiler_url"`
	Database      string `json:"database"`
	PollSeconds   int    `json:"poll_seconds"`
	RetentionDays int    `json:"retention_days"`
	CSVDir        string `json:"csv_dir"`
	ImportMinutes int    `json:"import_minutes"`
	BackupDir     string `json:"backup_dir"`
}
type Sample struct {
	TS                                                time.Time `json:"ts"`
	Ext, HK1, HK2, WW1, Modulation, StockKG, StockMax float64   `json:"ext"`
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
	c := Config{Listen: ":8888", Database: "./chaudiere.db", PollSeconds: 60, RetentionDays: 730, CSVDir: "/mnt/nas/googledrive/chaudiere_csv", ImportMinutes: 60, BackupDir: ""}
	b, e := os.ReadFile(path)
	if e != nil {
		return c, e
	}
	e = json.Unmarshal(b, &c)
	return c, e
}
func New(c Config) (*App, error) {
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
		a.liveErr = e.Error()
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

const page = `<!doctype html><html lang="fr"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"><meta name="theme-color" content="#101722"><title>OKOKO v2.0</title><style>
*{box-sizing:border-box}html{color-scheme:dark}:root{font-family:Inter,ui-sans-serif,system-ui,-apple-system,BlinkMacSystemFont,"Segoe UI",sans-serif;color:#edf3f8;background:#0b1018;--panel:#121a25;--panel2:#172231;--line:#253347;--muted:#8fa1b5;--accent:#ff9f43;--blue:#55a7ff;--green:#48d597;--red:#ff6b6b}body{margin:0;min-height:100vh;background:radial-gradient(circle at 80% -10%,#243750 0,transparent 34%),linear-gradient(180deg,#0e1520,#090e15 55%);letter-spacing:.01em}nav{position:sticky;top:0;z-index:20;display:flex;align-items:center;gap:6px;padding:12px max(18px,calc((100vw - 1240px)/2));background:#0b111bd9;backdrop-filter:blur(16px);border-bottom:1px solid #ffffff0d;overflow-x:auto}nav:before{content:"OKOKO v2.0";font-weight:900;font-size:1.15rem;color:var(--accent);margin-right:16px;letter-spacing:.08em}nav a{color:#aab8c8;text-decoration:none;padding:9px 11px;border-radius:9px;white-space:nowrap;font-size:.9rem;font-weight:650}nav a:hover{color:white;background:#ffffff0b}.card.wide{grid-column:span 2}.kv{display:grid;grid-template-columns:1fr auto;gap:8px 16px;margin-top:12px}.kv span:nth-child(odd){color:var(--muted)}.kv span:nth-child(even){font-weight:750;text-align:right;min-width:0;overflow-wrap:anywhere}.kv{grid-template-columns:minmax(0,1fr) minmax(0,1fr)}.sectionhead{display:flex;justify-content:space-between;align-items:center;gap:12px}.weather{display:flex;gap:12px;flex-wrap:wrap}.weather .pill{min-width:125px}.dangerbtn{background:#6b2d2d;border-color:#914343}.mini{font-size:.78rem;color:var(--muted)}main{max-width:1240px;margin:0 auto;padding:34px 20px 64px}h1{font-size:clamp(1.8rem,4vw,2.65rem);margin:0 0 22px;letter-spacing:-.04em}h2{margin:28px 0 14px;font-size:1.15rem;color:#dfe8f1}.hero{display:flex;justify-content:space-between;align-items:end;gap:18px;margin-bottom:26px}.eyebrow{text-transform:uppercase;letter-spacing:.14em;font-size:.72rem;color:var(--accent);font-weight:800}.subtitle{color:var(--muted);margin-top:7px}.grid{display:grid;grid-template-columns:repeat(auto-fit,minmax(min(100%,265px),1fr));gap:14px}.card{min-width:0;overflow-wrap:anywhere;position:relative;overflow:hidden;background:linear-gradient(145deg,#162130,#101822);border:1px solid #ffffff0d;border-radius:18px;padding:20px;box-shadow:0 18px 45px #00000022}.card:after{content:"";position:absolute;width:100px;height:100px;border-radius:50%;right:-45px;top:-55px;background:#ffffff08}.label,.muted{color:var(--muted)}.label{font-size:.82rem;font-weight:700;margin-bottom:9px}.big{overflow-wrap:anywhere;font-size:clamp(1.55rem,2.6vw,2.25rem);font-weight:820;letter-spacing:-.045em}.unit{font-size:.85rem;color:#8294a8;margin-left:4px;font-weight:650}.status{display:inline-flex;align-items:center;gap:8px;padding:8px 12px;border-radius:999px;font-size:.8rem;font-weight:750;margin-bottom:24px}.status.good{color:#78e8b4;background:#143426;border:1px solid #23563e}.status.bad{color:#ff9999;background:#381d24;border:1px solid #63303b}.dot{width:8px;height:8px;border-radius:50%;background:currentColor;box-shadow:0 0 12px currentColor}.chartbox{background:linear-gradient(145deg,#131d29,#0e151f);border:1px solid #ffffff0d;border-radius:20px;padding:16px;box-shadow:0 18px 45px #0002;overflow:hidden}.toolbar{display:flex;justify-content:space-between;align-items:center;gap:12px;margin:0 0 12px}.toolbar .range{display:flex;gap:6px}.range button,button,.btn{border:1px solid #304158;background:#172334;color:#dbe6f0;padding:9px 13px;border-radius:10px;font-weight:700;cursor:pointer;text-decoration:none}.range button:hover,button:hover,.btn:hover{background:#203047}.range button.active{background:#2b425f;color:white;border-color:#45688e}canvas{display:block;width:100%;height:390px;border-radius:12px}table{width:100%;border-collapse:separate;border-spacing:0;background:#111a25;border:1px solid #ffffff0d;border-radius:16px;overflow:hidden}td,th{padding:13px 16px;border-bottom:1px solid #ffffff0a;text-align:left}th{color:#8fa1b5;font-size:.75rem;text-transform:uppercase;letter-spacing:.08em}tr:last-child td,tr:last-child th{border-bottom:0}code,pre{font-family:"SFMono-Regular",Consolas,monospace}code{background:#1a2635;padding:3px 6px;border-radius:6px;color:#b9d9ff}pre{white-space:pre-wrap;background:#0a1018;border:1px solid #243144;padding:16px;border-radius:14px;overflow:auto;color:#c5d2df}.meter{height:12px;background:#0b1119;border-radius:99px;overflow:hidden;margin-top:14px;border:1px solid #ffffff0d}.meter>i{display:block;height:100%;background:linear-gradient(90deg,#ff8c42,#ffbf69);border-radius:99px}.summary{display:flex;gap:10px;flex-wrap:wrap;margin:0 0 18px}.pill{background:#131e2a;border:1px solid #ffffff0d;padding:9px 12px;border-radius:11px;color:#aebdcb}.pill strong{color:white}.good{color:#48d597}.bad{color:#ff7777}a{color:#73b7ff}.homegrid{grid-template-columns:repeat(2,minmax(0,1fr))}.homegrid .card.wide{grid-column:span 2}.homegrid .card{min-height:210px}.homegrid .card.wide{min-height:0}.healthbar{display:flex;flex-wrap:wrap;gap:10px;margin-bottom:20px}.healthbar .status{margin:0}.diag-raw summary{cursor:pointer;font-weight:750;color:#9dc9ff;padding:14px}.diag-raw pre{max-height:460px;overflow:auto}.diag-table td{overflow-wrap:anywhere}.health-note{color:var(--muted);font-size:.85rem;margin:4px 0 18px}@media(max-width:850px){.homegrid{grid-template-columns:1fr}.homegrid .card.wide{grid-column:auto}}@media(max-width:650px){main{padding:24px 14px 50px}.hero{align-items:start;flex-direction:column}.grid{grid-template-columns:repeat(2,minmax(0,1fr))}.card{padding:16px}.big{font-size:1.65rem}canvas{height:300px}nav{padding-left:12px;padding-right:12px}nav:before{margin-right:7px}}

.gauge-wrap{display:grid;place-items:center;padding:4px 0 0}.gauge{width:min(100%,390px);height:auto;overflow:visible}.gauge .track{fill:none;stroke:#263447;stroke-width:22;stroke-linecap:butt}.gauge .band{fill:none;stroke-width:22;stroke-linecap:butt}.gauge .needle{stroke:#e7edf4;stroke-width:4;stroke-linecap:round}.gauge .hub{fill:#e7edf4}.gauge-value{font-size:28px;font-weight:850;fill:#edf3f8;text-anchor:middle}.gauge-pct{font-size:14px;font-weight:700;fill:#8fa1b5;text-anchor:middle}.gauge-label{font-size:11px;fill:#718398}.conso-chart{height:380px;width:100%;display:block}.yearnav{display:flex;gap:7px;flex-wrap:wrap}.yearnav a{border:1px solid #304158;background:#172334;color:#dbe6f0;padding:8px 12px;border-radius:10px;text-decoration:none;font-weight:750}.yearnav a.active{border-color:#ff9f43;color:#ffb66f;background:#2b2119}.total-row{font-weight:850;background:#172231}.table-scroll{overflow-x:auto;border-radius:16px}.table-scroll table{min-width:620px}.matrix td,.matrix th{text-align:right;white-space:nowrap}.matrix td:first-child,.matrix th:first-child{text-align:left}.matrix tbody tr:nth-child(even){background:#ffffff06}.month-value{background:transparent;border:0;padding:2px 5px;color:inherit;font:inherit;font-weight:inherit;cursor:pointer}.month-value:hover{color:#ffb66f;text-decoration:underline}.editable-month form{display:flex;gap:5px;align-items:center}.editable-month form[hidden]{display:none}.editable-month input[type=number]{width:90px;min-width:0;background:#0b1119;color:#edf3f8;border:1px solid #45688e;border-radius:6px;padding:6px}.editable-month form button{padding:6px}.matrix thead small{font-size:.7rem;color:#ffb66f;text-transform:none;letter-spacing:0}.bar-legend{display:flex;gap:8px;flex-wrap:wrap;margin:8px 0 14px}.bar-legend button.off{opacity:.38;text-decoration:line-through}

.meteo-banner{margin:22px 0;padding:18px 22px;background:linear-gradient(110deg,#20364e,#13212d 60%,#33271c);border:1px solid #3a4c5e;border-radius:18px}.meteo-banner h2{margin:0 0 14px}.meteo-banner .pill{flex:1;min-width:125px}.card .big{overflow-wrap:anywhere}.kv span{min-width:0}.healthbar{align-items:center}.calendar{display:grid;grid-template-columns:repeat(7,minmax(0,1fr));gap:6px}.calday{border:1px solid #2b3a4a;background:#152131;border-radius:9px;padding:8px 3px;text-align:center;font-size:.8rem;min-height:56px}.calday small{display:block;color:#91a8bc}.calday.missing{opacity:.45}.calday.warn{border-color:#dc9250;background:#352619}.calday.ok{border-color:#275d49}.actions{display:flex;gap:9px;flex-wrap:wrap;margin:15px 0}.notice{padding:12px 15px;border-radius:12px;background:#2d251a;border:1px solid #7e6036;color:#ffdca9}@media(max-width:650px){.grid{grid-template-columns:1fr}.calendar{gap:3px}.calday{font-size:.7rem;padding:6px 2px}.meteo-banner{padding:14px}}
 .site-footer{max-width:1240px;margin:0 auto 24px;padding:0 20px}.site-footer .meteo-banner{margin:0}.table-scroll tr:hover td{background:#1b2938}@media(max-width:650px){.toolbar{flex-wrap:wrap}.range{flex-wrap:wrap}.site-footer{padding:0 14px}nav{scrollbar-width:thin}}

.weather-now{display:flex;align-items:center;justify-content:space-between;gap:20px;flex-wrap:wrap;padding:8px 0 18px}.weather-headline{display:flex;align-items:center;gap:18px}.weather-symbol{font-size:2.25rem;line-height:1.25}.weather-temp{font-size:clamp(1.8rem,4vw,2.7rem)}.weather-facts{display:flex;gap:12px 22px;flex-wrap:wrap;color:#c3d3e3}.weather-days{display:grid;grid-template-columns:repeat(4,minmax(0,1fr));gap:12px}.weather-day{display:flex;flex-direction:column;align-items:center;text-align:center;gap:7px;padding:18px 10px;border:1px solid #34465b;border-radius:14px;background:#101c29}.weather-day strong{text-transform:capitalize}.weather-range{font-size:1.1rem;font-weight:750}.weather-description{font-size:.88rem;color:#afc2d6}.weather-detail{margin-top:18px}.weather-detail summary{cursor:pointer;font-weight:700;padding:12px 0}.weather-detail table{min-width:550px}.refill-form{display:grid;grid-template-columns:repeat(2,minmax(0,1fr));gap:16px;align-items:end}.refill-form label{display:flex;flex-direction:column;gap:7px;color:#c6d5e3;font-weight:650}.refill-form input,.refill-form select,.recharge-table input{width:100%;min-width:0;background:#0c1520;color:#eaf2fa;border:1px solid #43546a;border-radius:9px;padding:10px;font:inherit}.refill-form .full{grid-column:1/-1}.refill-form button{justify-self:start}.recharge-table td{padding:11px 14px}.recharge-actions[hidden],.recharge-actions [hidden]{display:none!important}.recharge-stats{display:flex;gap:10px;flex-wrap:wrap;margin-bottom:16px}.recharge-stats .pill{flex:1;min-width:150px}@media(max-width:650px){.weather-days{grid-template-columns:1fr}.refill-form{grid-template-columns:1fr}}

nav a.selected{background:#27394d;color:#f7fbff;box-shadow:inset 0 -2px 0 #f3ac68}
.section-intro{max-width:780px;color:#9dafc2;line-height:1.65;margin:-8px 0 22px}
.hub-links{display:flex;flex-wrap:wrap;gap:10px;margin:14px 0 24px}
.hub-links a{border:1px solid #34465c;background:#18283a;border-radius:11px;color:#e0ecf8;padding:11px 16px;text-decoration:none;font-weight:700}
.hub-links a:hover{background:#263b51}
.install-flow{display:grid;grid-template-columns:repeat(3,minmax(0,1fr));gap:14px;position:relative}
.install-node{text-align:center;border:1px solid #334b60;border-radius:16px;background:#172536;padding:22px 10px}
.install-node strong{display:block;font-size:1.05rem;margin-bottom:12px}
.install-node .big{color:#7ac8ee}
.install-center{text-align:center;margin:14px 0 22px;color:#e8ae6b;font-weight:800}
.install-kpis{margin-top:18px}
.eyebrow:empty{display:none}
@media(max-width:650px){.install-flow{grid-template-columns:1fr}nav{gap:3px}nav a{font-size:.81rem;padding:9px 8px}}
.subnav{display:flex;flex-wrap:wrap;gap:7px;max-width:1240px;margin:16px auto 0;padding:0 20px}.subnav:empty{display:none}.subnav a{font-size:.82rem;text-decoration:none;color:#b8cce0;background:#142130;border:1px solid #293d53;padding:8px 12px;border-radius:9px}.subnav a.selected{background:#28435e;color:white;border-color:#50769b}.subnav a:hover{background:#263d53}.homegrid .card{min-height:0}.homegrid .kv{gap:7px 12px}.homegrid .kv span:nth-child(even){text-align:right}.install-node .big{font-size:clamp(1.5rem,2.5vw,2rem)}@media(max-width:650px){.subnav{padding:0 14px;gap:5px}.subnav a{font-size:.75rem;padding:7px 9px}}

/* OKOKO 1.9 : cockpit premium */
nav{gap:9px;padding-top:16px;padding-bottom:16px;box-shadow:0 12px 35px #0003}nav:before{font-size:1.27rem;letter-spacing:.13em}nav a{border:1px solid transparent;transition:background .2s,border-color .2s,color .2s}nav a.selected{background:#ff9f431c;border-color:#ff9f4350;color:#ffc180}nav a:hover{border-color:#ffffff2a}main{max-width:1440px}.cockpit-heading{display:flex;justify-content:space-between;align-items:center;gap:20px;margin:0 0 25px}.cockpit-heading h1{margin:7px 0 0}.cockpit-actions{display:flex;gap:9px;flex-wrap:wrap}.cockpit-actions #cockpit-edit{background:linear-gradient(120deg,#f8a24d,#dd7830);border-color:#ffbc78;color:#17110c}.cockpit-grid{display:grid;grid-template-columns:repeat(4,minmax(0,1fr));gap:17px;align-items:stretch}.cockpit-module{min-width:0;grid-column:span 1;border:1px solid #2d3d52;border-radius:20px;background:linear-gradient(155deg,#192637,#0e1723);box-shadow:0 16px 40px #0004;overflow:hidden;transition:border-color .2s,box-shadow .2s}.cockpit-module:hover{border-color:#49617c;box-shadow:0 20px 48px #0006}.cockpit-module.module-wide{grid-column:span 4}.cockpit-module[data-module="boiler"]{grid-column:span 1}.cockpit-module .card{border:0;background:none;box-shadow:none;border-radius:0;padding:12px 20px 20px;min-height:0}.cockpit-module .card:after{display:none}.cockpit-module .card.wide{grid-column:auto}.cockpit-module .big{font-size:clamp(1.45rem,2vw,2.1rem)}.cockpit-module .kv{grid-template-columns:minmax(0,1fr) minmax(0,1fr);font-size:.84rem;gap:9px}.module-head{display:flex;align-items:center;justify-content:space-between;gap:9px;padding:17px 19px 4px;font-size:.76rem;letter-spacing:.10em;text-transform:uppercase;font-weight:850;color:#a9c1d7}.module-controls{display:flex;gap:4px;letter-spacing:0;text-transform:none}.module-controls button{font-size:.72rem;padding:5px 7px}.module-hidden{display:none!important}.is-editing .cockpit-module{outline:1px dashed #ffae5c;cursor:grab}.is-editing .cockpit-module:active{cursor:grabbing}.cockpit-module[data-module="charts"] .grid{grid-template-columns:repeat(3,minmax(0,1fr));padding:0 10px 12px}.cockpit-module[data-module="charts"] .card{padding:14px}.weather-periods{display:grid;grid-template-columns:repeat(3,minmax(0,1fr));gap:5px;width:100%;margin-top:8px}.weather-period{display:flex;flex-direction:column;gap:6px;background:#1d3041;border-radius:9px;padding:9px 4px;font-size:.73rem}.weather-period strong{font-size:.86rem;color:#f2f6fb}.weather-day{min-width:0}.cockpit-module[data-module="boiler"] .sectionhead .big{display:flex;justify-content:center;align-items:center;text-align:center;border:1px solid #36516c;border-radius:13px;background:#0c1723;min-height:65px;margin:10px 0}.cockpit-module[data-module="boiler"] .sectionhead{display:block}.cockpit-module[data-module="boiler"] .sectionhead .label{display:none}
@media(max-width:1100px){.cockpit-grid{grid-template-columns:repeat(2,minmax(0,1fr))}.cockpit-module.module-wide{grid-column:span 2}.cockpit-module[data-module="charts"] .grid{grid-template-columns:1fr 1fr}.weather-days{grid-template-columns:repeat(2,minmax(0,1fr))}}
@media(max-width:650px){.cockpit-heading{align-items:flex-start;flex-direction:column}.cockpit-grid{grid-template-columns:1fr}.cockpit-module,.cockpit-module.module-wide,.cockpit-module[data-module="boiler"]{grid-column:span 1}.cockpit-module[data-module="charts"] .grid{grid-template-columns:1fr}.weather-days{grid-template-columns:1fr}.weather-periods{grid-template-columns:repeat(3,minmax(0,1fr))}}

/* OKOKO 2.0 — unified visual system */
:root{--panel:#111e30;--panel2:#192b41;--line:#29435b;--accent:#ffad55;--cyan:#56d5d2;--blue:#68aaff;--green:#50d5a6;color:#edf6ff;background:#08121f}
html{background:radial-gradient(ellipse at 80% -20%,#1a3b55 0%,transparent 55%),#08121f}body{background:transparent;color:#edf6ff;min-height:100vh}body:before{content:'';position:fixed;inset:0 0 auto;height:5px;background:linear-gradient(90deg,#ffac4e,#f46f43 35%,#4ac4cb 70%,#4c82ed);z-index:100;pointer-events:none}
nav{max-width:none!important;margin:0!important;padding:16px max(22px,calc((100vw - 1460px)/2))!important;display:flex;align-items:center;gap:8px!important;background:#0c1a2beb!important;border-bottom:1px solid #2c465b!important;box-shadow:0 12px 35px #0004!important;backdrop-filter:blur(20px)}nav:before{content:'◉  OKOKO'!important;color:#ffac59!important;font-size:1.28rem!important;font-weight:900!important;letter-spacing:.11em!important;margin-right:auto;white-space:nowrap}nav a{display:inline-flex;align-items:center;gap:9px;border-radius:11px!important;padding:12px 15px!important;color:#b4c9da!important;font-size:.86rem!important;font-weight:700!important}nav a:before{display:inline-block;font-size:1.1rem;color:#6e99ba}nav a[href='/']:before{content:'▦'}nav a[href='/installation']:before{content:'◈'}nav a[href='/analyse']:before{content:'⌁'}nav a[href='/pellets']:before{content:'◉'}nav a[href='/meteo']:before{content:'☀'}nav a[href='/system']:before{content:'⚙'}nav a.selected{color:#ffd09c!important;background:linear-gradient(120deg,#ffad5526,#ffad550b)!important;border:1px solid #f9a55355!important;box-shadow:inset 0 -2px 0 #f9a553!important}nav a:hover{background:#1b3145!important;color:#fff!important}
main{max-width:1480px!important;padding:24px 28px 60px!important}.subnav{max-width:1480px!important;padding:0 28px!important}.subnav a{border-radius:10px!important}.subnav a:hover{border-color:#5c8ca7!important;color:#fff!important}
h1{letter-spacing:-.035em!important;font-weight:850!important}.eyebrow{text-transform:uppercase;letter-spacing:.15em!important;font-size:.7rem!important;color:#55d4d2!important;font-weight:800!important}.subtitle,.muted,.mini{color:#9fb7cc!important}
.card,.chartbox,.install-node,.weather-day,.cockpit-module{background:linear-gradient(145deg,#1b2c41 0%,#111e2e 82%)!important;border:1px solid #2d465c!important;border-radius:20px!important;box-shadow:0 15px 45px #0003!important}.card:hover,.cockpit-module:hover{border-color:#476d87!important}.card{padding:22px!important}.big{letter-spacing:-.035em;font-weight:800!important}button,.btn{border-radius:11px!important;transition:background .18s,border-color .18s,transform .18s}button:hover,.btn:hover{transform:translateY(-1px)}button:focus-visible,a:focus-visible{outline:3px solid #55d5d2!important;outline-offset:3px}
.cockpit-heading{margin:10px 0 26px!important}.cockpit-heading h1{font-size:clamp(2rem,3.5vw,3rem)!important;line-height:1.15}.cockpit-actions{background:#142438;border:1px solid #2b4257;padding:7px;border-radius:15px}.cockpit-actions button{font-weight:750!important}.cockpit-grid{grid-template-columns:repeat(4,minmax(0,1fr))!important;gap:19px!important}.cockpit-module{position:relative;grid-column:span 1;min-height:210px;overflow:hidden}.cockpit-module.module-medium{grid-column:span 2}.cockpit-module.module-wide{grid-column:span 4}.cockpit-module:before{content:'';position:absolute;left:0;top:0;right:0;height:3px;background:linear-gradient(90deg,#ffac56,#59d5d1);opacity:.65}.cockpit-module[data-module='hk1']:before,.cockpit-module[data-module='hk2']:before{background:#59aaff}.cockpit-module[data-module='ecs']:before{background:#55d6d0}.cockpit-module[data-module='charts']:before{background:linear-gradient(90deg,#5eaeff,#ffad55)}.cockpit-module .card{background:none!important;border:0!important;box-shadow:none!important;padding:14px 19px 20px!important}.cockpit-module .card:hover{border:0!important}.module-head{font-size:.75rem!important;color:#c5d9e9!important;padding:21px 20px 8px!important;letter-spacing:.095em!important}.module-head span:before{content:'◈';font-size:1.1rem;color:#ffad55;margin-right:10px}.cockpit-module[data-module='hk1'] .module-head span:before,.cockpit-module[data-module='hk2'] .module-head span:before{content:'◉';color:#68aaff}.cockpit-module[data-module='ecs'] .module-head span:before{content:'◍';color:#5bd4d0}.cockpit-module[data-module='charts'] .module-head span:before{content:'⌁';color:#5bd4d0}.cockpit-module .big{font-size:clamp(1.55rem,2.4vw,2.45rem)!important}.cockpit-module .label{color:#9fb7cc!important}.cockpit-module[data-module='boiler'] .sectionhead .big{min-height:70px;background:linear-gradient(120deg,#152b3e,#0e1e2f)!important;border:1px solid #45627c!important;color:#ffbd79!important}.module-controls button{background:#24394e!important;border:1px solid #496178!important;color:#e3f2ff!important}.is-editing .cockpit-module{outline:2px dashed #ffae5c!important;outline-offset:3px}.is-editing .cockpit-module:after{content:'⠿';position:absolute;right:17px;bottom:9px;color:#ffbd79;pointer-events:none}.module-library{display:grid;grid-template-columns:repeat(auto-fill,minmax(215px,1fr));gap:10px;margin:0 0 22px;padding:20px;background:#132338;border:1px solid #42617a;border-radius:20px;box-shadow:0 20px 50px #0005}.library-heading{grid-column:1/-1;display:flex;align-items:center;gap:12px;margin-bottom:6px}.library-heading strong{font-size:1.15rem}.library-heading span{color:#9fb7cc}.library-heading button{margin-left:auto}.library-item{text-align:left;background:#1a3046;border:1px solid #36516a;color:#edf6ff;padding:15px;border-radius:12px;font-weight:700}.library-item[aria-pressed='true']{border-color:#51b7aa;background:#1b3b44;color:#a7f2e4}.weather-symbol{filter:drop-shadow(0 4px 12px #ffb44c55)}.weather-period{background:#20364c!important;border:1px solid #37516b;border-radius:13px!important}.weather-period strong{color:#ffcf96!important}.table-scroll{border-radius:17px;overflow:auto}th{color:#c4deef!important}tr:hover td{background:#1b3346!important}
@media(max-width:1180px){nav{flex-wrap:wrap}nav:before{width:100%;margin-bottom:3px}.cockpit-grid{grid-template-columns:repeat(2,minmax(0,1fr))!important}.cockpit-module.module-wide{grid-column:span 2}.cockpit-module.module-medium{grid-column:span 2}}
@media(max-width:680px){main{padding:16px 14px 50px!important}nav{padding:13px 12px!important;gap:4px!important}nav a{padding:9px 10px!important;font-size:.75rem!important}.cockpit-grid{grid-template-columns:1fr!important}.cockpit-module,.cockpit-module.module-medium,.cockpit-module.module-wide{grid-column:span 1!important}.cockpit-heading{flex-direction:column;align-items:stretch!important}.cockpit-actions{flex-wrap:wrap}.module-library{grid-template-columns:1fr}}
</style></head><body><nav aria-label="Navigation principale"><a href="/">Tableau de bord</a><a href="/installation">Chauffage</a><a href="/analyse">Analyse</a><a href="/pellets">Pellets</a><a href="/meteo">Météo</a><a href="/system">Système</a></nav><div class="subnav" id="section-nav" aria-label="Navigation secondaire"></div><main>{{.Body}}</main><footer class="site-footer">{{.Footer}}</footer><script>(function(){const path=location.pathname;
const sections=[{root:'/',paths:['/'],links:[]},{root:'/installation',paths:['/installation'],links:[['/installation','Circuits et chaudière'],['/transitions','Transitions'],['/modulation','Modulation']]},{root:'/analyse',paths:['/analyse','/temp','/modulation','/transitions','/calendar','/bilan'],links:[['/analyse','Vue générale'],['/temp','Températures'],['/modulation','Modulation'],['/transitions','Transitions'],['/calendar','Couverture']]},{root:'/pellets',paths:['/pellets','/stock','/conso','/recharge'],links:[['/pellets','Vue générale'],['/stock','Stock'],['/conso','Consommations'],['/recharge','Recharges']]},{root:'/meteo',paths:['/meteo'],links:[]},{root:'/system',paths:['/system'],links:[]}];
const active=sections.find(s=>s.paths.includes(path));if(active&&active.links.length){const el=document.getElementById('section-nav');active.links.forEach(([href,label])=>{const a=document.createElement('a');a.href=href;a.textContent=label;if(href===path){a.className='selected';a.setAttribute('aria-current','page')}el.append(a)});}
document.querySelectorAll('nav a').forEach(a=>{const href=a.getAttribute('href');if(href===path){a.classList.add('selected');a.setAttribute('aria-current','page')}})})();</script></body></html>`

var tpl = template.Must(template.New("p").Parse(page))

type view struct {
	Title  string
	Body   template.HTML
	Footer template.HTML
}

func weatherFooter() string {
	return `<section class="meteo-banner" aria-label="Météo et prévisions">
<h2>Météo et prévisions</h2><div id="weather-live" class="weather-now"><span class="muted">Chargement…</span></div>
<h3>Les 3 prochains jours · aujourd’hui inclus</h3><div id="weather-days" class="weather-days"></div>
<details class="weather-detail"><summary>Prévisions horaires détaillées</summary><div class="table-scroll"><table><thead><tr><th>Date et heure</th><th>Conditions</th><th>Température</th><th>Vent</th><th>Nuages</th></tr></thead><tbody id="weather-hours"></tbody></table></div></details>
</section><script>(function(){
function node(tag,cls,text){const e=document.createElement(tag);if(cls)e.className=cls;if(text!==undefined)e.textContent=text;return e}
function icon(x){const c=Number(x.clouds),t=(x.condition||'').toLowerCase();return t.includes('neige')?'❄':t.includes('pluie')?'☂':Number.isFinite(c)?(c>=75?'☁':c>=35?'⛅':'☀'):'◌'}
function parseDate(raw){
 const s=String(raw||'').trim();
 const months={janv:0,janvier:0,fevr:1,fevrier:1,mars:2,avr:3,avril:3,mai:4,juin:5,juil:6,juillet:6,aout:7,sept:8,septembre:8,oct:9,octobre:9,nov:10,novembre:10,dec:11,decembre:11};
 const clean=s.normalize('NFD').replace(/[\u0300-\u036f]/g,'').toLowerCase();
 const m=clean.match(/(?:lun|mar|mer|jeu|ven|sam|dim)[^0-9]*(\d{1,2})\s+([a-z]+)\.?\s*(?:(\d{4})\s*)?(\d{1,2}):(\d{2})/);
 if(m){const month=months[m[2]];if(month===undefined)return null;const year=m[3]?Number(m[3]):new Date().getFullYear();return new Date(year,month,Number(m[1]),Number(m[4]),Number(m[5]));}
 const d=new Date(s);return Number.isFinite(d.getTime())?d:null;
}
fetch('/api/weather').then(r=>{if(!r.ok)throw Error('Météo indisponible');return r.json()}).then(d=>{
const now=document.getElementById('weather-live');now.replaceChildren();
const cur=d.current||{};const headline=node('div','weather-headline');headline.append(node('span','weather-symbol',icon((d.forecast||[])[0]||{})),node('strong','weather-temp',cur['Température']||'—'));now.append(headline);
const details=node('div','weather-facts');for(const key of ['Vent','Lever','Coucher','Durée du jour']){if(cur[key])details.append(node('span','',key+' : '+cur[key]))}now.append(details);
const groups=new Map();const hourly=document.getElementById('weather-hours');
for(const x of d.forecast||[]){const dt=parseDate(x.date),key=dt?dt.toLocaleDateString('fr-FR',{weekday:'long',day:'numeric',month:'short'}):String(x.date).slice(0,15);if(!groups.has(key))groups.set(key,[]);groups.get(key).push(x);
const tr=node('tr');for(const v of [x.date,icon(x)+' '+x.condition,x.temp+' °C',x.wind,x.clouds+' %'])tr.append(node('td','',v));hourly.append(tr)}
const days=document.getElementById('weather-days');
for(const [day,rows] of [...groups].slice(0,4)){const card=node('div','weather-day'),temps=rows.map(x=>Number(x.temp)).filter(Number.isFinite),mid=rows[Math.floor(rows.length/2)];card.append(node('strong','',day),node('div','weather-symbol',icon(mid)),node('div','weather-range',temps.length?Math.min(...temps)+' à '+Math.max(...temps)+' °C':'—'),node('div','weather-description',mid.condition||'Prévisions disponibles'));const parts=node('div','weather-periods');for(const [label,lo,hi] of [['Matin',5,11],['Midi',11,16],['Soir',16,23]]){const subset=rows.filter(x=>{const dt=parseDate(x.date);return dt&&dt.getHours()>=lo&&dt.getHours()<hi});const period=node('div','weather-period');const value=subset.length?subset[Math.floor(subset.length/2)]:null;period.append(node('span','',label),node('strong','',value?icon(value)+' '+value.temp+' °C':'—'));parts.append(period)}card.append(parts);days.append(card)}
if(!hourly.children.length)hourly.append(node('tr','','Aucune prévision disponible'));
}).catch(e=>{document.getElementById('weather-live').textContent=e.message})})();</script>`
}
func render(w http.ResponseWriter, title, body string) {
	renderWithFooter(w, title, body, "")
}
func renderWithFooter(w http.ResponseWriter, title, body, footer string) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	_ = tpl.Execute(w, view{Title: title, Body: template.HTML(body), Footer: template.HTML(footer)})
}
func esc(s string) string { return template.HTMLEscapeString(s) }
func (a *App) status() string {
	a.mu.RLock()
	defer a.mu.RUnlock()
	if a.liveErr != "" || (!a.liveAt.IsZero() && time.Since(a.liveAt) > 3*time.Minute) {
		return `<div class="status bad"><span class="dot"></span>Chaudière inaccessible · ` + esc(a.liveErr) + `</div>`
	}
	if a.liveAt.IsZero() {
		return `<div class="status"><span class="dot"></span>En attente de la première lecture…</div>`
	}
	return `<div class="status good"><span class="dot"></span>Chaudière connectée · ` + a.liveAt.Format("02/01/2006 15:04:05") + `</div>`
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

func (a *App) health() string {
	boiler := a.status()
	csv := `<span class="status bad"><span class="dot"></span>NAS / CSV indisponibles</span>`
	if a.cfg.CSVDir != "" {
		if isNFSMounted(a.cfg.CSVDir) {
			if true {
				csv = `<span class="status good"><span class="dot"></span>Montage NAS OK · CSV accessibles</span>`
			}
		}
	}
	return `<div class="healthbar">` + boiler + csv + `<span class="status good"><span class="dot"></span>Service web OK</span></div>`
}
func (a *App) liveNum(cat, key string) float64 {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return first(a.live, [2]string{cat, key})
}
func card(name string, v float64, unit string) string {
	return fmt.Sprintf(`<div class="card"><div class="label">%s</div><div class="big">%.1f<span class="unit">%s</span></div></div>`, name, v, unit)
}
func (a *App) liveAny(cat, key string) any {
	a.mu.RLock()
	defer a.mu.RUnlock()
	c, ok := a.live[cat].(map[string]any)
	if !ok {
		return nil
	}
	v, ok := c[key]
	if !ok {
		return nil
	}
	if m, ok := v.(map[string]any); ok {
		val := m["val"]
		if f, ok := num(m["factor"]); ok {
			if n, ok := num(val); ok {
				val = n * f
			}
		}
		if fm, ok := m["format"].(string); ok {
			if n, ok := num(m["val"]); ok {
				parts := strings.Split(fm, "|")
				i := int(n)
				if i >= 0 && i < len(parts) {
					q := strings.SplitN(parts[i], ":", 2)
					if len(q) == 2 {
						return cleanText(q[1])
					}
				}
			}
		}
		if u, ok := m["unit"].(string); ok && u != "" {
			return fmt.Sprintf("%s %s", fmtNum(val), cleanText(u))
		}
		return val
	}
	return v
}
func cleanText(v string) string {
	v = strings.ToValidUTF8(v, "°")
	// The ÖkoFEN HTTP JSON can contain Latin-1 degree bytes.
	v = strings.ReplaceAll(v, "�C", "°C")
	v = strings.ReplaceAll(v, "�", "°")
	return v
}
func fmtNum(v any) string {
	if n, ok := num(v); ok && !math.IsNaN(n) && !math.IsInf(n, 0) {
		return strconv.FormatFloat(n, 'f', 1, 64)
	}
	return show(v)
}
func (a *App) display(cat, key string) string {
	v := a.liveAny(cat, key)
	if v == nil {
		return "—"
	}
	return cleanText(show(v))
}
func show(v any) string {
	if v == nil {
		return "—"
	}
	switch x := v.(type) {
	case float64:
		return strconv.FormatFloat(x, 'f', 1, 64)
	case json.Number:
		return x.String()
	default:
		return cleanText(fmt.Sprint(x))
	}
}
func (a *App) weatherNow() (date, temp, speed, sunrise, sunset, daylen string) {
	a.mu.RLock()
	defer a.mu.RUnlock()
	date, temp, speed, sunrise, sunset, daylen = "—", "—", "—", "—", "—", "—"
	f, ok := a.live["forecast"].(map[string]any)
	if !ok {
		return
	}
	x, ok := f["L_w_0"].(map[string]any)
	if !ok {
		return
	}
	raw := fmt.Sprint(x["val"])
	p := strings.Split(raw, "|")
	if len(p) > 0 {
		date = p[0]
	}
	if len(p) > 1 {
		temp = p[1]
	}
	if len(p) > 3 {
		speed = p[3]
	}
	if len(p) > 7 {
		sunrise = p[7]
	}
	if len(p) > 8 {
		sunset = p[8]
	}
	if sunrise != "—" && sunset != "—" {
		a1, e1 := time.Parse("15:04", sunrise)
		a2, e2 := time.Parse("15:04", sunset)
		if e1 == nil && e2 == nil {
			d := a2.Sub(a1)
			daylen = fmt.Sprintf("%02dh%02d", int(d.Hours()), int(d.Minutes())%60)
		}
	}
	return
}
func modeHTML(v string) string {
	v = translateStatus(v)
	parts := strings.Split(v, "|")
	for i := range parts {
		parts[i] = esc(strings.TrimSpace(parts[i]))
	}
	return strings.Join(parts, "<br>")
}
func (a *App) home(w http.ResponseWriter, r *http.Request) {
	boiler := fmt.Sprintf(`<div class="card wide"><div class="sectionhead"><div><div class="label">CHAUDIÈRE · %s</div><div class="big">%s</div></div></div><div class="kv"><span>Température</span><span>%s</span><span>Modulation</span><span>%s</span><span>Flamme</span><span>%s</span><span>Ventilation / débit air</span><span>%s</span><span>Dépression</span><span>%s</span><span>Pompe primaire</span><span>%s</span><span>Température extérieure</span><span>%s</span></div></div>`, esc(a.display("pe1", "L_type")), modeHTML(a.display("pe1", "L_statetext")), esc(a.display("pe1", "L_temp_act")), esc(a.display("pe1", "L_modulation")), esc(a.display("pe1", "L_frt_temp_act")), esc(a.display("pe1", "L_currentairflow")), esc(a.display("pe1", "L_lowpressure")), esc(a.display("pe1", "L_uw_speed")), esc(a.display("system", "L_ambient")))
	circuit := func(cat, title string) string {
		return fmt.Sprintf(`<div class="card"><div class="label">%s</div><div class="big">%s</div><div class="kv"><span>Mode</span><span>%s</span><span>Consigne</span><span>%s</span><span>Départ eau</span><span>%s</span><span>Pompe</span><span>%s</span><span>Programme</span><span>%s</span></div></div>`, title, esc(show(a.liveAny(cat, "L_roomtemp_act"))), modeHTML(show(a.liveAny(cat, "L_statetext"))), esc(a.programmedSetpoint(cat)), esc(show(a.liveAny(cat, "L_flowtemp_act"))), esc(show(a.liveAny(cat, "L_pump"))), esc(show(a.liveAny(cat, "time_prg"))))
	}
	ecs := fmt.Sprintf(`<div class="card"><div class="label">EAU CHAUDE SANITAIRE</div><div class="big">%s</div><div class="kv"><span>Mode</span><span>%s</span><span>Réglage min</span><span>%s</span><span>Réglage max</span><span>%s</span><span>Pompe</span><span>%s</span></div></div>`, esc(a.display("ww1", "L_ontemp_act")), modeHTML(a.display("ww1", "L_statetext")), esc(a.display("ww1", "temp_min_set")), esc(a.display("ww1", "temp_max_set")), esc(a.display("ww1", "L_pump")))
	stats := fmt.Sprintf(`<div class="card wide"><div class="label">PERFORMANCE CHAUDIÈRE</div><div class="kv"><span>Heures de fonctionnement</span><span>%s</span><span>Démarrages</span><span>%s</span><span>Durée moyenne des cycles</span><span>%s</span><span>Température foyer</span><span>%s</span><span>Erreurs système</span><span>%s</span></div></div>`, esc(a.display("pe1", "L_runtime")), esc(a.display("pe1", "L_starts")), esc(a.display("pe1", "L_avg_runtime")), esc(a.display("pe1", "L_frt_temp_act")), esc(a.display("system", "L_errors")))
	a.mu.RLock()
	lastLive, liveError := a.liveAt, a.liveErr
	a.mu.RUnlock()
	freshness := `<div class="health-note">Données temps réel non encore reçues</div>`
	if !lastLive.IsZero() {
		age := time.Since(lastLive)
		freshness = fmt.Sprintf(`<div class="health-note">Dernière actualisation : %s · %s</div>`, lastLive.Local().Format("15:04:05"), func() string {
			if age > 3*time.Minute {
				return "Données anciennes"
			}
			return "Données récentes"
		}())
	}
	if liveError != "" {
		freshness += `<div class="notice">Chaudière momentanément inaccessible : dernières valeurs conservées.</div>`
	}
	b := `<div class="cockpit-heading"><div><div class="eyebrow">Vue personnalisée</div><h1>Mon cockpit</h1><p class="subtitle">Votre installation, à votre façon.</p></div><div class="cockpit-actions"><button type="button" id="cockpit-edit" aria-pressed="false">Personnaliser</button><button type="button" id="cockpit-add" hidden>+ Ajouter un module</button><button type="button" id="cockpit-reset" hidden>Réinitialiser</button></div></div>` + freshness + `<div class="cockpit-grid" id="cockpit-grid">` + `<section class="cockpit-module" data-module="boiler"><div class="module-head"><span>Chaudière ÖkoFEN</span></div>` + boiler + `</section>` + `<section class="cockpit-module" data-module="hk1"><div class="module-head"><span>Chambres · HK1</span></div>` + circuit("hk1", "CHAMBRES · HK1") + `</section>` + `<section class="cockpit-module" data-module="hk2"><div class="module-head"><span>Plancher · HK2</span></div>` + circuit("hk2", "PLANCHER · HK2") + `</section>` + `<section class="cockpit-module" data-module="ecs"><div class="module-head"><span>Eau chaude sanitaire</span></div>` + ecs + `</section>` + `<section class="cockpit-module" data-module="performance"><div class="module-head"><span>Performance chaudière</span></div>` + stats + `</section>`

	b += `<section class="cockpit-module module-wide" data-module="charts"><div class="module-head"><span>Températures et consignes · 24 h</span></div><div class="grid"><div class="card"><div class="label">Chambres · HK1</div><canvas class="spark" data-kind="hk1"></canvas></div><div class="card"><div class="label">Plancher · HK2</div><canvas class="spark" data-kind="hk2"></canvas></div><div class="card"><div class="label">Eau chaude · WW1</div><canvas class="spark" data-kind="ww1"></canvas></div></div></section></div><script>
fetch('/api/mini').then(r=>r.json()).then(d=>{document.querySelectorAll('canvas.spark').forEach(c=>{const ctx=c.getContext('2d'),p=d[c.dataset.kind]||[],W=440,H=170,L=52,R=12,T=12,B=32;c.width=W;c.height=H;c.style.height='170px';if(!p.length){ctx.fillStyle='#8fa1b5';ctx.fillText('Pas de mesures',L,40);return;}const vals=p.flatMap(x=>[Number(x[1]),Number(x[2])]).filter(Number.isFinite);if(!vals.length)return;const lo=Math.min(...vals)-1,hi=Math.max(...vals)+1;const first=new Date(p[0][0]).getTime(),last=new Date(p[p.length-1][0]).getTime();function xAt(i){const t=new Date(p[i][0]).getTime();return Number.isFinite(t)&&last>first?L+(t-first)/(last-first)*(W-L-R):L+i*(W-L-R)/Math.max(1,p.length-1)}function line(k,color){ctx.strokeStyle=color;ctx.lineWidth=2;ctx.beginPath();let started=false;p.forEach((row,i)=>{if(row[k]===null||row[k]===undefined)return;const v=Number(row[k]);if(!Number.isFinite(v))return;const x=xAt(i),y=T+(hi-v)*(H-T-B)/(hi-lo);if(!started){ctx.moveTo(x,y);started=true}else ctx.lineTo(x,y)});ctx.stroke()}ctx.font='11px system-ui';ctx.textAlign='left';for(let j=0;j<=4;j++){const yy=T+(H-T-B)*j/4;ctx.strokeStyle='#29394b';ctx.beginPath();ctx.moveTo(L,yy);ctx.lineTo(W-R,yy);ctx.stroke();ctx.fillStyle='#afc0d1';ctx.fillText((hi-(hi-lo)*j/4).toFixed(1)+'°',3,yy+4)}ctx.textAlign='center';for(let i=0;i<4;i++){const x=L+i*(W-L-R)/3;ctx.strokeStyle='#2c3b4d';ctx.beginPath();ctx.moveTo(x,T);ctx.lineTo(x,H-B+3);ctx.stroke();if(Number.isFinite(first)&&Number.isFinite(last)){const dt=new Date(first+(last-first)*i/3);ctx.fillStyle='#9dafc2';ctx.fillText(dt.toLocaleTimeString('fr-FR',{hour:'2-digit',minute:'2-digit'}),x,H-10)}}line(1,'#55a7ff');line(2,'#ff9f43')})}).catch(()=>{});</script><script>(function(){
const grid=document.getElementById('cockpit-grid');if(!grid)return;
const key='okoko20-cockpit';const edit=document.getElementById('cockpit-edit'),add=document.getElementById('cockpit-add'),reset=document.getElementById('cockpit-reset');let editing=false,drag=null;
const modules=[...grid.querySelectorAll('[data-module]')];const known=new Map(modules.map(m=>[m.dataset.module,m]));
function save(){try{localStorage.setItem(key,JSON.stringify([...grid.children].map(m=>({id:m.dataset.module,hidden:m.classList.contains('module-hidden'),size:m.classList.contains('module-wide')?4:m.classList.contains('module-medium')?2:1}))))}catch(e){}}
function load(){try{const data=JSON.parse(localStorage.getItem(key)||'[]');for(const x of data){const m=known.get(x.id);if(!m)continue;m.classList.toggle('module-hidden',!!x.hidden);m.classList.toggle('module-wide',x.size===4||(!x.size&&!!x.wide));m.classList.toggle('module-medium',x.size===2);grid.append(m)}}catch(e){}}
function refresh(){modules.forEach(m=>{m.draggable=editing;let controls=m.querySelector('.module-controls');if(!controls){controls=document.createElement('div');controls.className='module-controls';const wide=document.createElement('button');wide.type='button';wide.textContent='▦ Taille';wide.title='Changer la largeur : 1, 2 ou 4 colonnes';wide.onclick=()=>{const n=m.classList.contains('module-wide')?1:m.classList.contains('module-medium')?4:2;m.classList.toggle('module-wide',n===4);m.classList.toggle('module-medium',n===2);wide.setAttribute('aria-label','Taille '+n+' colonnes');save()};const hide=document.createElement('button');hide.type='button';hide.textContent='× Masquer';hide.onclick=()=>{m.classList.add('module-hidden');save()};controls.append(wide,hide);m.querySelector('.module-head').append(controls)}controls.hidden=!editing});grid.classList.toggle('is-editing',editing);add.hidden=reset.hidden=!editing;edit.setAttribute('aria-pressed',String(editing));edit.textContent=editing?'Terminer':'Personnaliser'}
modules.forEach(m=>{m.addEventListener('dragstart',e=>{if(!editing){e.preventDefault();return}drag=m;e.dataTransfer.effectAllowed='move'});m.addEventListener('dragover',e=>{if(editing&&drag&&drag!==m)e.preventDefault()});m.addEventListener('drop',e=>{if(!editing||!drag||drag===m)return;e.preventDefault();const nodes=[...grid.children];if(nodes.indexOf(drag)<nodes.indexOf(m))m.after(drag);else m.before(drag);save()});m.addEventListener('dragend',()=>drag=null)});
edit.onclick=()=>{editing=!editing;refresh()};add.onclick=()=>{let panel=document.getElementById('module-library');if(panel){panel.remove();return}panel=document.createElement('div');panel.id='module-library';panel.className='module-library';panel.setAttribute('role','dialog');panel.setAttribute('aria-label','Bibliothèque de modules');const heading=document.createElement('div');heading.className='library-heading';heading.innerHTML='<strong>Bibliothèque de modules</strong><span>Choisissez les éléments à afficher</span>';const close=document.createElement('button');close.type='button';close.textContent='✕';close.setAttribute('aria-label','Fermer');close.onclick=()=>panel.remove();heading.append(close);panel.append(heading);modules.forEach(m=>{const item=document.createElement('button');item.type='button';item.className='library-item';const name=m.querySelector('.module-head span').textContent;item.textContent=(m.classList.contains('module-hidden')?'＋  ':'✓  ')+name;item.setAttribute('aria-pressed',String(!m.classList.contains('module-hidden')));item.onclick=()=>{m.classList.toggle('module-hidden');item.textContent=(m.classList.contains('module-hidden')?'＋  ':'✓  ')+name;item.setAttribute('aria-pressed',String(!m.classList.contains('module-hidden')));save()};panel.append(item)});document.querySelector('.cockpit-heading').after(panel)};reset.onclick=()=>{if(!confirm('Rétablir la disposition par défaut ?'))return;try{localStorage.removeItem(key)}catch(e){}modules.forEach(m=>{m.classList.remove('module-hidden');m.classList.toggle('module-wide',m.dataset.module==='charts');m.classList.remove('module-medium');grid.append(m)});refresh()};load();refresh();
})();</script><p class="mini">Bleu : température réelle · Orange : consigne. Échelle horaire locale. La disposition est conservée dans ce navigateur.</p>`
	render(w, "Tableau de bord", b)
}

func chartPage(title, endpoint string) string {
	return `<div class="hero"><div><div class="eyebrow">Historique</div><h1>` + esc(title) + `</h1><div class="subtitle">Clique sur une légende pour masquer ou réafficher une courbe</div></div></div><div class="chartbox"><div class="toolbar"><span class="muted" id="rangeLabel">24 dernières heures</span><div class="range"><button class="active" data-days="1">1 j</button><button data-days="7">7 j</button><button data-days="30">30 j</button><button data-days="90">90 j</button><button data-days="365">1 an</button></div></div><div id="legend" class="summary"></div><canvas id="c"></canvas><div id="chartTip" class="muted" aria-live="polite">Survolez la courbe pour voir les mesures</div></div><script>
const base='` + endpoint + `',c=document.getElementById('c'),ctx=c.getContext('2d'),legend=document.getElementById('legend');let data=[],hidden={};const colors=['#55a7ff','#ff9f43','#48d597','#b38cff','#ff6b6b','#e7d36b'];
function load(days,b){document.querySelectorAll('.range button').forEach(x=>x.classList.remove('active'));if(b)b.classList.add('active');document.getElementById('rangeLabel').textContent=days==365?'12 derniers mois':(days==1?'24 dernières heures':days+' derniers jours');let u=new URL(base,location.origin);u.searchParams.set('days',days);fetch(u).then(r=>r.json()).then(d=>{data=d;buildLegend();draw()})}
function keys(){return data.length?Object.keys(data[0]).filter(k=>k!='ts'):[]}
function buildLegend(){legend.innerHTML='';keys().forEach((k,i)=>{let b=document.createElement('button');b.type='button';b.className='btn';b.style.opacity=hidden[k]?.45:1;b.innerHTML='<span style="color:'+colors[i%colors.length]+'">●</span> '+k;b.addEventListener('click',()=>{hidden[k]=!hidden[k];buildLegend();draw()});legend.appendChild(b)})}
function draw(){const dpr=devicePixelRatio||1,W=c.clientWidth,H=c.clientHeight;c.width=W*dpr;c.height=H*dpr;ctx.setTransform(dpr,0,0,dpr,0,0);ctx.clearRect(0,0,W,H);if(!data.length){ctx.fillStyle='#8fa1b5';ctx.font='14px system-ui';ctx.fillText('Pas de données sur cette période',24,45);return}const ks=keys().filter(k=>!hidden[k]),vals=data.flatMap(o=>ks.map(k=>+o[k]).filter(Number.isFinite));if(!vals.length){ctx.fillStyle='#8fa1b5';ctx.fillText('Toutes les courbes sont masquées',24,45);return}let mn=Math.min(...vals),mx=Math.max(...vals);let pad=Math.max(1,(mx-mn)*.12);mn-=pad;mx+=pad;const L=52,R=18,T=20,B=35;ctx.font='12px system-ui';for(let i=0;i<=8;i++){let y=T+(H-T-B)*i/8;ctx.strokeStyle='#253347';ctx.beginPath();ctx.moveTo(L,y);ctx.lineTo(W-R,y);ctx.stroke();ctx.fillStyle='#73869a';ctx.fillText((mx-(mx-mn)*i/8).toFixed(1),5,y+4)}keys().forEach((k,ki)=>{if(hidden[k])return;ctx.strokeStyle=colors[ki%colors.length];ctx.lineWidth=2;ctx.beginPath();let started=false;data.forEach((o,i)=>{let v=+o[k];if(!Number.isFinite(v))return;let x=L+i*(W-L-R)/Math.max(1,data.length-1),y=T+(mx-v)*(H-T-B)/(mx-mn);if(!started){ctx.moveTo(x,y);started=true}else{const prev=data[i-1],pv=prev?+prev[k]:v,px=L+(i-1)*(W-L-R)/Math.max(1,data.length-1),py=T+(mx-pv)*(H-T-B)/(mx-mn);ctx.bezierCurveTo((px+x)/2,py,(px+x)/2,y,x,y)}});ctx.stroke()});ctx.fillStyle='#73869a';ctx.textAlign='right';ctx.textAlign='left';const start=new Date(data[0].ts).getTime(),end=new Date(data[data.length-1].ts).getTime();if(Number.isFinite(start)&&Number.isFinite(end)&&end>start){const ticks=Math.max(2,Math.min(9,Math.floor((W-L-R)/100)));ctx.textAlign='center';ctx.font='11px system-ui';for(let j=0;j<ticks;j++){let f=j/(ticks-1),x=L+f*(W-L-R),date=new Date(start+f*(end-start));ctx.strokeStyle='#334354';ctx.beginPath();ctx.moveTo(x,T);ctx.lineTo(x,H-B+5);ctx.stroke();ctx.fillStyle='#9dafc2';let days=(end-start)/86400000,label=days<=2?date.toLocaleTimeString('fr-FR',{hour:'2-digit',minute:'2-digit'}):days<=90?date.toLocaleDateString('fr-FR',{day:'2-digit',month:'2-digit'}):date.toLocaleDateString('fr-FR',{month:'short',year:'2-digit'});ctx.fillText(label,x,H-20)}ctx.textAlign='left'}}
c.addEventListener('mousemove',e=>{if(!data.length)return;const r=c.getBoundingClientRect(),i=Math.max(0,Math.min(data.length-1,Math.round((e.clientX-r.left-52)/(r.width-70)*(data.length-1)))),v=data[i];document.getElementById('chartTip').textContent=new Date(v.ts).toLocaleString('fr-FR')+' · '+keys().filter(k=>!hidden[k]).map(k=>k+': '+v[k]).join(' · ')});document.querySelectorAll('.range button').forEach(b=>b.addEventListener('click',()=>load(+b.dataset.days,b)));addEventListener('resize',draw);load(1,document.querySelector('.range .active'));</script>`
}
func (a *App) series(w http.ResponseWriter, r *http.Request) {
	days := 30
	if n, _ := strconv.Atoi(r.URL.Query().Get("days")); n > 0 && n <= 730 {
		days = n
	}
	kind := r.URL.Query().Get("kind")
	cols := "ext,hk1_room,hk2_room,dhw_in"
	if kind == "modulation" {
		cols = "modulation"
	}
	if kind == "stock" {
		cols = "stock_kg"
	}
	q := "SELECT ts," + cols + " FROM measurements WHERE ts>=? ORDER BY ts"
	rows, e := a.db.Query(q, time.Now().AddDate(0, 0, -days).Format(time.RFC3339))
	if e != nil {
		http.Error(w, e.Error(), 500)
		return
	}
	defer rows.Close()
	out := []map[string]any{}
	for rows.Next() {
		var ts string
		if kind == "modulation" || kind == "stock" {
			var v sql.NullFloat64
			_ = rows.Scan(&ts, &v)
			out = append(out, map[string]any{"ts": ts, kind: v.Float64})
		} else {
			var x, y, z, t sql.NullFloat64
			_ = rows.Scan(&ts, &x, &y, &z, &t)
			out = append(out, map[string]any{"ts": ts, "extérieur": x.Float64, "pièce": y.Float64, "chambres": z.Float64, "ecs": t.Float64})
		}
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(out)
}
func (a *App) debug(w http.ResponseWriter, r *http.Request) {
	a.mu.RLock()
	raw, _ := json.MarshalIndent(a.live, "", "  ")
	liveErr := a.liveErr
	liveAt := a.liveAt
	a.mu.RUnlock()
	var nfiles, nmeas, nrecharges int
	var firstTS, lastTS sql.NullString
	_ = a.db.QueryRow(`SELECT COUNT(*) FROM imported_files`).Scan(&nfiles)
	_ = a.db.QueryRow(`SELECT COUNT(*) FROM measurements`).Scan(&nmeas)
	_ = a.db.QueryRow(`SELECT COUNT(*) FROM recharges`).Scan(&nrecharges)
	_ = a.db.QueryRow(`SELECT MIN(ts),MAX(ts) FROM measurements`).Scan(&firstTS, &lastTS)
	st, statErr := os.Stat(a.cfg.Database)
	dbSize := int64(0)
	if statErr == nil {
		dbSize = st.Size()
	}
	b := `<div class="hero"><div><div class="eyebrow">Diagnostic</div><h1>Debug système</h1><div class="subtitle">Chaudière HTTP · NAS/CSV · SQLite</div></div></div>` + a.health() + fmt.Sprintf(`<div class="grid"><div class="card"><div class="label">Mesures SQLite</div><div class="big">%d</div></div><div class="card"><div class="label">CSV indexés</div><div class="big">%d</div></div><div class="card"><div class="label">Recharges</div><div class="big">%d</div></div><div class="card"><div class="label">Base SQLite</div><div class="big">%.1f<span class="unit">Mo</span></div></div></div><h2>Plage historique</h2><p><code>%s</code> → <code>%s</code></p><h2>Configuration</h2><table class="diag-table"><tr><th>Chaudière</th><td>%s</td></tr><tr><th>CSV/NAS</th><td>%s</td></tr><tr><th>SQLite</th><td>%s</td></tr><tr><th>Dernière lecture HTTP</th><td>%s</td></tr><tr><th>Erreur HTTP</th><td>%s</td></tr></table><h2>Données techniques</h2><details class="diag-raw"><summary>Afficher / masquer le JSON chaudière</summary><pre>%s</pre></details>`, nmeas, nfiles, nrecharges, float64(dbSize)/1048576, esc(firstTS.String), esc(lastTS.String), esc(a.cfg.BoilerURL), esc(a.cfg.CSVDir), esc(a.cfg.Database), esc(liveAt.Format("02/01/2006 15:04:05")), esc(liveErr), esc(string(raw)))
	render(w, "Debug", b)
}
func gaugeHTML(stock, capacity float64) string {
	if capacity <= 0 {
		capacity = 1
	}
	pct := stock / capacity * 100
	if pct < 0 {
		pct = 0
	}
	if pct > 100 {
		pct = 100
	}
	angle := -90 + pct*1.8
	return fmt.Sprintf(`<div class="gauge-wrap"><svg class="gauge" viewBox="0 0 360 235" role="img" aria-label="Stock %.0f kg, %.1f pour cent"><path class="track" d="M45 165 A135 135 0 0 1 315 165" pathLength="100"/><path class="band" d="M45 165 A135 135 0 0 1 315 165" pathLength="100" stroke="#ff6b6b" stroke-dasharray="20 80"/><path class="band" d="M45 165 A135 135 0 0 1 315 165" pathLength="100" stroke="#e7c95b" stroke-dasharray="40 60" stroke-dashoffset="-20"/><path class="band" d="M45 165 A135 135 0 0 1 315 165" pathLength="100" stroke="#48d597" stroke-dasharray="40 60" stroke-dashoffset="-60"/><g transform="rotate(%.2f 180 165)"><line class="needle" x1="180" y1="165" x2="180" y2="55"/></g><circle class="hub" cx="180" cy="165" r="7"/><text class="gauge-value" x="180" y="115">%.0f kg</text><text class="gauge-pct" x="180" y="139">%.1f %%</text><text class="gauge-label" x="24" y="198">0 %%</text><text class="gauge-label" x="300" y="198">100 %%</text><text class="gauge-label" x="180" y="222" text-anchor="middle">Capacité : %.0f kg · Rouge 0–20 %% · Jaune 20–60 %% · Vert 60–100 %%</text></svg></div>`, stock, pct, angle, stock, pct, capacity)
}

func (a *App) stock(w http.ResponseWriter, r *http.Request) {
	v := a.liveNum("pe1", "L_storage_fill")
	m := a.liveNum("pe1", "L_storage_max")
	body := `<div class="hero"><div><div class="eyebrow">Granulés</div><h1>Stock du silo</h1></div></div><div class="card">` + gaugeHTML(v, m) + `</div>`
	render(w, "Stock", body)
}

// rechargeRows provides a unified view, suppressing same-delivery manual duplicates.
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
func (a *App) editHistorical(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Méthode interdite", 405)
		return
	}
	old := r.FormValue("ts")
	action := r.FormValue("action")
	manual := r.FormValue("manual") == "1"
	if len(old) < 10 || len(old) > 64 {
		http.Error(w, "Date invalide", 400)
		return
	}
	tx, e := a.db.Begin()
	if e != nil {
		http.Error(w, e.Error(), 500)
		return
	}
	defer tx.Rollback()
	if action == "delete" {
		if manual {
			_, e = tx.Exec(`DELETE FROM recharges WHERE ts=? AND source='manual'`, old)
		} else {
			_, e = tx.Exec(`INSERT OR IGNORE INTO recharge_tombstones(ts) VALUES(?)`, old)
			if e == nil {
				_, e = tx.Exec(`DELETE FROM historical_recharges WHERE ts=?`, old)
			}
		}
	} else if action == "save" {
		d := r.FormValue("date")
		kg, er := strconv.ParseFloat(strings.ReplaceAll(r.FormValue("kg"), ",", "."), 64)
		f := strings.TrimSpace(r.FormValue("fournisseur"))
		kind := strings.TrimSpace(r.FormValue("type"))
		note := strings.TrimSpace(r.FormValue("note"))
		if !validRefillDate(d) || er != nil || kg <= 0 || kg > 10000 || len(f) > 100 || len(kind) > 60 || len(note) > 500 {
			http.Error(w, "Champs invalides", 400)
			return
		}
		newTS := d + old[10:]
		if manual {
			_, e = tx.Exec(`UPDATE recharges SET ts=?,kg=?,note=? WHERE ts=? AND source='manual'`, newTS, kg, note, old)
		} else {
			_, e = tx.Exec(`UPDATE historical_recharges SET ts=?,kg=?,fournisseur=?,conditionnement=?,origine=? WHERE ts=?`, newTS, kg, f, kind, note, old)
			if e == nil {
				_, e = tx.Exec(`INSERT OR IGNORE INTO recharge_tombstones(ts) VALUES(?)`, old)
			}
		}
	} else {
		http.Error(w, "Action inconnue", 400)
		return
	}
	if e != nil {
		http.Error(w, e.Error(), 409)
		return
	}
	if e = tx.Commit(); e != nil {
		http.Error(w, e.Error(), 500)
		return
	}
	http.Redirect(w, r, "/recharge", 303)
}
func (a *App) recharge(w http.ResponseWriter, r *http.Request) {
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
	rr, e := a.rechargeRows()
	if e != nil {
		http.Error(w, e.Error(), 500)
		return
	}
	b := `<div class="hero"><h1>Historique des recharges</h1><div class="subtitle">Une seule liste pour les livraisons et les ajouts manuels</div></div><div class="card"><h2>Ajouter une recharge</h2><form method="post" class="refill-form"><label>Date <input type="date" name="date" required></label><label>Quantité (kg) <input type="number" name="kg" min="0.01" max="10000" step="0.01" required></label><label>Fournisseur <input name="fournisseur"></label><label>Type <select name="type"><option>Vrac</option><option>Sacs</option></select></label><label class="full">Note (facultative) <input name="note"></label><button class="full">Ajouter la recharge</button></form></div><div class="recharge-stats"><span class="pill"><strong>Recharges :</strong> %d</span><span class="pill"><strong>Total historique :</strong> %.0f kg</span><span class="pill"><strong>Dernière recharge :</strong> %s</span></div><div class="actions"><button type="button" id="toggle-recharge" aria-pressed="false" onclick="toggleRecharge()">Mode édition</button></div><div class="table-scroll"><table class="recharge-table"><thead><tr><th>Date</th><th>Quantité</th><th>Fournisseur</th><th>Type</th><th>Note</th><th class="recharge-actions" hidden>Actions</th></tr></thead><tbody>`
	totalKG := 0.0
	for _, entry := range rr {
		totalKG += entry.kg
	}
	latest := "—"
	if len(rr) > 0 && len(rr[0].ts) >= 10 {
		latest = rr[0].ts[:10]
	}
	b = fmt.Sprintf(b, len(rr), totalKG, esc(latest))
	for i, v := range rr {
		id := fmt.Sprintf("r%d", i)
		b += fmt.Sprintf(`<tr id="%s"><td><span class="view">%s</span><input class="edit" form="f%s" name="date" type="date" value="%s" required hidden></td><td><span class="view">%.0f kg</span><input class="edit" form="f%s" name="kg" type="number" min="0.01" max="10000" step="0.01" value="%.2f" required hidden></td><td><span class="view">%s</span><input class="edit" form="f%s" name="fournisseur" value="%s" hidden></td><td><span class="view">%s</span><input class="edit" form="f%s" name="type" value="%s" hidden></td><td><span class="view">%s</span><input class="edit" form="f%s" name="note" value="%s" hidden></td><td class="recharge-actions" hidden><button type="button" class="start" onclick="editRow('%s',true)">Modifier</button><button class="edit" form="f%s" hidden>Enregistrer</button><button type="button" class="edit" onclick="editRow('%s',false)" hidden>Annuler</button><form id="f%s" method="post" action="/recharge/edit"><input type="hidden" name="ts" value="%s"><input type="hidden" name="manual" value="%d"><input type="hidden" name="action" value="save"></form><form method="post" action="/recharge/edit" onsubmit="return confirm('Supprimer cette recharge ?')"><input type="hidden" name="ts" value="%s"><input type="hidden" name="manual" value="%d"><input type="hidden" name="action" value="delete"><button class="dangerbtn">Supprimer</button></form></td></tr>`, id, esc(v.ts[:10]), id, esc(v.ts[:10]), v.kg, id, v.kg, esc(v.supplier), id, esc(v.supplier), esc(v.kind), id, esc(v.kind), esc(v.note), id, esc(v.note), id, id, id, id, esc(v.ts), boolInt(v.manual), esc(v.ts), boolInt(v.manual))
	}
	b += `</tbody></table></div><script>function toggleRecharge(){const b=document.getElementById('toggle-recharge');const on=b.getAttribute('aria-pressed')!=='true';b.setAttribute('aria-pressed',String(on));b.textContent=on?'Terminer l’édition':'Mode édition';document.querySelectorAll('.recharge-actions').forEach(e=>e.hidden=!on);if(!on){document.querySelectorAll('.recharge-table tbody tr').forEach(tr=>editRow(tr.id,false))}}function editRow(id,on){const tr=document.getElementById(id);tr.querySelectorAll('.view').forEach(x=>x.hidden=on);tr.querySelectorAll('.edit').forEach(x=>x.hidden=!on);tr.querySelector('.start').hidden=on}</script>`
	render(w, "Recharges", b)
}
func boolInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

func monthlyBarsScript(labels []string, values []float64) string {
	lb, _ := json.Marshal(labels)
	vv, _ := json.Marshal(values)
	return fmt.Sprintf(`<script>(function(){const labels=%s,vals=%s,c=document.getElementById('consoBars'),x=c.getContext('2d');function draw(){const d=devicePixelRatio||1,W=c.clientWidth,H=c.clientHeight;c.width=W*d;c.height=H*d;x.setTransform(d,0,0,d,0,0);x.clearRect(0,0,W,H);const L=46,R=14,T=24,B=55,mx=Math.max(1,...vals)*1.15;for(let i=0;i<=8;i++){let y=T+(H-T-B)*i/8;x.strokeStyle='#253347';x.beginPath();x.moveTo(L,y);x.lineTo(W-R,y);x.stroke();x.fillStyle='#7f91a4';x.font='11px system-ui';x.fillText(Math.round(mx*(1-i/8))+' kg',3,y+4)}const gap=(W-L-R)/Math.max(1,vals.length),bw=Math.min(46,gap*.62);vals.forEach((v,i)=>{const h=(H-T-B)*v/mx,px=L+i*gap+(gap-bw)/2,py=H-B-h;x.fillStyle='#ff9f43';x.fillRect(px,py,bw,h);x.fillStyle='#edf3f8';x.font='700 11px system-ui';x.textAlign='center';x.fillText(Math.round(v)+' kg',px+bw/2,Math.max(14,py-7));x.save();x.translate(px+bw/2,H-B+12);x.rotate(-.55);x.fillStyle='#8fa1b5';x.font='11px system-ui';x.fillText(labels[i],0,0);x.restore()});x.textAlign='left'}addEventListener('resize',draw);draw()})();</script>`, lb, vv)
}

// monthlyStockMethod is a read-only comparison with archived consumption.
// Refills are recorded in recharges; an unknown day in a month is represented
// by a technical timestamp and explained in the note.
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

func (a *App) conso(w http.ResponseWriter, r *http.Request) {
	year := time.Now().Year()
	if y, _ := strconv.Atoi(r.URL.Query().Get("year")); y > 2000 && y < 2200 {
		year = y
	}
	rows, e := a.db.Query(`SELECT month,kg FROM monthly_consumption WHERE year=? ORDER BY month`, year)
	if e != nil {
		http.Error(w, e.Error(), 500)
		return
	}
	defer rows.Close()
	vals := make([]float64, 12)
	present := make([]bool, 12)
	total := 0.0
	for rows.Next() {
		var m int
		var kg float64
		if err := rows.Scan(&m, &kg); err != nil {
			rows.Close()
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		if m >= 1 && m <= 12 {
			vals[m-1] = kg
			present[m-1] = true
			total += kg
		}
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	// Release the result set before monthlyStockMethod issues additional queries.
	rows.Close()
	b := `<div class="hero"><div><div class="eyebrow">Granulés</div><h1>Consommation et bilan</h1><div class="subtitle">Comparaison multiannée, année courante sélectionnée par défaut</div></div></div>`
	b += a.multiYearChart()
	b += `<div class="actions"><a class="btn" href="/export?type=monthly">Exporter CSV</a><a class="btn" href="/recharge">Historique des recharges</a></div>`
	b += a.consoMatrix()
	b += `<p><a class="btn" href="/recharge">Voir et modifier les pellets ajoutés</a></p>`
	render(w, "Consommation", b)
}

func (a *App) monthlyEdit(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Méthode interdite", 405)
		return
	}
	y, e1 := strconv.Atoi(r.FormValue("year"))
	m, e2 := strconv.Atoi(r.FormValue("month"))
	kg, e3 := strconv.ParseFloat(strings.ReplaceAll(r.FormValue("kg"), ",", "."), 64)
	if e1 != nil || e2 != nil || e3 != nil || y < 2000 || y > 2200 || m < 1 || m > 12 || kg < 0 || kg > 100000 {
		http.Error(w, "Valeur invalide", 400)
		return
	}
	_, e := a.db.Exec(`INSERT INTO monthly_overrides(year,month,kg,original_kg) VALUES(?,?,?,(SELECT kg FROM monthly_consumption WHERE year=? AND month=?)) ON CONFLICT(year,month) DO UPDATE SET kg=excluded.kg`, y, m, kg, y, m)
	if e != nil {
		http.Error(w, e.Error(), 500)
		return
	}
	http.Redirect(w, r, "/conso", 303)
}
func (a *App) consoMatrix() string {
	rows, err := a.db.Query(`SELECT m.year,m.month,COALESCE(o.kg,m.kg) FROM monthly_consumption m LEFT JOIN monthly_overrides o ON m.year=o.year AND m.month=o.month ORDER BY m.year,m.month`)
	if err != nil {
		return `<p>Historique indisponible</p>`
	}
	defer rows.Close()
	values := map[int]map[int]float64{}
	totals := map[int]float64{}
	years := []int{}
	for rows.Next() {
		var y, m int
		var kg float64
		if rows.Scan(&y, &m, &kg) != nil {
			continue
		}
		if values[y] == nil {
			values[y] = map[int]float64{}
			years = append(years, y)
		}
		values[y][m] = kg
		totals[y] += kg
	}
	sort.Sort(sort.Reverse(sort.IntSlice(years)))
	if len(years) == 0 {
		return `<p>Aucune consommation mensuelle archivée.</p>`
	}
	added := map[int]float64{}
	for _, q := range []string{`SELECT substr(ts,1,4),sum(kg) FROM historical_recharges GROUP BY substr(ts,1,4)`} {
		rr, e := a.db.Query(q)
		if e != nil {
			continue
		}
		for rr.Next() {
			var y string
			var kg float64
			if rr.Scan(&y, &kg) == nil {
				n, _ := strconv.Atoi(y)
				added[n] += kg
			}
		}
		rr.Close()
	}
	months := []string{"", "Janvier", "Février", "Mars", "Avril", "Mai", "Juin", "Juillet", "Août", "Septembre", "Octobre", "Novembre", "Décembre"}
	b := `<h2>Consommation par mois et par année</h2><div class="table-scroll"><table class="matrix"><thead><tr><th>Mois</th>`
	for _, y := range years {
		b += fmt.Sprintf(`<th>%d</th>`, y)
	}
	b += `</tr></thead><tbody>`
	for m := 1; m <= 12; m++ {
		b += `<tr><th>` + months[m] + `</th>`
		for _, y := range years {
			if kg, ok := values[y][m]; ok {
				b += fmt.Sprintf(`<td class="editable-month"><button type="button" class="month-value" onclick="editMonth(this)" title="Cliquer pour modifier">%.0f kg</button><form hidden method="post" action="/conso/edit"><input type="hidden" name="year" value="%d"><input type="hidden" name="month" value="%d"><input aria-label="Consommation en kg" type="number" name="kg" step="0.01" min="0" value="%.2f" required><button type="submit">Enregistrer</button><button type="button" onclick="cancelMonth(this)">Annuler</button></form></td>`, kg, y, m, kg)
			} else {
				b += `<td>—</td>`
			}
		}
		b += `</tr>`
	}
	b += `<tr class="total-row"><th>Total consommé</th>`
	for _, y := range years {
		b += fmt.Sprintf(`<td>%.0f kg</td>`, totals[y])
	}
	b += `</tr><tr class="total-row"><th>Pellets ajoutés</th>`
	for _, y := range years {
		b += fmt.Sprintf(`<td>%.0f kg</td>`, added[y])
	}
	b += `</tr></tbody></table></div><script>function editMonth(btn){const cell=btn.closest("td");btn.hidden=true;const form=cell.querySelector("form");form.hidden=false;form.querySelector("input[name=kg]").focus()}function cancelMonth(btn){const cell=btn.closest("td");cell.querySelector("form").hidden=true;cell.querySelector(".month-value").hidden=false}</script>`
	return b
}

func (a *App) multiYearChart() string {
	rows, e := a.db.Query(`SELECT m.year,m.month,COALESCE(o.kg,m.kg) FROM monthly_consumption m LEFT JOIN monthly_overrides o ON m.year=o.year AND m.month=o.month ORDER BY m.year,m.month`)
	if e != nil {
		return `<p class="notice">Graphique indisponible</p>`
	}
	defer rows.Close()
	type yr struct {
		Year  int
		V     [12]float64
		Has   [12]bool
		Total float64
	}
	ym := map[int]*yr{}
	years := []int{}
	for rows.Next() {
		var y, m int
		var kg float64
		_ = rows.Scan(&y, &m, &kg)
		if ym[y] == nil {
			ym[y] = &yr{Year: y}
			years = append(years, y)
		}
		if m >= 1 && m <= 12 {
			ym[y].V[m-1] = kg
			ym[y].Has[m-1] = true
			ym[y].Total += kg
		}
	}
	if len(years) == 0 {
		return `<p>Aucune consommation disponible.</p>`
	}
	labels := []string{"Jan", "Fév", "Mar", "Avr", "Mai", "Juin", "Juil", "Août", "Sep", "Oct", "Nov", "Déc"}
	data := make([]map[string]any, 12)
	for i, n := range labels {
		data[i] = map[string]any{"mois": n}
	}
	series := []map[string]any{}
	for _, y := range years {
		k := fmt.Sprintf("y%d", y)
		series = append(series, map[string]any{"key": k, "label": strconv.Itoa(y)})
		for i := 0; i < 12; i++ {
			if ym[y].Has[i] {
				data[i][k] = ym[y].V[i]
			}
		}
	}
	jb, _ := json.Marshal(data)
	js, _ := json.Marshal(series)
	b := `<h2>Comparaison mensuelle</h2><div class="chartbox"><div id="bilanLegend" class="bar-legend"></div><canvas id="bilanBars" class="conso-chart"></canvas></div>`
	b += fmt.Sprintf(`<script>(function(){const rows=%s,series=%s,colors=['#55a7ff','#ff9f43','#48d597','#b38cff','#ff6b6b','#e7d36b'],hidden={};series.forEach(s=>{if(s.label!==String(new Date().getFullYear()))hidden[s.key]=true});if(series.length&&!series.some(s=>!hidden[s.key]))hidden[series[series.length-1].key]=false;const c=document.getElementById('bilanBars'),ctx=c.getContext('2d'),leg=document.getElementById('bilanLegend');function legend(){leg.innerHTML='';series.forEach((s,i)=>{const b=document.createElement('button');b.type='button';b.className='btn'+(hidden[s.key]?' off':'');b.innerHTML='<span style="color:'+colors[i%%colors.length]+'">●</span> '+s.label;b.onclick=()=>{hidden[s.key]=!hidden[s.key];legend();draw()};leg.appendChild(b)})}function draw(){const d=devicePixelRatio||1,W=c.clientWidth,H=c.clientHeight;c.width=W*d;c.height=H*d;ctx.setTransform(d,0,0,d,0,0);ctx.clearRect(0,0,W,H);const active=series.filter(s=>!hidden[s.key]);let mx=1;rows.forEach(r=>active.forEach(s=>{if(Number.isFinite(+r[s.key]))mx=Math.max(mx,+r[s.key])}));mx*=1.12;const L=48,R=14,T=20,B=46,gw=(W-L-R)/rows.length;for(let i=0;i<=8;i++){let y=T+(H-T-B)*i/8;ctx.strokeStyle='#253347';ctx.beginPath();ctx.moveTo(L,y);ctx.lineTo(W-R,y);ctx.stroke();ctx.fillStyle='#7f91a4';ctx.font='11px system-ui';ctx.fillText(Math.round(mx*(1-i/8)),6,y+4)}rows.forEach((r,ri)=>{const bw=Math.max(3,Math.min(22,(gw*.78)/Math.max(1,active.length)));const all=active.length*bw;active.forEach((s,si)=>{const v=+r[s.key];if(!Number.isFinite(v))return;const h=(H-T-B)*v/mx,x=L+ri*gw+(gw-all)/2+si*bw,y=H-B-h;ctx.fillStyle=colors[series.indexOf(s)%%colors.length];ctx.fillRect(x,y,bw-2,h)});ctx.fillStyle='#8fa1b5';ctx.font='11px system-ui';ctx.textAlign='center';ctx.fillText(r.mois,L+ri*gw+gw/2,H-B+18)});ctx.textAlign='left'}legend();addEventListener('resize',draw);draw()})();</script>`, jb, js)
	return b
}

func (a *App) bilan(w http.ResponseWriter, r *http.Request) {
	http.Redirect(w, r, "/conso", http.StatusFound)
}

// dailyConsumption uses only decreases in the boiler stock counter.
// Increases (refills or counter resets) are NOT consumption. Large negative
// jumps are flagged for review rather than silently accepted as usage.
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

func (a *App) recalc(w http.ResponseWriter, r *http.Request) {
	year, _ := strconv.Atoi(r.URL.Query().Get("year"))
	if year == 0 {
		year = time.Now().Year()
	}
	if err := a.recalculateYear(year); err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	http.Redirect(w, r, fmt.Sprintf("/conso?year=%d", year), 302)
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
		if !a.importMu.TryLock() {
			return
		}
		defer a.importMu.Unlock()
		f, n, e := a.ImportCSVDir()
		if e != nil {
			a.logRun("CSV · NAS", n, n, e)
			log.Printf("Import CSV: %v", e)
		} else if f > 0 {
			a.logRun("CSV · NAS", n, n, nil)
			log.Printf("Import CSV: %d fichier(s), %d nouvelle(s) ligne(s)", f, n)
		}
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
func (a *App) importCSV(w http.ResponseWriter, r *http.Request) {
	if r.Method == "POST" {
		f, n, e := a.ImportCSVDir()
		if e != nil {
			http.Error(w, e.Error(), 500)
			return
		}
		render(w, "Import", fmt.Sprintf("<h1>Import terminé</h1><p>%d fichier(s) traité(s), %d nouvelle(s) ligne(s).</p><p><a href='/import'>Retour</a></p>", f, n))
		return
	}
	var n int
	_ = a.db.QueryRow(`SELECT COUNT(*) FROM measurements`).Scan(&n)
	var f int
	_ = a.db.QueryRow(`SELECT COUNT(*) FROM imported_files`).Scan(&f)
	render(w, "Import", fmt.Sprintf("<div class='hero'><div><div class='eyebrow'>Synology · NFS</div><h1>Import des CSV</h1><div class='subtitle'>Synchronisation de l'historique chaudière vers SQLite</div></div></div><div class='grid'><div class='card'><div class='label'>Fichiers indexés</div><div class='big'>%d</div></div><div class='card'><div class='label'>Mesures SQLite</div><div class='big'>%d</div></div></div><h2>Source</h2><p><code>%s</code></p><form method='post'><button>Importer maintenant</button></form>", f, n, esc(a.cfg.CSVDir)))
}
func (a *App) calendar(w http.ResponseWriter, r *http.Request) {
	year, month := time.Now().Year(), int(time.Now().Month())
	if y, e := strconv.Atoi(r.URL.Query().Get("year")); e == nil && y >= 2020 && y <= 2100 {
		year = y
	}
	if m, e := strconv.Atoi(r.URL.Query().Get("month")); e == nil && m >= 1 && m <= 12 {
		month = m
	}
	first := time.Date(year, time.Month(month), 1, 0, 0, 0, 0, time.Local)
	last := first.AddDate(0, 1, 0)
	rows, err := a.db.Query(`SELECT date,kg,anomalies,samples FROM daily_consumption WHERE date>=? AND date<? ORDER BY date`, first.Format("2006-01-02"), last.Format("2006-01-02"))
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	type record struct {
		kg                 float64
		anomalies, samples int
	}
	data := map[string]record{}
	for rows.Next() {
		var date string
		var d record
		if rows.Scan(&date, &d.kg, &d.anomalies, &d.samples) == nil {
			data[date] = d
		}
	}
	rows.Close()
	b := fmt.Sprintf(`<div class="hero"><div><div class="eyebrow">Historique</div><h1>Calendrier de consommation</h1><div class="subtitle">%02d/%d · orange : mesure à contrôler · atténué : données absentes</div></div></div>`, month, year)
	b += `<div class="actions"><a class="btn" href="/export?type=daily">Exporter les jours (CSV)</a><a class="btn" href="/conso">Consommation mensuelle</a></div><div class="calendar">`
	for _, name := range []string{"Lun", "Mar", "Mer", "Jeu", "Ven", "Sam", "Dim"} {
		b += `<div class="calday"><strong>` + name + `</strong></div>`
	}
	for i := 0; i < (int(first.Weekday())+6)%7; i++ {
		b += `<div></div>`
	}
	for d := first; d.Before(last); d = d.AddDate(0, 0, 1) {
		key := d.Format("2006-01-02")
		v, ok := data[key]
		class := "missing"
		detail := "Absence"
		if ok {
			class = "ok"
			detail = fmt.Sprintf("%.1f kg", v.kg)
			if v.anomalies > 0 {
				class = "warn"
				detail += fmt.Sprintf(" · %d anomalie(s)", v.anomalies)
			}
		}
		b += fmt.Sprintf(`<div class="calday %s" title="%s"><strong>%d</strong><small>%s</small></div>`, class, esc(detail), d.Day(), esc(detail))
	}
	b += `</div><p class="mini">Les hausses du compteur sont ignorées. Les variations supérieures à 30 kg entre deux relevés sont signalées, pas comptabilisées.</p>`
	render(w, "Calendrier", b)
}

func (a *App) exportCSV(w http.ResponseWriter, r *http.Request) {
	typ := r.URL.Query().Get("type")
	var query, filename string
	switch typ {
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

func (a *App) backupPage(w http.ResponseWriter, r *http.Request) {
	http.Redirect(w, r, "/backup/download", http.StatusSeeOther)
}
func (a *App) RunBackups(ctx context.Context) { /* disabled: manual ZIP download only */ }

func (a *App) systemCenter(w http.ResponseWriter, r *http.Request) {
	var measurements, imported, recharges int
	_ = a.db.QueryRow(`SELECT COUNT(*) FROM measurements`).Scan(&measurements)
	_ = a.db.QueryRow(`SELECT COUNT(*) FROM imported_files`).Scan(&imported)
	_ = a.db.QueryRow(`SELECT (SELECT COUNT(*) FROM historical_recharges) + (SELECT COUNT(*) FROM recharges WHERE source='manual')`).Scan(&recharges)
	body := `<div class="hero"><div><div class="eyebrow">OKOKO 1.7</div><h1>Centre système</h1><div class="subtitle">Services, collecte et sauvegarde</div></div></div>` + a.health()
	body += fmt.Sprintf(`<div class="grid"><div class="card"><div class="label">Mesures SQLite</div><div class="big">%d</div></div><div class="card"><div class="label">CSV indexés</div><div class="big">%d</div></div><div class="card"><div class="label">Recharges</div><div class="big">%d</div></div></div>`, measurements, imported, recharges)
	body += `<div class="card" style="margin-top:18px"><h2>Synchronisation</h2><p class="muted">Journaux chaudière et CSV NAS, collecte automatique en arrière-plan.</p><form method="post" action="/import/all"><button>Importer les données</button></form>`
	var at, source, problem string
	var added int
	if a.db.QueryRow(`SELECT at,source,added,error FROM import_runs ORDER BY id DESC LIMIT 1`).Scan(&at, &source, &added, &problem) == nil {
		body += `<p class="mini">Dernière opération : ` + esc(at) + ` · ` + esc(source) + ` · ` + fmt.Sprint(added) + ` ajout(s)`
		if problem != "" {
			body += ` · Erreur : ` + esc(problem)
		}
		body += `</p>`
	}
	a.importStateMu.RLock()
	busy := a.importActive
	a.importStateMu.RUnlock()
	if busy {
		body += `<p class="notice">Synchronisation en cours. Les résultats seront visibles à la prochaine actualisation.</p>`
	}
	body += `</div><div class="card" style="margin-top:18px"><h2>Sauvegarde et diagnostic</h2><div class="actions"><a class="btn" href="/backup/download">Télécharger la sauvegarde SQLite</a><a class="btn" href="/system/details">Détails techniques et imports</a></div></div>`
	render(w, "Centre système", body)
}
func (a *App) systemDetails(w http.ResponseWriter, r *http.Request) {
	body := `<div class="hero"><div><div class="eyebrow">OKOKO 1.9</div><h1>Diagnostic technique</h1><div class="subtitle"><a href="/system">← Retour au Centre système</a></div></div></div>`
	body += `<h2>Historique des acquisitions et imports</h2><p class="muted">Acquisitions JSON et imports CSV, classés par date.</p><div class="table-scroll"><table><tr><th>Date</th><th>Source</th><th>Lues</th><th>Ajouts</th><th>Doublons</th><th>Erreur</th></tr>`
	pageNum, _ := strconv.Atoi(r.URL.Query().Get("page"))
	if pageNum < 1 {
		pageNum = 1
	}
	if pageNum > 10000 {
		pageNum = 10000
	}
	runs, e := a.db.Query(`SELECT at,source,added,error,read_count,duplicate_count FROM import_runs ORDER BY id DESC LIMIT 25 OFFSET ?`, (pageNum-1)*25)
	if e == nil {
		for runs.Next() {
			var at, src, problem string
			var n, read, dup int
			if runs.Scan(&at, &src, &n, &problem, &read, &dup) == nil {
				body += fmt.Sprintf(`<tr><td>%s</td><td>%s</td><td>%d</td><td>%d</td><td>%d</td><td>%s</td></tr>`, esc(localImportDate(at)), esc(src), read, n, dup, esc(problem))
			}
		}
		runs.Close()
	}
	body += fmt.Sprintf(`</table></div><div class="actions"><a class="btn" href="/system/details?page=%d">← Précédents</a><span class="pill">Page %d · 25 lignes maximum</span><a class="btn" href="/system/details?page=%d">Suivants →</a></div><h2>Sources et diagnostics</h2>`, max(1, pageNum-1), pageNum, pageNum+1)
	body += `<p>NAS : <code>` + esc(a.cfg.CSVDir) + `</code></p><p><a class="btn" href="/debug">Diagnostic brut</a></p>`
	render(w, "Diagnostic technique", body)
}
func localImportDate(s string) string {
	for _, layout := range []string{time.RFC3339, "2006-01-02 15:04:05"} {
		if t, e := time.Parse(layout, s); e == nil {
			if layout == time.RFC3339 {
				return t.Local().Format("02/01/2006 15:04:05")
			}
			return t.Format("02/01/2006 15:04:05")
		}
	}
	return s
}
func (a *App) weatherAPI(w http.ResponseWriter, r *http.Request) {
	date, temp, wind, rise, set, day := a.weatherNow()
	current := map[string]string{"Date": date, "Température": temp + " °C", "Vent": wind, "Lever": rise, "Coucher": set, "Durée du jour": day}
	forecast := make([]map[string]string, 0, 25)
	a.mu.RLock()
	if weather, ok := a.live["weather"].(map[string]any); ok {
		for _, item := range []struct{ key, label string }{{"L_clouds", "Nuages"}, {"L_forecast_temp", "Température prévue"}} {
			if obj, ok := weather[item.key].(map[string]any); ok {
				if v, ok := obj["val"].(float64); ok {
					if item.key == "L_forecast_temp" {
						v *= 0.1
					}
					current[item.label] = strconv.FormatFloat(v, 'f', 1, 64)
					if item.key == "L_clouds" {
						current[item.label] += " %"
					} else {
						current[item.label] += " °C"
					}
				}
			}
		}
	}
	if f, ok := a.live["forecast"].(map[string]any); ok {
		for i := 0; i < 25; i++ {
			if x, ok := f[fmt.Sprintf("L_w_%d", i)].(map[string]any); ok {
				parts := strings.Split(fmt.Sprint(x["val"]), "|")
				if len(parts) >= 6 {
					condition := map[string]string{"800": "Ciel dégagé", "801": "Peu nuageux", "802": "Partiellement nuageux", "803": "Nuageux", "804": "Couvert", "500": "Pluie légère", "501": "Pluie", "502": "Forte pluie", "600": "Neige"}[parts[5]]
					if condition == "" {
						condition = "Conditions : " + parts[5]
					}
					forecast = append(forecast, map[string]string{"date": parts[0], "temp": parts[1], "clouds": parts[2], "wind": parts[3], "condition": condition})
				}
			}
		}
	}
	a.mu.RUnlock()
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_ = json.NewEncoder(w).Encode(map[string]any{"current": current, "forecast": forecast})
}

// installation presents the current boiler state without inventing a hydraulic topology.
func (a *App) installation(w http.ResponseWriter, r *http.Request) {
	b := `<div class="hero"><div><div class="eyebrow">Fonctionnement</div><h1>Chauffage</h1><p class="section-intro">Les trois circuits et la chaudière, à partir des dernières mesures disponibles.</p></div></div>`
	b += `<div class="card"><div class="label">CHAUDIÈRE ÖKOFEN</div><div class="big">` + esc(a.display("pe1", "L_modulation")) + `</div><div class="subtitle">` + modeHTML(a.display("pe1", "L_statetext")) + `</div></div>`
	b += `<div class="install-center">↓ &nbsp; Distribution thermique &nbsp; ↓</div><div class="install-flow">`
	for _, c := range []struct{ cat, title, temp, set string }{
		{"hk1", "HK1 · Chambres", "L_roomtemp_act", "L_roomtemp_set"},
		{"hk2", "HK2 · Plancher", "L_roomtemp_act", "L_roomtemp_set"},
		{"ww1", "ECS · Eau chaude", "L_ontemp_act", "temp_max_set"},
	} {
		b += `<div class="install-node"><strong>` + c.title + `</strong><div class="big">` + esc(a.display(c.cat, c.temp)) + `</div><p class="muted">Consigne : ` + esc(a.installSetpoint(c.cat, c.set)) + `</p><p class="mini">` + modeHTML(a.display(c.cat, "L_statetext")) + `</p></div>`
	}
	b += `</div><div class="grid install-kpis">`
	for _, x := range []struct{ label, cat, key string }{
		{"Température chaudière", "pe1", "L_temp_act"},
		{"Heures de fonctionnement", "pe1", "L_runtime"},
		{"Démarrages", "pe1", "L_starts"},
		{"Durée moyenne des cycles", "pe1", "L_avg_runtime"},
	} {
		b += `<div class="card"><div class="label">` + x.label + `</div><div class="big">` + esc(a.display(x.cat, x.key)) + `</div></div>`
	}
	b += `</div><p class="health-note">Représentation fonctionnelle, et non schéma hydraulique. Les valeurs proviennent de la chaudière.</p><div class="hub-links"><a href="/analyse">Analyser les courbes →</a><a href="/modulation">Modulation détaillée →</a><a href="/transitions">Historique des transitions →</a></div>`
	render(w, "Chauffage", b)
}
func (a *App) analyse(w http.ResponseWriter, r *http.Request) {
	b := `<div class="hero"><div><div class="eyebrow">Historique</div><h1>Analyse</h1><p class="section-intro">Comparer les températures, les consignes et les évolutions de la chaudière.</p></div></div>`
	b += `<div class="hub-links"><a href="/temp">Températures détaillées</a><a href="/modulation">Modulation</a><a href="/transitions">Transitions</a><a href="/calendar">Couverture des données</a><a href="/bilan">Bilan</a></div>`
	b += chartPage("Températures et consignes", "/api/series?days=1")
	render(w, "Analyse", b)
}
func (a *App) pellets(w http.ResponseWriter, r *http.Request) {
	b := `<div class="hero"><div><div class="eyebrow">Énergie</div><h1>Pellets</h1><p class="section-intro">Stock, consommation et historique des approvisionnements.</p></div></div>`
	b += `<div class="hub-links"><a href="/stock">Stock et autonomie</a><a href="/conso">Consommations</a><a href="/recharge">Historique des recharges</a></div>`
	b += `<div class="grid"><div class="card"><div class="label">Stock et autonomie</div><p class="muted">Évolution du stock estimé, seuils et capacité du silo.</p><a class="btn" href="/stock">Voir le stock →</a></div><div class="card"><div class="label">Consommation</div><p class="muted">Historique mensuel, annuel et comparaison des années.</p><a class="btn" href="/conso">Voir les consommations →</a></div><div class="card"><div class="label">Approvisionnements</div><p class="muted">Recharges historiques et saisies manuelles.</p><a class="btn" href="/recharge">Voir les recharges →</a></div></div>`
	render(w, "Pellets", b)
}
func (a *App) meteo(w http.ResponseWriter, r *http.Request) {
	b := `<div class="hero"><div><div class="eyebrow">Conditions extérieures</div><h1>Météo</h1><p class="section-intro">Situation actuelle, aujourd’hui, trois prochains jours et détail horaire.</p></div></div>`
	renderWithFooter(w, "Météo", b, weatherFooter())
}
func (a *App) Handler() http.Handler {
	m := http.NewServeMux()
	m.HandleFunc("/", a.home)
	m.HandleFunc("/installation", a.installation)
	m.HandleFunc("/analyse", a.analyse)
	m.HandleFunc("/transitions", a.transitionsPage)
	m.HandleFunc("/pellets", a.pellets)
	m.HandleFunc("/meteo", a.meteo)
	m.HandleFunc("/temp", func(w http.ResponseWriter, r *http.Request) {
		render(w, "Températures", chartPage("Températures", "/api/series?days=1"))
	})
	m.HandleFunc("/modulation", func(w http.ResponseWriter, r *http.Request) {
		render(w, "Modulation", chartPage("Modulation", "/api/series?kind=modulation&days=1"))
	})
	m.HandleFunc("/stock", a.stock)
	m.HandleFunc("/system", a.systemCenter)
	m.HandleFunc("/system/details", a.systemDetails)
	m.HandleFunc("/api/weather", a.weatherAPI)
	m.HandleFunc("/recharge", a.recharge)
	m.HandleFunc("/recharge/edit", a.editHistorical)
	m.HandleFunc("/backup/download", a.downloadBackup)
	m.HandleFunc("/conso", a.conso)
	m.HandleFunc("/conso/edit", a.monthlyEdit)
	m.HandleFunc("/bilan", a.bilan)
	m.HandleFunc("/conso2023", func(w http.ResponseWriter, r *http.Request) { http.Redirect(w, r, "/conso?year=2023", 302) })
	m.HandleFunc("/conso2024", func(w http.ResponseWriter, r *http.Request) { http.Redirect(w, r, "/conso?year=2024", 302) })
	m.HandleFunc("/conso2025", func(w http.ResponseWriter, r *http.Request) { http.Redirect(w, r, "/conso?year=2025", 302) })
	m.HandleFunc("/conso2026", func(w http.ResponseWriter, r *http.Request) { http.Redirect(w, r, "/conso?year=2026", 302) })
	m.HandleFunc("/consoRecap", a.conso)
	m.HandleFunc("/debug", a.debug)
	m.HandleFunc("/import", a.importCSV)
	m.HandleFunc("/import/logs", a.triggerLogs)
	m.HandleFunc("/import/all", a.triggerAll)
	m.HandleFunc("/api/mini", a.miniAPI)
	m.HandleFunc("/api/series", a.series)
	m.HandleFunc("/api/recalculate", a.recalc)
	m.HandleFunc("/calendar", a.calendar)
	m.HandleFunc("/export", a.exportCSV)
	m.HandleFunc("/backup", a.backupPage)
	m.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) { io.WriteString(w, "ok\n") })
	return m
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
