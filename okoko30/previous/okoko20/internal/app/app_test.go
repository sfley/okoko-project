package app

import (
	"net/http/httptest"
	"strings"
	"testing"
)

func TestDecodeBoilerText(t *testing.T) {
	got := decodeBoilerText([]byte{'A', 'r', 'r', 0xea, 't'})
	if got != "Arrêt" {
		t.Fatalf("Latin-1: %q", got)
	}
	if got := decodeBoilerText([]byte("Chaudière °C")); got != "Chaudière °C" {
		t.Fatalf("UTF-8: %q", got)
	}
}
func TestRenderUTF8AndBrand(t *testing.T) {
	w := httptest.NewRecorder()
	render(w, "Accueil", "<h1>État chaudière</h1>")
	if !strings.Contains(w.Header().Get("Content-Type"), "charset=utf-8") {
		t.Error("missing UTF-8 content type")
	}
	body := w.Body.String()
	for _, expected := range []string{"OKOKO", "État chaudière"} {
		if !strings.Contains(body, expected) {
			t.Errorf("HTML missing %q", expected)
		}
	}
}
func TestNFSMountCheckDoesNotAcceptUnrelatedPath(t *testing.T) {
	if isNFSMounted("/tmp/okoko-nonexistent") {
		t.Fatal("unrelated path detected as NFS")
	}
}

func TestV16Navigation(t *testing.T) {
 w := httptest.NewRecorder()
 render(w, "Accueil", "<h1>Test</h1>")
 body := w.Body.String()
 for _, expected := range []string{
  "OKOKO", `href="/"`, `href="/installation"`,
  `href="/analyse"`, `href="/pellets"`,
  `href="/meteo"`, `href="/system"`,
 } {
  if !strings.Contains(body, expected) {
   t.Errorf("missing main navigation %q", expected)
  }
 }
 for _, unexpected := range []string{`href="/stock"`, `href="/calendar"`, `href="/bilan"`, "Météo et prévisions"} {
  if strings.Contains(body, unexpected) {
   t.Errorf("unexpected in global navigation: %q", unexpected)
  }
 }
}
func TestV16WeatherPage(t *testing.T) {
 w := httptest.NewRecorder()
 renderWithFooter(w, "Météo", "<h1>Météo</h1>", weatherFooter())
 body := w.Body.String()
 for _, expected := range []string{"Météo et prévisions", "Les 3 prochains jours · aujourd’hui inclus", "Prévisions horaires détaillées"} {
  if !strings.Contains(body, expected) {
   t.Errorf("missing weather content %q", expected)
  }
 }
}
func TestModeSeparators(t *testing.T) {
 if got:=modeHTML("Mode confort actif|T ambiante atteinte"); got!="Mode confort<br>Température souhaitée atteinte" {t.Fatalf("mode: %q",got)}
 if got:=modeHTML("<script>|ok"); strings.Contains(got,"<script>") {t.Fatal("HTML non échappé")}
}

// The browser title must remain version-free; version is shown in Centre système only.
func TestBrowserTitleWithoutVersion(t *testing.T) {
 w := httptest.NewRecorder()
 render(w, "Accueil", "<h1>Test</h1>")
 body := w.Body.String()
 if strings.Contains(body, "<title>Accueil · OKOKO12</title>") || strings.Contains(body, "<title>Accueil · OKOKO V12</title>") { t.Fatal("version found in browser title") }
}
