package app

import (
	"encoding/json"
	"testing"
)

func TestSetpoints(t *testing.T) {
	for _, tc := range []struct{ state, want string }{{"Mode confort actif|T ambiante atteinte", "19.6 °C"}, {"Mode réduit actif|T ambiante atteinte", "19.2 °C"}, {"Inconnu", "—"}} {
		m := map[string]any{"hk1": map[string]any{"L_statetext": tc.state, "L_roomtemp_set": json.Number("80"), "temp_heat": json.Number("196"), "temp_setback": json.Number("192")}}
		if got := setpoint(m, "hk1"); got != tc.want {
			t.Fatalf("%s: %s != %s", tc.state, got, tc.want)
		}
	}
}
func TestTranslations(t *testing.T) {
	if got := translateStatus("Mode confort actif|T ambiante atteinte"); got != "Mode confort|Température souhaitée atteinte" {
		t.Fatal(got)
	}
}
