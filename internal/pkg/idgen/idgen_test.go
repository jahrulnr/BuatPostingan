package idgen_test

import (
	"strings"
	"testing"

	"buatpostingan/internal/pkg/idgen"
)

func TestNewPrefixes(t *testing.T) {
	t.Parallel()
	thr := idgen.ThreadID()
	trn := idgen.TurnID()
	itm := idgen.ItemID()
	if !strings.HasPrefix(thr, "thr_") || !strings.HasPrefix(trn, "trn_") || !strings.HasPrefix(itm, "itm_") {
		t.Fatalf("%s %s %s", thr, trn, itm)
	}
	custom := idgen.New("abc")
	if !strings.HasPrefix(custom, "abc_") {
		t.Fatalf("%s", custom)
	}
	// uniqueness for a burst
	seen := map[string]struct{}{}
	for i := 0; i < 20; i++ {
		id := idgen.New("x")
		if _, ok := seen[id]; ok {
			t.Fatalf("duplicate %s", id)
		}
		seen[id] = struct{}{}
	}
}
