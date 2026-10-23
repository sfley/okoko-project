package app

import (
	"fmt"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"
)

var frenchForecastDate = regexp.MustCompile(`(?i)(\d{1,2})\s+([a-zéûô]+)\.?\s+(?:(\d{4})\s+)?(\d{1,2}):(\d{2})$`)
var frenchMonths = map[string]time.Month{"janv": 1, "janvier": 1, "févr": 2, "février": 2, "fevr": 2, "mars": 3, "avr": 4, "avril": 4, "mai": 5, "juin": 6, "juil": 7, "juillet": 7, "août": 8, "aout": 8, "sept": 9, "septembre": 9, "oct": 10, "octobre": 10, "nov": 11, "novembre": 11, "déc": 12, "dec": 12, "décembre": 12}

// Dates without a year are tied to the actual collection time, never the browser clock.
func parseForecastTime(raw string, reference time.Time, loc *time.Location) (time.Time, bool) {
	raw = strings.TrimSpace(raw)
	for _, layout := range []string{time.RFC3339, "2006-01-02 15:04:05", "2006-01-02 15:04", "02.01.2006 15:04", "2006-01-02", "02.01.2006"} {
		if date, err := time.ParseInLocation(layout, raw, loc); err == nil {
			return date, true
		}
	}
	match := frenchForecastDate.FindStringSubmatch(raw)
	if match == nil {
		return time.Time{}, false
	}
	day, _ := strconv.Atoi(match[1])
	month, ok := frenchMonths[strings.ToLower(match[2])]
	if !ok {
		return time.Time{}, false
	}
	hour, _ := strconv.Atoi(match[4])
	minute, _ := strconv.Atoi(match[5])
	if hour > 23 || minute > 59 {
		return time.Time{}, false
	}
	year := reference.In(loc).Year()
	explicit := match[3] != ""
	if explicit {
		year, _ = strconv.Atoi(match[3])
	}
	chosen := time.Time{}
	distance := time.Duration(1<<63 - 1)
	for _, y := range []int{year - 1, year, year + 1} {
		if explicit && y != year {
			continue
		}
		candidate := time.Date(y, month, day, hour, minute, 0, 0, loc)
		if candidate.Day() != day || candidate.Month() != month {
			continue
		}
		delta := candidate.Sub(reference)
		if delta < 0 {
			delta = -delta
		}
		if delta < distance {
			chosen = candidate
			distance = delta
		}
	}
	if chosen.IsZero() || (!explicit && distance > 14*24*time.Hour) {
		return time.Time{}, false
	}
	return chosen, true
}
func forecastWind(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	if strings.Contains(strings.ToLower(raw), "km/h") {
		return raw
	}
	if _, err := strconv.ParseFloat(strings.ReplaceAll(raw, ",", "."), 64); err == nil {
		return raw + " km/h"
	}
	return raw
}
func weatherCondition(code string) string {
	known := map[string]string{"800": "Ciel dégagé", "801": "Peu nuageux", "802": "Partiellement nuageux", "803": "Nuageux", "804": "Couvert", "500": "Pluie légère", "501": "Pluie", "502": "Forte pluie", "600": "Neige", "601": "Neige", "200": "Orage"}
	if label, ok := known[code]; ok {
		return label
	}
	return "Conditions : " + code
}
func (a *App) weatherAPI(w http.ResponseWriter, r *http.Request) {
	a.mu.RLock()
	m, at := a.live, a.liveAt
	a.mu.RUnlock()
	loc := a.location()
	reference := at
	if reference.IsZero() {
		reference = time.Now()
	}
	current := map[string]string{"Lever": "—", "Coucher": "—", "Durée du jour": "—"}
	forecasts := []map[string]any{}
	for i := 0; i < 25; i++ {
		value := rawValue(m, "forecast", fmt.Sprintf("L_w_%d", i))
		raw, ok := value.(string)
		if !ok {
			continue
		}
		parts := strings.Split(raw, "|")
		if len(parts) < 6 {
			continue
		}
		row := map[string]any{"source_date": parts[0], "date": parts[0], "temp": strings.TrimSpace(parts[1]), "clouds": strings.TrimSpace(parts[2]), "wind": forecastWind(parts[3]), "icon": parts[4], "code": parts[5], "condition": weatherCondition(parts[5]), "is_current": i == 0}
		if date, valid := parseForecastTime(parts[0], reference, loc); valid {
			row["timestamp"] = date.Format(time.RFC3339)
			row["date"] = date.Format("2006-01-02 15:04")
			row["day"] = date.Format("2006-01-02")
			row["hour"] = date.Hour()
			row["minute"] = date.Minute()
		}
		if i == 0 {
			current["Date"] = parts[0]
			current["Température"] = parts[1] + " °C"
			current["Vent"] = forecastWind(parts[3])
			if len(parts) > 8 {
				current["Lever"] = parts[7]
				current["Coucher"] = parts[8]
				rise, e1 := time.Parse("15:04", parts[7])
				set, e2 := time.Parse("15:04", parts[8])
				if e1 == nil && e2 == nil && set.After(rise) {
					duration := set.Sub(rise)
					current["Durée du jour"] = fmt.Sprintf("%02dh%02d", int(duration.Hours()), int(duration.Minutes())%60)
				}
			}
		}
		forecasts = append(forecasts, row)
	}
	writeJSON(w, map[string]any{"current": current, "forecast": forecasts, "timezone": a.timeZone(), "reference_time": reference.Format(time.RFC3339), "reference_day": reference.In(loc).Format("2006-01-02"), "current_day": time.Now().In(loc).Format("2006-01-02"), "observed_temperature": temperature(m, "weather", "L_temp")})
}
