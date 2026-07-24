package valueobject_test

import (
	"testing"

	"buatpostingan/internal/domain/valueobject"
)

func TestNewTitle(t *testing.T) {
	t.Parallel()
	if _, err := valueobject.NewTitle("  "); err == nil {
		t.Fatal("expected error for blank title")
	}
	title, err := valueobject.NewTitle("  Hello  ")
	if err != nil {
		t.Fatal(err)
	}
	if title.String() != "Hello" {
		t.Fatalf("got %q", title.String())
	}
	runes := make([]rune, 61)
	for i := range runes {
		runes[i] = 'a'
	}
	if _, err := valueobject.NewTitle(string(runes)); err == nil {
		t.Fatal("expected error for >60 runes")
	}
}

func TestNewThreadID(t *testing.T) {
	t.Parallel()
	if _, err := valueobject.NewThreadID(""); err == nil {
		t.Fatal("expected error")
	}
	id, err := valueobject.NewThreadID("thr_x")
	if err != nil || id.String() != "thr_x" {
		t.Fatalf("got %v %v", id, err)
	}
}

func TestNewTurnID(t *testing.T) {
	t.Parallel()
	if _, err := valueobject.NewTurnID("  "); err == nil {
		t.Fatal("expected error")
	}
	id, err := valueobject.NewTurnID(" trn_1 ")
	if err != nil || id.String() != "trn_1" {
		t.Fatalf("got %v %v", id, err)
	}
}

func TestNewItemID(t *testing.T) {
	t.Parallel()
	if _, err := valueobject.NewItemID(""); err == nil {
		t.Fatal("expected error")
	}
	id, err := valueobject.NewItemID("itm_abc")
	if err != nil || id.String() != "itm_abc" {
		t.Fatalf("got %v %v", id, err)
	}
}
