package app

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"chaudiere/internal/web"
)

func (a *App) redactError(s string) string {
	if a.cfg.BoilerURL != "" {
		s = strings.ReplaceAll(s, a.cfg.BoilerURL, "[connexion chaudière]")
		if u, e := url.Parse(a.cfg.BoilerURL); e == nil {
			// Log imports use /logN instead of /all: hide the access prefix too.
			for _, accessPath := range []string{u.Path, u.EscapedPath()} {
				prefix := strings.TrimSuffix(strings.TrimRight(accessPath, "/"), "/all")
				if prefix != "" && prefix != "/" {
					s = strings.ReplaceAll(s, prefix, "/[accès masqué]")
				}
			}
			if u.User != nil {
				s = strings.ReplaceAll(s, u.User.String(), "[accès masqué]")
			}
		}
	}
	return s
}
func writeJSON(w http.ResponseWriter, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	_ = json.NewEncoder(w).Encode(value)
}
func number(m map[string]any, c, k string) any {
	if n, ok := leaf(m, c, k); ok && !math.IsNaN(n) && !math.IsInf(n, 0) {
		return n
	}
	return nil
}

// Raw ÖkoFEN temperatures use tenths; an explicit factor is already applied by leaf.
func scaledNumber(m map[string]any, c, k string, scale float64) any {
	raw := rawValue(m, c, k)
	if n, ok := num(raw); !ok || n == -32768 || n == 32767 {
		return nil
	}
	n, ok := leaf(m, c, k)
	if !ok || math.IsNaN(n) || math.IsInf(n, 0) {
		return nil
	}
	if section, ok := m[c].(map[string]any); ok {
		if meta, ok := section[k].(map[string]any); ok {
			if _, hasFactor := num(meta["factor"]); hasFactor {
				return n
			}
		}
	}
	return n / (1 / scale)
}
func temperature(m map[string]any, c, k string) any { return scaledNumber(m, c, k, 0.1) }
func text(m map[string]any, c, k string) string {
	v := rawValue(m, c, k)
	if v == nil {
		return ""
	}
	return cleanText(fmt.Sprint(v))
}
func pumpValue(m map[string]any, c, k string) any {
	if n, ok := leaf(m, c, k); ok {
		return n > 0
	}
	return nil
}
func (a *App) snapshotV3(w http.ResponseWriter, r *http.Request) {
	a.mu.RLock()
	m, at, problem := a.live, a.liveAt, a.liveErr
	a.mu.RUnlock()
	timestamp := any(nil)
	if !at.IsZero() {
		timestamp = at.Format(time.RFC3339)
	}
	circuits := map[string]any{}
	for _, c := range []string{"hk1", "hk2", "ww1"} {
		currentKey := "L_roomtemp_act"
		if c == "ww1" {
			currentKey = "L_ontemp_act"
		}
		set := setpoint(m, c)
		if c == "ww1" {
			set = "—"
			if n, ok := temperature(m, c, "temp_min_set").(float64); ok && n != 8 {
				set = fmt.Sprintf("%.1f °C", n)
			}
		}
		circuits[c] = map[string]any{"name": text(m, c, "name"), "temperature": temperature(m, c, currentKey), "setpoint": set, "status": translateStatus(text(m, c, "L_statetext")), "pump": pumpValue(m, c, "L_pump"), "flow": temperature(m, c, "L_flowtemp_act"), "minimum": temperature(m, c, "temp_min_set"), "maximum": temperature(m, c, "temp_max_set")}
	}
	writeJSON(w, map[string]any{"version": Version, "timezone": a.timeZone(), "instance_label": os.Getenv("OKOKO_INSTANCE_LABEL"), "configured": a.cfg.BoilerURL != "", "connected": !at.IsZero() && problem == "" && time.Since(at) < 3*time.Minute, "last_seen": timestamp, "stale": !at.IsZero() && time.Since(at) >= 3*time.Minute, "poll_seconds": max(10, a.cfg.PollSeconds), "error": problem,
		"boiler":   map[string]any{"temperature": temperature(m, "pe1", "L_temp_act"), "modulation": number(m, "pe1", "L_modulation"), "flame": temperature(m, "pe1", "L_frt_temp_act"), "airflow": number(m, "pe1", "L_currentairflow"), "pressure": scaledNumber(m, "pe1", "L_lowpressure", 0.1), "pump": number(m, "pe1", "L_uw_speed"), "runtime": number(m, "pe1", "L_runtime"), "starts": number(m, "pe1", "L_starts"), "average_runtime": number(m, "pe1", "L_avg_runtime"), "status": translateStatus(text(m, "pe1", "L_statetext"))},
		"circuits": circuits, "pellets": map[string]any{"stock": number(m, "pe1", "L_storage_fill"), "capacity": number(m, "pe1", "L_storage_max"), "today": number(m, "pe1", "storage_fill_today"), "yesterday": number(m, "pe1", "storage_fill_yesterday")}, "outdoor": temperature(m, "system", "L_ambient")})
}
func (a *App) historyV3(w http.ResponseWriter, r *http.Request) {
	days := 1
	if n, e := strconv.Atoi(r.URL.Query().Get("days")); e == nil && n > 0 && n <= 730 {
		days = n
	}
	ctx, cancel := context.WithTimeout(r.Context(), 4*time.Second)
	defer cancel()
	rows, err := a.db.QueryContext(ctx, `WITH buckets AS (SELECT MAX(ts) AS ts FROM measurements WHERE ts>=? GROUP BY CAST(strftime('%s',ts) AS INTEGER)/?) SELECT m.ts,m.ext,m.hk1_room,m.hk2_room,m.dhw_in,m.modulation,m.stock_kg FROM measurements m JOIN buckets b ON m.ts=b.ts ORDER BY m.ts DESC`, time.Now().In(a.location()).AddDate(0, 0, -days).Format("2006-01-02T15:04:05"), max(60, days*86400/1100))
	if err != nil {
		http.Error(w, "Historique indisponible", 503)
		return
	}
	defer rows.Close()
	out := []map[string]any{}
	for rows.Next() {
		var ts string
		var v [6]sql.NullFloat64
		if err = rows.Scan(&ts, &v[0], &v[1], &v[2], &v[3], &v[4], &v[5]); err != nil {
			http.Error(w, "Historique illisible", 500)
			return
		}
		row := map[string]any{"ts": a.historyTime(ts)}
		for i, k := range []string{"outdoor", "hk1", "hk2", "ww1", "modulation", "stock"} {
			if v[i].Valid {
				row[k] = v[i].Float64
			} else {
				row[k] = nil
			}
		}
		out = append(out, row)
	}
	if rows.Err() != nil {
		http.Error(w, "Historique interrompu", 503)
		return
	}
	for i, j := 0, len(out)-1; i < j; i, j = i+1, j-1 {
		out[i], out[j] = out[j], out[i]
	}
	// Bounded response; sample evenly for graph rendering, preserving source NULLs.
	if len(out) > 1200 {
		sampled := make([]map[string]any, 0, 1200)
		for i := 0; i < 1200; i++ {
			sampled = append(sampled, out[i*(len(out)-1)/1199])
		}
		out = sampled
	}
	writeJSON(w, out)
}
func (a *App) pelletsV3(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 4*time.Second)
	defer cancel()
	rows, err := a.db.QueryContext(ctx, `SELECT m.year,m.month,COALESCE(o.kg,m.kg) FROM monthly_consumption m LEFT JOIN monthly_overrides o ON m.year=o.year AND m.month=o.month ORDER BY m.year,m.month`)
	if err != nil {
		http.Error(w, "Bilan indisponible", 503)
		return
	}
	monthly := []map[string]any{}
	for rows.Next() {
		var y, m int
		var kg float64
		if rows.Scan(&y, &m, &kg) == nil {
			monthly = append(monthly, map[string]any{"year": y, "month": m, "kg": kg})
		}
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		http.Error(w, "Bilan incomplet", 503)
		return
	}
	rr, err := a.rechargeRows()
	if err != nil {
		http.Error(w, "Recharges indisponibles", 503)
		return
	}
	refills := []map[string]any{}
	for _, v := range rr {
		refills = append(refills, map[string]any{"date": v.ts, "kg": v.kg, "supplier": v.supplier, "kind": v.kind, "note": v.note})
	}
	writeJSON(w, map[string]any{"monthly": monthly, "refills": refills, "method": "premier stock − dernier stock + recharges manuelles", "source": "Calculs et archives conservés de la version 2.0"})
}
func (a *App) transitionsV3(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
	defer cancel()
	rows, e := a.db.QueryContext(ctx, `SELECT ts,circuit,kind,old_value,new_value FROM okoko17_transitions ORDER BY ts DESC LIMIT 100`)
	if e != nil {
		http.Error(w, "Transitions indisponibles", 503)
		return
	}
	defer rows.Close()
	out := []map[string]string{}
	for rows.Next() {
		var ts, c, k, old, next string
		if rows.Scan(&ts, &c, &k, &old, &next) == nil {
			out = append(out, map[string]string{"ts": ts, "circuit": c, "kind": k, "before": old, "after": next})
		}
	}
	if rows.Err() != nil {
		http.Error(w, "Transitions incomplètes", 503)
		return
	}
	writeJSON(w, out)
}
func (a *App) systemV3(w http.ResponseWriter, r *http.Request) {
	page := 1
	if raw := r.URL.Query().Get("page"); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil || parsed < 1 || parsed > 1000000 {
			http.Error(w, "Page invalide", 400)
			return
		}
		page = parsed
	}
	const pageSize = 10
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
	defer cancel()
	var count, files int64
	if err := a.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM measurements`).Scan(&count); err != nil {
		http.Error(w, "SQLite indisponible", 503)
		return
	}
	if err := a.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM imported_files`).Scan(&files); err != nil {
		http.Error(w, "SQLite indisponible", 503)
		return
	}
	var total int
	if err := a.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM import_runs`).Scan(&total); err != nil {
		http.Error(w, "Journal indisponible", 503)
		return
	}
	pages := max(1, (total+pageSize-1)/pageSize)
	if page > pages {
		page = pages
	}
	rows, e := a.db.QueryContext(ctx, `SELECT id,at,source,added,error,read_count,duplicate_count FROM import_runs ORDER BY id DESC LIMIT ? OFFSET ?`, pageSize, (page-1)*pageSize)
	if e != nil {
		http.Error(w, "Journal indisponible", 503)
		return
	}
	defer rows.Close()
	runs := []map[string]any{}
	for rows.Next() {
		var at, source, problem string
		var id, added, read, duplicates int
		if rows.Scan(&id, &at, &source, &added, &problem, &read, &duplicates) == nil {
			canonical, known := a.importTime(at)
			runs = append(runs, map[string]any{"id": id, "at": canonical, "at_raw": at, "time_known": known, "source": source, "added": added, "error": a.redactError(problem), "read": read, "duplicates": duplicates})
		}
	}
	if rows.Err() != nil {
		http.Error(w, "Journal incomplet", 503)
		return
	}
	a.importStateMu.RLock()
	active := a.importActive
	a.importStateMu.RUnlock()
	writeJSON(w, map[string]any{"version": Version, "sqlite": true, "measurements": count, "files": files, "nas_configured": a.cfg.CSVDir != "", "nas_mounted": a.cfg.CSVDir != "" && isNFSMounted(a.cfg.CSVDir), "backup": "Téléchargement SQLite cohérent, contrôlé avant envoi", "import_active": active, "runs": runs, "pagination": map[string]any{"page": page, "page_size": pageSize, "total": total, "pages": pages}, "timezone": a.timeZone(), "poll_seconds": max(10, a.cfg.PollSeconds), "import_minutes": a.cfg.ImportMinutes})
}
func mutationAllowed(w http.ResponseWriter, r *http.Request) bool {
	if r.Method != http.MethodPost {
		http.Error(w, "POST requis", 405)
		return false
	}
	if origin := r.Header.Get("Origin"); origin != "" {
		u, e := url.Parse(origin)
		if e != nil || u.Host != r.Host {
			http.Error(w, "Origine refusée", 403)
			return false
		}
	}
	if !strings.HasPrefix(r.Header.Get("Content-Type"), "application/json") {
		http.Error(w, "JSON requis", 415)
		return false
	}
	return true
}

// Existing refill validation and storage are reused without changing pellet calculations.
type capturedResponse struct {
	header http.Header
	code   int
	body   bytes.Buffer
}

func (c *capturedResponse) Header() http.Header { return c.header }
func (c *capturedResponse) WriteHeader(n int)   { c.code = n }
func (c *capturedResponse) Write(b []byte) (int, error) {
	if c.code == 0 {
		c.code = 200
	}
	return c.body.Write(b)
}
func (a *App) refillV3(w http.ResponseWriter, r *http.Request) {
	if !mutationAllowed(w, r) {
		return
	}
	var in struct {
		Date     string  `json:"date"`
		KG       float64 `json:"kg"`
		Supplier string  `json:"supplier"`
		Kind     string  `json:"kind"`
		Note     string  `json:"note"`
	}
	if json.NewDecoder(http.MaxBytesReader(w, r.Body, 8192)).Decode(&in) != nil {
		http.Error(w, "Recharge invalide", 400)
		return
	}
	v := url.Values{"date": {in.Date}, "kg": {strconv.FormatFloat(in.KG, 'f', 2, 64)}, "fournisseur": {in.Supplier}, "type": {in.Kind}, "note": {in.Note}}
	clone := r.Clone(r.Context())
	clone.Form = v
	clone.PostForm = v
	result := &capturedResponse{header: make(http.Header)}
	a.addRecharge(result, clone)
	if result.code == 303 {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.WriteHeader(201)
		writeJSON(w, map[string]bool{"saved": true})
		return
	}
	http.Error(w, strings.TrimSpace(result.body.String()), result.code)
}
func (a *App) importV3(w http.ResponseWriter, r *http.Request) {
	if !mutationAllowed(w, r) {
		return
	}
	if !a.importMu.TryLock() {
		http.Error(w, "Un import est déjà en cours", 409)
		return
	}
	a.importStateMu.Lock()
	a.importActive = true
	a.importStateMu.Unlock()
	go func() {
		defer func() { a.importStateMu.Lock(); a.importActive = false; a.importStateMu.Unlock(); a.importMu.Unlock() }()
		if a.cfg.BoilerURL != "" {
			ctx, cancel := context.WithTimeout(context.Background(), 150*time.Second)
			a.importLogsLocked(ctx)
			cancel()
		}
		if a.cfg.CSVDir != "" && isNFSMounted(a.cfg.CSVDir) {
			_, n, err := a.ImportCSVDir()
			a.logRun("CSV · NAS", n, n, err)
		}
	}()
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(202)
	writeJSON(w, map[string]bool{"started": true})
}
func (a *App) Handler() http.Handler {
	m := http.NewServeMux()
	m.HandleFunc("GET /api/v3/snapshot", a.snapshotV3)
	m.HandleFunc("GET /api/v3/history", a.historyV3)
	m.HandleFunc("GET /api/v3/pellets", a.pelletsV3)
	m.HandleFunc("GET /api/v3/transitions", a.transitionsV3)
	m.HandleFunc("GET /api/v3/system", a.systemV3)
	m.HandleFunc("GET /api/v3/weather", a.weatherAPI)
	m.HandleFunc("POST /api/v3/refills", a.refillV3)
	m.HandleFunc("POST /api/v3/import", a.importV3)
	m.HandleFunc("GET /api/v3/backup", a.downloadBackup)
	m.HandleFunc("GET /api/v3/export", a.exportCSV)
	m.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), time.Second)
		defer cancel()
		if a.db.PingContext(ctx) != nil {
			http.Error(w, "database unavailable", 503)
			return
		}
		_, _ = w.Write([]byte("ok\n"))
	})
	m.Handle("/api/", http.NotFoundHandler())
	m.Handle("/", web.Handler())
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Referrer-Policy", "same-origin")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Content-Security-Policy", "default-src 'self'; script-src 'self'; style-src 'self' 'unsafe-inline'; img-src 'self' data:; font-src 'self'; connect-src 'self'; object-src 'none'; base-uri 'self'; frame-ancestors 'none'")
		m.ServeHTTP(w, r)
	})
}
