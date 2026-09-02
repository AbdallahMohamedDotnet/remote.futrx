package constants

import "testing"

func TestMiniMaxModelCatalogReturnsIndependentCopies(t *testing.T) {
	first := MiniMaxModelCatalog()
	first[0] = 'x'
	if MiniMaxModelCatalog()[0] == 'x' {
		t.Fatal("MiniMax model catalog mutation escaped configuration ownership")
	}
}
