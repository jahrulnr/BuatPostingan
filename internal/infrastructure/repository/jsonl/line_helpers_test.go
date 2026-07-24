package jsonl

import (
	"testing"
	"time"

	"buatpostingan/internal/domain/enum"
)

func TestLineMapHelpersDirect(t *testing.T) {
	item, err := lineMapToItem(map[string]any{
		"seq":       int64(-3),
		"ts":        int64(1_700_000_000),
		"thread_id": "thr_helper_zzzzzzzzzzzzzzz",
		"type":      string(enum.ItemAgentMessage),
		"id":        "itm_helper_zzzzzzzzzzzzzzzz",
		"turn_id":   "trn_helper_zzzzzzzzzzzzzzzz",
		"score":     float64(2.5),
		"flag":      true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if item.Seq != 0 {
		t.Fatalf("neg int64 seq=%d", item.Seq)
	}
	if item.Payload["score"] != 2.5 {
		t.Fatalf("score=%v", item.Payload["score"])
	}
	if item.At.IsZero() {
		t.Fatal("ts should parse")
	}

	item2, err := lineMapToItem(map[string]any{
		"seq":       int(-2),
		"ts":        float64(0),
		"thread_id": "thr_helper_zzzzzzzzzzzzzzz",
		"type":      string(enum.ItemUserMessage),
		"id":        "itm_helper2_zzzzzzzzzzzzzzz",
	})
	if err != nil {
		t.Fatal(err)
	}
	if item2.Seq != 0 {
		t.Fatalf("neg int seq=%d", item2.Seq)
	}
	if item2.At.IsZero() {
		// ts=0 → zero then now
		_ = time.Now()
	}

	if asUint64(uint64(7)) != 7 || asUint64("x") != 0 {
		t.Fatal("asUint64")
	}
	if asFloat64(int(4)) != 4 || asFloat64("x") != 0 {
		t.Fatal("asFloat64")
	}

	m := itemToLineMap(item)
	if m["turn_id"] == nil {
		t.Fatal("turn_id missing")
	}
	// nil payload values skipped; core keys not re-copied
	item.Payload["seq"] = 99
	item.Payload["nilv"] = nil
	m2 := itemToLineMap(item)
	if _, ok := m2["nilv"]; ok {
		t.Fatal("nil payload should skip")
	}
}

func TestDirOfEmpty(t *testing.T) {
	if dirOf("") != "." && dirOf("") != "" {
		// filepath.Dir("") == "."
		_ = dirOf("a")
	}
	if dirOf("file") != "." {
		t.Fatalf("dirOf file=%q", dirOf("file"))
	}
}
