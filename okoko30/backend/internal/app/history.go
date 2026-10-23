package app

import (
	"embed"
	"encoding/csv"
	"fmt"
	"strconv"
	"strings"
)

//go:embed historical_monthly.csv
var historicalFS embed.FS

// seedHistoricalConsumption imports the archived MariaDB monthly reference.
// ON CONFLICT DO NOTHING deliberately protects existing archived corrections.
func (a *App) seedHistoricalConsumption() error {
	b, err := historicalFS.ReadFile("historical_monthly.csv")
	if err != nil {
		return err
	}
	records, err := csv.NewReader(strings.NewReader(string(b))).ReadAll()
	if err != nil {
		return err
	}
	tx, err := a.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	for i, r := range records {
		if i == 0 {
			continue
		}
		y, e1 := strconv.Atoi(r[0])
		m, e2 := strconv.Atoi(r[1])
		kg, e3 := strconv.ParseFloat(r[2], 64)
		if e1 != nil || e2 != nil || e3 != nil || m < 1 || m > 12 || kg < 0 {
			return fmt.Errorf("invalid historical row %d", i+1)
		}
		if _, err = tx.Exec(`INSERT INTO legacy_monthly_consumption(year,month,kg) VALUES(?,?,?) ON CONFLICT(year,month) DO NOTHING`, y, m, kg); err != nil {
			return err
		}
		if _, err = tx.Exec(`INSERT INTO monthly_consumption(year,month,kg) VALUES(?,?,?) ON CONFLICT(year,month) DO NOTHING`, y, m, kg); err != nil {
			return err
		}
	}
	return tx.Commit()
}
