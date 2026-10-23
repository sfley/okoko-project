package app

import (
	"database/sql"
	"fmt"
	"strconv"
	"strings"
	"time"
)

// translateStatus preserves unknown ÖkoFEN diagnostics unchanged.
func translateStatus(raw string) string {
	parts := strings.Split(raw, "|")
	for i, p := range parts {
		p = strings.TrimSpace(p)
		switch p {
		case "Mode réduit actif":
			p = "Mode réduit"
		case "Mode confort actif":
			p = "Mode confort"
		case "T ambiante atteinte":
			p = "Température souhaitée atteinte"
		case "Demande marche off":
			p = "Aucune demande de chauffe"
		case "Demande marche on":
			p = "Demande de chauffe active"
		case "t hors prog horaire":
			p = "Hors programmation horaire"
		}
		parts[i] = p
	}
	return strings.Join(parts, "|")
}

func rawValue(m map[string]any, cat, key string) any {
	section, ok := m[cat].(map[string]any)
	if !ok {
		return nil
	}
	v := section[key]
	if obj, ok := v.(map[string]any); ok {
		return obj["val"]
	}
	return v
}
func setpoint(m map[string]any, cat string) string {
	raw := fmt.Sprint(rawValue(m, cat, "L_statetext"))
	var key string
	switch {
	case strings.Contains(raw, "Mode confort actif"):
		key = "temp_heat"
	case strings.Contains(raw, "Mode réduit actif"):
		key = "temp_setback"
	default:
		return "—"
	}
	n, ok := num(rawValue(m, cat, key))
	if !ok || n < 0 || n > 500 {
		return "—"
	}
	return fmt.Sprintf("%.1f °C", n/10)
}

func (a *App) initTransitions() error {
	_, e := a.db.Exec(`CREATE TABLE IF NOT EXISTS okoko17_transitions(id INTEGER PRIMARY KEY AUTOINCREMENT,ts TEXT NOT NULL,circuit TEXT NOT NULL,kind TEXT NOT NULL,old_value TEXT NOT NULL,new_value TEXT NOT NULL,origin TEXT NOT NULL DEFAULT 'observed');
 CREATE INDEX IF NOT EXISTS idx_okoko17_transitions_ts ON okoko17_transitions(ts);
 CREATE TABLE IF NOT EXISTS okoko17_last_state(circuit TEXT NOT NULL,kind TEXT NOT NULL,value TEXT NOT NULL,PRIMARY KEY(circuit,kind));`)
	return e
}
func transitionValues(m map[string]any) map[string]string {
	values := map[string]string{}
	for _, cat := range []string{"hk1", "hk2", "ww1"} {
		if raw := rawValue(m, cat, "L_statetext"); raw != nil {
			parts := strings.Split(fmt.Sprint(raw), "|")
			for i, part := range parts {
				if i > 1 {
					break
				}
				part = strings.TrimSpace(part)
				if part != "" {
					values[cat+"|status"+strconv.Itoa(i)] = translateStatus(part)
				}
			}
		}
		if v := rawValue(m, cat, "L_pump"); v != nil {
			values[cat+"|pump"] = fmt.Sprint(v)
		}
	}
	if v := rawValue(m, "pe1", "L_statetext"); v != nil {
		values["pe1|status"] = translateStatus(fmt.Sprint(v))
	}
	return values
}
func (a *App) recordTransitions(m map[string]any, at time.Time) {
	vals := transitionValues(m)
	tx, e := a.db.Begin()
	if e != nil {
		return
	}
	defer tx.Rollback()
	for k, v := range vals {
		parts := strings.SplitN(k, "|", 2)
		var old string
		e = tx.QueryRow(`SELECT value FROM okoko17_last_state WHERE circuit=? AND kind=?`, parts[0], parts[1]).Scan(&old)
		if e != nil && e != sql.ErrNoRows {
			return
		}
		if e == nil && old != v {
			if _, e = tx.Exec(`INSERT INTO okoko17_transitions(ts,circuit,kind,old_value,new_value) VALUES(?,?,?,?,?)`, at.Format(time.RFC3339), parts[0], parts[1], old, v); e != nil {
				return
			}
		}
		if e == sql.ErrNoRows || old != v {
			if _, e = tx.Exec(`INSERT INTO okoko17_last_state(circuit,kind,value) VALUES(?,?,?) ON CONFLICT(circuit,kind) DO UPDATE SET value=excluded.value`, parts[0], parts[1], v); e != nil {
				return
			}
		}
	}
	_ = tx.Commit()
}
