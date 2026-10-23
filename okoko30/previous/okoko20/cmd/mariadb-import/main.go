package main

import (
	"bufio"
	"database/sql"
	"fmt"
	"os"
	"strconv"
	"strings"

	_ "modernc.org/sqlite"
)

func splitTuples(s string) [][]string {
	var out [][]string
	var row []string
	var b strings.Builder
	in := false
	esc := false
	depth := 0
	for i := 0; i < len(s); i++ {
		c := s[i]
		if in {
			if esc {
				b.WriteByte(c)
				esc = false
				continue
			}
			if c == '\\' {
				esc = true
				continue
			}
			if c == '\'' {
				in = false
				continue
			}
			b.WriteByte(c)
			continue
		}
		if c == '\'' {
			in = true
			continue
		}
		if c == '(' {
			if depth == 0 {
				row = nil
				b.Reset()
			}
			depth++
			continue
		}
		if c == ')' {
			if depth == 1 {
				row = append(row, strings.TrimSpace(b.String()))
				out = append(out, row)
				b.Reset()
			}
			depth--
			continue
		}
		if c == ',' && depth == 1 {
			row = append(row, strings.TrimSpace(b.String()))
			b.Reset()
			continue
		}
		if depth == 1 {
			b.WriteByte(c)
		}
	}
	return out
}
func clean(s string) string {
	s = strings.TrimSpace(s)
	s = strings.TrimPrefix(s, "_binary ")
	if s == "NULL" {
		return ""
	}
	return s
}
func f(s string) any {
	s = clean(s)
	if s == "" {
		return nil
	}
	v, e := strconv.ParseFloat(s, 64)
	if e != nil {
		return nil
	}
	return v
}
func i(s string) any {
	s = clean(s)
	if s == "" {
		return nil
	}
	if len(s) == 1 && s[0] < 32 {
		if s[0] == 1 {
			return 1
		}
		return 0
	}
	v, e := strconv.Atoi(s)
	if e != nil {
		return nil
	}
	return v
}
func month(s string) int {
	m := map[string]int{"janvier": 1, "février": 2, "fevrier": 2, "mars": 3, "avril": 4, "mai": 5, "juin": 6, "juillet": 7, "août": 8, "aout": 8, "septembre": 9, "octobre": 10, "novembre": 11, "décembre": 12, "decembre": 12}
	return m[strings.ToLower(clean(s))]
}
func main() {
	if len(os.Args) < 3 {
		fmt.Fprintln(os.Stderr, "usage: mariadb-import dump.sql chaudiere.db")
		os.Exit(2)
	}
	db, err := sql.Open("sqlite", os.Args[2])
	if err != nil {
		panic(err)
	}
	defer db.Close()
	db.SetMaxOpenConns(1)
	_, err = db.Exec(`PRAGMA journal_mode=WAL; PRAGMA busy_timeout=10000;
 CREATE TABLE IF NOT EXISTS measurements(ts TEXT PRIMARY KEY,date TEXT NOT NULL,time TEXT NOT NULL,ext REAL,ext_actual REAL,boiler REAL,boiler_set REAL,hk1_flow REAL,hk1_flow_set REAL,hk1_room REAL,hk1_room_set REAL,hk2_flow REAL,hk2_flow_set REAL,hk2_room REAL,hk2_room_set REAL,dhw_in REAL,dhw_out REAL,dhw_set REAL,modulation REAL,flame REAL,flame_set REAL,stock_kg REAL,stock_zwb_kg REAL,boiler_status INTEGER,burner INTEGER,hk1_pump INTEGER,hk2_pump INTEGER,dhw_pump INTEGER,error1 TEXT,error2 TEXT,error3 TEXT,source_file TEXT);
 CREATE TABLE IF NOT EXISTS legacy_monthly_consumption(year INTEGER,month INTEGER,kg REAL,PRIMARY KEY(year,month));`)
	if err != nil {
		panic(err)
	}
	file, err := os.Open(os.Args[1])
	if err != nil {
		panic(err)
	}
	defer file.Close()
	sc := bufio.NewScanner(file)
	sc.Buffer(make([]byte, 1024*1024), 128*1024*1024)
	added, dup, bad, monthly := 0, 0, 0, 0
	for sc.Scan() {
		line := sc.Text()
		if strings.HasPrefix(line, "INSERT INTO `conso_mensuelle`") {
			for _, r := range splitTuples(line) {
				if len(r) < 4 {
					continue
				}
				y, _ := strconv.Atoi(clean(r[1]))
				mo := month(r[2])
				kg, _ := strconv.ParseFloat(clean(r[3]), 64)
				if y > 0 && mo > 0 {
					_, _ = db.Exec(`INSERT INTO legacy_monthly_consumption(year,month,kg) VALUES(?,?,?) ON CONFLICT(year,month) DO UPDATE SET kg=excluded.kg`, y, mo, kg)
					monthly++
				}
			}
			continue
		}
		if !strings.HasPrefix(line, "INSERT INTO `json_import`") {
			continue
		}
		tx, err := db.Begin()
		if err != nil {
			panic(err)
		}
		st, err := tx.Prepare(`INSERT OR IGNORE INTO measurements(ts,date,time,ext,ext_actual,boiler,boiler_set,hk1_flow,hk1_flow_set,hk1_room,hk1_room_set,hk2_flow,hk2_flow_set,hk2_room,hk2_room_set,dhw_in,dhw_out,dhw_set,modulation,flame,flame_set,stock_kg,stock_zwb_kg,boiler_status,burner,hk1_pump,hk2_pump,dhw_pump,error1,error2,error3,source_file) VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`)
		if err != nil {
			panic(err)
		}
		for _, r := range splitTuples(line) {
			if len(r) < 67 || clean(r[0]) == "0000-00-00" {
				bad++
				continue
			}
			date, tim := clean(r[0]), clean(r[1])
			if len(date) != 10 || len(tim) != 8 {
				bad++
				continue
			}
			ts := date + "T" + tim
			vals := []any{ts, date, tim, f(r[2]), f(r[3]), f(r[4]), f(r[5]), f(r[9]), f(r[10]), f(r[11]), f(r[12]), f(r[17]), f(r[18]), f(r[19]), f(r[20]), f(r[25]), f(r[26]), f(r[27]), f(r[34]), f(r[35]), f(r[36]), f(r[44]), f(r[45]), i(r[46]), i(r[6]), i(r[13]), i(r[21]), i(r[28]), clean(r[64]), clean(r[65]), clean(r[66]), "mariadb"}
			res, e := st.Exec(vals...)
			if e != nil {
				bad++
				continue
			}
			n, _ := res.RowsAffected()
			if n == 1 {
				added++
			} else {
				dup++
			}
		}
		st.Close()
		if err = tx.Commit(); err != nil {
			panic(err)
		}
		fmt.Printf("\rAjoutées: %d · déjà présentes: %d · ignorées: %d", added, dup, bad)
	}
	if err := sc.Err(); err != nil {
		panic(err)
	}
	fmt.Printf("\nTerminé. MariaDB → SQLite: %d ajoutées, %d déjà présentes, %d invalides, %d consommations mensuelles.\n", added, dup, bad, monthly)
}
