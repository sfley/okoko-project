package app

import (
	"context"
	"encoding/csv"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

func (a *App) logURL(name string) (string, error) {
	u, e := url.Parse(a.cfg.BoilerURL)
	if e != nil {
		return "", e
	}
	u.RawQuery = ""
	u.Fragment = ""
	u.Path = strings.TrimSuffix(u.Path, "/all") + "/" + name
	return u.String(), nil
}
func (a *App) logRun(source string, n, read int, e error) {
	problem := ""
	if e != nil {
		problem = a.redactError(e.Error())
	}
	_, _ = a.db.Exec(`INSERT INTO import_runs(at,source,added,error,read_count,duplicate_count) VALUES(?,?,?,?,?,?)`, time.Now().UTC().Format(time.RFC3339Nano), source, n, problem, read, max(0, read-n))
}
func (a *App) importLog(ctx context.Context, name string) (int, int, error) {
	address, e := a.logURL(name)
	if e != nil {
		return 0, 0, e
	}
	req, e := http.NewRequestWithContext(ctx, "GET", address, nil)
	if e != nil {
		return 0, 0, e
	}
	resp, e := a.boilerRequest(req)
	if e != nil {
		return 0, 0, e
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return 0, 0, fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	cr := csv.NewReader(io.LimitReader(resp.Body, 8<<20))
	cr.Comma = ';'
	cr.FieldsPerRecord = -1
	if _, e = cr.Read(); e != nil {
		return 0, 0, e
	}
	tx, e := a.db.BeginTx(ctx, nil)
	if e != nil {
		return 0, 0, e
	}
	defer tx.Rollback()
	stmt, e := tx.PrepareContext(ctx, `INSERT OR IGNORE INTO measurements(ts,date,time,ext,ext_actual,boiler,boiler_set,hk1_flow,hk1_flow_set,hk1_room,hk1_room_set,hk2_flow,hk2_flow_set,hk2_room,hk2_room_set,dhw_in,dhw_out,dhw_set,modulation,flame,flame_set,stock_kg,stock_zwb_kg,boiler_status,burner,hk1_pump,hk2_pump,dhw_pump,error1,error2,error3,source_file) VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`)
	if e != nil {
		return 0, 0, e
	}
	defer stmt.Close()
	added := 0
	valid := 0
	for {
		row, err := cr.Read()
		if err == io.EOF {
			break
		}
		if err != nil || len(row) < 67 {
			continue
		}
		t, err := time.ParseInLocation("02.01.2006 15:04:05", field(row, 0)+" "+field(row, 1), time.Local)
		if err != nil {
			continue
		}
		valid++
		vals := []any{t.Format("2006-01-02T15:04:05"), t.Format("2006-01-02"), t.Format("15:04:05")}
		for _, i := range []int{2, 3, 4, 5, 9, 10, 11, 12, 17, 18, 19, 20, 25, 26, 27, 34, 35, 36, 44, 45} {
			v, _ := csvFloat(field(row, i))
			vals = append(vals, v)
		}
		vals = append(vals, csvInt(field(row, 46)), csvInt(field(row, 6)), csvInt(field(row, 13)), csvInt(field(row, 21)), csvInt(field(row, 28)), field(row, 64), field(row, 65), field(row, 66), "HTTP:"+name)
		res, err := stmt.ExecContext(ctx, vals...)
		if err != nil {
			return 0, 0, err
		}
		n, _ := res.RowsAffected()
		added += int(n)
	}
	if valid == 0 {
		return 0, 0, fmt.Errorf("aucune mesure valide")
	}
	return added, valid, tx.Commit()
}

// importLogsLocked must be called with importMu held.
func (a *App) importLogsLocked(ctx context.Context) bool {
	ok := true
	for _, name := range []string{"log", "log1", "log2", "log3"} {
		one, cancel := context.WithTimeout(ctx, 35*time.Second)
		n, read, err := a.importLog(one, name)
		cancel()
		a.logRun("JSON · "+name, n, read, err)
		if err != nil {
			ok = false
		}
		if ctx.Err() != nil {
			return false
		}
	}
	return ok
}
func (a *App) withImportLock(fn func()) bool {
	if !a.importMu.TryLock() {
		return false
	}
	a.importStateMu.Lock()
	a.importActive = true
	a.importStateMu.Unlock()
	defer func() { a.importStateMu.Lock(); a.importActive = false; a.importStateMu.Unlock(); a.importMu.Unlock() }()
	fn()
	return true
}
func (a *App) importLogs(ctx context.Context) bool {
	success := false
	acquired := a.withImportLock(func() { success = a.importLogsLocked(ctx) })
	return acquired && success
}
func (a *App) RunLogImporter(ctx context.Context) {
	// A failed or skipped startup run is retried hourly; successful runs are daily.
	lastSuccess := ""
	timer := time.NewTicker(time.Hour)
	defer timer.Stop()
	run := func() {
		if a.importLogs(ctx) {
			lastSuccess = time.Now().Format("2006-01-02")
		}
	}
	select {
	case <-ctx.Done():
		return
	case <-time.After(20 * time.Second):
	}
	run()
	for {
		select {
		case <-ctx.Done():
			return
		case <-timer.C:
			now := time.Now()
			if lastSuccess == "" || (now.Hour() >= 6 && now.Format("2006-01-02") != lastSuccess) {
				run()
			}
		}
	}
}
