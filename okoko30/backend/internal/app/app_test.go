package app

import "testing"

func TestDecodeBoilerText(t *testing.T) {
	got := decodeBoilerText([]byte{'A', 'r', 'r', 0xea, 't'})
	if got != "Arrêt" {
		t.Fatalf("Latin-1: %q", got)
	}
	if got := decodeBoilerText([]byte("Chaudière °C")); got != "Chaudière °C" {
		t.Fatalf("UTF-8: %q", got)
	}
}

func TestNFSMountCheckDoesNotAcceptUnrelatedPath(t *testing.T) {
	if isNFSMounted("/tmp/okoko-nonexistent") {
		t.Fatal("unrelated path detected as NFS")
	}
}
