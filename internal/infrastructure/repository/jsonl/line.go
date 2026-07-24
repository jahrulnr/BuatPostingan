package jsonl

import (
	"fmt"
	"time"

	"buatpostingan/internal/domain/entity"
	"buatpostingan/internal/domain/enum"
	"buatpostingan/internal/domain/valueobject"
)

// Known top-level JSONL keys that are not Payload.
var coreLineKeys = map[string]struct{}{
	"seq": {}, "ts": {}, "thread_id": {}, "turn_id": {}, "type": {}, "id": {},
}

func itemToLineMap(item entity.TranscriptItem) map[string]any {
	m := map[string]any{
		"seq":       item.Seq,
		"ts":        timeToUnixFloat(item.At),
		"thread_id": item.ThreadID.String(),
		"type":      string(item.Type),
		"id":        item.ID.String(),
	}
	if item.TurnID.String() != "" {
		m["turn_id"] = item.TurnID.String()
	}
	for k, v := range item.Payload {
		if _, core := coreLineKeys[k]; core {
			continue
		}
		if v == nil {
			continue
		}
		m[k] = v
	}
	return m
}

func lineMapToItem(m map[string]any) (entity.TranscriptItem, error) {
	idRaw, _ := m["id"].(string)
	tidRaw, _ := m["thread_id"].(string)
	typeRaw, _ := m["type"].(string)
	if idRaw == "" || tidRaw == "" || typeRaw == "" {
		return entity.TranscriptItem{}, fmt.Errorf("incomplete line")
	}
	id, err := valueobject.NewItemID(idRaw)
	if err != nil {
		return entity.TranscriptItem{}, err
	}
	tid, err := valueobject.NewThreadID(tidRaw)
	if err != nil {
		return entity.TranscriptItem{}, err
	}
	item := entity.TranscriptItem{
		Seq:      asUint64(m["seq"]),
		ID:       id,
		ThreadID: tid,
		Type:     enum.ItemType(typeRaw),
		Payload:  map[string]any{},
		At:       unixFloatToTime(asFloat64(m["ts"])),
	}
	if turnRaw, _ := m["turn_id"].(string); turnRaw != "" {
		turn, terr := valueobject.NewTurnID(turnRaw)
		if terr == nil {
			item.TurnID = turn
		}
	}
	for k, v := range m {
		if _, core := coreLineKeys[k]; core {
			continue
		}
		item.Payload[k] = normalizeJSONNumber(v)
	}
	if item.At.IsZero() {
		item.At = time.Now().UTC()
	}
	return item, nil
}

func asUint64(v any) uint64 {
	switch n := v.(type) {
	case uint64:
		return n
	case float64:
		if n < 0 {
			return 0
		}
		return uint64(n)
	case int64:
		if n < 0 {
			return 0
		}
		return uint64(n)
	case int:
		if n < 0 {
			return 0
		}
		return uint64(n)
	default:
		return 0
	}
}

func asFloat64(v any) float64 {
	switch n := v.(type) {
	case float64:
		return n
	case int64:
		return float64(n)
	case int:
		return float64(n)
	default:
		return 0
	}
}

func normalizeJSONNumber(v any) any {
	switch n := v.(type) {
	case float64:
		// Prefer int64 when the value is integral (admin_user_id etc.).
		if n == float64(int64(n)) {
			return int64(n)
		}
		return n
	default:
		return v
	}
}
