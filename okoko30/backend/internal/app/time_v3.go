package app

import (
	"strings"
	"time"
	_ "time/tzdata" // OpenWrt does not need a separate zoneinfo package.
)

func (a *App) timeZone() string {
	if a.cfg.Timezone != "" {
		return a.cfg.Timezone
	}
	return "Europe/Paris"
}
func (a *App) location() *time.Location {
	loc, err := time.LoadLocation(a.timeZone())
	if err != nil {
		return time.UTC
	}
	return loc
}
func (a *App) importTime(raw string) (string, bool) {
	if parsed, err := time.Parse(time.RFC3339Nano, raw); err == nil {
		return parsed.Format(time.RFC3339Nano), true
	}
	zone := a.cfg.LegacyImportTimezone
	if zone == "" {
		zone = "UTC" // Confirmed source clock: 07:50 UTC was displayed instead of 09:50 Paris.
	}
	if loc, err := time.LoadLocation(zone); err == nil {
		for _, layout := range []string{"2006-01-02 15:04:05", "2006-01-02T15:04:05"} {
			if parsed, err := time.ParseInLocation(layout, raw, loc); err == nil {
				return parsed.Format(time.RFC3339), true
			}
		}
	}
	// Unrecognised dates remain intact rather than receiving an invented timestamp.
	return strings.TrimSpace(raw), false
}
func (a *App) historyTime(raw string) string {
	if parsed, err := time.ParseInLocation("2006-01-02T15:04:05", raw, a.location()); err == nil {
		return parsed.Format(time.RFC3339)
	}
	return raw
}
